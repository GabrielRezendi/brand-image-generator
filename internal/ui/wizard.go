package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GabrielRezendi/brand-image-generator/internal/config"
)

type step int

const (
	stepAPIKeys step = iota
	stepProvider
	stepSVGModel
	stepImageModel
	stepGeneralModel
	stepBrandGuide
	stepBrandVisual
	stepAssets
	stepSummary
	stepDone
)

// SetupDoneMsg is emitted after the wizard saves configuration.
type SetupDoneMsg struct {
	Cfg config.Config
}

// WizardQuitMsg asks the root app to exit.
type WizardQuitMsg struct{}

var suggestedLabels = []string{
	"logo",
	"produto",
	"referência",
	"tipografia",
	"paleta",
	"outro",
}

var allowedExt = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".svg":  true,
}

type Model struct {
	cfg    config.Config
	step   step
	width  int
	height int
	err    string
	saved  bool
	path   string

	// stepAPIKeys
	openRouterInput textinput.Model
	openAIInput     textinput.Model
	apiFocus        int

	// model steps
	modelInput textinput.Model

	// brand guide
	guideMode      int // 0 = path, 1 = text
	guidePathInput textinput.Model
	guideText      textarea.Model

	// assets
	assetPathInput  textinput.Model
	assetLabelInput textinput.Model
	assetFocus      int
	labelCursor     int
	customLabel     bool

	// brand visual
	visualFocus int // 0..3
	primaryFont textinput.Model
	secondaryFont textinput.Model
	primaryColor  textinput.Model
	secondaryColor textinput.Model

	// provider
	providerCursor int
}

func New(cfg config.Config) Model {
	or := textinput.New()
	or.Placeholder = "sk-or-... ou ${OPENROUTER_API_KEY}"
	or.EchoMode = textinput.EchoPassword
	or.EchoCharacter = '•'
	or.CharLimit = 512
	or.Width = 56
	or.SetValue(cfg.OpenRouterAPIKey)
	or.Focus()

	oa := textinput.New()
	oa.Placeholder = "sk-... ou ${OPENAI_API_KEY}"
	oa.EchoMode = textinput.EchoPassword
	oa.EchoCharacter = '•'
	oa.CharLimit = 512
	oa.Width = 56
	oa.SetValue(cfg.OpenAIAPIKey)

	mi := textinput.New()
	mi.CharLimit = 200
	mi.Width = 56

	gp := textinput.New()
	gp.Placeholder = "/caminho/para/guia.md"
	gp.CharLimit = 1024
	gp.Width = 56
	gp.SetValue(cfg.BrandGuide.Path)

	gt := textarea.New()
	gt.Placeholder = "Cole ou escreva o guia da marca em Markdown…"
	gt.SetWidth(56)
	gt.SetHeight(8)
	gt.SetValue(cfg.BrandGuide.Content)

	ap := textinput.New()
	ap.Placeholder = "~/imagens/logo.png ou /caminho/absoluto.png"
	ap.CharLimit = 1024
	ap.Width = 56

	al := textinput.New()
	al.Placeholder = "label personalizado"
	al.CharLimit = 80
	al.Width = 40

	pf, sf, pc, sc := newBrandInputs(cfg.BrandVisual)

	path, _ := config.Path()
	providerCursor := 0
	if cfg.DefaultProvider == config.ProviderOpenAI {
		providerCursor = 1
	}

	return Model{
		cfg:             cfg,
		step:            stepAPIKeys,
		path:            path,
		openRouterInput: or,
		openAIInput:     oa,
		modelInput:      mi,
		guidePathInput:  gp,
		guideText:       gt,
		assetPathInput:  ap,
		assetLabelInput: al,
		providerCursor:  providerCursor,
		primaryFont:     pf,
		secondaryFont:   sf,
		primaryColor:    pc,
		secondaryColor:  sc,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, func() tea.Msg { return WizardQuitMsg{} }
		case "esc":
			if m.step == stepDone {
				return m, func() tea.Msg { return SetupDoneMsg{Cfg: m.cfg} }
			}
			if m.step > stepAPIKeys {
				m.err = ""
				m.step--
				m.prepareStep()
				return m, nil
			}
			return m, func() tea.Msg { return WizardQuitMsg{} }
		}
	}

	switch m.step {
	case stepAPIKeys:
		return m.updateAPIKeys(msg)
	case stepProvider:
		return m.updateProvider(msg)
	case stepSVGModel, stepImageModel, stepGeneralModel:
		return m.updateModelStep(msg)
	case stepBrandGuide:
		return m.updateBrandGuide(msg)
	case stepBrandVisual:
		return m.updateBrandVisual(msg)
	case stepAssets:
		return m.updateAssets(msg)
	case stepSummary:
		return m.updateSummary(msg)
	case stepDone:
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, func() tea.Msg { return SetupDoneMsg{Cfg: m.cfg} }
		}
	}
	return m, nil
}

