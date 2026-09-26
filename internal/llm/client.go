package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GabrielRezendi/brand-image-generator/internal/config"
)

type Client struct {
	provider   config.Provider
	apiKey     string
	httpClient *http.Client
	baseURL    string
	onEvent    func(role, content string)
}

func New(provider config.Provider, apiKey string, onEvent func(role, content string)) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("chave de API vazia para %s", provider)
	}
	base := "https://openrouter.ai/api/v1"
	if provider == config.ProviderOpenAI {
		base = "https://api.openai.com/v1"
	}
	return &Client{
		provider: provider,
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
		baseURL: base,
		onEvent: onEvent,
	}, nil
}

func (c *Client) log(role, content string) {
	if c.onEvent != nil {
		c.onEvent(role, content)
	}
}

type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []Message     `json:"messages"`
	Usage    *usageInclude `json:"usage,omitempty"`
}

type usageInclude struct {
	Include bool `json:"include"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage json.RawMessage `json:"usage,omitempty"`
	Error *apiError       `json:"error,omitempty"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (c *Client) Chat(ctx context.Context, model string, messages []Message) (string, Usage, error) {
	model = config.NormalizeModelID(model, c.provider)
	c.log("user", summarizeMessages(messages))
	req := chatRequest{Model: model, Messages: messages}
	if c.provider == config.ProviderOpenRouter {
		req.Usage = &usageInclude{Include: true}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", Usage{}, err
	}
	var resp chatResponse
	if err := c.doJSON(ctx, http.MethodPost, c.baseURL+"/chat/completions", body, &resp); err != nil {
		return "", Usage{}, err
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return "", Usage{}, fmt.Errorf("API: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("resposta vazia do modelo %s", model)
	}
	out := strings.TrimSpace(resp.Choices[0].Message.Content)
	c.log("assistant", out)
	return out, parseUsage(resp.Usage, model), nil
}

type imageRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
}

type imageResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
		URL     string `json:"url"`
	} `json:"data"`
	Usage json.RawMessage `json:"usage,omitempty"`
	Error *apiError       `json:"error,omitempty"`
}

type orImageRequest struct {
	Model  string        `json:"model"`
	Prompt string        `json:"prompt"`
	N      int           `json:"n,omitempty"`
	Size   string        `json:"size,omitempty"`
	Usage  *usageInclude `json:"usage,omitempty"`
}

// GenerateImage returns raw PNG/JPEG bytes.
func (c *Client) GenerateImage(ctx context.Context, model, prompt string) ([]byte, Usage, error) {
	model = config.NormalizeModelID(model, c.provider)
	c.log("user", "[image] "+prompt)

	endpoint := c.baseURL + "/images/generations"
	if c.provider == config.ProviderOpenRouter {
		endpoint = c.baseURL + "/images"
	}

	var body []byte
	var err error
	if c.provider == config.ProviderOpenRouter {
		body, err = json.Marshal(orImageRequest{
			Model:  model,
			Prompt: prompt,
			N:      1,
			Size:   "1024x1024",
			Usage:  &usageInclude{Include: true},
		})
	} else {
		imgReq := imageRequest{
			Model:          model,
			Prompt:         prompt,
			N:              1,
			Size:           "1024x1024",
			ResponseFormat: "b64_json",
		}
		body, err = json.Marshal(imgReq)
	}
	if err != nil {
		return nil, Usage{}, err
	}

	var resp imageResponse
	if err := c.doJSON(ctx, http.MethodPost, endpoint, body, &resp); err != nil {
		return nil, Usage{}, err
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return nil, Usage{}, fmt.Errorf("API imagem: %s", resp.Error.Message)
	}
	if len(resp.Data) == 0 {
		return nil, Usage{}, fmt.Errorf("nenhuma imagem devolvida por %s", model)
	}
	usage := parseUsage(resp.Usage, model)
	item := resp.Data[0]
	if item.B64JSON != "" {
		raw, err := base64.StdEncoding.DecodeString(item.B64JSON)
		if err != nil {
			return nil, Usage{}, fmt.Errorf("decode b64 imagem: %w", err)
		}
		c.log("assistant", "[image] bytes recebidos (b64)")
		return raw, usage, nil
	}
	if item.URL != "" {
		raw, err := c.download(ctx, item.URL)
		if err != nil {
			return nil, Usage{}, err
		}
		c.log("assistant", "[image] bytes recebidos (url)")
		return raw, usage, nil
	}
	return nil, Usage{}, fmt.Errorf("imagem sem b64_json nem url")
}

func (c *Client) doJSON(ctx context.Context, method, url string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if c.provider == config.ProviderOpenRouter {
		req.Header.Set("HTTP-Referer", "https://github.com/GabrielRezendi/brand-image-generator")
		req.Header.Set("X-Title", "Brand Image Generator")
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", res.StatusCode, truncate(string(data), 500))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("json: %w (%s)", err, truncate(string(data), 200))
	}
	return nil
}

func (c *Client) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("download HTTP %d", res.StatusCode)
	}
	return io.ReadAll(res.Body)
}

func FileDataURL(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mime := "image/png"
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".svg":
		mime = "image/svg+xml"
	case ".webp":
		mime = "image/webp"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

func BytesDataURL(raw []byte, mime string) string {
	if mime == "" {
		mime = "image/png"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw)
}

func summarizeMessages(messages []Message) string {
	var b strings.Builder
	for _, m := range messages {
		b.WriteString("[")
		b.WriteString(m.Role)
		b.WriteString("] ")
		switch v := m.Content.(type) {
		case string:
			b.WriteString(truncate(v, 2000))
		default:
			raw, _ := json.Marshal(v)
			b.WriteString(truncate(string(raw), 2000))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
