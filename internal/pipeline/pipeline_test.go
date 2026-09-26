package pipeline

import "testing"

func TestInjectBackgroundPlaceholder(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><image href="__BACKGROUND_IMAGE__" width="100" height="100"/></svg>`
	got := InjectBackground(svg, "background.png")
	if got != `<svg xmlns="http://www.w3.org/2000/svg"><image href="background.png" width="100" height="100"/></svg>` {
		t.Fatalf("got %s", got)
	}
}

func TestInjectBackgroundExistingHref(t *testing.T) {
	svg := `<svg><image xlink:href="old.png" /></svg>`
	got := InjectBackground(svg, "background.png")
	if !contains(got, `xlink:href="background.png"`) && !contains(got, `href="background.png"`) {
		t.Fatalf("got %s", got)
	}
}

func TestExtractSVG(t *testing.T) {
	in := "Aqui vai:\n```svg\n<svg><rect/></svg>\n```"
	got := ExtractSVG(in)
	if got != "<svg><rect/></svg>" {
		t.Fatalf("got %q", got)
	}
}

func TestParseReview(t *testing.T) {
	r := parseReview(`{"approved":false,"feedback":"logo pequena"}`)
	if r.Approved || r.Feedback != "logo pequena" {
		t.Fatalf("%+v", r)
	}
	r = parseReview(`claro {"approved": true, "feedback":"ok"} fim`)
	if !r.Approved {
		t.Fatalf("%+v", r)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
