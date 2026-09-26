package brandfonts

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/GabrielRezendi/brand-image-generator/internal/config"
)

var (
	cssURLRe   = regexp.MustCompile(`(?i)url\(['"]?([^)'"]+)['"]?\)`)
	familyRe   = regexp.MustCompile(`(?i)family=([^&:]+)`)
	specimenRe = regexp.MustCompile(`(?i)fonts\.google\.com/specimen/([^/?#]+)`)
)

type Resolved struct {
	Family       string // BrandPrimary / BrandSecondary (alias used in SVG)
	NativeFamily string // real family name inside the font file (fontconfig)
	Source       string // original config value
	FilePath     string // absolute path to local font file
	FileName     string // basename copied into fonts/
	Format       string // truetype | opentype | woff | woff2
}

type Set struct {
	Primary   *Resolved
	Secondary *Resolved
	// FontConfig is absolute path to fonts.conf used by rsvg-convert (empty if no fonts).
	FontConfig string
}

func CacheDir() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	out := filepath.Join(dir, "fonts-cache")
	return out, os.MkdirAll(out, 0o700)
}

// Resolve prepares primary/secondary fonts into destDir/fonts/ and writes fonts.conf
// so librsvg (via fontconfig) can resolve BrandPrimary / BrandSecondary.
func Resolve(visual config.BrandVisual, destDir string) (Set, error) {
	fontsDir := filepath.Join(destDir, "fonts")
	if err := os.MkdirAll(fontsDir, 0o755); err != nil {
		return Set{}, err
	}
	var out Set
	var err error
	if strings.TrimSpace(visual.PrimaryFont) != "" {
		out.Primary, err = resolveOne(visual.PrimaryFont, config.FontFamilyPrimary, fontsDir, "primary")
		if err != nil {
			return Set{}, fmt.Errorf("fonte primária: %w", err)
		}
	}
	if strings.TrimSpace(visual.SecondaryFont) != "" {
		out.Secondary, err = resolveOne(visual.SecondaryFont, config.FontFamilySecondary, fontsDir, "secondary")
		if err != nil {
			return Set{}, fmt.Errorf("fonte secundária: %w", err)
		}
	}
	if out.Primary == nil && out.Secondary == nil {
		return out, nil
	}
	conf, err := writeFontsConf(fontsDir, out)
	if err != nil {
		return Set{}, err
	}
	out.FontConfig = conf
	return out, nil
}

func resolveOne(src, family, fontsDir, stem string) (*Resolved, error) {
	src = strings.TrimSpace(src)
	local, err := localFontPath(src)
	if err != nil {
		return nil, err
	}
	if local == "" {
		local, err = downloadGoogleFont(src)
		if err != nil {
			return nil, err
		}
	}
	ext := strings.ToLower(filepath.Ext(local))
	if ext == "" {
		ext = ".ttf"
	}
	name := stem + ext
	dest := filepath.Join(fontsDir, name)
	if err := copyFile(local, dest); err != nil {
		return nil, err
	}
	native, err := fontFamilyName(dest)
	if err != nil || native == "" {
		native = family // last resort: hope file registers under Brand*
	}
	return &Resolved{
		Family:       family,
		NativeFamily: native,
		Source:       src,
		FilePath:     dest,
		FileName:     name,
		Format:       fontFormat(ext),
	}, nil
}

func localFontPath(src string) (string, error) {
	lower := strings.ToLower(src)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return "", nil
	}
	ext := strings.ToLower(filepath.Ext(src))
	switch ext {
	case ".ttf", ".otf", ".woff", ".woff2":
		path := expandHome(src)
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("ficheiro não encontrado: %s", path)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		return abs, nil
	default:
		// bare family name → google fonts
		if looksLikeFamilyName(src) {
			return "", nil
		}
		// maybe path without known ext
		path := expandHome(src)
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			abs, err := filepath.Abs(path)
			return abs, err
		}
		return "", nil
	}
}

func looksLikeFamilyName(s string) bool {
	if strings.Contains(s, "/") || strings.Contains(s, "\\") {
		return false
	}
	if strings.HasPrefix(s, "~") {
		return false
	}
	return true
}

