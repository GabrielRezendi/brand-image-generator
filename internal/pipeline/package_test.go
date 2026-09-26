package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakeSelfContained(t *testing.T) {
	dir := t.TempDir()
	logo := filepath.Join(dir, "logo.png")
	if err := os.WriteFile(logo, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	attempt := filepath.Join(dir, "attempt-1")
	if err := os.MkdirAll(filepath.Join(attempt, "elements"), 0o755); err != nil {
		t.Fatal(err)
	}
	bg := filepath.Join(attempt, "elements", "background.png")
	if err := os.WriteFile(bg, []byte("bg"), 0o644); err != nil {
		t.Fatal(err)
	}

	svg := `<svg xmlns="http://www.w3.org/2000/svg">
  <image href="elements/background.png"/>
  <image href="` + logo + `"/>
</svg>`

	out, err := MakeSelfContained(svg, attempt, map[string]string{logo: logo})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `href="elements/background.png"`) {
		t.Fatalf("background href: %s", out)
	}
	if !strings.Contains(out, `href="elements/logo.png"`) {
		t.Fatalf("logo href: %s", out)
	}
	if _, err := os.Stat(filepath.Join(attempt, "elements", "logo.png")); err != nil {
		t.Fatal(err)
	}
}

func TestRasterizeSVG(t *testing.T) {
	if _, err := os.Stat("/usr/bin/rsvg-convert"); err != nil {
		t.Skip("rsvg-convert missing")
	}
	dir := t.TempDir()
	svgPath := filepath.Join(dir, "final.svg")
	pngPath := filepath.Join(dir, "final.png")
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100">
  <rect width="100" height="100" fill="#2DD4BF"/>
  <text x="10" y="50" font-size="16" fill="#000">Hi</text>
</svg>`
	if err := os.WriteFile(svgPath, []byte(svg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RasterizeSVG(svgPath, pngPath); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(pngPath)
	if err != nil || info.Size() < 50 {
		t.Fatalf("bad png: %v %+v", err, info)
	}
}
