package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

func openAIToGemini(req map[string]interface{}) (map[string]interface{}, error) {
	out := make(map[string]interface{})

	messages, _ := req["messages"].([]interface{})
	var systemParts []map[string]interface{}
	var contents []map[string]interface{}

	for _, m := range messages {
		msg, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		switch role {
		case "system":
			if c, ok := msg["content"].(string); ok {
				systemParts = append(systemParts, map[string]interface{}{"text": c})
			}
		case "user":
			contents = append(contents, map[string]interface{}{
				"role":  "user",
				"parts": contentToParts(msg["content"]),
			})
		case "assistant":
			contents = append(contents, map[string]interface{}{
				"role":  "model",
				"parts": contentToParts(msg["content"]),
			})
		case "tool":
			toolCallID, _ := msg["tool_call_id"].(string)
			content, _ := msg["content"].(string)
			contents = append(contents, map[string]interface{}{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"functionResponse": map[string]interface{}{
							"name":     toolCallID,
							"response": map[string]interface{}{"output": content},
						},
					},
				},
			})
		}
	}

	out["contents"] = contents

	if len(systemParts) > 0 {
		out["systemInstruction"] = map[string]interface{}{"parts": systemParts}
	}

	// generationConfig
	genCfg := map[string]interface{}{}
	if mt, ok := req["max_tokens"]; ok {
		genCfg["maxOutputTokens"] = mt
	} else if mc, ok := req["max_completion_tokens"]; ok {
		genCfg["maxOutputTokens"] = mc
	}
	if t, ok := req["temperature"]; ok {
		genCfg["temperature"] = t
	}
	if tp, ok := req["top_p"]; ok {
		genCfg["topP"] = tp
	}
	// stream is controlled via URL suffix for Gemini, not body field

	// response_modalities → generationConfig.responseModalities
	// 用于图片生成等需要 IMAGE 输出的场景（如 gemini-2.5-flash-image）
	if rm, ok := req["response_modalities"]; ok {
		genCfg["responseModalities"] = rm
	}

	if len(genCfg) > 0 {
		out["generationConfig"] = genCfg
	}

	// tools
	if tools, ok := req["tools"].([]interface{}); ok && len(tools) > 0 {
		out["tools"] = []map[string]interface{}{
			{"functionDeclarations": convertOpenAIToolsToGemini(tools)},
		}
	}

	return out, nil
}

func convertOpenAIToolsToGemini(tools []interface{}) []map[string]interface{} {
	var out []map[string]interface{}
	for _, t := range tools {
		tm, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		if tm["type"] != "function" {
			continue
		}
		fn, _ := tm["function"].(map[string]interface{})
		if fn == nil {
			continue
		}
		decl := map[string]interface{}{
			"name": fn["name"],
		}
		if desc, ok := fn["description"].(string); ok {
			decl["description"] = desc
		}
		if params, ok := fn["parameters"]; ok {
			decl["parameters"] = params
		}
		out = append(out, decl)
	}
	return out
}

func contentToParts(content interface{}) []map[string]interface{} {
	switch c := content.(type) {
	case string:
		return []map[string]interface{}{{"text": c}}
	case []interface{}:
		var parts []map[string]interface{}
		for _, item := range c {
			im, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			switch im["type"] {
			case "text":
				parts = append(parts, map[string]interface{}{"text": im["text"]})
			case "image_url":
				if iu, ok := im["image_url"].(map[string]interface{}); ok {
					if url, ok := iu["url"].(string); ok {
						if strings.HasPrefix(url, "data:") {
							// base64 inline
							parts = append(parts, map[string]interface{}{
								"inlineData": map[string]interface{}{
									"mimeType": extractMimeType(url),
									"data":     extractBase64Data(url),
								},
							})
						} else {
							parts = append(parts, map[string]interface{}{
								"fileData": map[string]interface{}{
									"mimeType": "image/jpeg",
									"fileUri":  url,
								},
							})
						}
					}
				}
			}
		}
		return parts
	}
	return []map[string]interface{}{{"text": fmt.Sprintf("%v", content)}}
}

