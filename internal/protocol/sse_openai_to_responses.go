package protocol

import (
	"encoding/json"
	"strings"
)

type openAIToResponsesSSE struct {
	respID   string
	itemID   string
	model    string
	fullText string
	// 状态标记
	headerSent bool
	doneSent   bool
	// token 统计
	inputTokens     int64
	outputTokens    int64
	textOutputIndex int
	textStarted     bool
	textDone        bool
	nextOutputIndex int
	toolCalls       map[int]responsesToolCall
}

type responsesToolCall struct {
	outputIndex int
	itemID      string
	callID      string
	name        string
	arguments   string
	done        bool
}

func (r *openAIToResponsesSSE) Convert(line string) []string {
	if line == "" {
		return nil
	}
	if !strings.HasPrefix(line, "data: ") {
		return nil
	}
	payload := strings.TrimPrefix(line, "data: ")
	if payload == "[DONE]" {
		return nil // 在 Flush 中处理收尾事件
	}

	var chunk map[string]interface{}
	if json.Unmarshal([]byte(payload), &chunk) != nil {
		return nil
	}

	// 首个 chunk：提取 id 和 model，发送 header 事件
	var out []string
	if !r.headerSent {
		r.headerSent = true
		if id, ok := chunk["id"].(string); ok {
			r.respID = id
		} else {
			r.respID = "resp_" + newShortID()
		}
		if m, ok := chunk["model"].(string); ok {
			r.model = m
		}
		out = append(out, r.emitCreated()...)
	}

	// 收集 usage（最后一个 chunk 会携带）
	if usg, ok := chunk["usage"].(map[string]interface{}); ok {
		if n, _ := usg["prompt_tokens"].(float64); n > 0 {
			r.inputTokens = int64(n)
		}
		if n, _ := usg["completion_tokens"].(float64); n > 0 {
			r.outputTokens = int64(n)
		}
	}

	// 提取 delta 文本
	choices, _ := chunk["choices"].([]interface{})
	if len(choices) == 0 {
		return out
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return out
	}

	// delta 文本增量
	if delta, ok := choice["delta"].(map[string]interface{}); ok {
		if text, ok := delta["content"].(string); ok && text != "" {
			out = append(out, r.ensureTextOutput()...)
			r.fullText += text
			out = append(out, r.emitTextDelta(text)...)
		}
		if toolCalls, ok := delta["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
			out = append(out, r.toolCallDeltaLines(toolCalls)...)
		}
	}

	return out
}

func (r *openAIToResponsesSSE) Flush() []string {
	if r.doneSent {
		return nil
	}
	r.doneSent = true
	var out []string
	out = append(out, r.finishTextOutput()...)
	out = append(out, r.finishToolOutputs()...)
	out = append(out, r.emitCompleted()...)
	return out
}

func (r *openAIToResponsesSSE) emitCreated() []string {
	resp := map[string]interface{}{
		"type": "response.created",
		"response": map[string]interface{}{
			"id":     r.respID,
			"object": "response",
			"status": "in_progress",
			"model":  r.model,
			"output": []interface{}{},
		},
	}
	b, _ := json.Marshal(resp)
	return []string{"event: response.created", "data: " + string(b), ""}
}

func (r *openAIToResponsesSSE) ensureTextOutput() []string {
	if r.textStarted {
		return nil
	}
	r.textStarted = true
	r.itemID = "msg_" + newShortID()
	r.textOutputIndex = r.nextOutputIndex
	r.nextOutputIndex++
	out := r.emitOutputItemAdded()
	out = append(out, r.emitContentPartAdded()...)
	return out
}

func (r *openAIToResponsesSSE) emitOutputItemAdded() []string {
	item := map[string]interface{}{
		"type":         "response.output_item.added",
		"output_index": r.textOutputIndex,
		"item": map[string]interface{}{
			"id":      r.itemID,
			"type":    "message",
			"role":    "assistant",
			"content": []interface{}{},
			"status":  "in_progress",
		},
	}
	b, _ := json.Marshal(item)
	return []string{"event: response.output_item.added", "data: " + string(b), ""}
}

