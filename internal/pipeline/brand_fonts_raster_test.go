package pipeline

import (
	"crypto/md5"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/joao/brand-image-generator/internal/brandfonts"
	"github.com/joao/brand-image-generator/internal/config"
)

func TestBrandFontsActuallyRender(t *testing.T) {
	if _, err := exec.LookPath("rsvg-convert"); err != nil {
		t.Skip("rsvg-convert missing")
	}
	dir := t.TempDir()
	set, err := brandfonts.Resolve(config.BrandVisual{
		PrimaryFont:   "Elms Sans",
		SecondaryFont: "Cormorant Garamond",
	}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if set.FontConfig == "" || set.Primary == nil || set.Secondary == nil {
		t.Fatalf("incomplete resolve: %+v", set)
	}
	t.Logf("primary native=%q file=%s", set.Primary.NativeFamily, set.Primary.FileName)
	t.Logf("secondary native=%q file=%s", set.Secondary.NativeFamily, set.Secondary.FileName)

	visual := config.BrandVisual{PrimaryColor: "#7B060C", SecondaryColor: "#333333", PrimaryFont: "x", SecondaryFont: "y"}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="700" height="200">
<style>text{font-family:Arial;font-size:48px}</style>
<rect width="100%" height="100%" fill="#fff"/>
<text x="24" y="80">Elms — títulos BrandPrimary</text>
<text class="secondary" x="24" y="150">Cormorant — corpo BrandSecondary</text>
</svg>`
	svg = ApplyBrandVisual(svg, visual, set)
	svg = EnsureBrandFonts(svg, set)
	svgPath := filepath.Join(dir, "final.svg")
	pngPath := filepath.Join(dir, "final.png")
	if err := os.WriteFile(svgPath, []byte(svg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RasterizeSVG(svgPath, pngPath); err != nil {
		t.Fatal(err)
	}

	// Default sans reference
	defDir := t.TempDir()
	defSVG := `<svg xmlns="http://www.w3.org/2000/svg" width="700" height="200">
<style>text{font-family:sans-serif;font-size:48px;fill:#7B060C}</style>
<rect width="100%" height="100%" fill="#fff"/>
<text x="24" y="80">Elms — títulos BrandPrimary</text>
<text x="24" y="150" fill="#333">Cormorant — corpo BrandSecondary</text>
</svg>`
	defPath := filepath.Join(defDir, "final.svg")
	defPNG := filepath.Join(defDir, "final.png")
	os.WriteFile(defPath, []byte(defSVG), 0o644)
	if err := RasterizeSVG(defPath, defPNG); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(pngPath)
	b, _ := os.ReadFile(defPNG)
	if fmt.Sprintf("%x", md5.Sum(a)) == fmt.Sprintf("%x", md5.Sum(b)) {
		t.Fatal("brand PNG identical to sans-serif — fonts not applied")
	}
}
