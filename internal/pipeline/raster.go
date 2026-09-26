package pipeline

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// RasterizeSVG converts svgPath to pngPath using rsvg-convert (librsvg).
// Working directory is the SVG's directory so relative elements/ hrefs resolve.
// When fonts/fonts.conf exists beside the SVG, FONTCONFIG_FILE is set so Brand*
// aliases resolve (librsvg ignores @font-face for custom webfonts).
func RasterizeSVG(svgPath, pngPath string) error {
	bin, err := exec.LookPath("rsvg-convert")
	if err != nil {
		return fmt.Errorf("rsvg-convert não encontrado — instala librsvg2-bin")
	}
	svgPath, err = filepath.Abs(svgPath)
	if err != nil {
		return err
	}
	pngPath, err = filepath.Abs(pngPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(pngPath), 0o755); err != nil {
		return err
	}
	cmd := exec.Command(bin, "-f", "png", "-o", pngPath, svgPath)
	cmd.Dir = filepath.Dir(svgPath)
	env := os.Environ()
	if conf := fontConfigBeside(svgPath); conf != "" {
		env = append(env, "FONTCONFIG_FILE="+conf)
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rsvg-convert: %w (%s)", err, truncate(string(out), 300))
	}
	if _, err := os.Stat(pngPath); err != nil {
		return fmt.Errorf("PNG final não foi criado")
	}
	return nil
}

func fontConfigBeside(svgPath string) string {
	conf := filepath.Join(filepath.Dir(svgPath), "fonts", "fonts.conf")
	if st, err := os.Stat(conf); err == nil && st.Size() > 0 {
		abs, err := filepath.Abs(conf)
		if err == nil {
			return abs
		}
		return conf
	}
	return ""
}