func (r *openAIToResponsesSSE) emitContentPartAdded() []string {
	ev := map[string]interface{}{
		"type":          "response.content_part.added",
		"item_id":       r.itemID,
		"output_index":  r.textOutputIndex,
		"content_index": 0,
		"part": map[string]interface{}{
			"type": "output_text",
			"text": "",
		},
	}
	b, _ := json.Marshal(ev)
	return []string{"event: response.content_part.added", "data: " + string(b), ""}
}

func (r *openAIToResponsesSSE) emitTextDelta(delta string) []string {
	ev := map[string]interface{}{
		"type":          "response.output_text.delta",
		"item_id":       r.itemID,
		"output_index":  r.textOutputIndex,
		"content_index": 0,
		"delta":         delta,
	}
	b, _ := json.Marshal(ev)
	return []string{"event: response.output_text.delta", "data: " + string(b), ""}
}

func (r *openAIToResponsesSSE) emitTextDone() []string {
	ev := map[string]interface{}{
		"type":          "response.output_text.done",
		"item_id":       r.itemID,
		"output_index":  r.textOutputIndex,
		"content_index": 0,
		"text":          r.fullText,
	}
	b, _ := json.Marshal(ev)
	return []string{"event: response.output_text.done", "data: " + string(b), ""}
}

func (r *openAIToResponsesSSE) emitContentPartDone() []string {
	ev := map[string]interface{}{
		"type":          "response.content_part.done",
		"item_id":       r.itemID,
		"output_index":  r.textOutputIndex,
		"content_index": 0,
		"part": map[string]interface{}{
			"type": "output_text",
			"text": r.fullText,
		},
	}
	b, _ := json.Marshal(ev)
	return []string{"event: response.content_part.done", "data: " + string(b), ""}
}

func (r *openAIToResponsesSSE) emitOutputItemDone() []string {
	ev := map[string]interface{}{
		"type":         "response.output_item.done",
		"output_index": r.textOutputIndex,
		"item": map[string]interface{}{
			"id":     r.itemID,
			"type":   "message",
			"role":   "assistant",
			"status": "completed",
			"content": []interface{}{
				map[string]interface{}{
					"type": "output_text",
					"text": r.fullText,
				},
			},
		},
	}
	b, _ := json.Marshal(ev)
	return []string{"event: response.output_item.done", "data: " + string(b), ""}
}

func (r *openAIToResponsesSSE) finishTextOutput() []string {
	if !r.textStarted || r.textDone {
		return nil
	}
	r.textDone = true
	var out []string
	out = append(out, r.emitTextDone()...)
	out = append(out, r.emitContentPartDone()...)
	out = append(out, r.emitOutputItemDone()...)
	return out
}

func (r *openAIToResponsesSSE) toolCallDeltaLines(toolCalls []interface{}) []string {
	var out []string
	for _, raw := range toolCalls {
		tc, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		idx := intFromJSON(tc["index"])
		fn, _ := tc["function"].(map[string]interface{})
		id, _ := tc["id"].(string)
		name, _ := fn["name"].(string)
		args, _ := fn["arguments"].(string)

		block, exists := r.openAIToolCall(idx)
		if id != "" {
			block.callID = id
		}
		if name != "" {
			block.name = name
		}
		if block.callID == "" {
			block.callID = "call_" + newShortID()
		}
		if !exists {
			out = append(out, r.emitToolOutputItemAdded(block)...)
		}
		if args != "" {
			block.arguments += args
			out = append(out, r.emitToolArgumentsDelta(block, args)...)
		}
		r.toolCalls[idx] = block
	}
	return out
}

func (r *openAIToResponsesSSE) openAIToolCall(index int) (responsesToolCall, bool) {
	if r.toolCalls == nil {
		r.toolCalls = make(map[int]responsesToolCall)
	}
	block, exists := r.toolCalls[index]
	if !exists {
		block = responsesToolCall{
			outputIndex: r.nextOutputIndex,
			itemID:      "fc_" + newShortID(),
		}
		r.nextOutputIndex++
	}
	return block, exists
}

