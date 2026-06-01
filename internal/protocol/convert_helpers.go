package protocol

import (
	"strings"
)

func extractMimeType(dataURI string) string {
	if after, ok := strings.CutPrefix(dataURI, "data:"); ok {
		if idx := strings.Index(after, ";"); idx > 0 {
			return after[:idx]
		}
	}
	return "image/jpeg"
}

func extractBase64Data(dataURI string) string {
	if idx := strings.Index(dataURI, ","); idx >= 0 {
		return dataURI[idx+1:]
	}
	return ""
}

func convertOpenAIContentPartToClaude(part map[string]interface{}) map[string]interface{} {
	partType, _ := part["type"].(string)
	switch partType {
	case "text":
		return map[string]interface{}{
			"type": "text",
			"text": responsesStringValue(part["text"]),
		}
	case "image_url":
		var url string
		switch iv := part["image_url"].(type) {
		case map[string]interface{}:
			url, _ = iv["url"].(string)
		case string:
			url = iv
		}
		if strings.HasPrefix(url, "data:") {
			return map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"type":       "base64",
					"media_type": extractMimeType(url),
					"data":       extractBase64Data(url),
				},
			}
		}
		if url != "" {
			return map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"type": "url",
					"url":  url,
				},
			}
		}
	case "image":
		if _, ok := part["source"].(map[string]interface{}); ok {
			return part
		}
	}
	return part
}
