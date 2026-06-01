package protocol

import (
	"encoding/json"
	"strings"
)

type responsesToOpenAISSE struct {
	lastEvent string
	respID    string
	model     string
	doneSent  bool
	roleSent  bool
	buffer    strings.Builder
}

func (r *responsesToOpenAISSE) Convert(line string) []string {
	if line == "" {
		return nil
	}
	if strings.HasPrefix(line, "event: ") {
		r.lastEvent = strings.TrimPrefix(line, "event: ")
		return nil
	}
	if !strings.HasPrefix(line, "data: ") {
		return nil
	}

	payload := strings.TrimPrefix(line, "data: ")
	var chunk map[string]interface{}
	if json.Unmarshal([]byte(payload), &chunk) != nil {
		return nil
	}

	if evType, _ := chunk["type"].(string); evType != "" {
		r.lastEvent = evType
	}

	if responseObj, ok := chunk["response"].(map[string]interface{}); ok {
		if id, ok := responseObj["id"].(string); ok && id != "" {
			r.respID = id
		}
		if m, ok := responseObj["model"].(string); ok && m != "" {
			r.model = m
		}
	}

	switch r.lastEvent {
	case "response.output_text.delta":
		delta, _ := chunk["delta"].(string)
		if delta == "" {
			return nil
		}
		r.buffer.WriteString(delta)
		if !r.roleSent {
			r.roleSent = true
			return r.emitDeltaWithRole(delta)
		}
		return r.emitDelta(delta)

	case "response.completed":
		if r.doneSent {
			return nil
		}
		r.doneSent = true

		prompt := int64(0)
		completion := int64(0)
		if responseObj, ok := chunk["response"].(map[string]interface{}); ok {
			if usg, ok := responseObj["usage"].(map[string]interface{}); ok {
				if n, _ := usg["input_tokens"].(float64); n > 0 {
					prompt = int64(n)
				}
				if n, _ := usg["output_tokens"].(float64); n > 0 {
					completion = int64(n)
				}
			}
		}

		return r.emitFinish(prompt, completion)
	}

	return nil
}

func (r *responsesToOpenAISSE) Flush() []string {
	if r.doneSent {
		return nil
	}
	r.doneSent = true
	return []string{"data: [DONE]", ""}
}

func (r *responsesToOpenAISSE) emitDeltaWithRole(delta string) []string {
	out := map[string]interface{}{
		"id":     r.chunkID(),
		"object": "chat.completion.chunk",
		"model":  r.model,
		"choices": []interface{}{map[string]interface{}{
			"index": 0,
			"delta": map[string]interface{}{
				"role":    "assistant",
				"content": delta,
			},
			"finish_reason": nil,
		}},
	}
	b, _ := json.Marshal(out)
	return []string{"data: " + string(b), ""}
}

func (r *responsesToOpenAISSE) emitDelta(delta string) []string {
	out := map[string]interface{}{
		"id":     r.chunkID(),
		"object": "chat.completion.chunk",
		"model":  r.model,
		"choices": []interface{}{map[string]interface{}{
			"index":         0,
			"delta":         map[string]interface{}{"content": delta},
			"finish_reason": nil,
		}},
	}
	b, _ := json.Marshal(out)
	return []string{"data: " + string(b), ""}
}

func (r *responsesToOpenAISSE) emitFinish(prompt, completion int64) []string {
	out := map[string]interface{}{
		"id":     r.chunkID(),
		"object": "chat.completion.chunk",
		"model":  r.model,
		"choices": []interface{}{map[string]interface{}{
			"index":         0,
			"delta":         map[string]interface{}{},
			"finish_reason": "stop",
		}},
		"usage": map[string]interface{}{
			"prompt_tokens":     prompt,
			"completion_tokens": completion,
			"total_tokens":      prompt + completion,
		},
	}
	b, _ := json.Marshal(out)
	return []string{"data: " + string(b), "", "data: [DONE]", ""}
}

func (r *responsesToOpenAISSE) chunkID() string {
	if r.respID != "" {
		return r.respID
	}
	return "chatcmpl-" + newShortID()
}

// ─────────────────────────────────────────────
// OpenAI SSE → Claude SSE
// ─────────────────────────────────────────────
