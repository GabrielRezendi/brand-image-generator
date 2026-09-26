package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/GabrielRezendi/brand-image-generator/internal/brandfonts"
	"github.com/GabrielRezendi/brand-image-generator/internal/config"
	"github.com/GabrielRezendi/brand-image-generator/internal/llm"
	"github.com/GabrielRezendi/brand-image-generator/internal/output"
)

const MaxAttempts = 3

var (
	svgBlockRe    = regexp.MustCompile(`(?is)<svg[\s\S]*?</svg>`)
	imageHrefRe   = regexp.MustCompile(`(?i)(<image\b[^>]*?\b(?:href|xlink:href)\s*=\s*")([^"]*)(")`)
	bgPlaceholder = "__BACKGROUND_IMAGE__"
)

type ProgressFunc func(Progress)

type Progress struct {
	Attempt   int
	Phase     string // machine key
	Label     string // human-readable PT stage title
	Detail    string
	StartedAt time.Time
	Completed []StageStat
}

type StageStat struct {
	Attempt          int           `json:"attempt"`
	Phase            string        `json:"phase"`
	Label            string        `json:"label"`
	Duration         time.Duration `json:"duration_ns"`
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
	TotalTokens      int           `json:"total_tokens"`
	CostUSD          float64       `json:"cost_usd"`
	Estimated        bool          `json:"estimated,omitempty"`
}

func (s StageStat) AddUsage(u llm.Usage) StageStat {
	s.PromptTokens += u.PromptTokens
	s.CompletionTokens += u.CompletionTokens
	s.TotalTokens += u.TotalTokens
	if s.TotalTokens == 0 {
		s.TotalTokens = s.PromptTokens + s.CompletionTokens
	}
	s.CostUSD += u.CostUSD
	s.Estimated = s.Estimated || u.Estimated
	return s
}

type Request struct {
	Prompt       string
	Attachments  []config.Asset
	Provider     config.Provider
	Models       config.Models
	BrandGuide   string
	BrandVisual  config.BrandVisual
	BrandAssets  []config.Asset
	OutputBase   string
	EnableReview bool
}

type Review struct {
	Approved bool   `json:"approved"`
	Feedback string `json:"feedback"`
	Raw      string `json:"raw,omitempty"`
}

type AttemptResult struct {
	Index          int
	Dir            string
	SVGRawPath     string
	BackgroundPath string
	ElementsDir    string
	FinalSVGPath   string
	FinalPNGPath   string
	ReviewPath     string
	Review         Review
}

type Result struct {
	JobDir        string
	Prompt        string
	Attempts      []AttemptResult
	Stages        []StageStat
	Approved      bool
	ReviewSkipped bool // true when generation finished without the vision review loop
	FinalIndex    int
	Canceled      bool
	Err           error
}

type Runner struct {
	cfg config.Config
}

func New(cfg config.Config) *Runner {
	return &Runner{cfg: cfg}
}

