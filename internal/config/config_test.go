package config

import (
	"os"
	"testing"
)

func TestResolveSecretLiteral(t *testing.T) {
	got, err := ResolveSecret("sk-test-key")
	if err != nil {
		t.Fatal(err)
	}
	if got != "sk-test-key" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSecretEnv(t *testing.T) {
	t.Setenv("BIG_TEST_KEY", "from-env")
	got, err := ResolveSecret("${BIG_TEST_KEY}")
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-env" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSecretMissingEnv(t *testing.T) {
	_ = os.Unsetenv("BIG_MISSING_KEY")
	_, err := ResolveSecret("${BIG_MISSING_KEY}")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMaskSecret(t *testing.T) {
	if MaskSecret("${OPENAI_API_KEY}") != "${OPENAI_API_KEY}" {
		t.Fatal("env refs should stay visible")
	}
	masked := MaskSecret("sk-abcdefghijklmnop")
	if masked == "sk-abcdefghijklmnop" {
		t.Fatal("literal keys should be masked")
	}
}

func TestNormalizeHex(t *testing.T) {
	if got := NormalizeHex("#abc", DefaultPrimaryColor); got != "#AABBCC" {
		t.Fatalf("got %s", got)
	}
	if got := NormalizeHex("ff00aa", DefaultPrimaryColor); got != "#FF00AA" {
		t.Fatalf("got %s", got)
	}
	if got := NormalizeHex("nope", "#111111"); got != "#111111" {
		t.Fatalf("got %s", got)
	}
}

func TestValidHex(t *testing.T) {
	if !ValidHex("#fff") || !ValidHex("00ff00") {
		t.Fatal("expected valid")
	}
	if ValidHex("#gg0000") {
		t.Fatal("expected invalid")
	}
}

func TestDefaultReviewOff(t *testing.T) {
	if Default().EnableReview {
		t.Fatal("enable_review should default to false")
	}
}