func (m *Model) prepareStep() {
	m.err = ""
	switch m.step {
	case stepAPIKeys:
		m.openRouterInput.Focus()
		m.openAIInput.Blur()
		m.apiFocus = 0
	case stepProvider:
		if m.cfg.DefaultProvider == config.ProviderOpenAI {
			m.providerCursor = 1
		} else {
			m.providerCursor = 0
		}
	case stepSVGModel:
		m.modelInput.SetValue(m.cfg.Models.SVG)
		m.modelInput.Focus()
	case stepImageModel:
		m.modelInput.SetValue(m.cfg.Models.Image)
		m.modelInput.Focus()
	case stepGeneralModel:
		m.modelInput.SetValue(m.cfg.Models.General)
		m.modelInput.Focus()
	case stepBrandGuide:
		if m.guideMode == 0 {
			m.guidePathInput.Focus()
			m.guideText.Blur()
		} else {
			m.guidePathInput.Blur()
			m.guideText.Focus()
		}
	case stepBrandVisual:
		m.visualFocus = 0
		m.blurBrandVisual()
		m.primaryFont.Focus()
	case stepAssets:
		m.assetPathInput.Focus()
		m.assetLabelInput.Blur()
		m.assetFocus = 0
		m.customLabel = false
		m.labelCursor = 0
	}
}

func (m Model) updateAPIKeys(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "up", "down":
			if m.apiFocus == 0 {
				m.apiFocus = 1
				m.openRouterInput.Blur()
				m.openAIInput.Focus()
			} else {
				m.apiFocus = 0
				m.openAIInput.Blur()
				m.openRouterInput.Focus()
			}
			return m, textinput.Blink
		case "enter":
			m.cfg.OpenRouterAPIKey = strings.TrimSpace(m.openRouterInput.Value())
			m.cfg.OpenAIAPIKey = strings.TrimSpace(m.openAIInput.Value())
			if !config.HasAnyAPIKey(m.cfg) {
				m.err = "define pelo menos uma chave (OpenRouter ou OpenAI)"
				return m, nil
			}
			if _, err := config.ResolveSecret(m.cfg.OpenRouterAPIKey); err != nil {
				m.err = err.Error()
				return m, nil
			}
			if _, err := config.ResolveSecret(m.cfg.OpenAIAPIKey); err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.err = ""
			m.step = stepProvider
			m.prepareStep()
			return m, textinput.Blink
		}
	}
	if m.apiFocus == 0 {
		m.openRouterInput, cmd = m.openRouterInput.Update(msg)
	} else {
		m.openAIInput, cmd = m.openAIInput.Update(msg)
	}
	return m, cmd
}

func (m Model) updateProvider(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k", "left", "h":
			m.providerCursor = 0
			return m, nil
		case "down", "j", "right", "l":
			m.providerCursor = 1
			return m, nil
		case "enter":
			if m.providerCursor == 1 {
				m.cfg.DefaultProvider = config.ProviderOpenAI
				if key, _ := config.APIKeyFor(m.cfg, config.ProviderOpenAI); key == "" {
					m.err = "chave OpenAI vazia — escolhe OpenRouter ou preenche a chave"
					return m, nil
				}
			} else {
				m.cfg.DefaultProvider = config.ProviderOpenRouter
				if key, _ := config.APIKeyFor(m.cfg, config.ProviderOpenRouter); key == "" {
					m.err = "chave OpenRouter vazia — escolhe OpenAI ou preenche a chave"
					return m, nil
				}
			}
			m.err = ""
			m.step = stepSVGModel
			m.prepareStep()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m Model) updateModelStep(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			val := strings.TrimSpace(m.modelInput.Value())
			if val == "" {
				m.err = "o modelo não pode ficar vazio"
				return m, nil
			}
			switch m.step {
			case stepSVGModel:
				m.cfg.Models.SVG = val
				m.step = stepImageModel
			case stepImageModel:
				m.cfg.Models.Image = val
				m.step = stepGeneralModel
			case stepGeneralModel:
				m.cfg.Models.General = val
				m.step = stepBrandGuide
			}
			m.prepareStep()
			return m, textinput.Blink
		}
	}
	m.modelInput, cmd = m.modelInput.Update(msg)
	return m, cmd
}

