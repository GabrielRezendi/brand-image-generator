package pipeline

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/joao/brand-image-generator/internal/brandfonts"
	"github.com/joao/brand-image-generator/internal/config"
)

var (
	svgCloseRe   = regexp.MustCompile(`(?is)</svg\s*>`)
	svgOpenRe    = regexp.MustCompile(`(?is)<svg\b[^>]*>`)
	fontFamilyRe = regexp.MustCompile(`(?i)font-family\s*[:=]\s*["']?[^;"'\s>]+["']?`)
)

// ApplyBrandVisual injects @font-face and brand text defaults.
// The style block is placed just before </svg> so it wins the CSS cascade over
// model-generated rules in earlier <style> tags.
func ApplyBrandVisual(svg string, visual config.BrandVisual, fonts brandfonts.Set) string {
	css := brandfonts.CSSFace(fonts.Primary) + brandfonts.CSSFace(fonts.Secondary)
	root := fmt.Sprintf(
		":root{--brand-primary:%s;--brand-secondary:%s;}",
		config.NormalizeHex(visual.PrimaryColor, config.DefaultPrimaryColor),
		config.NormalizeHex(visual.SecondaryColor, config.DefaultSecondaryColor),
	)
	textDefaults := ""
	if fonts.Primary != nil || fonts.Secondary != nil {
		primary := config.FontFamilyPrimary
		secondary := config.FontFamilySecondary
		if fonts.Primary == nil {
			primary = secondary
		}
		if fonts.Secondary == nil {
			secondary = primary
		}
		textDefaults = fmt.Sprintf(
			`text,tspan{font-family:'%s','%s' !important;fill:var(--brand-primary);} text.secondary,tspan.secondary,.secondary{font-family:'%s' !important;fill:var(--brand-secondary);}`,
			primary, secondary, secondary,
		)
	} else {
		textDefaults = `text,tspan{fill:var(--brand-primary);} .secondary{fill:var(--brand-secondary);}`
	}
	block := "<style type=\"text/css\"><![CDATA[\n" + root + css + textDefaults + "\n]]></style>"

	if loc := svgCloseRe.FindStringIndex(svg); loc != nil {
		return svg[:loc[0]] + block + svg[loc[0]:]
	}
	if loc := svgOpenRe.FindStringIndex(svg); loc != nil {
		return svg[:loc[1]] + block + svg[loc[1]:]
	}
	return block + svg
}

// EnsureBrandFonts rewrites generic font-family on text to Brand* when configured.
func EnsureBrandFonts(svg string, fonts brandfonts.Set) string {
	if fonts.Primary == nil && fonts.Secondary == nil {
		return svg
	}
	primary := config.FontFamilyPrimary
	if fonts.Primary == nil {
		primary = config.FontFamilySecondary
	}
	return fontFamilyRe.ReplaceAllStringFunc(svg, func(m string) string {
		low := strings.ToLower(m)
		if strings.Contains(low, strings.ToLower(config.FontFamilyPrimary)) ||
			strings.Contains(low, strings.ToLower(config.FontFamilySecondary)) {
			return m
		}
		if strings.Contains(m, "=") {
			return `font-family="` + primary + `"`
		}
		return "font-family:" + primary
	})
}
