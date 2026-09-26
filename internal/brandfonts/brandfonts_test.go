package brandfonts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GabrielRezendi/brand-image-generator/internal/config"
)

func TestGoogleFamily(t *testing.T) {
	cases := map[string]string{
		"Inter": "Inter",
		"https://fonts.google.com/specimen/Roboto":                    "Roboto",
		"https://fonts.googleapis.com/css2?family=Open+Sans:wght@400": "Open Sans",
	}
	for in, want := range cases {
		if got := googleFamily(in); got != want {
			t.Fatalf("%q → %q want %q", in, got, want)
		}
	}
}

func TestResolveWritesFontsConf(t *testing.T) {
	if _, err := exec.LookPath("fc-query"); err != nil {
		t.Skip("fc-query not available")
	}
	dir := t.TempDir()
	set, err := Resolve(config.BrandVisual{PrimaryFont: "Inter"}, dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if set.Primary == nil {
		t.Fatal("expected primary font")
	}
	if set.FontConfig == "" {
		t.Fatal("expected fonts.conf path")
	}
	raw, err := os.ReadFile(set.FontConfig)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, "BrandPrimary") {
		t.Fatalf("fonts.conf missing BrandPrimary alias: %s", body)
	}
	if set.Primary.NativeFamily == "" {
		t.Fatal("expected native family name")
	}
	if !strings.HasSuffix(strings.ToLower(set.Primary.FileName), ".ttf") &&
		!strings.HasSuffix(strings.ToLower(set.Primary.FileName), ".otf") {
		t.Fatalf("expected ttf/otf, got %s", set.Primary.FileName)
	}
	if _, err := os.Stat(filepath.Join(dir, "fonts", set.Primary.FileName)); err != nil {
		t.Fatal(err)
	}
}
