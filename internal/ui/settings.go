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

// Settings pages: -1 = menu, 0..5 = sections.
const (
	setPageMenu   = -1
	setPageProv   = 0
	setPageModels = 1
	setPageGuide  = 2
	setPageVisual = 3
	setPageAssets = 4
	setPageReview = 5
)

func (a *App) resetSettings() {
	a.setPage = setPageMenu
	a.setMenuCursor = 0
	a.setModelFocus = 0
	a.loadSettingsFromCfg()
	a.layoutSettings()
}

func (a *App) loadSettingsFromCfg() {
	if a.cfg.DefaultProvider == config.ProviderOpenAI {
		a.setProv = 1
	} else {
		a.setProv = 0
	}
	if a.cfg.EnableReview {
		a.setReview = 1
	} else {
		a.setReview = 0
	}

	a.setSVG = textinput.New()
	a.setSVG.Placeholder = config.DefaultSVGModel
	a.setSVG.SetValue(a.cfg.Models.SVG)
	a.setSVG.CharLimit = 200

	a.setImage = textinput.New()
	a.setImage.Placeholder = config.DefaultImageModel
	a.setImage.SetValue(a.cfg.Models.Image)
	a.setImage.CharLimit = 200

	a.setGeneral = textinput.New()
	a.setGeneral.Placeholder = config.DefaultGeneralModel
	a.setGeneral.SetValue(a.cfg.Models.General)
	a.setGeneral.CharLimit = 200

	a.setGuideMode = 0
	if strings.TrimSpace(a.cfg.BrandGuide.Path) == "" && strings.TrimSpace(a.cfg.BrandGuide.Content) != "" {
		a.setGuideMode = 1
	}
	a.setGuidePath = textinput.New()
	a.setGuidePath.Placeholder = "~/marca/guia.md"
	a.setGuidePath.SetValue(a.cfg.BrandGuide.Path)
	a.setGuidePath.CharLimit = 1024

	a.setGuideText = textarea.New()
	a.setGuideText.Placeholder = "Guia da marca em Markdown…"
	a.setGuideText.SetValue(a.cfg.BrandGuide.Content)
	a.setGuideText.CharLimit = 100_000

	a.setAssets = append([]config.Asset(nil), a.cfg.Assets...)
	a.setAssetPath = textinput.New()
	a.setAssetPath.Placeholder = "~/imagens/logo.png"
	a.setAssetPath.CharLimit = 1024
	a.setAssetLabel = textinput.New()
	a.setAssetLabel.Placeholder = "label personalizado"
	a.setAssetLabel.CharLimit = 80
	a.setLabelCursor = 0
	a.setCustomLabel = false
	a.setAssetFocus = 0

	a.setVisualFocus = 0
	a.setPrimaryFont, a.setSecondaryFont, a.setPrimaryColor, a.setSecondaryColor = newBrandInputs(a.cfg.BrandVisual)
}

func (a *App) layoutSettings() {
	w := a.contentWidth()
	h := a.contentHeight()

	a.setSVG.Width = w
	a.setImage.Width = w
	a.setGeneral.Width = w
	a.setGuidePath.Width = w
	a.setAssetPath.Width = w
	a.setAssetLabel.Width = min(40, w)
	a.setPrimaryFont.Width = w
	a.setSecondaryFont.Width = w
	a.setPrimaryColor.Width = min(20, w)
	a.setSecondaryColor.Width = min(20, w)

	guideH := max(4, min(14, h-12))
	a.setGuideText.SetWidth(w)
	a.setGuideText.SetHeight(guideH)
}

func (a App) contentWidth() int {
	w := a.width - 8
	if w < 40 {
		w = 40
	}
	if w > 100 {
		w = 100
	}
	if a.width > 0 && w > a.width-6 {
		w = max(20, a.width-6)
	}
	return w
}

func (a App) contentHeight() int {
	h := a.height - 8
	if h < 12 {
		h = 12
	}
	return h
}

func (a App) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		a.layoutSettings()
	}

	switch a.setPage {
	case setPageMenu:
		return a.updateSettingsMenu(msg)
	case setPageProv:
		return a.updateSettingsProvider(msg)
	case setPageModels:
		return a.updateSettingsModels(msg)
	case setPageGuide:
		return a.updateSettingsGuide(msg)
	case setPageVisual:
		return a.updateSettingsVisual(msg)
	case setPageAssets:
		return a.updateSettingsAssets(msg)
	case setPageReview:
		return a.updateSettingsReview(msg)
	default:
		a.setPage = setPageMenu
		return a, nil
	}
}

