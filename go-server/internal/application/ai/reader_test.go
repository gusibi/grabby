package ai

import (
	"strings"
	"testing"
)

func TestParseTranslationArrayShapes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"object", `{"translations":["a","b"]}`, []string{"a", "b"}},
		{"bare array", `["a","b"]`, []string{"a", "b"}},
		{"fenced", "```json\n{\"translations\":[\"a\",\"b\"]}\n```", []string{"a", "b"}},
		{"indexed", `{"0":"a","1":"b"}`, []string{"a", "b"}},
		{"short is padded", `{"translations":["a"]}`, []string{"a", ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTranslationArray(tc.raw, 2)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %q, want %q", got, tc.want)
				}
			}
		})
	}
}

func TestParseTranslationArrayRejectsEmpty(t *testing.T) {
	if _, err := parseTranslationArray("no json here", 2); err == nil {
		t.Fatal("expected an error for prose without a translations array")
	}
	if _, err := parseTranslationArray(`{"error":"rate limit"}`, 2); err == nil {
		t.Fatal("expected an error for a JSON object with no numeric or translations keys")
	}
}

func TestTranslationChunksRespectLimits(t *testing.T) {
	texts := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		texts = append(texts, "paragraph")
	}
	chunks := translationChunks(texts)
	if len(chunks) != 2 {
		t.Fatalf("chunks = %d, want 2 (8 rows per chunk)", len(chunks))
	}
	if len(chunks[0].indices) != 8 || len(chunks[1].indices) != 4 {
		t.Fatalf("chunk sizes = %d/%d", len(chunks[0].indices), len(chunks[1].indices))
	}
}

func TestTranslationChunksSkipEmptyText(t *testing.T) {
	chunks := translationChunks([]string{"a", "  ", "b"})
	if len(chunks) != 1 {
		t.Fatalf("chunks = %d, want 1", len(chunks))
	}
	if len(chunks[0].indices) != 2 || chunks[0].indices[0] != 0 || chunks[0].indices[1] != 2 {
		t.Fatalf("indices = %v, want [0 2]", chunks[0].indices)
	}
}

func TestBuildReaderChatPromptKeepsArticleAndTranscript(t *testing.T) {
	prompt := buildReaderChatPrompt("标题", "来源", "摘要", "正文内容", []ChatMessage{
		{Role: "user", Content: "请解读这篇文章"},
		{Role: "assistant", Content: "这是解读。"},
		{Role: "user", Content: "证据是什么？"},
	})
	for _, want := range []string{"标题", "正文内容", "请解读这篇文章", "这是解读。", "证据是什么？", "助手："} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt misses %q: %s", want, prompt)
		}
	}
}
