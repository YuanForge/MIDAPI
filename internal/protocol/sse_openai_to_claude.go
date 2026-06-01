package protocol

import (
	"encoding/json"
	"strings"
)

type openAIToClaudeSSE struct {
	msgID            string
	model            string
	inputTokens      int64
	outputTokens     int64
	sentStart        bool
	doneSent         bool
	nextBlockIndex   int
	activeBlockIndex int
	activeBlockKind  string
	stopReason       string
	toolBlocks       map[int]openAIToClaudeToolBlock
}

type openAIToClaudeToolBlock struct {
	blockIndex int
	id         string
	name       string
	args       string
}

func (o *openAIToClaudeSSE) Convert(line string) []string {
	if line == "" {
		return nil
	}
	if !strings.HasPrefix(line, "data: ") {
		return nil
	}
	payload := strings.TrimPrefix(line, "data: ")
	if payload == "[DONE]" {
		o.doneSent = true
		return o.stopEvents()
	}

	var chunk map[string]interface{}
	if json.Unmarshal([]byte(payload), &chunk) != nil {
		return nil
	}

	if o.msgID == "" {
		o.msgID, _ = chunk["id"].(string)
		o.model, _ = chunk["model"].(string)
	}
	if usg, ok := chunk["usage"].(map[string]interface{}); ok {
		if pt, _ := usg["prompt_tokens"].(float64); pt > 0 {
			o.inputTokens = int64(pt)
		}
		if ct, _ := usg["completion_tokens"].(float64); ct > 0 {
			o.outputTokens = int64(ct)
		}
	}

	choices, _ := chunk["choices"].([]interface{})
	if len(choices) == 0 {
		return nil
	}
	choice, _ := choices[0].(map[string]interface{})
	if choice == nil {
		return nil
	}

	var result []string

	if !o.sentStart {
		o.sentStart = true
		result = append(result, o.messageStartLines()...)
		result = append(result, o.pingLines()...)
	}

	delta, _ := choice["delta"].(map[string]interface{})
	if content, _ := delta["content"].(string); content != "" {
		result = append(result, o.ensureTextBlock()...)
		result = append(result, o.contentDeltaLines(content)...)
	}
	if toolCalls, ok := delta["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
		result = append(result, o.toolCallDeltaLines(toolCalls)...)
	}
	if finish, _ := choice["finish_reason"].(string); finish != "" {
		if finish == "tool_calls" {
			o.stopReason = "tool_use"
		} else if finish == "length" {
			o.stopReason = "max_tokens"
		} else {
			o.stopReason = "end_turn"
		}
	}

	return result
}

func (o *openAIToClaudeSSE) Flush() []string {
	if !o.doneSent {
		return o.stopEvents()
	}
	return nil
}

func (o *openAIToClaudeSSE) messageStartLines() []string {
	msg := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":    o.msgID,
			"type":  "message",
			"role":  "assistant",
			"model": o.model,
			"usage": map[string]interface{}{"input_tokens": o.inputTokens, "output_tokens": 0},
		},
	}
	b, _ := json.Marshal(msg)
	return []string{"event: message_start", "data: " + string(b), ""}
}

func (o *openAIToClaudeSSE) pingLines() []string {
	return []string{"event: ping", `data: {"type":"ping"}`, ""}
}

func (o *openAIToClaudeSSE) ensureTextBlock() []string {
	if o.activeBlockKind == "text" {
		return nil
	}
	var out []string
	out = append(out, o.closeActiveBlock()...)
	index := o.nextBlockIndex
	o.nextBlockIndex++
	o.activeBlockKind = "text"
	o.activeBlockIndex = index
	data := map[string]interface{}{
		"type":  "content_block_start",
		"index": index,
		"content_block": map[string]interface{}{
			"type": "text",
			"text": "",
		},
	}
	b, _ := json.Marshal(data)
	return append(out, "event: content_block_start", "data: "+string(b), "")
}

func (o *openAIToClaudeSSE) contentDeltaLines(text string) []string {
	data := map[string]interface{}{
		"type":  "content_block_delta",
		"index": o.activeBlockIndex,
		"delta": map[string]interface{}{"type": "text_delta", "text": text},
	}
	b, _ := json.Marshal(data)
	return []string{"event: content_block_delta", "data: " + string(b), ""}
}