func (m Model) updateBrandGuide(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+t":
			if m.guideMode == 0 {
				m.guideMode = 1
				m.guidePathInput.Blur()
				m.guideText.Focus()
			} else {
				m.guideMode = 0
				m.guideText.Blur()
				m.guidePathInput.Focus()
			}
			return m, textinput.Blink
		case "ctrl+y":
			where, err := copyAllText(m.guideText.Value())
			if err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.err = "ok:copiado para " + where
			return m, nil
		case "enter":
			if m.guideMode == 1 {
				// textarea uses enter for newline; use ctrl+s / ctrl+enter pattern
				break
			}
			path := strings.TrimSpace(m.guidePathInput.Value())
			if path == "" && strings.TrimSpace(m.guideText.Value()) == "" {
				m.err = "importa um .md ou escreve o guia (Ctrl+T para alternar)"
				return m, nil
			}
			if path != "" {
				data, err := os.ReadFile(path)
				if err != nil {
					m.err = fmt.Sprintf("não foi possível ler o ficheiro: %v", err)
					return m, nil
				}
				m.cfg.BrandGuide.Path = path
				m.cfg.BrandGuide.Content = string(data)
			} else {
				m.cfg.BrandGuide.Path = ""
				m.cfg.BrandGuide.Content = m.guideText.Value()
			}
			m.err = ""
			m.step = stepBrandVisual
			m.prepareStep()
			return m, textinput.Blink
		case "ctrl+s":
			content := m.guideText.Value()
			path := strings.TrimSpace(m.guidePathInput.Value())
			if m.guideMode == 0 {
				if path == "" {
					m.err = "indica o caminho do .md"
					return m, nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					m.err = fmt.Sprintf("não foi possível ler o ficheiro: %v", err)
					return m, nil
				}
				m.cfg.BrandGuide.Path = path
				m.cfg.BrandGuide.Content = string(data)
			} else {
				if strings.TrimSpace(content) == "" && path == "" {
					m.err = "o guia não pode ficar vazio"
					return m, nil
				}
				if path != "" && strings.TrimSpace(content) == "" {
					data, err := os.ReadFile(path)
					if err != nil {
						m.err = fmt.Sprintf("não foi possível ler o ficheiro: %v", err)
						return m, nil
					}
					m.cfg.BrandGuide.Path = path
					m.cfg.BrandGuide.Content = string(data)
				} else {
					m.cfg.BrandGuide.Path = path
					m.cfg.BrandGuide.Content = content
				}
			}
			m.err = ""
			m.step = stepBrandVisual
			m.prepareStep()
			return m, textinput.Blink
		}
	}
	if m.guideMode == 0 {
		m.guidePathInput, cmd = m.guidePathInput.Update(msg)
	} else {
		m.guideText, cmd = m.guideText.Update(msg)
	}
	return m, cmd
}

func (m Model) updateBrandVisual(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			m.blurBrandVisual()
			m.visualFocus = (m.visualFocus + 1) % 4
			m.focusBrandVisual()
			return m, textinput.Blink
		case "shift+tab", "up":
			m.blurBrandVisual()
			m.visualFocus = (m.visualFocus + 3) % 4
			m.focusBrandVisual()
			return m, textinput.Blink
		case "enter", "ctrl+s":
			v, err := readBrandVisual(m.primaryFont, m.secondaryFont, m.primaryColor, m.secondaryColor)
			if err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.cfg.BrandVisual = v
			m.err = ""
			m.step = stepAssets
			m.prepareStep()
			return m, textinput.Blink
		}
	}
	switch m.visualFocus {
	case 0:
		m.primaryFont, cmd = m.primaryFont.Update(msg)
	case 1:
		m.secondaryFont, cmd = m.secondaryFont.Update(msg)
	case 2:
		m.primaryColor, cmd = m.primaryColor.Update(msg)
	case 3:
		m.secondaryColor, cmd = m.secondaryColor.Update(msg)
	}
	return m, cmd
}