// isGeminiThoughtPart 判断 Gemini part 是否为思考链内容（thought=true 或含 thoughtSignature 字段）。
func isGeminiThoughtPart(pm map[string]interface{}) bool {
	if thought, ok := pm["thought"].(bool); ok && thought {
		return true
	}
	if _, ok := pm["thoughtSignature"]; ok {
		return true
	}
	return false
}

func geminiToOpenAI(body []byte) ([]byte, error) {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return body, nil
	}

	finishReason := "stop"
	var content string
	var toolCalls []map[string]interface{}
	var inlineImages []map[string]interface{}

	candidates, _ := resp["candidates"].([]interface{})
	if len(candidates) > 0 {
		cand, _ := candidates[0].(map[string]interface{})
		if cand != nil {
			if fr, ok := cand["finishReason"].(string); ok {
				switch fr {
				case "MAX_TOKENS":
					finishReason = "length"
				case "STOP":
					finishReason = "stop"
				}
			}
			if contentObj, ok := cand["content"].(map[string]interface{}); ok {
				parts, _ := contentObj["parts"].([]interface{})
				for _, p := range parts {
					pm, ok := p.(map[string]interface{})
					if !ok {
						continue
					}
					// 跳过思考链 part（thought=true 或含 thoughtSignature 字段）
					if isGeminiThoughtPart(pm) {
						continue
					}
					if text, ok := pm["text"].(string); ok {
						content += text
					}
					if id, ok := pm["inlineData"].(map[string]interface{}); ok {
						mime, _ := id["mimeType"].(string)
						data, _ := id["data"].(string)
						if mime != "" && data != "" {
							inlineImages = append(inlineImages, map[string]interface{}{
								"type": "image_url",
								"image_url": map[string]interface{}{
									"url": "data:" + mime + ";base64," + data,
								},
							})
						}
					}
					if fc, ok := pm["functionCall"].(map[string]interface{}); ok {
						name, _ := fc["name"].(string)
						argsBytes, _ := json.Marshal(fc["args"])
						toolCalls = append(toolCalls, map[string]interface{}{
							"id":   "call_" + name,
							"type": "function",
							"function": map[string]interface{}{
								"name":      name,
								"arguments": string(argsBytes),
							},
						})
						finishReason = "tool_calls"
					}
				}
			}
		}
	}

	// 构建 message content：纯文本时用字符串，含图片时用 content array
	var messageContent interface{}
	if len(inlineImages) > 0 {
		var parts []map[string]interface{}
		if content != "" {
			parts = append(parts, map[string]interface{}{"type": "text", "text": content})
		}
		parts = append(parts, inlineImages...)
		messageContent = parts
	} else {
		messageContent = content
	}

	message := map[string]interface{}{"role": "assistant", "content": messageContent}
	if len(toolCalls) > 0 {
		message["content"] = nil
		message["tool_calls"] = toolCalls
	}
	choice := map[string]interface{}{
		"index":         0,
		"message":       message,
		"finish_reason": finishReason,
	}

	usage := map[string]interface{}{}
	if meta, ok := resp["usageMetadata"].(map[string]interface{}); ok {
		in, _ := meta["promptTokenCount"].(float64)
		out, _ := meta["candidatesTokenCount"].(float64)
		usage["prompt_tokens"] = int64(in)
		usage["completion_tokens"] = int64(out)
		usage["total_tokens"] = int64(in + out)
	}

	result := map[string]interface{}{
		"id":      "chatcmpl-gemini",
		"object":  "chat.completion",
		"model":   "",
		"choices": []interface{}{choice},
		"usage":   usage,
	}
	return json.Marshal(result)
}

