package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/joao/brand-image-generator/internal/config"
	"github.com/joao/brand-image-generator/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "aviso: não foi possível carregar config existente: %v\n", err)
		cfg = config.Default()
	}

	p := tea.NewProgram(ui.NewApp(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}