func (a App) updateSettingsMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	items := 6 // sections; esc goes home
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			a.err = ""
			a.screen = screenHome
			return a, nil
		case "up", "k":
			if a.setMenuCursor > 0 {
				a.setMenuCursor--
			}
		case "down", "j":
			if a.setMenuCursor < items-1 {
				a.setMenuCursor++
			}
		case "enter":
			a.err = ""
			a.blurSettings()
			a.setPage = a.setMenuCursor
			a.enterSettingsPage()
			return a, textinput.Blink
		}
	}
	return a, nil
}

func (a *App) enterSettingsPage() {
	a.layoutSettings()
	switch a.setPage {
	case setPageModels:
		a.setModelFocus = 0
		a.setSVG.Focus()
	case setPageGuide:
		if a.setGuideMode == 0 {
			a.setGuidePath.Focus()
		} else {
			a.setGuideText.Focus()
		}
	case setPageVisual:
		a.setVisualFocus = 0
		a.setPrimaryFont.Focus()
	case setPageAssets:
		a.setCustomLabel = false
		a.setAssetFocus = 0
		a.setAssetPath.Focus()
	}
}

func (a App) updateSettingsProvider(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			a.loadSettingsFromCfg()
			a.layoutSettings()
			a.setPage = setPageMenu
			a.err = ""
			return a, nil
		case "up", "k", "left":
			a.setProv = 0
		case "down", "j", "right":
			a.setProv = 1
		case "enter", "ctrl+s":
			return a.persistProvider()
		}
	}
	return a, nil
}

func (a App) updateSettingsModels(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			a.loadSettingsFromCfg()
			a.layoutSettings()
			a.setPage = setPageMenu
			a.err = ""
			return a, nil
		case "ctrl+s":
			return a.persistModels()
		case "tab", "down":
			a.setSVG.Blur()
			a.setImage.Blur()
			a.setGeneral.Blur()
			a.setModelFocus = (a.setModelFocus + 1) % 3
			a.focusModelField()
			return a, textinput.Blink
		case "shift+tab", "up":
			a.setSVG.Blur()
			a.setImage.Blur()
			a.setGeneral.Blur()
			a.setModelFocus = (a.setModelFocus + 2) % 3
			a.focusModelField()
			return a, textinput.Blink
		}
	}
	switch a.setModelFocus {
	case 0:
		a.setSVG, cmd = a.setSVG.Update(msg)
	case 1:
		a.setImage, cmd = a.setImage.Update(msg)
	case 2:
		a.setGeneral, cmd = a.setGeneral.Update(msg)
	}
	return a, cmd
}

func (a *App) focusModelField() {
	switch a.setModelFocus {
	case 0:
		a.setSVG.Focus()
	case 1:
		a.setImage.Focus()
	case 2:
		a.setGeneral.Focus()
	}
}

func (a App) updateSettingsGuide(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			a.loadSettingsFromCfg()
			a.layoutSettings()
			a.setPage = setPageMenu
			a.err = ""
			return a, nil
		case "ctrl+s":
			return a.persistGuide()
		case "ctrl+k":
			a.setGuideText.SetValue("")
			a.setGuidePath.SetValue("")
			a.err = ""
			return a, textinput.Blink
		case "ctrl+y":
			text := a.setGuideText.Value()
			where, err := copyAllText(text)
			if err != nil {
				a.err = err.Error()
				return a, nil
			}
			a.err = "ok:copiado para " + where
			return a, nil
		case "ctrl+t":
			if a.setGuideMode == 0 {
				a.setGuideMode = 1
				a.setGuidePath.Blur()
				a.setGuideText.Focus()
			} else {
				a.setGuideMode = 0
				a.setGuideText.Blur()
				a.setGuidePath.Focus()
			}
			return a, textinput.Blink
		case "enter":
			if a.setGuideMode == 0 {
				path := expandHome(strings.TrimSpace(a.setGuidePath.Value()))
				if path == "" {
					a.err = "indica o caminho do .md"
					return a, nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					a.err = fmt.Sprintf("não foi possível ler: %v", err)
					return a, nil
				}
				a.setGuidePath.SetValue(path)
				a.setGuideText.SetValue(string(data))
				a.err = ""
				return a, nil
			}
		}
	}
	if a.setGuideMode == 0 {
		a.setGuidePath, cmd = a.setGuidePath.Update(msg)
	} else {
		a.setGuideText, cmd = a.setGuideText.Update(msg)
	}
	return a, cmd
}