func (r *Runner) Run(ctx context.Context, req Request, onProgress ProgressFunc) Result {
	res := Result{Prompt: req.Prompt, FinalIndex: 0}
	logMu := sync.Mutex{}
	var conv strings.Builder

	jobDir, err := output.JobDir(req.OutputBase, req.Prompt, time.Now())
	if err != nil {
		res.Err = err
		return res
	}
	res.JobDir = jobDir

	_ = output.WriteFile(filepath.Join(jobDir, "prompt.md"), []byte(req.Prompt+"\n"))
	meta, _ := json.MarshalIndent(map[string]any{
		"provider":     req.Provider,
		"models":       req.Models,
		"attachments":  req.Attachments,
		"brand_assets": req.BrandAssets,
		"brand_visual": req.BrandVisual,
	}, "", "  ")
	_ = output.WriteFile(filepath.Join(jobDir, "meta.json"), meta)

	appendLog := func(role, content string) {
		logMu.Lock()
		defer logMu.Unlock()
		ts := time.Now().Format(time.RFC3339)
		conv.WriteString(fmt.Sprintf("\n===== %s | %s =====\n%s\n", ts, role, content))
		_ = output.WriteFile(filepath.Join(jobDir, "conversation.log"), []byte(conv.String()))
	}

	apiKey, err := config.APIKeyFor(r.cfg, req.Provider)
	if err != nil || apiKey == "" {
		res.Err = fmt.Errorf("chave indisponível para %s", req.Provider)
		return res
	}
	client, err := llm.New(req.Provider, apiKey, appendLog)
	if err != nil {
		res.Err = err
		return res
	}

	var stages []StageStat
	var current *StageStat
	var stageStart time.Time

	finishStage := func() {
		if current == nil {
			return
		}
		current.Duration = time.Since(stageStart)
		if current.TotalTokens == 0 {
			current.TotalTokens = current.PromptTokens + current.CompletionTokens
		}
		stages = append(stages, *current)
		res.Stages = append([]StageStat(nil), stages...)
		current = nil
	}

	startStage := func(attempt int, phase, label, detail string) {
		finishStage()
		now := time.Now()
		stageStart = now
		current = &StageStat{Attempt: attempt, Phase: phase, Label: label}
		if onProgress != nil {
			onProgress(Progress{
				Attempt:   attempt,
				Phase:     phase,
				Label:     label,
				Detail:    detail,
				StartedAt: now,
				Completed: append([]StageStat(nil), stages...),
			})
		}
	}

	addUsage := func(u llm.Usage) {
		if current == nil {
			return
		}
		*current = current.AddUsage(u)
	}

	extraSources := buildSourceIndex(req.BrandAssets, req.Attachments)

	maxAttempts := MaxAttempts
	if !req.EnableReview {
		maxAttempts = 1
	}

	var feedback string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			finishStage()
			writeUsageLog(jobDir, stages)
			res.Canceled = true
			res.Err = ctx.Err()
			res.Stages = stages
			return res
		}

		attemptDir, err := output.AttemptDir(jobDir, attempt)
		if err != nil {
			finishStage()
			res.Err = err
			return res
		}
		elementsDir := filepath.Join(attemptDir, "elements")
		if err := os.MkdirAll(elementsDir, 0o755); err != nil {
			finishStage()
			res.Err = err
			return res
		}

		ar := AttemptResult{
			Index:       attempt,
			Dir:         attemptDir,
			ElementsDir: elementsDir,
		}

		startStage(attempt, "svg", "Gerando SVG", "estrutura e conteúdo (sem ícones/artes)…")
		svgRaw, usage, err := generateSVG(ctx, client, req, feedback)
		if err != nil {
			finishStage()
			res.Err = fmt.Errorf("tentativa %d SVG: %w", attempt, err)
			return res
		}
		addUsage(usage)
		ar.SVGRawPath = filepath.Join(attemptDir, "structure.svg")
		if err := output.WriteFile(ar.SVGRawPath, []byte(svgRaw)); err != nil {
			finishStage()
			res.Err = err
			return res
		}

		startStage(attempt, "fonts", "Preparando fontes", "a resolver ficheiros / Google Fonts…")
		fontSet, err := brandfonts.Resolve(req.BrandVisual, attemptDir)
		if err != nil {
			finishStage()
			res.Err = fmt.Errorf("tentativa %d fontes: %w", attempt, err)
			return res
		}

		startStage(attempt, "image", "Gerando fundo", "a criar a imagem de background…")
		imgPrompt, usage, err := buildImagePrompt(ctx, client, req, svgRaw, feedback)
		if err != nil {
			finishStage()
			res.Err = fmt.Errorf("tentativa %d prompt imagem: %w", attempt, err)
			return res
		}
		addUsage(usage)
		_ = output.WriteFile(filepath.Join(attemptDir, "image-prompt.txt"), []byte(imgPrompt))

		imgBytes, usage, err := client.GenerateImage(ctx, req.Models.Image, imgPrompt)
		if err != nil {
			finishStage()
			res.Err = fmt.Errorf("tentativa %d imagem: %w", attempt, err)
			return res
		}
		addUsage(usage)
		ar.BackgroundPath = filepath.Join(elementsDir, "background.png")
		if err := output.WriteFile(ar.BackgroundPath, imgBytes); err != nil {
			res.Err = err
			return res
		}

		startStage(attempt, "compose", "Montando entrega", "a organizar elements/, fontes e referências do SVG…")
		composed := InjectBackground(svgRaw, "elements/background.png")
		composed, err = MakeSelfContained(composed, attemptDir, extraSources)
		if err != nil {
			finishStage()
			res.Err = fmt.Errorf("tentativa %d elements: %w", attempt, err)
			return res
		}
		composed = ApplyBrandVisual(composed, req.BrandVisual, fontSet)
		composed = EnsureBrandFonts(composed, fontSet)
		ar.FinalSVGPath = filepath.Join(attemptDir, "final.svg")
		if err := output.WriteFile(ar.FinalSVGPath, []byte(composed)); err != nil {
			finishStage()
			res.Err = err
			return res
		}

		startStage(attempt, "raster", "Convertendo SVG → PNG", "a rasterizar o resultado final…")
		ar.FinalPNGPath = filepath.Join(attemptDir, "final.png")
		if err := RasterizeSVG(ar.FinalSVGPath, ar.FinalPNGPath); err != nil {
			finishStage()
			res.Err = fmt.Errorf("tentativa %d PNG: %w", attempt, err)
			return res
		}

		if !req.EnableReview {
			ar.Review = Review{Approved: true, Feedback: "revisão desativada"}
			res.Attempts = append(res.Attempts, ar)
			res.Approved = true
			res.ReviewSkipped = true
			res.FinalIndex = attempt
			_ = copyFile(ar.FinalSVGPath, filepath.Join(jobDir, "final.svg"))
			_ = copyFile(ar.FinalPNGPath, filepath.Join(jobDir, "final.png"))
			_ = CopyElementsDir(attemptDir, jobDir)
			finishStage()
			writeUsageLog(jobDir, stages)
			startStage(attempt, "done", "Concluído", "entrega pronta (sem revisão)")
			finishStage()
			return res
		}

		pngBytes, err := os.ReadFile(ar.FinalPNGPath)
		if err != nil {
			res.Err = err
			return res
		}

		startStage(attempt, "review", "Revisando arte", "a avaliar pedido + guia da marca…")
		review, usage, err := reviewArt(ctx, client, req, composed, pngBytes, feedback)
		if err != nil {
			finishStage()
			res.Err = fmt.Errorf("tentativa %d revisão: %w", attempt, err)
			return res
		}
		addUsage(usage)
		ar.Review = review
		ar.ReviewPath = filepath.Join(attemptDir, "review.json")
		rawReview, _ := json.MarshalIndent(review, "", "  ")
		_ = output.WriteFile(ar.ReviewPath, rawReview)

		res.Attempts = append(res.Attempts, ar)

		if review.Approved {
			res.Approved = true
			res.FinalIndex = attempt
			_ = copyFile(ar.FinalSVGPath, filepath.Join(jobDir, "final.svg"))
			_ = copyFile(ar.FinalPNGPath, filepath.Join(jobDir, "final.png"))
			_ = CopyElementsDir(attemptDir, jobDir)
			finishStage()
			writeUsageLog(jobDir, stages)
			startStage(attempt, "done", "Concluído", "aprovado — entrega autocontida pronta")
			finishStage()
			return res
		}
		feedback = review.Feedback
		if feedback == "" {
			feedback = "A arte ainda não está aderente. Melhora composição, hierarquia e alinhamento à marca."
		}
		finishStage()
		startStage(attempt, "retry", "A reiterar", feedback)
	}

	finishStage()
	writeUsageLog(jobDir, stages)
	startStage(maxAttempts, "done", "Concluído", "3 tentativas sem aprovação — entregando todas")
	finishStage()
	return res
}

