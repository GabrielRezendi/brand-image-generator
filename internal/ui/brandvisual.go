package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/GabrielRezendi/brand-image-generator/internal/config"
)

func colorSwatch(hex string) string {
	hex = config.NormalizeHex(hex, "#888888")
	block := lipgloss.NewStyle().
		Background(lipgloss.Color(hex)).
		Foreground(lipgloss.Color(hex)).
		Render("████")
	return block + " " + mutedStyle.Render(hex)
}

func newBrandInputs(v config.BrandVisual) (pf, sf, pc, sc textinput.Model) {
	pf = textinput.New()
	pf.Placeholder = "~/fonts/Primary.ttf  ou  Inter  ou  URL Google Fonts"
	pf.SetValue(v.PrimaryFont)
	pf.CharLimit = 512

	sf = textinput.New()
	sf.Placeholder = "~/fonts/Secondary.ttf  ou  Roboto  ou  URL Google Fonts"
	sf.SetValue(v.SecondaryFont)
	sf.CharLimit = 512

	pc = textinput.New()
	pc.Placeholder = "#111111"
	pc.SetValue(config.NormalizeHex(v.PrimaryColor, config.DefaultPrimaryColor))
	pc.CharLimit = 16

	sc = textinput.New()
	sc.Placeholder = "#666666"
	sc.SetValue(config.NormalizeHex(v.SecondaryColor, config.DefaultSecondaryColor))
	sc.CharLimit = 16
	return
}

func readBrandVisual(pf, sf, pc, sc textinput.Model) (config.BrandVisual, error) {
	primary := strings.TrimSpace(pc.Value())
	secondary := strings.TrimSpace(sc.Value())
	if primary != "" && !config.ValidHex(primary) {
		return config.BrandVisual{}, fmt.Errorf("cor primária inválida — usa #RGB ou #RRGGBB")
	}
	if secondary != "" && !config.ValidHex(secondary) {
		return config.BrandVisual{}, fmt.Errorf("cor secundária inválida — usa #RGB ou #RRGGBB")
	}
	return config.BrandVisual{
		PrimaryFont:    normalizeFontRef(pf.Value()),
		SecondaryFont:  normalizeFontRef(sf.Value()),
		PrimaryColor:   config.NormalizeHex(primary, config.DefaultPrimaryColor),
		SecondaryColor: config.NormalizeHex(secondary, config.DefaultSecondaryColor),
	}, nil
}

func normalizeFontRef(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return s
	}
	if strings.Contains(s, "/") || strings.HasPrefix(s, "~") {
		return expandHome(s)
	}
	return s
}

func viewBrandVisualForm(title string, focus int, pf, sf, pc, sc textinput.Model) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("fonte: caminho local (.ttf/.otf) · nome Google Fonts · ou link"))
	b.WriteString("\n\n")

	labels := []string{
		"Fonte primária (títulos)",
		"Fonte secundária (corpo)",
		"Cor primária",
		"Cor secundária",
	}
	inputs := []textinput.Model{pf, sf, pc, sc}
	for i, label := range labels {
		if i == focus {
			b.WriteString(titleStyle.Render("▸ " + label))
		} else {
			b.WriteString(mutedStyle.Render("  " + label))
		}
		b.WriteString("\n")
		b.WriteString(inputs[i].View())
		if i >= 2 {
			b.WriteString("\n")
			b.WriteString("  " + colorSwatch(inputs[i].Value()))
		} else if p := strings.TrimSpace(inputs[i].Value()); strings.HasPrefix(p, "~") {
			b.WriteString("\n")
			b.WriteString(mutedStyle.Render("  → " + expandHome(p)))
		}
		b.WriteString("\n\n")
	}
	return b.String()
}