func (a App) updateSettingsVisual(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			a.loadSettingsFromCfg()
			a.layoutSettings()
			a.setPage = setPageMenu
			a.err = ""
			return a, nil
		case "ctrl+s", "enter":
			return a.persistVisual()
		case "tab", "down":
			a.blurVisualFields()
			a.setVisualFocus = (a.setVisualFocus + 1) % 4
			a.focusVisualField()
			return a, textinput.Blink
		case "shift+tab", "up":
			a.blurVisualFields()
			a.setVisualFocus = (a.setVisualFocus + 3) % 4
			a.focusVisualField()
			return a, textinput.Blink
		}
	}
	switch a.setVisualFocus {
	case 0:
		a.setPrimaryFont, cmd = a.setPrimaryFont.Update(msg)
	case 1:
		a.setSecondaryFont, cmd = a.setSecondaryFont.Update(msg)
	case 2:
		a.setPrimaryColor, cmd = a.setPrimaryColor.Update(msg)
	case 3:
		a.setSecondaryColor, cmd = a.setSecondaryColor.Update(msg)
	}
	return a, cmd
}

func (a *App) blurVisualFields() {
	a.setPrimaryFont.Blur()
	a.setSecondaryFont.Blur()
	a.setPrimaryColor.Blur()
	a.setSecondaryColor.Blur()
}

func (a *App) focusVisualField() {
	switch a.setVisualFocus {
	case 0:
		a.setPrimaryFont.Focus()
	case 1:
		a.setSecondaryFont.Focus()
	case 2:
		a.setPrimaryColor.Focus()
	case 3:
		a.setSecondaryColor.Focus()
	}
}

func (a App) persistVisual() (tea.Model, tea.Cmd) {
	v, err := readBrandVisual(a.setPrimaryFont, a.setSecondaryFont, a.setPrimaryColor, a.setSecondaryColor)
	if err != nil {
		a.err = err.Error()
		return a, nil
	}
	a.cfg.BrandVisual = v
	if err := config.Save(a.cfg); err != nil {
		a.err = err.Error()
		return a, nil
	}
	a.err = ""
	a.setPage = setPageMenu
	return a, nil
}

func (a App) updateSettingsAssets(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			a.loadSettingsFromCfg()
			a.layoutSettings()
			a.setPage = setPageMenu
			a.err = ""
			return a, nil
		case "ctrl+s":
			return a.persistAssets()
		case "ctrl+l":
			a.setCustomLabel = !a.setCustomLabel
			if a.setCustomLabel {
				a.setAssetFocus = 1
				a.setAssetPath.Blur()
				a.setAssetLabel.Focus()
			} else {
				a.setAssetFocus = 0
				a.setAssetLabel.Blur()
				a.setAssetPath.Focus()
			}
			return a, textinput.Blink
		case "ctrl+d":
			if len(a.setAssets) > 0 {
				a.setAssets = a.setAssets[:len(a.setAssets)-1]
			}
			return a, nil
		case "tab":
			if a.setCustomLabel {
				if a.setAssetFocus == 0 {
					a.setAssetFocus = 1
					a.setAssetPath.Blur()
					a.setAssetLabel.Focus()
				} else {
					a.setAssetFocus = 0
					a.setAssetLabel.Blur()
					a.setAssetPath.Focus()
				}
				return a, textinput.Blink
			}
		case "up":
			if !a.setCustomLabel && a.setLabelCursor > 0 {
				a.setLabelCursor--
				return a, nil
			}
		case "down":
			if !a.setCustomLabel && a.setLabelCursor < len(suggestedLabels)-1 {
				a.setLabelCursor++
				return a, nil
			}
		case "enter":
			return a.addSettingsAsset()
		}
	}
	if a.setCustomLabel && a.setAssetFocus == 1 {
		a.setAssetLabel, cmd = a.setAssetLabel.Update(msg)
	} else {
		a.setAssetPath, cmd = a.setAssetPath.Update(msg)
	}
	return a, cmd
}