func (m *Model) blurBrandVisual() {
	m.primaryFont.Blur()
	m.secondaryFont.Blur()
	m.primaryColor.Blur()
	m.secondaryColor.Blur()
}

func (m *Model) focusBrandVisual() {
	switch m.visualFocus {
	case 0:
		m.primaryFont.Focus()
	case 1:
		m.secondaryFont.Focus()
	case 2:
		m.primaryColor.Focus()
	case 3:
		m.secondaryColor.Focus()
	}
}

func (m Model) updateAssets(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+n":
			// skip / next without adding
			m.err = ""
			m.step = stepSummary
			return m, nil
		case "tab":
			if m.customLabel {
				if m.assetFocus == 0 {
					m.assetFocus = 1
					m.assetPathInput.Blur()
					m.assetLabelInput.Focus()
				} else {
					m.assetFocus = 0
					m.assetLabelInput.Blur()
					m.assetPathInput.Focus()
				}
				return m, textinput.Blink
			}
			return m, nil
		case "up":
			// cycle labels without stealing ←/→/h/l from the path field
			if !m.customLabel {
				if m.labelCursor > 0 {
					m.labelCursor--
				}
				return m, nil
			}
		case "down":
			if !m.customLabel {
				if m.labelCursor < len(suggestedLabels)-1 {
					m.labelCursor++
				}
				return m, nil
			}
		case "ctrl+l":
			m.customLabel = !m.customLabel
			if m.customLabel {
				m.assetFocus = 1
				m.assetPathInput.Blur()
				m.assetLabelInput.Focus()
			} else {
				m.assetFocus = 0
				m.assetLabelInput.Blur()
				m.assetPathInput.Focus()
			}
			return m, textinput.Blink
		case "ctrl+d":
			if len(m.cfg.Assets) > 0 {
				m.cfg.Assets = m.cfg.Assets[:len(m.cfg.Assets)-1]
			}
			return m, nil
		case "enter":
			path := expandHome(strings.TrimSpace(m.assetPathInput.Value()))
			if path == "" {
				// empty + enter advances if we already have assets or allow empty
				m.step = stepSummary
				return m, nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if !allowedExt[ext] {
				m.err = "formato inválido — usa png, jpg ou svg"
				return m, nil
			}
			if _, err := os.Stat(path); err != nil {
				m.err = fmt.Sprintf("ficheiro não encontrado: %v", err)
				return m, nil
			}
			label := suggestedLabels[m.labelCursor]
			if m.customLabel {
				label = strings.TrimSpace(m.assetLabelInput.Value())
				if label == "" {
					m.err = "define um label para a imagem"
					return m, nil
				}
			}
			m.cfg.Assets = append(m.cfg.Assets, config.Asset{Path: path, Label: label})
			m.assetPathInput.SetValue("")
			m.assetLabelInput.SetValue("")
			m.err = ""
			m.assetFocus = 0
			m.customLabel = false
			m.assetPathInput.Focus()
			m.assetLabelInput.Blur()
			return m, textinput.Blink
		}
	}

	if m.customLabel && m.assetFocus == 1 {
		m.assetLabelInput, cmd = m.assetLabelInput.Update(msg)
	} else {
		m.assetPathInput, cmd = m.assetPathInput.Update(msg)
	}
	return m, cmd
}

func expandHome(path string) string {
	path = strings.Trim(path, `"'`)
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func (m Model) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "s":
			if err := config.Save(m.cfg); err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.saved = true
			m.step = stepDone
			return m, func() tea.Msg { return SetupDoneMsg{Cfg: m.cfg} }
		}
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(brandStyle.Render("Brand Image Generator"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("configuração inicial · passo a passo"))
	b.WriteString("\n\n")
	b.WriteString(m.progressBar())
	b.WriteString("\n\n")

	switch m.step {
	case stepAPIKeys:
		b.WriteString(m.viewAPIKeys())
	case stepProvider:
		b.WriteString(m.viewProvider())
	case stepSVGModel:
		b.WriteString(m.viewModel("Modelo de geração de SVG", "Usado para criar a estrutura SVG da arte.", config.DefaultSVGModel))
	case stepImageModel:
		b.WriteString(m.viewModel("Modelo de geração de imagem", "Usado para gerar o PNG de fundo.", config.DefaultImageModel))
	case stepGeneralModel:
		b.WriteString(m.viewModel("Modelo de propósito geral", "Processa pedidos, refina e julga aderência à marca.", config.DefaultGeneralModel))
	case stepBrandGuide:
		b.WriteString(m.viewBrandGuide())
	case stepBrandVisual:
		b.WriteString(viewBrandVisualForm("Identidade visual", m.visualFocus, m.primaryFont, m.secondaryFont, m.primaryColor, m.secondaryColor))
	case stepAssets:
		b.WriteString(m.viewAssets())
	case stepSummary:
		b.WriteString(m.viewSummary())
	case stepDone:
		b.WriteString(m.viewDone())
	}

	if m.err != "" {
		b.WriteString("\n\n")
		if strings.HasPrefix(m.err, "ok:") {
			b.WriteString(okStyle.Render("✓ " + strings.TrimPrefix(m.err, "ok:")))
		} else {
			b.WriteString(errorStyle.Render("⚠ " + m.err))
		}
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render(m.helpLine()))
	return boxStyle.Width(max(60, m.width-4)).Render(b.String())
}