func downloadGoogleFont(src string) (string, error) {
	family := googleFamily(src)
	if family == "" {
		return "", fmt.Errorf("não foi possível identificar a família Google Fonts em %q", src)
	}
	cache, err := CacheDir()
	if err != nil {
		return "", err
	}
	safe := sanitize(family)

	// Prefer TTF/OTF — librsvg @font-face is unreliable; fontconfig+TTF is solid.
	for _, ext := range []string{".ttf", ".otf"} {
		p := filepath.Join(cache, safe+ext)
		if st, err := os.Stat(p); err == nil && st.Size() > 0 {
			return p, nil
		}
	}

	if path, err := downloadFontsourceTTF(family, cache, safe); err == nil {
		return path, nil
	}

	// Fall back to Google CSS; prefer ttf/otf/woff over woff2.
	cssURL := "https://fonts.googleapis.com/css2?family=" + url.QueryEscape(family) + ":wght@400;700&display=swap"
	css, err := httpGet(cssURL, "Mozilla/5.0 (compatible; MSIE 10.0; Windows NT 6.1; Trident/6.0)")
	if err != nil {
		return "", err
	}
	fontURL := firstFontURL(string(css))
	if fontURL == "" {
		css, err = httpGet(cssURL, "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		if err != nil {
			return "", err
		}
		fontURL = firstFontURL(string(css))
	}
	if fontURL == "" {
		return "", fmt.Errorf("não foi possível obter ficheiro de fonte para %s", family)
	}
	ext := strings.ToLower(filepath.Ext(strings.Split(fontURL, "?")[0]))
	if ext == "" {
		ext = ".ttf"
	}
	dest := filepath.Join(cache, safe+ext)
	raw, err := httpGet(fontURL, "")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, raw, 0o600); err != nil {
		return "", err
	}
	return dest, nil
}

// downloadFontsourceTTF fetches a static TTF from jsDelivr/fontsource.
func downloadFontsourceTTF(family, cache, safe string) (string, error) {
	id := sanitize(family)
	candidates := []string{
		fmt.Sprintf("https://cdn.jsdelivr.net/fontsource/fonts/%s@latest/latin-400-normal.ttf", id),
		fmt.Sprintf("https://cdn.jsdelivr.net/fontsource/fonts/%s@latest/latin-500-normal.ttf", id),
		fmt.Sprintf("https://cdn.jsdelivr.net/fontsource/fonts/%s@latest/latin-700-normal.ttf", id),
	}
	var lastErr error
	for _, u := range candidates {
		raw, err := httpGet(u, "")
		if err != nil {
			lastErr = err
			continue
		}
		if len(raw) < 1000 {
			lastErr = fmt.Errorf("resposta demasiado pequena")
			continue
		}
		dest := filepath.Join(cache, safe+".ttf")
		if err := os.WriteFile(dest, raw, 0o600); err != nil {
			return "", err
		}
		return dest, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("fontsource sem TTF para %s", family)
	}
	return "", lastErr
}

func googleFamily(src string) string {
	src = strings.TrimSpace(src)
	if m := specimenRe.FindStringSubmatch(src); len(m) == 2 {
		return strings.ReplaceAll(m[1], "+", " ")
	}
	if strings.Contains(strings.ToLower(src), "fonts.googleapis.com") {
		if m := familyRe.FindStringSubmatch(src); len(m) == 2 {
			name, _ := url.QueryUnescape(m[1])
			name = strings.ReplaceAll(name, "+", " ")
			if i := strings.IndexAny(name, ":|"); i >= 0 {
				name = name[:i]
			}
			return name
		}
	}
	if looksLikeFamilyName(src) && !strings.HasPrefix(strings.ToLower(src), "http") {
		return src
	}
	return ""
}

func firstFontURL(css string) string {
	matches := cssURLRe.FindAllStringSubmatch(css, -1)
	var woff, woff2 string
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		u := m[1]
		low := strings.ToLower(u)
		if strings.Contains(low, ".ttf") || strings.Contains(low, ".otf") {
			return u
		}
		if strings.Contains(low, ".woff") && !strings.Contains(low, ".woff2") && woff == "" {
			woff = u
		}
		if strings.Contains(low, ".woff2") && woff2 == "" {
			woff2 = u
		}
	}
	if woff != "" {
		return woff
	}
	return woff2
}