func (a App) addSettingsAsset() (tea.Model, tea.Cmd) {
	path := expandHome(strings.TrimSpace(a.setAssetPath.Value()))
	if path == "" {
		a.err = "indica o caminho da imagem"
		return a, nil
	}
	ext := strings.ToLower(filepath.Ext(path))
	if !allowedExt[ext] {
		a.err = "formato inválido — png, jpg ou svg"
		return a, nil
	}
	if _, err := os.Stat(path); err != nil {
		a.err = "ficheiro não encontrado"
		return a, nil
	}
	label := suggestedLabels[a.setLabelCursor]
	if a.setCustomLabel {
		label = strings.TrimSpace(a.setAssetLabel.Value())
		if label == "" {
			a.err = "define um label"
			return a, nil
		}
	}
	for _, x := range a.setAssets {
		if x.Path == path {
			a.err = "já está na lista"
			return a, nil
		}
	}
	a.setAssets = append(a.setAssets, config.Asset{Path: path, Label: label})
	a.setAssetPath.SetValue("")
	a.setAssetLabel.SetValue("")
	a.setCustomLabel = false
	a.setAssetFocus = 0
	a.setAssetPath.Focus()
	a.setAssetLabel.Blur()
	a.err = ""
	return a, textinput.Blink
}

func (a *App) blurSettings() {
	a.setSVG.Blur()
	a.setImage.Blur()
	a.setGeneral.Blur()
	a.setGuidePath.Blur()
	a.setGuideText.Blur()
	a.setAssetPath.Blur()
	a.setAssetLabel.Blur()
	a.blurVisualFields()
}

func (a App) persistProvider() (tea.Model, tea.Cmd) {
	if a.setProv == 1 {
		a.cfg.DefaultProvider = config.ProviderOpenAI
	} else {
		a.cfg.DefaultProvider = config.ProviderOpenRouter
	}
	if key, _ := config.APIKeyFor(a.cfg, a.cfg.DefaultProvider); key == "" {
		a.err = "esse provider não tem chave — escolhe o outro ou volta ao setup"
		return a, nil
	}
	if err := config.Save(a.cfg); err != nil {
		a.err = err.Error()
		return a, nil
	}
	a.sessionProv = a.cfg.DefaultProvider
	a.err = ""
	a.setPage = setPageMenu
	return a, nil
}

func (a App) updateSettingsReview(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			a.loadSettingsFromCfg()
			a.layoutSettings()
			a.setPage = setPageMenu
			a.err = ""
			return a, nil
		case "up", "k", "left":
			a.setReview = 0
		case "down", "j", "right":
			a.setReview = 1
		case "enter", "ctrl+s":
			return a.persistReview()
		}
	}
	return a, nil
}

func (a App) persistReview() (tea.Model, tea.Cmd) {
	a.cfg.EnableReview = a.setReview == 1
	if err := config.Save(a.cfg); err != nil {
		a.err = err.Error()
		return a, nil
	}
	a.err = ""
	a.setPage = setPageMenu
	return a, nil
}

func (a App) persistModels() (tea.Model, tea.Cmd) {
	svg := strings.TrimSpace(a.setSVG.Value())
	img := strings.TrimSpace(a.setImage.Value())
	gen := strings.TrimSpace(a.setGeneral.Value())
	if svg == "" || img == "" || gen == "" {
		a.err = "os 3 modelos são obrigatórios"
		return a, nil
	}
	a.cfg.Models.SVG = svg
	a.cfg.Models.Image = img
	a.cfg.Models.General = gen
	if err := config.Save(a.cfg); err != nil {
		a.err = err.Error()
		return a, nil
	}
	a.sessionModels = a.cfg.Models
	a.err = ""
	a.setPage = setPageMenu
	return a, nil
}

func (a App) persistGuide() (tea.Model, tea.Cmd) {
	path := expandHome(strings.TrimSpace(a.setGuidePath.Value()))
	content := a.setGuideText.Value()
	switch {
	case a.setGuideMode == 0 && path != "":
		data, err := os.ReadFile(path)
		if err != nil {
			a.err = fmt.Sprintf("não foi possível ler %s", path)
			return a, nil
		}
		a.cfg.BrandGuide.Path = path
		a.cfg.BrandGuide.Content = string(data)
	case strings.TrimSpace(content) != "":
		a.cfg.BrandGuide.Path = path
		a.cfg.BrandGuide.Content = content
	case path != "":
		data, err := os.ReadFile(path)
		if err != nil {
			a.err = fmt.Sprintf("não foi possível ler %s", path)
			return a, nil
		}
		a.cfg.BrandGuide.Path = path
		a.cfg.BrandGuide.Content = string(data)
	default:
		a.err = "o guia não pode ficar vazio"
		return a, nil
	}
	if err := config.Save(a.cfg); err != nil {
		a.err = err.Error()
		return a, nil
	}
	a.err = ""
	a.setPage = setPageMenu
	return a, nil
}