func (m Model) progressBar() string {
	labels := []string{"API", "Provider", "SVG", "IMG", "Geral", "Guia", "Visual", "Assets", "Resumo"}
	order := []step{stepAPIKeys, stepProvider, stepSVGModel, stepImageModel, stepGeneralModel, stepBrandGuide, stepBrandVisual, stepAssets, stepSummary}
	idx := 0
	for i, s := range order {
		if m.step == s || (m.step == stepDone && s == stepSummary) {
			idx = i
			break
		}
	}
	parts := make([]string, len(labels))
	for i, l := range labels {
		switch {
		case i < idx:
			parts[i] = okStyle.Render("✓ " + l)
		case i == idx:
			parts[i] = titleStyle.Render("● " + l)
		default:
			parts[i] = mutedStyle.Render("○ " + l)
		}
	}
	return strings.Join(parts, mutedStyle.Render(" · "))
}

func (m Model) viewAPIKeys() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("1. Chaves de API"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Podes usar OpenRouter, OpenAI, ou os dois. Para .env: ${NOME_DA_VAR}"))
	b.WriteString("\n\n")
	b.WriteString("OpenRouter\n")
	b.WriteString(m.openRouterInput.View())
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  " + keyPreview(m.openRouterInput.Value())))
	b.WriteString("\n\n")
	b.WriteString("OpenAI\n")
	b.WriteString(m.openAIInput.View())
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  " + keyPreview(m.openAIInput.Value())))
	return b.String()
}

func keyPreview(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "pré-visualização: (vazia)"
	}
	return "pré-visualização: " + config.MaskSecret(value)
}

func (m Model) viewProvider() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("2. Provider padrão"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Usado nas gerações. Podes mudar depois em Definições."))
	b.WriteString("\n\n")
	options := []string{"OpenRouter", "OpenAI"}
	for i, opt := range options {
		if i == m.providerCursor {
			b.WriteString(titleStyle.Render("▸ " + opt))
		} else {
			b.WriteString(mutedStyle.Render("  " + opt))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) viewModel(title, desc, def string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(desc))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("default: " + def))
	b.WriteString("\n\n")
	b.WriteString(m.modelInput.View())
	return b.String()
}

func (m Model) viewBrandGuide() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Guia da marca"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Importa um .md ou escreve/cola o conteúdo. Ctrl+T alterna · Ctrl+Y copia tudo."))
	b.WriteString("\n\n")
	if m.guideMode == 0 {
		b.WriteString(okStyle.Render("modo: ficheiro") + mutedStyle.Render("  |  texto"))
		b.WriteString("\n\n")
		b.WriteString(m.guidePathInput.View())
	} else {
		b.WriteString(mutedStyle.Render("modo: ficheiro  |  ") + okStyle.Render("texto"))
		b.WriteString("\n\n")
		b.WriteString(m.guideText.View())
	}
	return b.String()
}

