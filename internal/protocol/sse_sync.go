package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type parsedSSEEvent struct {
	name string
	data string
	json map[string]interface{}
}

// ConvertSSEToSyncResponse converts an upstream SSE response body into the
// equivalent non-streaming response for the same protocol. It returns detected=false
// when the body is not SSE.
func ConvertSSEToSyncResponse(body []byte, sourceProtocol string) ([]byte, bool, error) {
	trimmed := bytes.TrimSpace(body)
	if !bytes.HasPrefix(trimmed, []byte("data:")) && !bytes.HasPrefix(trimmed, []byte("event:")) {
		return body, false, nil
	}

	events, err := parseSSEJSONEvents(body)
	if err != nil {
		return body, true, err
	}
	if len(events) == 0 {
		return body, true, fmt.Errorf("empty SSE response")
	}

	switch sourceProtocol {
	case ProtocolResponses:
		out, err := responsesSSEToSync(events)
		return out, true, err
	case ProtocolClaude:
		out, err := claudeSSEToSync(events)
		return out, true, err
	case ProtocolGemini:
		out, err := geminiSSEToSync(events)
		return out, true, err
	default:
		out, err := openAISSEToSync(events)
		return out, true, err
	}
}

func parseSSEJSONEvents(body []byte) ([]parsedSSEEvent, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var events []parsedSSEEvent
	var eventName string
	var dataParts []string

	flush := func() error {
		if len(dataParts) == 0 {
			eventName = ""
			return nil
		}
		data := strings.TrimSpace(strings.Join(dataParts, "\n"))
		dataParts = nil
		name := eventName
		eventName = ""
		if data == "" || data == "[DONE]" {
			return nil
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			return fmt.Errorf("parse SSE data as JSON: %w", err)
		}
		events = append(events, parsedSSEEvent{name: name, data: data, json: payload})
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		switch {
		case line == "":
			if err := flush(); err != nil {
				return nil, err
			}
		case strings.HasPrefix(line, ":"):
			continue
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data := strings.TrimPrefix(line, "data:")
			if strings.HasPrefix(data, " ") {
				data = data[1:]
			}
			dataParts = append(dataParts, data)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return events, nil
}

func openAISSEToSync(events []parsedSSEEvent) ([]byte, error) {
	var id string
	var model string
	var created interface{} = time.Now().Unix()
	var role = "assistant"
	var finishReason interface{} = "stop"
	var text strings.Builder
	var usage map[string]interface{}

	for _, ev := range events {
		chunk := ev.json
		if isSSEErrorPayload(ev) {
			return json.Marshal(chunk)
		}
		if object, _ := chunk["object"].(string); object == "chat.completion" {
			return json.Marshal(chunk)
		}
		if v, _ := chunk["id"].(string); v != "" {
			id = v
		}
		if v, _ := chunk["model"].(string); v != "" {
			model = v
		}
		if v, ok := chunk["created"]; ok {
			created = v
		}
		if usg, ok := chunk["usage"].(map[string]interface{}); ok {
			usage = usg
		}
		choices, _ := chunk["choices"].([]interface{})
		if len(choices) == 0 {
			continue
		}
		choice, _ := choices[0].(map[string]interface{})
		if choice == nil {
			continue
		}
		if _, ok := choice["message"].(map[string]interface{}); ok {
			return json.Marshal(chunk)
		}
		if fr, exists := choice["finish_reason"]; exists && fr != nil {
			finishReason = fr
		}
		if delta, ok := choice["delta"].(map[string]interface{}); ok {
			if v, _ := delta["role"].(string); v != "" {
				role = v
			}
			if v, _ := delta["content"].(string); v != "" {
				text.WriteString(v)
			}
		}
	}

	if id == "" {
		id = "chatcmpl-" + newShortID()
	}
	if usage == nil {
		usage = map[string]interface{}{"prompt_tokens": int64(0), "completion_tokens": int64(0), "total_tokens": int64(0)}
	}
	out := map[string]interface{}{
		"id":      id,
		"object":  "chat.completion",
		"created": created,
		"model":   model,
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0,
				"message": map[string]interface{}{
					"role":    role,
					"content": text.String(),
				},
				"finish_reason": finishReason,
			},
		},
		"usage": usage,
	}
	return json.Marshal(out)
}

