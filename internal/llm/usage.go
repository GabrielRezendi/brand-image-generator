package llm

import (
	"encoding/json"
	"math"
	"strings"
)

// Usage is token/cost accounting from a single API call.
type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	Estimated        bool    `json:"estimated,omitempty"`
	Model            string  `json:"model,omitempty"`
}

func (u Usage) Add(other Usage) Usage {
	out := Usage{
		PromptTokens:     u.PromptTokens + other.PromptTokens,
		CompletionTokens: u.CompletionTokens + other.CompletionTokens,
		TotalTokens:      u.TotalTokens + other.TotalTokens,
		CostUSD:          u.CostUSD + other.CostUSD,
		Estimated:        u.Estimated || other.Estimated,
	}
	if out.TotalTokens == 0 {
		out.TotalTokens = out.PromptTokens + out.CompletionTokens
	}
	return out
}

func (u *Usage) Normalize(model string) {
	u.Model = model
	if u.TotalTokens == 0 {
		u.TotalTokens = u.PromptTokens + u.CompletionTokens
	}
	if u.CostUSD > 0 {
		return
	}
	if u.TotalTokens == 0 && u.PromptTokens == 0 && u.CompletionTokens == 0 {
		return
	}
	if c, ok := estimateCostUSD(model, u.PromptTokens, u.CompletionTokens); ok {
		u.CostUSD = c
		u.Estimated = true
	}
}

type apiUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	Cost             float64 `json:"cost"`
}

func parseUsage(raw json.RawMessage, model string) Usage {
	if len(raw) == 0 {
		return Usage{Model: model}
	}
	var u apiUsage
	if err := json.Unmarshal(raw, &u); err != nil {
		return Usage{Model: model}
	}
	out := Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
		CostUSD:          u.Cost,
		Model:            model,
	}
	if out.PromptTokens == 0 && u.InputTokens > 0 {
		out.PromptTokens = u.InputTokens
	}
	if out.CompletionTokens == 0 && u.OutputTokens > 0 {
		out.CompletionTokens = u.OutputTokens
	}
	out.Normalize(model)
	return out
}

func estimateCostUSD(model string, prompt, completion int) (float64, bool) {
	in, out, ok := ratesPerMillion(model)
	if !ok {
		return 0, false
	}
	c := (float64(prompt)/1_000_000)*in + (float64(completion)/1_000_000)*out
	return math.Round(c*1_000_000) / 1_000_000, true
}

func ratesPerMillion(model string) (input, output float64, ok bool) {
	m := strings.ToLower(strings.TrimSpace(model))
	m = strings.TrimPrefix(m, "openai/")
	switch {
	case strings.Contains(m, "gpt-6-astra"):
		return 10, 50, true
	case strings.Contains(m, "gpt-image-2.5"):
		return 5, 30, true
	case strings.Contains(m, "gpt-5"), strings.Contains(m, "gpt-4.1"):
		return 2, 8, true
	case strings.Contains(m, "gpt-4o"):
		return 2.5, 10, true
	default:
		return 0, 0, false
	}
}
