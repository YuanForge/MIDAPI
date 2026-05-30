package handler

import (
	"strings"
	"testing"

	"fanapi/internal/protocol"
)

func TestShouldConvertRequestBodyResponsesToResponsesWithMessages(t *testing.T) {
	reqData := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hello"},
		},
	}

	if !shouldConvertRequestBody(protocolResponses, protocolResponses, reqData) {
		t.Fatal("expected conversion for responses->responses when top-level messages is non-empty")
	}
}

func TestShouldConvertRequestBodyResponsesToResponsesNativeInput(t *testing.T) {
	reqData := map[string]interface{}{
		"input": []interface{}{
			map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{"type": "input_text", "text": "hello"},
				},
			},
		},
	}

	if shouldConvertRequestBody(protocolResponses, protocolResponses, reqData) {
		t.Fatal("expected no conversion for native responses input without top-level messages")
	}
}

func TestShouldConvertRequestBodyResponsesNativeAssistantOutputTextPreserved(t *testing.T) {
	reqData := map[string]interface{}{
		"input": []interface{}{
			map[string]interface{}{
				"role": "assistant",
				"content": []interface{}{
					map[string]interface{}{"type": "output_text", "text": "你好"},
				},
			},
		},
	}

	if shouldConvertRequestBody(protocolResponses, protocolResponses, reqData) {
		t.Fatal("expected no conversion for native responses input")
	}

	input, _ := reqData["input"].([]interface{})
	item, _ := input[0].(map[string]interface{})
	content, _ := item["content"].([]interface{})
	part, _ := content[0].(map[string]interface{})
	if part["type"] != "output_text" {
		t.Fatalf("expected assistant output_text part preserved, got %#v", part["type"])
	}

	normalized, err := protocol.NormalizeClientRequest(reqData, protocolResponses)
	if err != nil {
		t.Fatalf("unexpected normalize error: %v", err)
	}
	roundTripped, err := protocol.ConvertRequest(normalized, protocolResponses)
	if err != nil {
		t.Fatalf("unexpected convert error: %v", err)
	}
	rtInput, _ := roundTripped["input"].([]interface{})
	rtItem, _ := rtInput[0].(map[string]interface{})
	if _, isString := rtItem["content"].(string); !isString {
		t.Fatalf("expected current round-trip to alter assistant content shape for regression context, got %#v", rtItem["content"])
	}
}

func TestResolveLLMTargetURLResponsesCompact(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "responses endpoint",
			in:   "https://api.openai.com/v1/responses",
			want: "https://api.openai.com/v1/responses/compact",
		},
		{
			name: "responses endpoint with query",
			in:   "https://api.example.com/v1/responses?api-version=2026-05-01",
			want: "https://api.example.com/v1/responses/compact?api-version=2026-05-01",
		},
		{
			name: "already compact",
			in:   "https://api.example.com/v1/responses/compact",
			want: "https://api.example.com/v1/responses/compact",
		},
		{
			name: "base v1 endpoint",
			in:   "https://api.example.com/v1",
			want: "https://api.example.com/v1/responses/compact",
		},
		{
			name: "chat completions endpoint",
			in:   "https://api.example.com/v1/chat/completions",
			want: "https://api.example.com/v1/responses/compact",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveLLMTargetURL(tc.in, "gpt-test", false, responsesOperationCompact)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestResolveLLMTargetURLResponsesCompactWithModelPlaceholder(t *testing.T) {
	got := resolveLLMTargetURL("https://api.example.com/v1/models/{model}/responses", "gpt-test", false, responsesOperationCompact)
	if got != "https://api.example.com/v1/models/gpt-test/responses/compact" {
		t.Fatalf("unexpected target URL: %q", got)
	}
}

func TestResolveLLMTargetURLGeminiStreamUnchanged(t *testing.T) {
	got := resolveLLMTargetURL("https://generativelanguage.googleapis.com/v1beta/models/{model}:{stream_action}", "gemini-test", true, "")
	if !strings.Contains(got, "/gemini-test:streamGenerateContent") || !strings.HasSuffix(got, "?alt=sse") {
		t.Fatalf("unexpected Gemini stream URL: %q", got)
	}
}