func (m Model) viewAssets() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Imagens de apoio"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("png · jpg · svg  ·  aceita ~/…  ·  ↑/↓ label  ·  Ctrl+L custom  ·  Ctrl+N avançar"))
	b.WriteString("\n\n")
	b.WriteString("Caminho\n")
	b.WriteString(m.assetPathInput.View())
	if p := strings.TrimSpace(m.assetPathInput.Value()); strings.HasPrefix(p, "~") {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  → " + expandHome(p)))
	}
	b.WriteString("\n\nLabel: ")
	if m.customLabel {
		b.WriteString(m.assetLabelInput.View())
	} else {
		for i, l := range suggestedLabels {
			if i == m.labelCursor {
				b.WriteString(titleStyle.Render("[" + l + "]"))
			} else {
				b.WriteString(mutedStyle.Render(" " + l + " "))
			}
			b.WriteString(" ")
		}
	}
	b.WriteString("\n\n")
	if len(m.cfg.Assets) == 0 {
		b.WriteString(mutedStyle.Render("nenhuma imagem adicionada"))
	} else {
		b.WriteString(okStyle.Render(fmt.Sprintf("%d imagem(ns):", len(m.cfg.Assets))))
		b.WriteString("\n")
		for _, a := range m.cfg.Assets {
			b.WriteString(fmt.Sprintf("  • [%s] %s\n", a.Label, a.Path))
		}
		b.WriteString(mutedStyle.Render("Ctrl+D remove a última"))
	}
	return b.String()
}

func (m Model) viewSummary() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Resumo"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("OpenRouter: %s\n", config.MaskSecret(m.cfg.OpenRouterAPIKey)))
	b.WriteString(fmt.Sprintf("OpenAI:     %s\n", config.MaskSecret(m.cfg.OpenAIAPIKey)))
	b.WriteString(fmt.Sprintf("Provider:   %s\n", config.ProviderLabel(m.cfg.DefaultProvider)))
	b.WriteString(fmt.Sprintf("SVG:        %s\n", m.cfg.Models.SVG))
	b.WriteString(fmt.Sprintf("Imagem:     %s\n", m.cfg.Models.Image))
	b.WriteString(fmt.Sprintf("Geral:      %s\n", m.cfg.Models.General))
	guideSrc := "(texto inline)"
	if m.cfg.BrandGuide.Path != "" {
		guideSrc = m.cfg.BrandGuide.Path
	}
	preview := strings.TrimSpace(m.cfg.BrandGuide.Content)
	if len(preview) > 80 {
		preview = preview[:80] + "…"
	}
	b.WriteString(fmt.Sprintf("Guia:       %s\n", guideSrc))
	if preview != "" {
		b.WriteString(mutedStyle.Render("  " + preview))
		b.WriteString("\n")
	}
	b.WriteString("Visual:\n")
	b.WriteString(fmt.Sprintf("  fonte P: %s\n", emptyDash(m.cfg.BrandVisual.PrimaryFont)))
	b.WriteString(fmt.Sprintf("  fonte S: %s\n", emptyDash(m.cfg.BrandVisual.SecondaryFont)))
	b.WriteString("  cor P:   " + colorSwatch(m.cfg.BrandVisual.PrimaryColor) + "\n")
	b.WriteString("  cor S:   " + colorSwatch(m.cfg.BrandVisual.SecondaryColor) + "\n")
	b.WriteString(fmt.Sprintf("Assets:     %d\n", len(m.cfg.Assets)))
	for _, a := range m.cfg.Assets {
		b.WriteString(fmt.Sprintf("  • [%s] %s\n", a.Label, a.Path))
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Guardar em: " + m.path))
	return b.String()
}

func (m Model) viewDone() string {
	var b strings.Builder
	b.WriteString(okStyle.Render("✓ Configuração guardada"))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render(m.path))
	b.WriteString("\n\n")
	b.WriteString("A avançar para o ecrã principal…")
	return b.String()
}

func (m Model) helpLine() string {
	switch m.step {
	case stepAPIKeys:
		return "tab campos · enter continuar · esc sair"
	case stepProvider:
		return "↑/↓ escolher · enter continuar · esc voltar"
	case stepSVGModel, stepImageModel, stepGeneralModel:
		return "enter continuar · esc voltar"
	case stepBrandGuide:
		if m.guideMode == 0 {
			return "enter carregar ficheiro · ctrl+t modo texto · ctrl+y copiar · esc voltar"
		}
		return "ctrl+s continuar · ctrl+t modo ficheiro · ctrl+y copiar · esc voltar"
	case stepBrandVisual:
		return "tab campos · enter continuar · esc voltar"
	case stepAssets:
		return "←/→ no caminho · ↑/↓ label · enter adicionar · ctrl+n resumo · ctrl+l custom · esc"
	case stepSummary:
		return "enter/s guardar · esc voltar"
	case stepDone:
		return "qualquer tecla continuar"
	default:
		return ""
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(não definida)"
	}
	return s
}