func writeUsageLog(jobDir string, stages []StageStat) {
	type row struct {
		Attempt          int     `json:"attempt"`
		Phase            string  `json:"phase"`
		Label            string  `json:"label"`
		DurationMS       int64   `json:"duration_ms"`
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		TotalTokens      int     `json:"total_tokens"`
		CostUSD          float64 `json:"cost_usd"`
		Estimated        bool    `json:"estimated,omitempty"`
	}
	rows := make([]row, 0, len(stages))
	var total StageStat
	for _, s := range stages {
		if s.Phase == "done" || s.Phase == "retry" {
			continue
		}
		rows = append(rows, row{
			Attempt:          s.Attempt,
			Phase:            s.Phase,
			Label:            s.Label,
			DurationMS:       s.Duration.Milliseconds(),
			PromptTokens:     s.PromptTokens,
			CompletionTokens: s.CompletionTokens,
			TotalTokens:      s.TotalTokens,
			CostUSD:          s.CostUSD,
			Estimated:        s.Estimated,
		})
		total = total.AddUsage(llm.Usage{
			PromptTokens:     s.PromptTokens,
			CompletionTokens: s.CompletionTokens,
			TotalTokens:      s.TotalTokens,
			CostUSD:          s.CostUSD,
			Estimated:        s.Estimated,
		})
		total.Duration += s.Duration
	}
	payload, _ := json.MarshalIndent(map[string]any{
		"stages": rows,
		"total": map[string]any{
			"duration_ms":       total.Duration.Milliseconds(),
			"prompt_tokens":     total.PromptTokens,
			"completion_tokens": total.CompletionTokens,
			"total_tokens":      total.TotalTokens,
			"cost_usd":          total.CostUSD,
			"estimated":         total.Estimated,
		},
	}, "", "  ")
	_ = output.WriteFile(filepath.Join(jobDir, "usage.json"), payload)
}

