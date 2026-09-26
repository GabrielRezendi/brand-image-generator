package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GabrielRezendi/brand-image-generator/internal/config"
	"github.com/GabrielRezendi/brand-image-generator/internal/llm"
	"github.com/GabrielRezendi/brand-image-generator/internal/pipeline"
	"github.com/GabrielRezendi/brand-image-generator/internal/version"
)

type screen int

const (
	screenSetup screen = iota
	screenHome
	screenRequest
	screenSettings
	screenRunning
	screenResults
)

type resultMsg pipeline.Result

type App struct {
	cfg    config.Config
	screen screen
	width  int
	height int
	err    string

	wizard Model

	homeCursor int

	// request
	promptInput   textarea.Model
	attachInput   textinput.Model
	attachments   []config.Asset
	reqFocus      int // 0 prompt, 1 attach
	sessionProv   config.Provider
	sessionModels config.Models

	// settings (see settings.go)
	setPage        int // setPageMenu or section
	setMenuCursor  int
	setModelFocus  int
	setProv        int
	setReview      int // 0 off, 1 on
	setSVG         textinput.Model
	setImage       textinput.Model
	setGeneral     textinput.Model
	setGuideMode   int // 0 path, 1 text
	setGuidePath   textinput.Model
	setGuideText   textarea.Model
	setAssets      []config.Asset
	setAssetPath   textinput.Model
	setAssetLabel  textinput.Model
	setLabelCursor int
	setCustomLabel bool
	setAssetFocus  int // 0 path, 1 custom label
	setVisualFocus int // 0..3
	setPrimaryFont    textinput.Model
	setSecondaryFont  textinput.Model
	setPrimaryColor   textinput.Model
	setSecondaryColor textinput.Model

	// running
	cancel       context.CancelFunc
	progress     pipeline.Progress
	progressLog  []string
	runningSince time.Time

	// results
	result pipeline.Result
}

func NewApp(cfg config.Config) App {
	a := App{cfg: cfg}
	if config.IsReady(cfg) {
		a.screen = screenHome
	} else {
		a.screen = screenSetup
		a.wizard = New(cfg)
	}
	a.resetRequest()
	a.resetSettings()
	return a
}

func (a *App) resetRequest() {
	ta := textarea.New()
	ta.Placeholder = "Descreve a arte que queres gerar…"
	ta.SetWidth(64)
	ta.SetHeight(6)
	ta.Focus()
	ta.CharLimit = 8000

	ai := textinput.New()
	ai.Placeholder = "colar/arrastar caminho de imagem (png/jpg/svg) + enter"
	ai.CharLimit = 1024
	ai.Width = 64

	a.promptInput = ta
	a.attachInput = ai
	a.attachments = nil
	a.reqFocus = 0
	a.sessionProv = a.cfg.DefaultProvider
	a.sessionModels = a.cfg.Models
}


func (a App) Init() tea.Cmd {
	if a.screen == screenSetup {
		return a.wizard.Init()
	}
	return textarea.Blink
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.layoutSettings()
		a.promptInput.SetWidth(a.contentWidth())
		a.attachInput.Width = a.contentWidth()
		if a.screen == screenSetup {
			w, cmd := a.wizard.Update(msg)
			a.wizard = w.(Model)
			return a, cmd
		}
		return a, nil

	case SetupDoneMsg:
		a.cfg = msg.Cfg
		a.err = ""
		a.screen = screenHome
		a.homeCursor = 0
		a.resetRequest()
		a.resetSettings()
		return a, nil

	case WizardQuitMsg:
		return a, tea.Quit

	case genEventMsg:
		if msg.progress != nil {
			a.progress = *msg.progress
			label := msg.progress.Label
			if label == "" {
				label = msg.progress.Phase
			}
			line := fmt.Sprintf("Tentativa %d · %s — %s", msg.progress.Attempt, label, msg.progress.Detail)
			a.progressLog = append(a.progressLog, line)
			if len(a.progressLog) > 14 {
				a.progressLog = a.progressLog[len(a.progressLog)-14:]
			}
		}
		return a, pollGeneration(msg.pCh, msg.rCh)

	case resultMsg:
		a.result = pipeline.Result(msg)
		a.cancel = nil
		if a.result.Err != nil && !a.result.Canceled && len(a.result.Attempts) == 0 {
			a.err = a.result.Err.Error()
			a.screen = screenRequest
			return a, nil
		}
		a.screen = screenResults
		return a, nil

	case tickMsg:
		if a.screen == screenRunning {
			return a, tickElapsed()
		}
		return a, nil
	}

	if a.screen == screenSetup {
		w, cmd := a.wizard.Update(msg)
		a.wizard = w.(Model)
		return a, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if a.cancel != nil {
				a.cancel()
			}
			return a, tea.Quit
		}
	}

	switch a.screen {
	case screenHome:
		return a.updateHome(msg)
	case screenRequest:
		return a.updateRequest(msg)
	case screenSettings:
		return a.updateSettings(msg)
	case screenRunning:
		return a.updateRunning(msg)
	case screenResults:
		return a.updateResults(msg)
	}
	return a, nil
}

