package protocol

import (
	"strings"
	"testing"
)

func TestResponsesSSEToClaudeSSEViaChainedConverter(t *testing.T) {
	conv := NewSSEConverter(ProtocolResponses, ProtocolClaude)
	if conv == nil {
		t.Fatal("expected responses->claude SSE converter")
	}

	input := []string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_1","object":"response","created_at":1780104624,"status":"in_progress","model":"gpt-test","output":[]}}`,
		``,
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		``,
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","created_at":1780104624,"status":"completed","model":"gpt-test","usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`,
		``,
	}

	var got []string
	for _, line := range input {
		got = append(got, conv.Convert(line)...)
	}
	got = append(got, conv.Flush()...)
	joined := strings.Join(got, "\n")

	for _, want := range []string{
		"event: message_start",
		`"id":"resp_1"`,
		`"model":"gpt-test"`,
		"event: content_block_delta",
		`"text":"hello"`,
		"event: message_delta",
		`"output_tokens":2`,
		"event: message_stop",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected converted Claude SSE to contain %q, got:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "response.output_text.delta") {
		t.Fatalf("expected Responses SSE events to be converted away, got:\n%s", joined)
	}
}