func responsesSSEToSync(events []parsedSSEEvent) ([]byte, error) {
	var final map[string]interface{}
	var created map[string]interface{}
	var text strings.Builder
	var doneText string

	for _, ev := range events {
		chunk := ev.json
		if isSSEErrorPayload(ev) {
			return json.Marshal(chunk)
		}
		if object, _ := chunk["object"].(string); object == "response" {
			final = chunk
		}
		evType, _ := chunk["type"].(string)
		if evType == "" {
			evType = ev.name
		}
		if resp, ok := chunk["response"].(map[string]interface{}); ok {
			if evType == "response.completed" {
				final = resp
			} else if created == nil {
				created = resp
			}
		}
		switch evType {
		case "response.output_text.delta":
			if v, _ := chunk["delta"].(string); v != "" {
				text.WriteString(v)
			}
		case "response.output_text.done":
			if v, _ := chunk["text"].(string); v != "" {
				doneText = v
			}
		}
	}

	fullText := text.String()
	if doneText != "" {
		fullText = doneText
	}
	if final == nil {
		final = map[string]interface{}{}
		if created != nil {
			for k, v := range created {
				final[k] = v
			}
		}
	}
	if _, ok := final["id"].(string); !ok || final["id"] == "" {
		final["id"] = "resp_" + newShortID()
	}
	if _, ok := final["object"]; !ok {
		final["object"] = "response"
	}
	if _, ok := final["created_at"]; !ok {
		final["created_at"] = time.Now().Unix()
	}
	final["status"] = "completed"
	if _, ok := final["usage"]; !ok {
		final["usage"] = map[string]interface{}{"input_tokens": int64(0), "output_tokens": int64(0), "total_tokens": int64(0)}
	}
	if fullText != "" && !hasResponsesOutput(final) {
		final["output"] = []interface{}{
			map[string]interface{}{
				"id":     "msg_" + newShortID(),
				"type":   "message",
				"role":   "assistant",
				"status": "completed",
				"content": []interface{}{
					map[string]interface{}{"type": "output_text", "text": fullText},
				},
			},
		}
	}
	return json.Marshal(final)
}

func geminiSSEToSync(events []parsedSSEEvent) ([]byte, error) {
	var text strings.Builder
	var finishReason = "STOP"
	var usage map[string]interface{}

	for _, ev := range events {
		chunk := ev.json
		if isSSEErrorPayload(ev) {
			return json.Marshal(chunk)
		}
		if candidates, ok := chunk["candidates"].([]interface{}); ok && len(candidates) > 0 {
			if cand, ok := candidates[0].(map[string]interface{}); ok {
				if fr, _ := cand["finishReason"].(string); fr != "" && fr != "FINISH_REASON_UNSPECIFIED" {
					finishReason = fr
				}
				if content, ok := cand["content"].(map[string]interface{}); ok {
					if parts, ok := content["parts"].([]interface{}); ok {
						for _, p := range parts {
							pm, ok := p.(map[string]interface{})
							if !ok || isGeminiThoughtPart(pm) {
								continue
							}
							if v, _ := pm["text"].(string); v != "" {
								text.WriteString(v)
							}
						}
					}
				}
			}
		}
		if meta, ok := chunk["usageMetadata"].(map[string]interface{}); ok {
			usage = meta
		}
	}

	out := map[string]interface{}{
		"candidates": []interface{}{
			map[string]interface{}{
				"content": map[string]interface{}{
					"parts": []interface{}{map[string]interface{}{"text": text.String()}},
					"role":  "model",
				},
				"finishReason": finishReason,
				"index":        0,
			},
		},
	}
	if usage != nil {
		out["usageMetadata"] = usage
	}
	return json.Marshal(out)
}

func claudeSSEToSync(events []parsedSSEEvent) ([]byte, error) {
	var msg map[string]interface{}
	var text strings.Builder
	var stopReason = "end_turn"
	var usage map[string]interface{}

	for _, ev := range events {
		chunk := ev.json
		if isSSEErrorPayload(ev) {
			return json.Marshal(chunk)
		}
		if typ, _ := chunk["type"].(string); typ == "message" {
			return json.Marshal(chunk)
		}
		evType, _ := chunk["type"].(string)
		if evType == "" {
			evType = ev.name
		}
		switch evType {
		case "message_start":
			if m, ok := chunk["message"].(map[string]interface{}); ok {
				msg = m
				if usg, ok := m["usage"].(map[string]interface{}); ok {
					usage = usg
				}
			}
		case "content_block_delta":
			if delta, ok := chunk["delta"].(map[string]interface{}); ok {
				if v, _ := delta["text"].(string); v != "" {
					text.WriteString(v)
				}
			}
		case "message_delta":
			if delta, ok := chunk["delta"].(map[string]interface{}); ok {
				if v, _ := delta["stop_reason"].(string); v != "" {
					stopReason = v
				}
			}
			if usg, ok := chunk["usage"].(map[string]interface{}); ok {
				if usage == nil {
					usage = map[string]interface{}{}
				}
				for k, v := range usg {
					usage[k] = v
				}
			}
		}
	}

	if msg == nil {
		msg = map[string]interface{}{}
	}
	if _, ok := msg["id"].(string); !ok || msg["id"] == "" {
		msg["id"] = "msg_" + newShortID()
	}
	msg["type"] = "message"
	msg["role"] = "assistant"
	msg["content"] = []interface{}{map[string]interface{}{"type": "text", "text": text.String()}}
	msg["stop_reason"] = stopReason
	msg["stop_sequence"] = nil
	if usage == nil {
		usage = map[string]interface{}{"input_tokens": int64(0), "output_tokens": int64(0)}
	}
	msg["usage"] = usage
	return json.Marshal(msg)
}

func isSSEErrorPayload(ev parsedSSEEvent) bool {
	if ev.name == "error" {
		return true
	}
	if typ, _ := ev.json["type"].(string); typ == "error" || strings.HasSuffix(typ, ".failed") {
		return true
	}
	_, ok := ev.json["error"]
	return ok
}

func hasResponsesOutput(resp map[string]interface{}) bool {
	output, ok := resp["output"].([]interface{})
	return ok && len(output) > 0
}
