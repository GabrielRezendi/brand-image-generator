package output

import "testing"

func TestSlugify(t *testing.T) {
	got := Slugify("Campanha Verão 2026 — lançamento!")
	if got != "campanha-verao-2026-lancamento" {
		t.Fatalf("got %q", got)
	}
}
