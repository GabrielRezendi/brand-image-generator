package ui

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"

	"github.com/joao/brand-image-generator/internal/config"
)

// copyAllText copies text to the system/terminal clipboard.
// Falls back to OSC 52 (remote terminals) and finally to a file.
func copyAllText(text string) (string, error) {
	text = strings.ReplaceAll(text, "\x00", "")
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("nada para copiar")
	}

	if err := clipboard.WriteAll(text); err == nil {
		return "área de transferência", nil
	}

	if err := writeOSC52(text); err == nil {
		return "clipboard do terminal (OSC 52)", nil
	}

	path, err := writeCopyFallbackFile(text)
	if err != nil {
		return "", fmt.Errorf("clipboard indisponível e falha ao gravar ficheiro: %w", err)
	}
	return "ficheiro " + path, nil
}

func writeOSC52(text string) error {
	// Limit to ~100KB payload; many terminals cap OSC 52 size.
	const maxBytes = 100_000
	raw := []byte(text)
	if len(raw) > maxBytes {
		return fmt.Errorf("texto demasiado grande para OSC 52")
	}
	b64 := base64.StdEncoding.EncodeToString(raw)
	seq := "\033]52;c;" + b64 + "\a"
	_, err := fmt.Fprint(os.Stdout, seq)
	return err
}

func writeCopyFallbackFile(text string) (string, error) {
	dir, err := config.Dir()
	if err != nil {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "guia-copiado.md")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return "", err
	}
	return path, nil
}
