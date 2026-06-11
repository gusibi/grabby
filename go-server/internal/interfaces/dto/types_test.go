package dto

import "testing"

type parseArgsTarget struct {
	URL      string `json:"url"`
	FullPage bool   `json:"fullPage"`
}

func TestParseArgsParsesTypedStruct(t *testing.T) {
	got, err := ParseArgs[parseArgsTarget](map[string]any{
		"url":      "https://example.com",
		"fullPage": true,
	})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if got.URL != "https://example.com" {
		t.Fatalf("ParseArgs() URL = %q, want %q", got.URL, "https://example.com")
	}
	if !got.FullPage {
		t.Fatal("ParseArgs() FullPage = false, want true")
	}
}

func TestParseArgsReturnsMarshalError(t *testing.T) {
	_, err := ParseArgs[parseArgsTarget](map[string]any{
		"url": make(chan int),
	})
	if err == nil {
		t.Fatal("ParseArgs() error = nil, want non-nil")
	}
}
