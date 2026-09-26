package llm

import "testing"

func TestParseUsageCost(t *testing.T) {
	u := parseUsage([]byte(`{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120,"cost":0.0123}`), "openai/gpt-6-astra")
	if u.PromptTokens != 100 || u.CompletionTokens != 20 || u.CostUSD != 0.0123 {
		t.Fatalf("%+v", u)
	}
	if u.Estimated {
		t.Fatal("API cost should not be estimated")
	}
}

func TestEstimateAstra(t *testing.T) {
	u := parseUsage([]byte(`{"prompt_tokens":1000000,"completion_tokens":1000000}`), "openai/gpt-6-astra")
	if !u.Estimated {
		t.Fatal("expected estimate")
	}
	if u.CostUSD != 60 {
		t.Fatalf("cost %v", u.CostUSD)
	}
}

func TestUsageAdd(t *testing.T) {
	a := Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, CostUSD: 0.1}
	b := a.Add(Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5, CostUSD: 0.05})
	if b.PromptTokens != 12 || b.CompletionTokens != 8 {
		t.Fatalf("%+v", b)
	}
	if b.CostUSD < 0.149 || b.CostUSD > 0.151 {
		t.Fatalf("cost %v", b.CostUSD)
	}
}