func (a App) updateHome(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if a.homeCursor > 0 {
				a.homeCursor--
			}
		case "down", "j":
			if a.homeCursor < 2 {
				a.homeCursor++
			}
		case "enter":
			switch a.homeCursor {
			case 0:
				a.err = ""
				a.resetRequest()
				a.screen = screenRequest
			case 1:
				a.err = ""
				a.resetSettings()
				a.screen = screenSettings
			case 2:
				return a, tea.Quit
			}
		case "q":
			return a, tea.Quit
		case "n":
			a.resetRequest()
			a.screen = screenRequest
		case "s":
			a.resetSettings()
			a.screen = screenSettings
		}
	}
	return a, nil
}

func (a App) updateRequest(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			a.screen = screenHome
			return a, nil
		case "ctrl+p":
			if a.sessionProv == config.ProviderOpenRouter {
				a.sessionProv = config.ProviderOpenAI
			} else {
				a.sessionProv = config.ProviderOpenRouter
			}
			return a, nil
		case "tab":
			if a.reqFocus == 0 {
				a.reqFocus = 1
				a.promptInput.Blur()
				a.attachInput.Focus()
			} else {
				a.reqFocus = 0
				a.attachInput.Blur()
				a.promptInput.Focus()
			}
			return a, textinput.Blink
		case "ctrl+g":
			return a.startGeneration()
		case "ctrl+d":
			if len(a.attachments) > 0 {
				a.attachments = a.attachments[:len(a.attachments)-1]
			}
			return a, nil
		case "enter":
			if a.reqFocus == 1 {
				return a.addAttachment(strings.TrimSpace(a.attachInput.Value()))
			}
		}
	}

	var cmd tea.Cmd
	if a.reqFocus == 0 {
		a.promptInput, cmd = a.promptInput.Update(msg)
	} else {
		a.attachInput, cmd = a.attachInput.Update(msg)
		// drag-and-drop / paste often fills the path at once
		val := strings.TrimSpace(a.attachInput.Value())
		if isImagePath(strings.Trim(val, `"'`)) && strings.ContainsAny(val, " \n\t") == false {
			if _, err := os.Stat(strings.Trim(val, `"'`)); err == nil {
				// keep in field until enter — no auto-add
			}
		}
	}
	return a, cmd
}

func (a App) addAttachment(path string) (tea.Model, tea.Cmd) {
	path = strings.Trim(path, `"'`)
	if path == "" {
		return a, nil
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	if !isImagePath(path) {
		a.err = "formato inválido — png, jpg ou svg"
		return a, nil
	}
	if _, err := os.Stat(path); err != nil {
		a.err = "ficheiro não encontrado"
		return a, nil
	}
	a.appendAttachment(path, "anexo")
	a.attachInput.SetValue("")
	a.err = ""
	return a, nil
}

func (a *App) appendAttachment(path, label string) bool {
	path = strings.TrimSpace(path)
	for _, x := range a.attachments {
		if x.Path == path {
			return false
		}
	}
	a.attachments = append(a.attachments, config.Asset{Path: path, Label: label})
	return true
}

func isImagePath(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".svg":
		return strings.Contains(p, string(os.PathSeparator)) || strings.HasPrefix(p, ".") || strings.HasPrefix(p, "~")
	default:
		return false
	}
}