func (r *openAIToResponsesSSE) emitToolOutputItemAdded(block responsesToolCall) []string {
	ev := map[string]interface{}{
		"type":         "response.output_item.added",
		"output_index": block.outputIndex,
		"item":         r.toolOutputItem(block, "in_progress"),
	}
	b, _ := json.Marshal(ev)
	return []string{"event: response.output_item.added", "data: " + string(b), ""}
}

func (r *openAIToResponsesSSE) emitToolArgumentsDelta(block responsesToolCall, delta string) []string {
	ev := map[string]interface{}{
		"type":         "response.function_call_arguments.delta",
		"item_id":      block.itemID,
		"output_index": block.outputIndex,
		"delta":        delta,
	}
	b, _ := json.Marshal(ev)
	return []string{"event: response.function_call_arguments.delta", "data: " + string(b), ""}
}

func (r *openAIToResponsesSSE) emitToolArgumentsDone(block responsesToolCall) []string {
	ev := map[string]interface{}{
		"type":         "response.function_call_arguments.done",
		"item_id":      block.itemID,
		"output_index": block.outputIndex,
		"arguments":    block.arguments,
	}
	b, _ := json.Marshal(ev)
	return []string{"event: response.function_call_arguments.done", "data: " + string(b), ""}
}

func (r *openAIToResponsesSSE) emitToolOutputItemDone(block responsesToolCall) []string {
	ev := map[string]interface{}{
		"type":         "response.output_item.done",
		"output_index": block.outputIndex,
		"item":         r.toolOutputItem(block, "completed"),
	}
	b, _ := json.Marshal(ev)
	return []string{"event: response.output_item.done", "data: " + string(b), ""}
}

func (r *openAIToResponsesSSE) finishToolOutputs() []string {
	if len(r.toolCalls) == 0 {
		return nil
	}
	var out []string
	for _, idx := range r.sortedToolCallIndexes() {
		block := r.toolCalls[idx]
		if block.done {
			continue
		}
		block.done = true
		out = append(out, r.emitToolArgumentsDone(block)...)
		out = append(out, r.emitToolOutputItemDone(block)...)
		r.toolCalls[idx] = block
	}
	return out
}

func (r *openAIToResponsesSSE) sortedToolCallIndexes() []int {
	indexes := make([]int, 0, len(r.toolCalls))
	for idx := range r.toolCalls {
		indexes = append(indexes, idx)
	}
	for i := 1; i < len(indexes); i++ {
		for j := i; j > 0 && r.toolCalls[indexes[j-1]].outputIndex > r.toolCalls[indexes[j]].outputIndex; j-- {
			indexes[j-1], indexes[j] = indexes[j], indexes[j-1]
		}
	}
	return indexes
}

func (r *openAIToResponsesSSE) toolOutputItem(block responsesToolCall, status string) map[string]interface{} {
	return map[string]interface{}{
		"id":        block.itemID,
		"type":      "function_call",
		"status":    status,
		"call_id":   block.callID,
		"name":      block.name,
		"arguments": block.arguments,
	}
}

func (r *openAIToResponsesSSE) completedOutput() []interface{} {
	output := make([]interface{}, 0)
	if r.textStarted {
		output = append(output, map[string]interface{}{
			"id":     r.itemID,
			"type":   "message",
			"role":   "assistant",
			"status": "completed",
			"content": []interface{}{
				map[string]interface{}{
					"type": "output_text",
					"text": r.fullText,
				},
			},
		})
	}
	for _, idx := range r.sortedToolCallIndexes() {
		output = append(output, r.toolOutputItem(r.toolCalls[idx], "completed"))
	}
	return output
}

func (r *openAIToResponsesSSE) emitCompleted() []string {
	usage := map[string]interface{}{
		"input_tokens":  r.inputTokens,
		"output_tokens": r.outputTokens,
		"total_tokens":  r.inputTokens + r.outputTokens,
	}
	resp := map[string]interface{}{
		"type": "response.completed",
		"response": map[string]interface{}{
			"id":     r.respID,
			"object": "response",
			"status": "completed",
			"model":  r.model,
			"output": r.completedOutput(),
			"usage":  usage,
		},
	}
	b, _ := json.Marshal(resp)
	return []string{"event: response.completed", "data: " + string(b), ""}
}
