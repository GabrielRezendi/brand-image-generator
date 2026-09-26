package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Provider string

const (
	ProviderOpenRouter Provider = "openrouter"
	ProviderOpenAI     Provider = "openai"

	DefaultSVGModel     = "openai/gpt-6-astra"
	DefaultImageModel   = "openai/gpt-image-2.5-flare"
	DefaultGeneralModel = "openai/gpt-6-astra"
)

var envRefPattern = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)

// Config is the persisted application setup.
type Config struct {
	OpenRouterAPIKey string      `yaml:"openrouter_api_key"`
	OpenAIAPIKey     string      `yaml:"openai_api_key"`
	DefaultProvider  Provider    `yaml:"default_provider"`
	Models           Models      `yaml:"models"`
	BrandGuide       BrandGuide  `yaml:"brand_guide"`
	BrandVisual      BrandVisual `yaml:"brand_visual"`
	Assets           []Asset     `yaml:"assets"`
	// EnableReview runs the vision review loop (up to 3 attempts). Default false.
	EnableReview bool   `yaml:"enable_review"`
	OutputDir    string `yaml:"output_dir,omitempty"`
}

type Models struct {
	SVG     string `yaml:"svg"`
	Image   string `yaml:"image"`
	General string `yaml:"general"`
}

type BrandGuide struct {
	Path    string `yaml:"path,omitempty"`
	Content string `yaml:"content,omitempty"`
}

// BrandVisual holds typography and palette used in SVG generation/raster.
type BrandVisual struct {
	PrimaryFont    string `yaml:"primary_font"`
	SecondaryFont  string `yaml:"secondary_font"`
	PrimaryColor   string `yaml:"primary_color"`
	SecondaryColor string `yaml:"secondary_color"`
}

type Asset struct {
	Path  string `yaml:"path"`
	Label string `yaml:"label"`
}

const (
	DefaultPrimaryColor   = "#111111"
	DefaultSecondaryColor = "#666666"
	FontFamilyPrimary     = "BrandPrimary"
	FontFamilySecondary   = "BrandSecondary"
)

func Default() Config {
	return Config{
		DefaultProvider: ProviderOpenRouter,
		Models: Models{
			SVG:     DefaultSVGModel,
			Image:   DefaultImageModel,
			General: DefaultGeneralModel,
		},
		BrandVisual: BrandVisual{
			PrimaryColor:   DefaultPrimaryColor,
			SecondaryColor: DefaultSecondaryColor,
		},
		Assets:    []Asset{},
		OutputDir: "outputs",
	}
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "brand-image-generator"), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Default(), err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return Default(), err
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Default(), err
	}
	normalize(&cfg)
	if strings.TrimSpace(cfg.BrandGuide.Content) == "" && strings.TrimSpace(cfg.BrandGuide.Path) != "" {
		if raw, err := os.ReadFile(cfg.BrandGuide.Path); err == nil {
			cfg.BrandGuide.Content = string(raw)
		}
	}
	return cfg, nil
}

func normalize(cfg *Config) {
	if cfg.Models.SVG == "" {
		cfg.Models.SVG = DefaultSVGModel
	}
	if cfg.Models.Image == "" {
		cfg.Models.Image = DefaultImageModel
	}
	if cfg.Models.General == "" {
		cfg.Models.General = DefaultGeneralModel
	}
	if cfg.Assets == nil {
		cfg.Assets = []Asset{}
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "outputs"
	}
	cfg.BrandVisual.PrimaryColor = NormalizeHex(cfg.BrandVisual.PrimaryColor, DefaultPrimaryColor)
	cfg.BrandVisual.SecondaryColor = NormalizeHex(cfg.BrandVisual.SecondaryColor, DefaultSecondaryColor)
	cfg.BrandVisual.PrimaryFont = strings.TrimSpace(cfg.BrandVisual.PrimaryFont)
	cfg.BrandVisual.SecondaryFont = strings.TrimSpace(cfg.BrandVisual.SecondaryFont)
	if cfg.DefaultProvider != ProviderOpenAI && cfg.DefaultProvider != ProviderOpenRouter {
		cfg.DefaultProvider = ProviderOpenRouter
	}
	if key, _ := APIKeyFor(*cfg, cfg.DefaultProvider); key == "" {
		if key, _ := APIKeyFor(*cfg, ProviderOpenRouter); key != "" {
			cfg.DefaultProvider = ProviderOpenRouter
		} else if key, _ := APIKeyFor(*cfg, ProviderOpenAI); key != "" {
			cfg.DefaultProvider = ProviderOpenAI
		}
	}
}

