package config

import (
	"strings"
	"testing"
)

func TestResolveModelAlias(t *testing.T) {
	cases := map[string]string{
		"flash":           "deepseek-flash",
		"PRO":             "deepseek-v4-pro",
		"deepseek-pro":    "deepseek-v4-pro",
		"deepseek-v4-pro": "deepseek-v4-pro",
		"chat":            "deepseek-chat",
		"r1":              "deepseek-reasoner",
		"custom-model":    "custom-model",
	}

	for input, expected := range cases {
		got := ResolveModelAlias(input)
		if got != expected {
			t.Errorf("ResolveModelAlias(%q) = %q; want %q", input, got, expected)
		}
	}
}

func TestMaskKey(t *testing.T) {
	raw := "sk-" + "1234567890abcdef1234567890abcdef"
	masked := MaskKey(raw)
	if strings.Contains(masked, "90abcdef12345678") {
		t.Fatalf("MaskKey leaked middle of secret: %s", masked)
	}
	if !strings.HasPrefix(masked, "sk-12345...") || !strings.HasSuffix(masked, "cdef") {
		t.Errorf("Unexpected mask format: %s", masked)
	}
	if MaskKey("") != "<não configurada>" {
		t.Errorf("Empty key should return <não configurada>")
	}
}
