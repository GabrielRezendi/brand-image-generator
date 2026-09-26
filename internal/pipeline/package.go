package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

)

var (
	hrefAttrRe = regexp.MustCompile(`(?i)(\b(?:href|xlink:href)\s*=\s*")([^"]+)(")`)
)

// MakeSelfContained copies every local image referenced by the SVG into
// attemptDir/elements/ and rewrites hrefs to relative paths under elements/.
func MakeSelfContained(svg, attemptDir string, extraSources map[string]string) (string, error) {
	elementsDir := filepath.Join(attemptDir, "elements")
	if err := os.MkdirAll(elementsDir, 0o755); err != nil {
		return "", err
	}

	usedNames := map[string]string{} // abs source -> elements-relative path
	var rewriteErr error

	out := hrefAttrRe.ReplaceAllStringFunc(svg, func(match string) string {
		if rewriteErr != nil {
			return match
		}
		parts := hrefAttrRe.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		href := parts[2]
		if href == "" || strings.HasPrefix(href, "data:") || strings.HasPrefix(href, "#") {
			return match
		}

		src, err := resolveHrefSource(href, attemptDir, extraSources)
		if err != nil || src == "" {
			// leave href as-is when already relative under elements/ and present
			return match
		}
		// if already inside this attempt's elements/, keep relative path
		absEl, _ := filepath.Abs(filepath.Join(attemptDir, "elements"))
		absSrc, _ := filepath.Abs(src)
		if strings.HasPrefix(absSrc, absEl+string(os.PathSeparator)) {
			rel := filepath.ToSlash(filepath.Join("elements", filepath.Base(absSrc)))
			usedNames[absSrc] = rel
			return parts[1] + rel + parts[3]
		}
		abs, err := filepath.Abs(src)
		if err != nil {
			rewriteErr = err
			return match
		}
		if rel, ok := usedNames[abs]; ok {
			return parts[1] + rel + parts[3]
		}

		base := sanitizeElementName(filepath.Base(abs))
		destName := uniqueName(elementsDir, base)
		destAbs := filepath.Join(elementsDir, destName)
		if err := copyFile(abs, destAbs); err != nil {
			rewriteErr = fmt.Errorf("copiar %s: %w", abs, err)
			return match
		}
		rel := filepath.ToSlash(filepath.Join("elements", destName))
		usedNames[abs] = rel
		return parts[1] + rel + parts[3]
	})
	if rewriteErr != nil {
		return "", rewriteErr
	}
	return out, nil
}

func resolveHrefSource(href, attemptDir string, extraSources map[string]string) (string, error) {
	href = strings.TrimSpace(href)
	if mapped, ok := extraSources[href]; ok && mapped != "" {
		return mapped, nil
	}
	// try basename lookup in extras
	base := filepath.Base(href)
	if mapped, ok := extraSources[base]; ok && mapped != "" {
		return mapped, nil
	}
	if filepath.IsAbs(href) {
		if _, err := os.Stat(href); err == nil {
			return href, nil
		}
		return "", fmt.Errorf("fonte não encontrada: %s", href)
	}
	candidates := []string{
		filepath.Join(attemptDir, href),
		filepath.Join(attemptDir, "elements", filepath.Base(href)),
		href,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("fonte não encontrada: %s", href)
}

func sanitizeElementName(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, " ", "-")
	if name == "" || name == "." || name == ".." {
		return "asset.bin"
	}
	return name
}

func uniqueName(dir, base string) string {
	dest := filepath.Join(dir, base)
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return base
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if _, err := os.Stat(filepath.Join(dir, candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d%s", stem, os.Getpid(), ext)
}

// CopyElementsDir copies elements/ from attempt into destDir/elements.
func CopyElementsDir(attemptDir, destDir string) error {
	if err := copyDirFiles(filepath.Join(attemptDir, "elements"), filepath.Join(destDir, "elements")); err != nil {
		return err
	}
	return copyDirFiles(filepath.Join(attemptDir, "fonts"), filepath.Join(destDir, "fonts"))
}

func copyDirFiles(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