type genEventMsg struct {
	progress *pipeline.Progress
	pCh      <-chan pipeline.Progress
	rCh      <-chan pipeline.Result
}

func (a App) startGeneration() (tea.Model, tea.Cmd) {
	prompt := strings.TrimSpace(a.promptInput.Value())
	if prompt == "" {
		a.err = "escreve o pedido da arte"
		return a, nil
	}
	key, err := config.APIKeyFor(a.cfg, a.sessionProv)
	if err != nil || key == "" {
		a.err = fmt.Sprintf("sem chave para %s — muda com Ctrl+P ou em Definições", config.ProviderLabel(a.sessionProv))
		return a, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.progress = pipeline.Progress{}
	a.progressLog = nil
	a.runningSince = time.Now()
	a.err = ""
	a.screen = screenRunning

	cfg := a.cfg
	req := pipeline.Request{
		Prompt:       prompt,
		Attachments:  append([]config.Asset(nil), a.attachments...),
		Provider:     a.sessionProv,
		Models:       a.sessionModels,
		BrandGuide:   a.cfg.BrandGuide.Content,
		BrandVisual:  a.cfg.BrandVisual,
		BrandAssets:  append([]config.Asset(nil), a.cfg.Assets...),
		OutputBase:   a.cfg.OutputDir,
		EnableReview: a.cfg.EnableReview,
	}

	pCh := make(chan pipeline.Progress, 16)
	rCh := make(chan pipeline.Result, 1)
	go func() {
		res := pipeline.New(cfg).Run(ctx, req, func(p pipeline.Progress) {
			select {
			case pCh <- p:
			case <-ctx.Done():
			}
		})
		rCh <- res
		close(pCh)
	}()

	return a, tea.Batch(pollGeneration(pCh, rCh), tickElapsed())
}

func pollGeneration(pCh <-chan pipeline.Progress, rCh <-chan pipeline.Result) tea.Cmd {
	return func() tea.Msg {
		select {
		case p, ok := <-pCh:
			if !ok {
				return resultMsg(<-rCh)
			}
			return genEventMsg{progress: &p, pCh: pCh, rCh: rCh}
		case r := <-rCh:
			return resultMsg(r)
		}
	}
}

type tickMsg time.Time

func tickElapsed() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (a App) updateRunning(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+x":
			if a.cancel != nil {
				a.cancel()
				a.progressLog = append(a.progressLog, "cancelamento pedido…")
			}
		}
	}
	return a, nil
}

func (a App) updateResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "n":
			a.resetRequest()
			a.screen = screenRequest
		case "esc", "h":
			a.screen = screenHome
		case "q":
			return a, tea.Quit
		}
	}
	return a, nil
}

func (a App) View() string {
	if a.screen == screenSetup {
		return a.wizard.View()
	}

	var b strings.Builder
	b.WriteString(brandStyle.Render("Brand Image Generator"))
	b.WriteString("\n")

	switch a.screen {
	case screenHome:
		b.WriteString(mutedStyle.Render("menu principal"))
		b.WriteString("\n\n")
		b.WriteString(a.viewHome())
	case screenRequest:
		b.WriteString(mutedStyle.Render("novo pedido"))
		b.WriteString("\n\n")
		b.WriteString(a.viewRequest())
	case screenSettings:
		if a.setPage == setPageMenu {
			b.WriteString(mutedStyle.Render("definições"))
		} else {
			b.WriteString(mutedStyle.Render("definições · secção"))
		}
		b.WriteString("\n\n")
		b.WriteString(a.viewSettings())
	case screenRunning:
		b.WriteString(mutedStyle.Render("a gerar…"))
		b.WriteString("\n\n")
		b.WriteString(a.viewRunning())
	case screenResults:
		b.WriteString(mutedStyle.Render("resultado"))
		b.WriteString("\n\n")
		b.WriteString(a.viewResults())
	}

	if a.err != "" {
		b.WriteString("\n\n")
		if strings.HasPrefix(a.err, "ok:") {
			b.WriteString(okStyle.Render("✓ " + strings.TrimPrefix(a.err, "ok:")))
		} else {
			b.WriteString(errorStyle.Render("⚠ " + a.err))
		}
	}
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render(a.help()))
	boxW := a.contentWidth() + 4
	if a.width > 0 {
		boxW = min(a.width-2, max(44, a.contentWidth()+4))
	}
	return boxStyle.Width(boxW).Render(b.String())
}

