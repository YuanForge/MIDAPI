package protocol

import (
	"encoding/json"
	"strings"
)

type claudeToOpenAISSE struct {
	msgID            string
	model            string
	lastEvent        string
	inputTokens      int64
	sentRole         bool
	doneSent         bool
	nextToolIndex    int
	toolIndexByBlock map[int]int
}

func (c *claudeToOpenAISSE) Convert(line string) []string {
	if line == "" {
		return nil // skip Claude's blank event delimiters; we emit our own
	}
	if strings.HasPrefix(line, "event: ") {
		c.lastEvent = strings.TrimPrefix(line, "event: ")
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

	switch c.lastEvent {
	case "message_start":
		if msg, ok := chunk["message"].(map[string]interface{}); ok {
			c.msgID, _ = msg["id"].(string)
			c.model, _ = msg["model"].(string)
			if usg, ok := msg["usage"].(map[string]interface{}); ok {
				if n, _ := usg["input_tokens"].(float64); n > 0 {
					c.inputTokens = int64(n)
				}
			}
		}
		return c.emitRoleChunk()

	case "content_block_delta":
		if delta, ok := chunk["delta"].(map[string]interface{}); ok {
			if text, _ := delta["text"].(string); text != "" {
				return c.emitTextChunk(text)
			}
			if partial, _ := delta["partial_json"].(string); partial != "" {
				blockIndex := intFromJSON(chunk["index"])
				return c.emitToolArgsChunk(blockIndex, partial)
			}
		}

	case "content_block_start":
		if block, ok := chunk["content_block"].(map[string]interface{}); ok {
			if blockType, _ := block["type"].(string); blockType == "tool_use" {
				blockIndex := intFromJSON(chunk["index"])
				id, _ := block["id"].(string)
				name, _ := block["name"].(string)
				return c.emitToolStartChunk(blockIndex, id, name)
			}
		}

	case "message_delta":
		stopReason := "stop"
		var outputTokens int64
		if delta, ok := chunk["delta"].(map[string]interface{}); ok {
			if sr, _ := delta["stop_reason"].(string); sr != "" {
				switch sr {
				case "max_tokens":
					stopReason = "length"
				case "tool_use":
					stopReason = "tool_calls"
				}
			}
		}
		if usg, ok := chunk["usage"].(map[string]interface{}); ok {
			if n, _ := usg["output_tokens"].(float64); n > 0 {
				outputTokens = int64(n)
			}
		}
		return c.emitFinishChunk(stopReason, outputTokens)

	case "message_stop":
		if !c.doneSent {
			c.doneSent = true
			return []string{"data: [DONE]", ""}
		}
	}
	return nil
}

func (c *claudeToOpenAISSE) openAIToolIndex(blockIndex int) int {
	if c.toolIndexByBlock == nil {
		c.toolIndexByBlock = make(map[int]int)
	}
	if idx, ok := c.toolIndexByBlock[blockIndex]; ok {
		return idx
	}
	idx := c.nextToolIndex
	c.nextToolIndex++
	c.toolIndexByBlock[blockIndex] = idx
	return idx
}

func (c *claudeToOpenAISSE) Flush() []string {
	if !c.doneSent {
		c.doneSent = true
		return []string{"data: [DONE]", ""}
	}
	return nil
}

func (c *claudeToOpenAISSE) emitRoleChunk() []string {
	if c.sentRole {
		return nil
	}
	c.sentRole = true
	out := map[string]interface{}{
		"id":     c.msgID,
		"object": "chat.completion.chunk",
		"model":  c.model,
		"choices": []interface{}{map[string]interface{}{
			"index":         0,
			"delta":         map[string]interface{}{"role": "assistant", "content": ""},
			"finish_reason": nil,
		}},
	}
	b, _ := json.Marshal(out)
	return []string{"data: " + string(b), ""}
}

func (c *claudeToOpenAISSE) emitTextChunk(text string) []string {
	out := map[string]interface{}{
		"id":     c.msgID,
		"object": "chat.completion.chunk",
		"model":  c.model,
		"choices": []interface{}{map[string]interface{}{
			"index":         0,
			"delta":         map[string]interface{}{"content": text},
			"finish_reason": nil,
		}},
	}
	b, _ := json.Marshal(out)
	return []string{"data: " + string(b), ""}
}

func (c *claudeToOpenAISSE) emitToolStartChunk(blockIndex int, id, name string) []string {
	idx := c.openAIToolIndex(blockIndex)
	out := map[string]interface{}{
		"id":     c.msgID,
		"object": "chat.completion.chunk",
		"model":  c.model,
		"choices": []interface{}{map[string]interface{}{
			"index": 0,
			"delta": map[string]interface{}{
				"tool_calls": []interface{}{map[string]interface{}{
					"index": idx,
					"id":    id,
					"type":  "function",
					"function": map[string]interface{}{
						"name":      name,
						"arguments": "",
					},
				}},
			},
			"finish_reason": nil,
		}},
	}
	b, _ := json.Marshal(out)
	return []string{"data: " + string(b), ""}
}

func (c *claudeToOpenAISSE) emitToolArgsChunk(blockIndex int, partial string) []string {
	idx := c.openAIToolIndex(blockIndex)
	out := map[string]interface{}{
		"id":     c.msgID,
		"object": "chat.completion.chunk",
		"model":  c.model,
		"choices": []interface{}{map[string]interface{}{
			"index": 0,
			"delta": map[string]interface{}{
				"tool_calls": []interface{}{map[string]interface{}{
					"index": idx,
					"function": map[string]interface{}{
						"arguments": partial,
					},
				}},
			},
			"finish_reason": nil,
		}},
	}
	b, _ := json.Marshal(out)
	return []string{"data: " + string(b), ""}
}

func (c *claudeToOpenAISSE) emitFinishChunk(reason string, outputTokens int64) []string {
	out := map[string]interface{}{
		"id":     c.msgID,
		"object": "chat.completion.chunk",
		"model":  c.model,
		"choices": []interface{}{map[string]interface{}{
			"index":         0,
			"delta":         map[string]interface{}{},
			"finish_reason": reason,
		}},
		"usage": map[string]interface{}{
			"prompt_tokens":     c.inputTokens,
			"completion_tokens": outputTokens,
			"total_tokens":      c.inputTokens + outputTokens,
		},
	}
	b, _ := json.Marshal(out)
	return []string{"data: " + string(b), ""}
}

// ─────────────────────────────────────────────
// Gemini SSE → OpenAI SSE
// ─────────────────────────────────────────────