func httpGet(rawURL, ua string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d ao obter %s", res.StatusCode, rawURL)
	}
	return io.ReadAll(res.Body)
}

func fontFormat(ext string) string {
	switch strings.ToLower(ext) {
	case ".otf":
		return "opentype"
	case ".woff":
		return "woff"
	case ".woff2":
		return "woff2"
	default:
		return "truetype"
	}
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

func sanitize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return "font"
	}
	return out
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0o644)
}

// fontFamilyName reads the primary family name via fc-query (fontconfig).
func fontFamilyName(path string) (string, error) {
	bin, err := exec.LookPath("fc-query")
	if err != nil {
		return "", err
	}
	cmd := exec.Command(bin, "-f", "%{family[0]}", path)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(string(out))
	// fc-query may return comma-separated families
	if i := strings.IndexByte(name, ','); i >= 0 {
		name = strings.TrimSpace(name[:i])
	}
	return name, nil
}

func writeFontsConf(fontsDir string, set Set) (string, error) {
	absDir, err := filepath.Abs(fontsDir)
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(absDir, ".fc-cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "urn:fontconfig:fonts.dtd">
<fontconfig>
  <include ignore_missing="yes">/etc/fonts/fonts.conf</include>
`)
	fmt.Fprintf(&b, "  <dir>%s</dir>\n", absDir)
	fmt.Fprintf(&b, "  <cachedir>%s</cachedir>\n", cacheDir)

	addAlias := func(alias, native string) {
		if alias == "" || native == "" {
			return
		}
		fmt.Fprintf(&b, `  <match target="pattern">
    <test qual="any" name="family"><string>%s</string></test>
    <edit name="family" mode="assign" binding="strong"><string>%s</string></edit>
  </match>
`, xmlEscape(alias), xmlEscape(native))
	}
	if set.Primary != nil {
		addAlias(set.Primary.Family, set.Primary.NativeFamily)
	}
	if set.Secondary != nil {
		addAlias(set.Secondary.Family, set.Secondary.NativeFamily)
	}
	b.WriteString("</fontconfig>\n")

	confPath := filepath.Join(absDir, "fonts.conf")
	if err := os.WriteFile(confPath, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	// Warm fontconfig cache so rsvg sees the faces immediately.
	if bin, err := exec.LookPath("fc-cache"); err == nil {
		cmd := exec.Command(bin, "-f")
		cmd.Env = append(os.Environ(), "FONTCONFIG_FILE="+confPath)
		_ = cmd.Run()
	}
	return confPath, nil
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// CSSFace returns an @font-face block for a resolved font (browsers / SVG viewers).
func CSSFace(r *Resolved) string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf(`@font-face{font-family:'%s';src:url('fonts/%s') format('%s');font-weight:100 900;font-style:normal;font-display:swap;}`,
		r.Family, r.FileName, r.Format)
}

// PromptBlock describes brand visual rules for the LLM.
func PromptBlock(v config.BrandVisual) string {
	var b strings.Builder
	b.WriteString("Identidade visual obrigatória:\n")
	b.WriteString(fmt.Sprintf("- Cor primária: %s\n", config.NormalizeHex(v.PrimaryColor, config.DefaultPrimaryColor)))
	b.WriteString(fmt.Sprintf("- Cor secundária: %s\n", config.NormalizeHex(v.SecondaryColor, config.DefaultSecondaryColor)))
	if strings.TrimSpace(v.PrimaryFont) != "" {
		b.WriteString(fmt.Sprintf("- Fonte primária (títulos/destaques): font-family=\"%s\" (já disponível no SVG)\n", config.FontFamilyPrimary))
	}
	if strings.TrimSpace(v.SecondaryFont) != "" {
		b.WriteString(fmt.Sprintf("- Fonte secundária (corpo/apoio): font-family=\"%s\" (já disponível no SVG)\n", config.FontFamilySecondary))
	}
	b.WriteString("- Usa estas cores e famílias nos elementos <text>, fills e strokes relevantes.\n")
	b.WriteString("- Não inventes outras font-family; não uses Arial/Helvetica/sans-serif genéricos se as Brand* estiverem definidas.\n")
	return b.String()
}