func (a App) viewHome() string {
	items := []string{"Novo pedido", "Definições", "Sair"}
	var b strings.Builder
	b.WriteString(mutedStyle.Render("versão " + version.String()))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Provider: %s\n", config.ProviderLabel(a.cfg.DefaultProvider)))
	b.WriteString(mutedStyle.Render(fmt.Sprintf("SVG %s · IMG %s · Geral %s", a.cfg.Models.SVG, a.cfg.Models.Image, a.cfg.Models.General)))
	b.WriteString("\n")
	guideHint := "(texto inline)"
	if p := strings.TrimSpace(a.cfg.BrandGuide.Path); p != "" {
		guideHint = p
	} else if n := len([]rune(strings.TrimSpace(a.cfg.BrandGuide.Content))); n > 0 {
		guideHint = fmt.Sprintf("texto inline (%d chars)", n)
	}
	b.WriteString(mutedStyle.Render(fmt.Sprintf("Guia: %s · Visual: %s · Assets: %d",
		guideHint, truncateRunes(a.cfg.BrandVisual.Summary(), 40), len(a.cfg.Assets))))
	b.WriteString("\n")
	reviewHint := "Revisão: desligada"
	if a.cfg.EnableReview {
		reviewHint = "Revisão: ligada (até 3 tentativas)"
	}
	b.WriteString(mutedStyle.Render(reviewHint))
	b.WriteString("\n\n")
	for i, it := range items {
		if i == a.homeCursor {
			b.WriteString(titleStyle.Render("▸ " + it))
		} else {
			b.WriteString(mutedStyle.Render("  " + it))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (a App) viewRequest() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Pedido"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(fmt.Sprintf("provider: %s (Ctrl+P) · modelos da sessão = defaults", config.ProviderLabel(a.sessionProv))))
	b.WriteString("\n\n")
	b.WriteString(a.promptInput.View())
	b.WriteString("\n\n")
	b.WriteString("Anexos (opcional — arrasta/cola o caminho)\n")
	b.WriteString(a.attachInput.View())
	b.WriteString("\n")
	if len(a.attachments) == 0 {
		b.WriteString(mutedStyle.Render("nenhum anexo"))
	} else {
		for _, x := range a.attachments {
			b.WriteString(fmt.Sprintf("  • [%s] %s\n", x.Label, x.Path))
		}
	}
	return b.String()
}

func (a App) viewRunning() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("A gerar"))
	b.WriteString("\n")
	elapsed := time.Since(a.runningSince).Truncate(time.Second)
	b.WriteString(mutedStyle.Render(fmt.Sprintf("total: %s", formatDur(elapsed))))
	b.WriteString("\n\n")

	stages := []struct{ key, title string }{
		{"svg", "1. Gerando SVG"},
		{"fonts", "2. Preparando fontes"},
		{"image", "3. Gerando fundo"},
		{"compose", "4. Montando elements/"},
		{"raster", "5. Convertendo SVG → PNG"},
	}
	if a.cfg.EnableReview {
		stages = append(stages, struct{ key, title string }{"review", "6. Revisando arte"})
	}
	current := a.progress.Phase
	attempt := a.progress.Attempt
	if attempt == 0 {
		attempt = 1
	}
	maxAttempts := 1
	if a.cfg.EnableReview {
		maxAttempts = pipeline.MaxAttempts
	}
	b.WriteString(fmt.Sprintf("Tentativa %d/%d\n\n", attempt, maxAttempts))
	if !a.cfg.EnableReview {
		b.WriteString(mutedStyle.Render("revisão automática desligada"))
		b.WriteString("\n\n")
	}
	curIdx := stageIndex(current)
	if current == "" {
		curIdx = 0
	}
	for _, s := range stages {
		idx := stageIndex(s.key)
		stat := latestStage(a.progress.Completed, attempt, s.key)
		switch {
		case current == "retry" || current == "done" || idx < curIdx:
			line := "✓ " + s.title
			if stat != nil {
				line += "  " + formatDur(stat.Duration)
			}
			b.WriteString(okStyle.Render(line))
		case idx == curIdx:
			title := s.title
			if a.progress.Label != "" && s.key == current {
				title = a.progress.Label
			}
			live := time.Duration(0)
			if !a.progress.StartedAt.IsZero() {
				live = time.Since(a.progress.StartedAt)
			} else if current == "" {
				live = time.Since(a.runningSince)
			}
			b.WriteString(titleStyle.Render("▶ " + title + "  " + formatDur(live)))
			if a.progress.Detail != "" && (s.key == current || (current == "" && s.key == "svg")) {
				b.WriteString("\n  ")
				b.WriteString(mutedStyle.Render(a.progress.Detail))
			}
		default:
			b.WriteString(mutedStyle.Render("○ " + s.title))
		}
		b.WriteString("\n")
	}
	if current == "retry" {
		live := time.Duration(0)
		if !a.progress.StartedAt.IsZero() {
			live = time.Since(a.progress.StartedAt)
		}
		b.WriteString("\n")
		b.WriteString(titleStyle.Render("↻ A reiterar  " + formatDur(live)))
		b.WriteString("\n  ")
		b.WriteString(mutedStyle.Render(truncateRunes(a.progress.Detail, 160)))
		b.WriteString("\n")
	}

	if len(a.progress.Completed) > 0 {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("etapas concluídas"))
		b.WriteString("\n")
		shown := a.progress.Completed
		if len(shown) > 8 {
			shown = shown[len(shown)-8:]
			b.WriteString(mutedStyle.Render("…"))
			b.WriteString("\n")
		}
		for _, st := range shown {
			if st.Phase == "done" || st.Phase == "retry" {
				continue
			}
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  t%d %s  %s", st.Attempt, st.Label, formatDur(st.Duration))))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("esc / ctrl+x cancelar"))
	return b.String()
}

func latestStage(stats []pipeline.StageStat, attempt int, phase string) *pipeline.StageStat {
	for i := len(stats) - 1; i >= 0; i-- {
		if stats[i].Attempt == attempt && stats[i].Phase == phase {
			s := stats[i]
			return &s
		}
	}
	return nil
}

func formatDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Truncate(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func formatUSD(v float64) string {
	if v < 0 {
		v = 0
	}
	return fmt.Sprintf("$%.4f", v)
}

func formatTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%s", commaInt(n))
}

func commaInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		s = s[1:]
	}
	var out []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	if n < 0 {
		return "-" + string(out)
	}
	return string(out)
}