func buildSourceIndex(sets ...[]config.Asset) map[string]string {
	out := map[string]string{}
	for _, assets := range sets {
		for _, a := range assets {
			p := strings.TrimSpace(a.Path)
			if p == "" {
				continue
			}
			out[p] = p
			out[filepath.Base(p)] = p
			if a.Label != "" {
				out[a.Label] = p
				out["__ASSET:"+a.Label+"__"] = p
			}
		}
	}
	return out
}

func generateSVG(ctx context.Context, client *llm.Client, req Request, feedback string) (string, llm.Usage, error) {
	sys := `És um designer de layout que devolve APENAS um documento SVG válido (sem markdown).

O SVG deve ser ESTRUTURA e CONTEÚDO — não artes nem ícones.
Regras obrigatórias:
- Inclui um <image> de fundo com exatamente href="` + bgPlaceholder + `" (o sistema substitui pelo PNG gerado).
- Tipografia, hierarquia, contentores, grelha e texto do pedido ficam no SVG (elementos <text>, <rect>, etc.).
- NÃO desenhes ícones, pictogramas, ilustrações, ornamentos artísticos nem "artes" vetoriais decorativas.
- A única imagem ilustrativa/fotográfica é o fundo via esse <image>.
- Se precisares de logo ou asset de marca fornecido, usa <image href="CAMINHO_EXATO_DO_ASSET"> com o path indicado — não inventes gráficos.
- Tipografia: usa font-family="` + config.FontFamilyPrimary + `" para títulos/destaques e font-family="` + config.FontFamilySecondary + `" para corpo/apoio (as @font-face serão injectadas pelo sistema).
- Cores: usa a cor primária e secundária indicadas (fill/stroke), sem inventar uma paleta paralela.
- viewBox e dimensões coerentes (ex. 1080x1080 ou 1080x1350).
- Sem scripts externos.`

	user := "Pedido do utilizador:\n" + req.Prompt + "\n\nGuia da marca:\n" + req.BrandGuide
	user += "\n\n" + brandfonts.PromptBlock(req.BrandVisual)
	if feedback != "" {
		user += "\n\nFeedback da revisão anterior (corrige):\n" + feedback
	}
	user += "\n\nAssets de marca disponíveis (usa o path exacto no href se necessário):\n" + listAssets(req.BrandAssets)
	if len(req.Attachments) > 0 {
		user += "\nAnexos deste pedido:\n" + listAssets(req.Attachments)
	}

	parts := []llm.ContentPart{{Type: "text", Text: user}}
	parts = append(parts, imageParts(append(req.BrandAssets, req.Attachments...))...)

	content, usage, err := client.Chat(ctx, req.Models.SVG, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: parts},
	})
	if err != nil {
		return "", usage, err
	}
	svg := ExtractSVG(content)
	if svg == "" {
		return "", usage, fmt.Errorf("modelo não devolveu SVG válido")
	}
	if !strings.Contains(svg, bgPlaceholder) && !imageHrefRe.MatchString(svg) {
		if !strings.Contains(strings.ToLower(svg), "<image") {
			return "", usage, fmt.Errorf("SVG sem elemento <image> para o fundo")
		}
	}
	return svg, usage, nil
}

func buildImagePrompt(ctx context.Context, client *llm.Client, req Request, svg, feedback string) (string, llm.Usage, error) {
	sys := `És um diretor de arte. Escreve UM prompt curto (pt ou en) para um modelo de imagem gerar o FUNDO da arte.
O SVG já contém tipografia e estrutura — NÃO peças texto, logos nem ícones na imagem de fundo.
Podes ecoar a atmosfera das cores da marca no fundo, sem tipografia.
Responde só com o prompt, sem aspas nem markdown.`
	user := "Pedido:\n" + req.Prompt + "\n\nGuia:\n" + truncate(req.BrandGuide, 3000) +
		"\n\n" + brandfonts.PromptBlock(req.BrandVisual) +
		"\n\nSVG (contexto de composição):\n" + truncate(svg, 4000)
	if feedback != "" {
		user += "\n\nCorrigir com base neste feedback:\n" + feedback
	}
	out, usage, err := client.Chat(ctx, req.Models.General, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	})
	if err != nil {
		return "", usage, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return req.Prompt, usage, nil
	}
	return out, usage, nil
}