func (o *openAIToClaudeSSE) toolCallDeltaLines(toolCalls []interface{}) []string {
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

		block, _ := o.openAIToolBlock(idx)
		if id != "" {
			block.id = id
		}
		if name != "" {
			block.name = name
		}
		if block.id == "" {
			block.id = "toolu_" + newShortID()
		}
		if args != "" {
			block.args += args
		}
		o.toolBlocks[idx] = block
	}
	return nil
}

func (o *openAIToClaudeSSE) openAIToolBlock(index int) (openAIToClaudeToolBlock, bool) {
	if o.toolBlocks == nil {
		o.toolBlocks = make(map[int]openAIToClaudeToolBlock)
	}
	block, exists := o.toolBlocks[index]
	if !exists {
		block = openAIToClaudeToolBlock{
			blockIndex: o.nextBlockIndex,
		}
		o.nextBlockIndex++
	}
	return block, exists
}

func (o *openAIToClaudeSSE) toolBlockStartLines(block openAIToClaudeToolBlock) []string {
	data := map[string]interface{}{
		"type":  "content_block_start",
		"index": block.blockIndex,
		"content_block": map[string]interface{}{
			"type":  "tool_use",
			"id":    block.id,
			"name":  block.name,
			"input": map[string]interface{}{},
		},
	}
	b, _ := json.Marshal(data)
	return []string{"event: content_block_start", "data: " + string(b), ""}
}

func (o *openAIToClaudeSSE) toolArgsDeltaLines(blockIndex int, args string) []string {
	data := map[string]interface{}{
		"type":  "content_block_delta",
		"index": blockIndex,
		"delta": map[string]interface{}{
			"type":         "input_json_delta",
			"partial_json": args,
		},
	}
	b, _ := json.Marshal(data)
	return []string{"event: content_block_delta", "data: " + string(b), ""}
}

func (o *openAIToClaudeSSE) closeActiveBlock() []string {
	if o.activeBlockKind == "" {
		return nil
	}
	data := map[string]interface{}{
		"type":  "content_block_stop",
		"index": o.activeBlockIndex,
	}
	b, _ := json.Marshal(data)
	o.activeBlockKind = ""
	return []string{"event: content_block_stop", "data: " + string(b), ""}
}

func (o *openAIToClaudeSSE) bufferedToolBlockLines() []string {
	if len(o.toolBlocks) == 0 {
		return nil
	}
	maxIndex := -1
	for idx := range o.toolBlocks {
		if idx > maxIndex {
			maxIndex = idx
		}
	}
	var out []string
	for idx := 0; idx <= maxIndex; idx++ {
		block, ok := o.toolBlocks[idx]
		if !ok {
			continue
		}
		out = append(out, o.toolBlockStartLines(block)...)
		if block.args != "" {
			out = append(out, o.toolArgsDeltaLines(block.blockIndex, block.args)...)
		}
		data := map[string]interface{}{
			"type":  "content_block_stop",
			"index": block.blockIndex,
		}
		b, _ := json.Marshal(data)
		out = append(out, "event: content_block_stop", "data: "+string(b), "")
	}
	return out
}

func (o *openAIToClaudeSSE) stopEvents() []string {
	var out []string
	if !o.sentStart {
		o.sentStart = true
		out = append(out, o.messageStartLines()...)
	}
	out = append(out, o.closeActiveBlock()...)
	out = append(out, o.bufferedToolBlockLines()...)

	stopReason := o.stopReason
	if stopReason == "" {
		stopReason = "end_turn"
	}
	outTok := o.outputTokens
	msgDelta := map[string]interface{}{
		"type":  "message_delta",
		"delta": map[string]interface{}{"stop_reason": stopReason, "stop_sequence": nil},
		"usage": map[string]interface{}{"output_tokens": outTok},
	}
	b, _ := json.Marshal(msgDelta)
	out = append(out,
		"event: message_delta",
		"data: "+string(b),
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	)
	return out
}

// ─────────────────────────────────────────────
// OpenAI SSE → Responses API SSE
//
// Codex CLI 使用 OpenAI Responses API（POST /v1/responses），
// 其 SSE 事件格式与 Chat Completions 完全不同。
// 此转换器将上游 OpenAI Chat Completions SSE 流转换为 Responses API SSE 事件。
//
// 事件顺序：
//   response.created → response.output_item.added → response.content_part.added
//   → (N×) response.output_text.delta
//   → response.output_text.done → response.content_part.done
//   → response.output_item.done → response.completed
// ─────────────────────────────────────────────