func Save(cfg Config) error {
	normalize(&cfg)
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path, err := Path()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ResolveSecret expands a literal key or ${ENV_VAR} reference.
func ResolveSecret(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if m := envRefPattern.FindStringSubmatch(value); m != nil {
		v, ok := os.LookupEnv(m[1])
		if !ok || strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("variável de ambiente %s não definida ou vazia", m[1])
		}
		return v, nil
	}
	return value, nil
}

func HasAnyAPIKey(cfg Config) bool {
	or, _ := ResolveSecret(cfg.OpenRouterAPIKey)
	oa, _ := ResolveSecret(cfg.OpenAIAPIKey)
	return or != "" || oa != ""
}

func APIKeyFor(cfg Config, provider Provider) (string, error) {
	switch provider {
	case ProviderOpenRouter:
		return ResolveSecret(cfg.OpenRouterAPIKey)
	case ProviderOpenAI:
		return ResolveSecret(cfg.OpenAIAPIKey)
	default:
		return "", fmt.Errorf("provider desconhecido: %s", provider)
	}
}

func IsReady(cfg Config) bool {
	if !HasAnyAPIKey(cfg) {
		return false
	}
	if strings.TrimSpace(cfg.BrandGuide.Content) == "" && strings.TrimSpace(cfg.BrandGuide.Path) == "" {
		return false
	}
	key, err := APIKeyFor(cfg, cfg.DefaultProvider)
	if err != nil || key == "" {
		// allow ready if the other provider has a key and default is wrong — still require a usable default
		return false
	}
	return cfg.Models.SVG != "" && cfg.Models.Image != "" && cfg.Models.General != ""
}

func MaskSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "(vazio)"
	}
	if envRefPattern.MatchString(value) {
		return value
	}
	if len(value) <= 8 {
		return strings.Repeat("•", len(value))
	}
	return value[:4] + strings.Repeat("•", len(value)-8) + value[len(value)-4:]
}

// NormalizeModelID adapts a model id to the selected provider.
func NormalizeModelID(model string, provider Provider) string {
	model = strings.TrimSpace(model)
	switch provider {
	case ProviderOpenAI:
		if strings.HasPrefix(model, "openai/") {
			return strings.TrimPrefix(model, "openai/")
		}
		return model
	case ProviderOpenRouter:
		if !strings.Contains(model, "/") {
			return "openai/" + model
		}
		return model
	default:
		return model
	}
}

func ProviderLabel(p Provider) string {
	switch p {
	case ProviderOpenAI:
		return "OpenAI"
	default:
		return "OpenRouter"
	}
}

var hexColorRe = regexp.MustCompile(`(?i)^#([0-9a-f]{3}|[0-9a-f]{6})$`)

// NormalizeHex returns a #RRGGBB color or the fallback.
func NormalizeHex(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if !strings.HasPrefix(value, "#") {
		value = "#" + value
	}
	if !hexColorRe.MatchString(value) {
		return fallback
	}
	if len(value) == 4 {
		r, g, b := value[1], value[2], value[3]
		return strings.ToUpper(fmt.Sprintf("#%c%c%c%c%c%c", r, r, g, g, b, b))
	}
	return strings.ToUpper(value)
}

// ValidHex reports whether value is a usable CSS hex color.
func ValidHex(value string) bool {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "#") {
		value = "#" + value
	}
	return hexColorRe.MatchString(value)
}

func (v BrandVisual) HasFonts() bool {
	return strings.TrimSpace(v.PrimaryFont) != "" || strings.TrimSpace(v.SecondaryFont) != ""
}

func (v BrandVisual) Summary() string {
	parts := []string{}
	if v.PrimaryFont != "" {
		parts = append(parts, "P:"+shortRef(v.PrimaryFont))
	}
	if v.SecondaryFont != "" {
		parts = append(parts, "S:"+shortRef(v.SecondaryFont))
	}
	parts = append(parts, v.PrimaryColor, v.SecondaryColor)
	return strings.Join(parts, " · ")
}

func shortRef(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 28 {
		return s
	}
	base := filepath.Base(s)
	if base != "" && base != "." && base != "/" {
		return base
	}
	return s[:25] + "…"
}