func (a App) persistAssets() (tea.Model, tea.Cmd) {
	a.cfg.Assets = append([]config.Asset(nil), a.setAssets...)
	if err := config.Save(a.cfg); err != nil {
		a.err = err.Error()
		return a, nil
	}
	a.err = ""
	a.setPage = setPageMenu
	return a, nil
}

func (a App) viewSettings() string {
	switch a.setPage {
	case setPageMenu:
		return a.viewSettingsMenu()
	case setPageProv:
		return a.viewSettingsProvider()
	case setPageModels:
		return a.viewSettingsModels()
	case setPageGuide:
		return a.viewSettingsGuide()
	case setPageVisual:
		return viewBrandVisualForm("Identidade visual", a.setVisualFocus, a.setPrimaryFont, a.setSecondaryFont, a.setPrimaryColor, a.setSecondaryColor)
	case setPageAssets:
		return a.viewSettingsAssets()
	case setPageReview:
		return a.viewSettingsReview()
	default:
		return a.viewSettingsMenu()
	}
}

func (a App) viewSettingsMenu() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Definições"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("escolhe uma secção · cada uma guarda à parte"))
	b.WriteString("\n\n")

	type item struct {
		title string
		hint  string
	}
	items := []item{
		{"Provider", config.ProviderLabel(a.cfg.DefaultProvider)},
		{"Modelos", fmt.Sprintf("SVG · IMG · Geral")},
		{"Guia da marca", a.guideSummary()},
		{"Identidade visual", a.cfg.BrandVisual.Summary()},
		{"Imagens de referência", fmt.Sprintf("%d ficheiro(s)", len(a.cfg.Assets))},
		{"Revisão automática", reviewSettingLabel(a.cfg.EnableReview)},
	}
	for i, it := range items {
		if i == a.setMenuCursor {
			b.WriteString(titleStyle.Render("▸ " + it.title))
			b.WriteString("\n  ")
			b.WriteString(mutedStyle.Render(truncateRunes(it.hint, a.contentWidth()-2)))
		} else {
			b.WriteString(mutedStyle.Render("  " + it.title))
			b.WriteString("\n  ")
			b.WriteString(mutedStyle.Render(truncateRunes(it.hint, a.contentWidth()-2)))
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

func (a App) guideSummary() string {
	if p := strings.TrimSpace(a.cfg.BrandGuide.Path); p != "" {
		return p
	}
	n := len([]rune(strings.TrimSpace(a.cfg.BrandGuide.Content)))
	if n == 0 {
		return "(vazio)"
	}
	return fmt.Sprintf("texto inline · %d caracteres", n)
}

func (a App) viewSettingsProvider() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Provider padrão"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("usado nas novas gerações"))
	b.WriteString("\n\n")
	options := []string{"OpenRouter", "OpenAI"}
	for i, opt := range options {
		if i == a.setProv {
			b.WriteString(titleStyle.Render("▸ " + opt))
		} else {
			b.WriteString(mutedStyle.Render("  " + opt))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (a App) viewSettingsReview() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Revisão automática"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("vision no PNG final · até 3 tentativas se ligada"))
	b.WriteString("\n\n")
	options := []string{
		"Desligada (padrão) — uma geração e entrega",
		"Ligada — revisor avalia e pode reiterar",
	}
	for i, opt := range options {
		if i == a.setReview {
			b.WriteString(titleStyle.Render("▸ " + opt))
		} else {
			b.WriteString(mutedStyle.Render("  " + opt))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("enter / ctrl+s guardar"))
	return b.String()
}

func reviewSettingLabel(on bool) string {
	if on {
		return "ligada · até 3 tentativas"
	}
	return "desligada"
}

func (a App) viewSettingsModels() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Modelos"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Tab troca o campo"))
	b.WriteString("\n\n")
	labels := []string{"SVG (estrutura)", "Imagem (fundo)", "Geral (revisão)"}
	inputs := []textinput.Model{a.setSVG, a.setImage, a.setGeneral}
	for i, label := range labels {
		if i == a.setModelFocus {
			b.WriteString(titleStyle.Render("▸ " + label))
		} else {
			b.WriteString(mutedStyle.Render("  " + label))
		}
		b.WriteString("\n")
		b.WriteString(inputs[i].View())
		b.WriteString("\n\n")
	}
	return b.String()
}

func (a App) viewSettingsGuide() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Guia da marca"))
	b.WriteString("\n")
	if a.setGuideMode == 0 {
		b.WriteString(okStyle.Render("ficheiro") + mutedStyle.Render(" | texto  ·  Ctrl+T alterna  ·  Ctrl+Y copia  ·  Ctrl+K limpa"))
		b.WriteString("\n\n")
		b.WriteString(a.setGuidePath.View())
		if p := strings.TrimSpace(a.setGuidePath.Value()); strings.HasPrefix(p, "~") {
			b.WriteString("\n")
			b.WriteString(mutedStyle.Render("→ " + expandHome(p)))
		}
		preview := strings.TrimSpace(a.setGuideText.Value())
		if preview != "" {
			b.WriteString("\n\n")
			b.WriteString(mutedStyle.Render("pré-visualização:"))
			b.WriteString("\n")
			b.WriteString(mutedStyle.Render(truncateRunes(preview, a.contentWidth()*3)))
		}
	} else {
		b.WriteString(mutedStyle.Render("ficheiro | ") + okStyle.Render("texto") + mutedStyle.Render("  ·  Ctrl+T alterna  ·  Ctrl+Y copia  ·  Ctrl+K limpa"))
		b.WriteString("\n\n")
		b.WriteString(a.setGuideText.View())
	}
	return b.String()
}

