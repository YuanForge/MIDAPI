// Package protocol handles request/response format conversion between
// OpenAI, Claude (Anthropic), and Gemini (Google) API formats.
//
// Conversion matrix (input format -> channel protocol):
//   - OpenAI  -> openai     : pass-through (no-op)
//   - OpenAI  -> claude     : ConvertRequest / ConvertSyncResponse
//   - OpenAI  -> gemini     : ConvertRequest / ConvertSyncResponse
//   - OpenAI  -> responses  : ConvertRequest / ConvertSyncResponse
//
// All functions operate on plain map[string]interface{} so they compose
// cleanly with the existing request_script / response_script JS hooks.
package protocol

const (
	ProtocolOpenAI    = "openai"
	ProtocolClaude    = "claude"
	ProtocolGemini    = "gemini"
	ProtocolResponses = "responses" // OpenAI Responses API（Codex CLI 使用）
)

// ConvertRequest converts an OpenAI-format request map to the target protocol.
// Returns the same map unchanged when targetProtocol == "openai".
func ConvertRequest(req map[string]interface{}, targetProtocol string) (map[string]interface{}, error) {
	switch targetProtocol {
	case ProtocolClaude:
		return openAIToClaude(req)
	case ProtocolGemini:
		return openAIToGemini(req)
	case ProtocolResponses:
		return openAIToResponsesRequest(req)
	default:
		return req, nil
	}
}

// ConvertSyncResponse converts a sync (non-streaming) response body from the
// upstream protocol back to OpenAI format.
func ConvertSyncResponse(respBody []byte, sourceProtocol string) ([]byte, error) {
	switch sourceProtocol {
	case ProtocolClaude:
		return claudeToOpenAI(respBody)
	case ProtocolGemini:
		return geminiToOpenAI(respBody)
	case ProtocolResponses:
		return responsesToOpenAISync(respBody)
	default:
		return respBody, nil
	}
}

// NormalizeClientRequest converts a client's native-format request to OpenAI format.
// Used when clients send Claude or Gemini native format so the conversion pipeline
// always operates on a canonical OpenAI intermediate representation.
// Returns the same map unchanged when clientProto == "openai".
func NormalizeClientRequest(req map[string]interface{}, clientProto string) (map[string]interface{}, error) {
	switch clientProto {
	case ProtocolClaude:
		return claudeRequestToOpenAI(req)
	case ProtocolGemini:
		return geminiRequestToOpenAI(req)
	case ProtocolResponses:
		return responsesToOpenAI(req)
	default:
		return req, nil
	}
}

// ConvertResponseToClient converts an OpenAI-format sync response to the client's native format.
// Used after the upstream response has been normalised to OpenAI via ConvertSyncResponse.
// Returns the same bytes unchanged when clientProto == "openai".
func ConvertResponseToClient(respBytes []byte, clientProto string) ([]byte, error) {
	switch clientProto {
	case ProtocolClaude:
		return openAIToClaudeResponse(respBytes)
	case ProtocolGemini:
		return openAIToGeminiResponse(respBytes)
	case ProtocolResponses:
		return openAIToResponsesSync(respBytes)
	default:
		return respBytes, nil
	}
}
