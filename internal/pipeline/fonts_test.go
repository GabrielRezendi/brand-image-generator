package pipeline

import (
	"strings"
	"testing"

	"github.com/GabrielRezendi/brand-image-generator/internal/brandfonts"
	"github.com/GabrielRezendi/brand-image-generator/internal/config"
)

func TestApplyBrandVisual(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><style>text{font-family:Arial}</style><text>Hi</text></svg>`
	fonts := brandfonts.Set{
		Primary: &brandfonts.Resolved{Family: config.FontFamilyPrimary, FileName: "primary.ttf", Format: "truetype"},
	}
	out := ApplyBrandVisual(svg, config.BrandVisual{PrimaryColor: "#112233", SecondaryColor: "#aabbcc"}, fonts)
	if !strings.Contains(out, "@font-face") || !strings.Contains(out, "BrandPrimary") {
		t.Fatalf("missing font-face: %s", out)
	}
	if !strings.Contains(out, "#112233") {
		t.Fatalf("missing primary color: %s", out)
	}
	// Brand style must come after model styles so cascade wins.
	brandAt := strings.LastIndex(out, "BrandPrimary")
	closeAt := strings.LastIndex(out, "</svg>")
	if brandAt < 0 || closeAt < 0 || brandAt > closeAt {
		t.Fatalf("brand CSS should sit before </svg>: %s", out)
	}
}