func (a App) viewSettingsAssets() string {
	var b strings.Builder
	w := a.contentWidth()
	b.WriteString(titleStyle.Render("Imagens de referência"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("png/jpg/svg · Enter adiciona · Ctrl+S guarda a lista"))
	b.WriteString("\n\n")
	b.WriteString(a.setAssetPath.View())
	if p := strings.TrimSpace(a.setAssetPath.Value()); strings.HasPrefix(p, "~") {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("→ " + expandHome(p)))
	}
	b.WriteString("\n\nLabel  ")
	if a.setCustomLabel {
		b.WriteString(a.setAssetLabel.View())
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("Ctrl+L volta aos labels sugeridos"))
	} else {
		b.WriteString(mutedStyle.Render("↑/↓ escolhe · Ctrl+L custom"))
		b.WriteString("\n")
		// wrap labels to width
		line := strings.Builder{}
		for i, l := range suggestedLabels {
			token := " " + l + " "
			if i == a.setLabelCursor {
				token = "[" + l + "]"
			}
			if line.Len()+len(token) > w && line.Len() > 0 {
				b.WriteString(line.String())
				b.WriteString("\n")
				line.Reset()
			}
			if i == a.setLabelCursor {
				line.WriteString(titleStyle.Render(token))
			} else {
				line.WriteString(mutedStyle.Render(token))
			}
			line.WriteString(" ")
		}
		b.WriteString(line.String())
	}
	b.WriteString("\n\n")
	maxList := max(3, a.contentHeight()-16)
	if len(a.setAssets) == 0 {
		b.WriteString(mutedStyle.Render("lista vazia"))
	} else {
		b.WriteString(okStyle.Render(fmt.Sprintf("%d na lista", len(a.setAssets))))
		b.WriteString(mutedStyle.Render("  ·  Ctrl+D remove a última"))
		b.WriteString("\n")
		start := 0
		if len(a.setAssets) > maxList {
			start = len(a.setAssets) - maxList
			b.WriteString(mutedStyle.Render(fmt.Sprintf("… %d anteriores ocultos", start)))
			b.WriteString("\n")
		}
		for _, x := range a.setAssets[start:] {
			b.WriteString("  • ")
			b.WriteString(titleStyle.Render("[" + x.Label + "]"))
			b.WriteString(" ")
			b.WriteString(mutedStyle.Render(truncateRunes(x.Path, w-len(x.Label)-8)))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (a App) settingsHelp() string {
	switch a.setPage {
	case setPageMenu:
		return "↑/↓ · enter abrir · esc voltar ao menu principal"
	case setPageProv:
		return "↑/↓ escolher · enter/ctrl+s guardar · esc cancelar"
	case setPageReview:
		return "↑/↓ escolher · enter/ctrl+s guardar · esc cancelar"
	case setPageModels:
		return "tab campo seguinte · ctrl+s guardar · esc cancelar"
	case setPageGuide:
		return "ctrl+t ficheiro/texto · ctrl+y copiar tudo · ctrl+k limpar · enter carregar .md · ctrl+s guardar · esc"
	case setPageVisual:
		return "tab campos · enter/ctrl+s guardar · esc cancelar"
	case setPageAssets:
		return "enter adicionar · ↑/↓ label · ctrl+l custom · ctrl+d remover · ctrl+s guardar · esc"
	default:
		return ""
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
