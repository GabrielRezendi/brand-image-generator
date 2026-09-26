package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GabrielRezendi/brand-image-generator/internal/config"
	"github.com/GabrielRezendi/brand-image-generator/internal/ui"
	"github.com/GabrielRezendi/brand-image-generator/internal/version"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-v", "--version", "version":
			fmt.Printf("brand-image-generator %s\n", version.String())
			if version.Commit != "" && version.Commit != "none" {
				fmt.Printf("commit: %s\n", version.Commit)
			}
			if version.Date != "" && version.Date != "unknown" {
				fmt.Printf("built:  %s\n", version.Date)
			}
			return
		case "-h", "--help", "help":
			fmt.Print(`Brand Image Generator — TUI para artes de marca.

Uso:
  brand-image-generator
  brand-image-generator --version

Instalação (sem Go):
  curl -fsSL https://raw.githubusercontent.com/GabrielRezendi/brand-image-generator/main/scripts/install.sh | bash

Dependência de sistema: rsvg-convert (librsvg2-bin / librsvg)
`)
			return
		}
	}

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