func stageIndex(phase string) int {
	switch phase {
	case "svg":
		return 0
	case "fonts":
		return 1
	case "image":
		return 2
	case "compose":
		return 3
	case "raster":
		return 4
	case "review":
		return 5
	case "retry", "done":
		return 6
	default:
		return -1
	}
}

func (a App) viewResults() string {
	var b strings.Builder
	r := a.result
	if r.Canceled {
		b.WriteString(errorStyle.Render("Geração cancelada"))
	} else if r.ReviewSkipped {
		b.WriteString(okStyle.Render("✓ Gerado (sem revisão)"))
	} else if r.Approved {
		b.WriteString(okStyle.Render(fmt.Sprintf("✓ Aprovado na tentativa %d", r.FinalIndex)))
	} else {
		b.WriteString(titleStyle.Render("3 tentativas sem aprovação — resultados entregues"))
	}
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Pasta: %s\n\n", r.JobDir))
	for _, at := range r.Attempts {
		status := "rejeitado"
		if r.ReviewSkipped {
			status = "entregue"
		} else if at.Review.Approved {
			status = "aprovado"
		}
		b.WriteString(fmt.Sprintf("Attempt %d [%s]\n", at.Index, status))
		if at.FinalPNGPath != "" {
			b.WriteString(mutedStyle.Render("  png  " + at.FinalPNGPath))
			b.WriteString("\n")
		}
		if at.FinalSVGPath != "" {
			b.WriteString(mutedStyle.Render("  svg  " + at.FinalSVGPath))
			b.WriteString("\n")
		}
		if at.ElementsDir != "" {
			b.WriteString(mutedStyle.Render("  elems " + at.ElementsDir))
			b.WriteString("\n")
		}
		if !r.ReviewSkipped && at.Review.Feedback != "" {
			b.WriteString(mutedStyle.Render("  feedback: " + truncateRunes(at.Review.Feedback, 120)))
			b.WriteString("\n")
		}
	}
	if r.Err != nil && !r.Canceled {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(r.Err.Error()))
	}
	b.WriteString("\n")
	b.WriteString(a.viewUsage(r.Stages))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Artefactos: final.png · final.svg · elements/ · usage.json · conversation.log"))
	return b.String()
}

