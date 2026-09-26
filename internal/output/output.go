package output

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// JobDir builds outputs/{DD}/{MM}/{YYYY}/{resumo}/ under baseDir (relative to cwd unless absolute).
func JobDir(baseDir, prompt string, now time.Time) (string, error) {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = "outputs"
	}
	if !filepath.IsAbs(baseDir) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(cwd, baseDir)
	}
	day := fmt.Sprintf("%02d", now.Day())
	month := fmt.Sprintf("%02d", int(now.Month()))
	year := fmt.Sprintf("%04d", now.Year())
	slug := Slugify(prompt)
	if slug == "" {
		slug = "pedido"
	}
	dir := filepath.Join(baseDir, day, month, year, slug)
	dir, err := uniqueDir(dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func AttemptDir(jobDir string, attempt int) (string, error) {
	dir := filepath.Join(jobDir, fmt.Sprintf("attempt-%d", attempt))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func uniqueDir(dir string) (string, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return dir, nil
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", dir, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("não foi possível criar pasta única para %s", dir)
}

func Slugify(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch r {
		case 'á', 'à', 'ã', 'â', 'ä':
			b.WriteByte('a')
		case 'é', 'ê', 'è', 'ë':
			b.WriteByte('e')
		case 'í', 'ì', 'î', 'ï':
			b.WriteByte('i')
		case 'ó', 'ô', 'õ', 'ò', 'ö':
			b.WriteByte('o')
		case 'ú', 'ù', 'û', 'ü':
			b.WriteByte('u')
		case 'ç':
			b.WriteByte('c')
		case 'ñ':
			b.WriteByte('n')
		default:
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' {
				b.WriteRune(r)
			} else {
				b.WriteByte(' ')
			}
		}
	}
	s = b.String()
	if len(s) > 80 {
		s = s[:80]
	}
	s = nonSlug.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	parts := strings.Split(s, "-")
	if len(parts) > 8 {
		parts = parts[:8]
	}
	return strings.Join(parts, "-")
}

func WriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