// geminiRequestToOpenAI converts a Gemini generateContent request body to OpenAI format.
func geminiRequestToOpenAI(req map[string]interface{}) (map[string]interface{}, error) {
	out := make(map[string]interface{})

	if m, ok := req["model"].(string); ok {
		out["model"] = m
	}
	if s, ok := req["stream"]; ok {
		out["stream"] = s
	}

	var messages []interface{}

	// systemInstruction
	if si, ok := req["systemInstruction"].(map[string]interface{}); ok {
		if parts, ok := si["parts"].([]interface{}); ok {
			var sysText string
			for _, p := range parts {
				if pm, ok := p.(map[string]interface{}); ok {
					if t, ok := pm["text"].(string); ok {
						sysText += t
					}
				}
			}
			if sysText != "" {
				messages = append(messages, map[string]interface{}{
					"role":    "system",
					"content": sysText,
				})
			}
		}
	}

	// contents
	if contents, ok := req["contents"].([]interface{}); ok {
		for _, c := range contents {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			role, _ := cm["role"].(string)
			if role == "model" {
				role = "assistant"
			}

			parts, _ := cm["parts"].([]interface{})
			var text string
			var richParts []map[string]interface{}
			hasRich := false

			for _, p := range parts {
				pm, ok := p.(map[string]interface{})
				if !ok {
					continue
				}
				if t, ok := pm["text"].(string); ok {
					text += t
					richParts = append(richParts, map[string]interface{}{"type": "text", "text": t})
				} else if id, ok := pm["inlineData"].(map[string]interface{}); ok {
					hasRich = true
					mime, _ := id["mimeType"].(string)
					data, _ := id["data"].(string)
					richParts = append(richParts, map[string]interface{}{
						"type":      "image_url",
						"image_url": map[string]interface{}{"url": "data:" + mime + ";base64," + data},
					})
				} else if fd, ok := pm["fileData"].(map[string]interface{}); ok {
					hasRich = true
					uri, _ := fd["fileUri"].(string)
					richParts = append(richParts, map[string]interface{}{
						"type":      "image_url",
						"image_url": map[string]interface{}{"url": uri},
					})
				}
			}

			if hasRich {
				messages = append(messages, map[string]interface{}{"role": role, "content": richParts})
			} else {
				messages = append(messages, map[string]interface{}{"role": role, "content": text})
			}
		}
	}

	out["messages"] = messages

	// generationConfig
	if gc, ok := req["generationConfig"].(map[string]interface{}); ok {
		if mt, ok := gc["maxOutputTokens"]; ok {
			out["max_tokens"] = mt
		}
		if t, ok := gc["temperature"]; ok {
			out["temperature"] = t
		}
		if tp, ok := gc["topP"]; ok {
			out["top_p"] = tp
		}
	}

	return out, nil
}

// openAIToGeminiResponse converts an OpenAI sync response to Gemini generateContent format.
func openAIToGeminiResponse(body []byte) ([]byte, error) {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return body, nil
	}

	var content string
	finishReason := "STOP"

	if choices, ok := resp["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				content, _ = msg["content"].(string)
			}
			if fr, ok := choice["finish_reason"].(string); ok && fr == "length" {
				finishReason = "MAX_TOKENS"
			}
		}
	}

	usageMeta := map[string]interface{}{
		"promptTokenCount":     int64(0),
		"candidatesTokenCount": int64(0),
		"totalTokenCount":      int64(0),
	}
	if usg, ok := resp["usage"].(map[string]interface{}); ok {
		pt, _ := usg["prompt_tokens"].(float64)
		ct, _ := usg["completion_tokens"].(float64)
		usageMeta["promptTokenCount"] = int64(pt)
		usageMeta["candidatesTokenCount"] = int64(ct)
		usageMeta["totalTokenCount"] = int64(pt + ct)
	}

	out := map[string]interface{}{
		"candidates": []interface{}{
			map[string]interface{}{
				"content": map[string]interface{}{
					"parts": []interface{}{map[string]interface{}{"text": content}},
					"role":  "model",
				},
				"finishReason": finishReason,
				"index":        0,
			},
		},
		"usageMetadata": usageMeta,
	}
	return json.Marshal(out)
}