func reviewArt(ctx context.Context, client *llm.Client, req Request, finalSVG string, png []byte, prevFeedback string) (Review, llm.Usage, error) {
	sys := `És um revisor de brand design. Avalia se a arte (PNG final renderizado do SVG) cumpre o pedido e o guia da marca.
Responde APENAS JSON válido:
{"approved":true|false,"feedback":"razão curta e acionável em português"}
Aprova só se estiver claramente aderente.`

	text := "Pedido:\n" + req.Prompt + "\n\nGuia da marca:\n" + truncate(req.BrandGuide, 4000) +
		"\n\nSVG final (estrutura):\n" + truncate(finalSVG, 5000)
	if prevFeedback != "" {
		text += "\n\nFeedback anterior que deveria ter sido corrigido:\n" + prevFeedback
	}
	parts := []llm.ContentPart{
		{Type: "text", Text: text},
		{Type: "image_url", ImageURL: &llm.ImageURL{URL: llm.BytesDataURL(png, "image/png")}},
	}
	parts = append(parts, imageParts(req.BrandAssets)...)

	raw, usage, err := client.Chat(ctx, req.Models.General, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: parts},
	})
	if err != nil {
		return Review{}, usage, err
	}
	return parseReview(raw), usage, nil
}

func parseReview(raw string) Review {
	raw = strings.TrimSpace(raw)
	r := Review{Raw: raw, Approved: false, Feedback: raw}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		var parsed Review
		if err := json.Unmarshal([]byte(raw[start:end+1]), &parsed); err == nil {
			parsed.Raw = raw
			if parsed.Feedback == "" && !parsed.Approved {
				parsed.Feedback = "Rejeitado sem feedback detalhado."
			}
			return parsed
		}
	}
	low := strings.ToLower(raw)
	if strings.Contains(low, `"approved": true`) || strings.Contains(low, `"approved":true`) {
		r.Approved = true
	}
	return r
}

func ExtractSVG(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```svg")
	s = strings.TrimPrefix(s, "```xml")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	if m := svgBlockRe.FindString(s); m != "" {
		return m
	}
	if strings.HasPrefix(strings.ToLower(s), "<svg") {
		return s
	}
	return ""
}

// InjectBackground replaces placeholder or first <image> href with filename.
func InjectBackground(svg, filename string) string {
	if strings.Contains(svg, bgPlaceholder) {
		return strings.ReplaceAll(svg, bgPlaceholder, filename)
	}
	if imageHrefRe.MatchString(svg) {
		return imageHrefRe.ReplaceAllString(svg, `${1}`+filename+`${3}`)
	}
	lower := strings.ToLower(svg)
	idx := strings.Index(lower, "<svg")
	if idx < 0 {
		return svg
	}
	end := strings.Index(svg[idx:], ">")
	if end < 0 {
		return svg
	}
	insertAt := idx + end + 1
	tag := fmt.Sprintf(`<image href="%s" x="0" y="0" width="100%%" height="100%%" preserveAspectRatio="xMidYMid slice"/>`, filename)
	return svg[:insertAt] + tag + svg[insertAt:]
}

func listAssets(assets []config.Asset) string {
	if len(assets) == 0 {
		return "(nenhum)"
	}
	var b strings.Builder
	for _, a := range assets {
		b.WriteString(fmt.Sprintf("- [%s] %s\n", a.Label, a.Path))
	}
	return b.String()
}

func imageParts(assets []config.Asset) []llm.ContentPart {
	var parts []llm.ContentPart
	for _, a := range assets {
		ext := strings.ToLower(filepath.Ext(a.Path))
		if ext == ".svg" {
			continue
		}
		if _, err := os.Stat(a.Path); err != nil {
			continue
		}
		url, err := llm.FileDataURL(a.Path)
		if err != nil {
			continue
		}
		parts = append(parts, llm.ContentPart{
			Type: "text",
			Text: fmt.Sprintf("Imagem label=%q path=%s", a.Label, a.Path),
		})
		parts = append(parts, llm.ContentPart{
			Type:     "image_url",
			ImageURL: &llm.ImageURL{URL: url},
		})
	}
	return parts
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return output.WriteFile(dst, data)
}
