package protocol

import (
	"encoding/json"
	"strings"
)

type geminiToOpenAISSE struct {
	doneSent bool
}

func (g *geminiToOpenAISSE) Convert(line string) []string {
	if line == "" || !strings.HasPrefix(line, "data: ") {
		return nil
	}
	payload := strings.TrimPrefix(line, "data: ")

	var chunk map[string]interface{}
	if json.Unmarshal([]byte(payload), &chunk) != nil {
		return nil
	}

	var text string
	var finishReason interface{} = nil
	isFinish := false

	if candidates, ok := chunk["candidates"].([]interface{}); ok && len(candidates) > 0 {
		if cand, ok := candidates[0].(map[string]interface{}); ok {
			if contentObj, ok := cand["content"].(map[string]interface{}); ok {
				if parts, ok := contentObj["parts"].([]interface{}); ok {
					for _, p := range parts {
						if pm, ok := p.(map[string]interface{}); ok {
							// 跳过思考链 part（thought=true 或含 thoughtSignature 字段）
							if isGeminiThoughtPart(pm) {
								continue
							}
							if t, ok := pm["text"].(string); ok {
								text += t
							}
						}
					}
				}
			}
			if fr, ok := cand["finishReason"].(string); ok && fr != "" && fr != "FINISH_REASON_UNSPECIFIED" {
				isFinish = true
				if fr == "MAX_TOKENS" {
					finishReason = "length"
				} else {
					finishReason = "stop"
				}
			}
		}
	}

	deltaChunk := map[string]interface{}{
		"id":     "chatcmpl-gemini",
		"object": "chat.completion.chunk",
		"model":  "",
		"choices": []interface{}{map[string]interface{}{
			"index":         0,
			"delta":         map[string]interface{}{"content": text},
			"finish_reason": finishReason,
		}},
	}

	if meta, ok := chunk["usageMetadata"].(map[string]interface{}); ok {
		in, _ := meta["promptTokenCount"].(float64)
		out, _ := meta["candidatesTokenCount"].(float64)
		thoughts, _ := meta["thoughtsTokenCount"].(float64)
		deltaChunk["usage"] = map[string]interface{}{
			"prompt_tokens":     int64(in),
			"completion_tokens": int64(out + thoughts),
			"total_tokens":      int64(in + out + thoughts),
		}
	}

	b, _ := json.Marshal(deltaChunk)
	result := []string{"data: " + string(b), ""}

	if isFinish && !g.doneSent {
		g.doneSent = true
		result = append(result, "data: [DONE]", "")
	}

	if text == "" && !isFinish {
		return nil // 跳过没有内容且非结束的中间块（如纯 usageMetadata chunk）
	}

	return result
}

func (g *geminiToOpenAISSE) Flush() []string {
	if !g.doneSent {
		g.doneSent = true
		return []string{"data: [DONE]", ""}
	}
	return nil
}

// ─────────────────────────────────────────────
// Responses API SSE → OpenAI SSE
// ─────────────────────────────────────────────