func (a App) viewUsage(stages []pipeline.StageStat) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Tempo, tokens e custo"))
	b.WriteString("\n")
	var total pipeline.StageStat
	anyEst := false
	count := 0
	for _, s := range stages {
		if s.Phase == "done" || s.Phase == "retry" {
			continue
		}
		count++
		est := ""
		if s.Estimated {
			est = " ~"
			anyEst = true
		}
		b.WriteString(fmt.Sprintf("  t%d %-22s  %6s  %7s tok  %s%s\n",
			s.Attempt,
			truncateRunes(s.Label, 22),
			formatDur(s.Duration),
			formatTokens(s.TotalTokens),
			formatUSD(s.CostUSD),
			est,
		))
		total.Duration += s.Duration
		total = total.AddUsage(llm.Usage{
			PromptTokens:     s.PromptTokens,
			CompletionTokens: s.CompletionTokens,
			TotalTokens:      s.TotalTokens,
			CostUSD:          s.CostUSD,
			Estimated:        s.Estimated,
		})
	}
	if count == 0 {
		b.WriteString(mutedStyle.Render("  (sem métricas)"))
		b.WriteString("\n")
		return b.String()
	}
	b.WriteString(okStyle.Render(fmt.Sprintf("  total                  %6s  %7s tok  %s",
		formatDur(total.Duration),
		formatTokens(total.TotalTokens),
		formatUSD(total.CostUSD),
	)))
	b.WriteString("\n")
	if anyEst || total.Estimated {
		b.WriteString(mutedStyle.Render("  ~ custo estimado quando a API não envia o valor"))
		b.WriteString("\n")
	}
	b.WriteString(mutedStyle.Render(fmt.Sprintf("  in %s · out %s", formatTokens(total.PromptTokens), formatTokens(total.CompletionTokens))))
	b.WriteString("\n")
	return b.String()
}

func (a App) help() string {
	switch a.screen {
	case screenHome:
		return "↑/↓ · enter · n novo · s definições · q sair"
	case screenRequest:
		return "tab anexos · enter anexar · ctrl+g gerar · ctrl+p provider · ctrl+d remover anexo · esc"
	case screenSettings:
		return a.settingsHelp()
	case screenRunning:
		return "esc / ctrl+x cancelar · ctrl+c sair"
	case screenResults:
		return "enter/n novo pedido · h menu · q sair"
	default:
		return ""
	}
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
