package handler

// ResponsesWSProxy 处理 GET /v1/responses 的 WebSocket 升级。
//
// OpenAI Responses API WebSocket 协议：
//   1. 客户端以 wss://.../v1/responses?model=xxx 发起连接。
//   2. 连接建立后，客户端发送 {"type":"response.create","response":{...}} 消息。
//   3. 服务端将上游 OpenAI Chat Completions SSE 流转换为 Responses API 事件，
//      以 WebSocket Text 消息（纯 JSON）逐条推送给客户端。
//   4. 一次连接可顺序发起多次 response.create。

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"fanapi/internal/billing"
	"fanapi/internal/db"
	"fanapi/internal/model"
	"fanapi/internal/protocol"
	"fanapi/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	// 生产环境应校验 Origin；此处允许所有来源（与现有 CORS 策略保持一致）
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ResponsesWSProxy 处理 GET /v1/responses — WebSocket 升级入口。
//
// @Summary      OpenAI Responses API（WebSocket 双向流）
// @Description  通过 WebSocket 连接使用 OpenAI Responses API。建立连接后发送 response.create 事件即可发起对话，服务端实时推送 Responses API 格式事件。
// @Tags         LLM
// @Security     ApiKeyAuth
// @Param        model  query  string  false  "默认模型名称（routing_model），可在 response.create 消息中覆盖"
// @Router       /v1/responses [get]
func ResponsesWSProxy(c *gin.Context) {
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ws-responses] upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// 从查询参数获取默认模型（客户端也可在每条 response.create 消息中覆盖）
	defaultModel := c.Query("model")

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			// 客户端正常断开或网络错误，退出循环
			break
		}

		var msg map[string]interface{}
		if jsonErr := json.Unmarshal(msgBytes, &msg); jsonErr != nil {
			sendWSResponseError(conn, "invalid_json", "消息格式错误")
			continue
		}

		msgType, _ := msg["type"].(string)
		switch msgType {
		case "response.create":
			// response.create 消息格式：
			// { "type": "response.create", "response": { "model": "...", "input": "..." } }
			// 或直接把请求字段放在顶层（兼容部分客户端）
			responseData, ok := msg["response"].(map[string]interface{})
			if !ok {
				// 兼容：顶层即为请求体
				responseData = make(map[string]interface{})
				for k, v := range msg {
					if k != "type" {
						responseData[k] = v
					}
				}
			}
			// 若消息中未指定 model，使用连接时的 query 参数
			if _, hasModel := responseData["model"]; !hasModel && defaultModel != "" {
				responseData["model"] = defaultModel
			}
			if handleErr := handleWSResponseCreate(c, conn, responseData); handleErr != nil {
				log.Printf("[ws-responses] response.create error: %v", handleErr)
				sendWSResponseError(conn, "server_error", handleErr.Error())
			}
		default:
			sendWSResponseError(conn, "unknown_event_type", "未知事件类型: "+msgType)
		}
	}
}

// handleWSResponseCreate 处理单条 response.create 请求。
// responseData 已是 Responses API 格式，此处执行：
//   - 请求格式转换（Responses → OpenAI Chat Completions）
//   - 渠道选择与计费
//   - 上游流式请求
//   - SSE → Responses API WS 事件推送
//   - 结算
func handleWSResponseCreate(c *gin.Context, conn *websocket.Conn, responseData map[string]interface{}) error {
	userID := c.MustGet("user_id").(int64)
	var apiKeyIDVal int64
	if apiKeyID, ok := c.Get("api_key_id"); ok && apiKeyID != nil {
		apiKeyIDVal, _ = apiKeyID.(int64)
	}
	var userGroup string
	if raw, ok := c.Get("user_group"); ok {
		userGroup, _ = raw.(string)
	}

	// 始终启用流式（WS 模式仅支持 stream）
	responseData["stream"] = true

	// Responses API → OpenAI Chat Completions
	openAIReq, convErr := protocol.NormalizeClientRequest(responseData, protocol.ProtocolResponses)
	if convErr != nil {
		return convErr
	}
	openAIReq["stream"] = true

	// 注入 stream_options include_usage（供计费）
	if _, hasOpts := openAIReq["stream_options"]; !hasOpts {
		openAIReq["stream_options"] = map[string]interface{}{"include_usage": true}
	} else if opts, ok := openAIReq["stream_options"].(map[string]interface{}); ok {
		opts["include_usage"] = true
	}

	// 余额前置检查
	if bal, balErr := billing.GetBalance(c.Request.Context(), userID); balErr == nil && bal <= 0 {
		return fmt.Errorf("余额不足，请充值后继续使用")
	}

	// 渠道选择
	routingKey, _ := openAIReq["model"].(string)
	if routingKey == "" {
		routingKey, _ = responseData["model"].(string)
	}
	if routingKey == "" {
		return fmt.Errorf("请在请求体 model 字段填写模型名称")
	}

	ch, chErr := service.SelectChannel(c.Request.Context(), routingKey)
	if chErr != nil {
		ch, chErr = service.GetChannelByName(c.Request.Context(), routingKey)
		if chErr != nil {
			return fmt.Errorf("渠道不存在: %s", routingKey)
		}
	}

	// 使用渠道配置的真实模型名
	if ch.Model != "" {
		openAIReq["model"] = ch.Model
	}
	resolvedModel, _ := openAIReq["model"].(string)
	proto := effectiveProtocol(ch)

	// 保存原始请求（用于计费估算）
	origReqData := make(map[string]interface{}, len(openAIReq))
	for k, v := range openAIReq {
		origReqData[k] = v
	}

	// 号池 Key 分配
	entityID := apiKeyIDVal
	if entityID == 0 {
		entityID = userID
	}
	var poolKey *model.PoolKey
	var poolKeyIDVal int64
	if ch.KeyPoolID > 0 {
		if pk, pkErr := service.GetOrAssignPoolKey(c.Request.Context(), ch.KeyPoolID, entityID); pkErr == nil {
			poolKey = pk
			poolKeyIDVal = pk.ID
		}
	}

	// 计费预扣
	inputHold, outputHold, calcErr := billing.CalcForUser(ch, origReqData, userGroup)
	if calcErr != nil {
		return calcErr
	}
	totalHold := inputHold + outputHold
	upstreamCostHold, _ := billing.CalcUpstreamCost(ch, origReqData)

	var modelCreditCharged int64
	var generalCreditCharged int64
	if totalHold > 0 {
		if routingKey != "" {
			modelCreditCharged, _ = billing.ChargeModelCredit(c.Request.Context(), userID, routingKey, totalHold)
		}
		generalCreditCharged = totalHold - modelCreditCharged
		if generalCreditCharged > 0 {
			if chargeErr := billing.Charge(c.Request.Context(), userID, generalCreditCharged); chargeErr != nil {
				if modelCreditCharged > 0 {
					_ = billing.RefundModelCredit(c.Request.Context(), userID, routingKey, modelCreditCharged)
				}
				return chargeErr
			}
		}
	}

	// refundHold 在错误路径下退还本次预扣
	refundHold := func(reason string) {
		if totalHold <= 0 {
			return
		}
		if generalCreditCharged > 0 {
			_ = billing.Refund(c.Request.Context(), userID, generalCreditCharged)
		}
		if modelCreditCharged > 0 {
			_ = billing.RefundModelCredit(c.Request.Context(), userID, routingKey, modelCreditCharged)
		}
	}

	corrID := uuid.New().String()
	if totalHold > 0 {
		_ = service.WriteTx(c.Request.Context(), userID, ch.ID, apiKeyIDVal, poolKeyIDVal, corrID, "hold", totalHold, upstreamCostHold, modelCreditCharged, model.JSON{
			"input_hold":  inputHold,
			"output_hold": outputHold,
			"user_group":  userGroup,
			"via":         "websocket",
		})
	}

	// LLM 日志
	llmLog := &model.LLMLog{
		UserID:          userID,
		ChannelID:       ch.ID,
		APIKeyID:        apiKeyIDVal,
		CorrID:          corrID,
		Model:           resolvedModel,
		IsStream:        true,
		UpstreamRequest: model.JSON(openAIReq),
		ClientRequest:   model.JSON(responseData),
		Status:          "pending",
	}
	_, _ = db.Engine.Insert(llmLog)

	// 发送上游请求（强制流式）
	_, resp, reqErr := sendLLMRequest(c, ch, openAIReq, poolKey, proto, resolvedModel, true)
	if reqErr != nil {
		service.RecordChannelError(c.Request.Context(), ch.ID)
		refundHold("upstream_error")
		if totalHold > 0 {
			_ = service.WriteTx(c.Request.Context(), userID, ch.ID, apiKeyIDVal, poolKeyIDVal, corrID, "refund", totalHold, upstreamCostHold, modelCreditCharged, model.JSON{"reason": "upstream_error"})
		}
		_, _ = db.Engine.Where("corr_id = ?", corrID).Cols("status", "error_msg").
			Update(&model.LLMLog{Status: "error", ErrorMsg: reqErr.Error()})
		return reqErr
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyErr, _ := io.ReadAll(resp.Body)
		service.RecordChannelError(c.Request.Context(), ch.ID)
		refundHold("upstream_error")
		if totalHold > 0 {
			_ = service.WriteTx(c.Request.Context(), userID, ch.ID, apiKeyIDVal, poolKeyIDVal, corrID, "refund", totalHold, upstreamCostHold, modelCreditCharged, model.JSON{"reason": "upstream_error"})
		}
		_, _ = db.Engine.Where("corr_id = ?", corrID).Cols("status", "upstream_status", "error_msg").
			Update(&model.LLMLog{Status: "error", UpstreamStatus: resp.StatusCode, ErrorMsg: string(bodyErr)})
		return fmt.Errorf("上游返回 %d: %s", resp.StatusCode, string(bodyErr))
	}

	service.RecordChannelSuccess(c.Request.Context(), ch.ID)

	// 流式 SSE → Responses API WS 事件
	usg := &usageState{protocol: proto}
	sseConv := protocol.NewSSEConverter(proto, protocol.ProtocolResponses)

	const maxSSELogBytes = 200 * 1024
	var rawSSELines []string
	var rawSSEBytes int

	scanner := bufio.NewScanner(resp.Body)
	wsError := false
	for scanner.Scan() {
		line := scanner.Text()
		usg.processLine(line)
		if rawSSEBytes < maxSSELogBytes {
			rawSSELines = append(rawSSELines, line)
			rawSSEBytes += len(line) + 1
		}

		var outLines []string
		if sseConv != nil {
			outLines = sseConv.Convert(line)
		} else {
			// 上游协议 == responses 时直接透传（不常见，保留兜底）
			outLines = []string{line}
		}
		for _, l := range outLines {
			if !strings.HasPrefix(l, "data: ") {
				continue
			}
			data := strings.TrimPrefix(l, "data: ")
			if data == "[DONE]" {
				continue
			}
			if writeErr := conn.WriteMessage(websocket.TextMessage, []byte(data)); writeErr != nil {
				wsError = true
				break
			}
		}
		if wsError {
			break
		}
	}

	// 冲刷 SSE 转换器末尾事件（response.completed 等）
	if !wsError && sseConv != nil {
		for _, l := range sseConv.Flush() {
			if !strings.HasPrefix(l, "data: ") {
				continue
			}
			data := strings.TrimPrefix(l, "data: ")
			if data == "[DONE]" {
				continue
			}
			_ = conn.WriteMessage(websocket.TextMessage, []byte(data))
		}
	}

	// 日志回写
	_, _ = db.Engine.Where("corr_id = ?", corrID).Cols("upstream_status", "upstream_response", "client_response").
		Update(&model.LLMLog{
			UpstreamStatus:   http.StatusOK,
			UpstreamResponse: model.JSON{"lines": rawSSELines},
			ClientResponse:   buildStreamClientResponse(rawSSELines, proto),
		})

	// 将预扣/退款状态写入 gin context 供 llmSettle 内部 llmRefundCredits 读取
	c.Set("model_credit_routing_key", routingKey)
	c.Set("model_credit_charged", modelCreditCharged)
	c.Set("model_credit_general_charged", generalCreditCharged)

	llmSettle(c, ch, origReqData, usg.normalized(origReqData), totalHold, userID, ch.ID, apiKeyIDVal, poolKeyIDVal, corrID, userGroup)
	return nil
}

// sendWSResponseError 向客户端发送 Responses API 格式错误事件。
func sendWSResponseError(conn *websocket.Conn, code, message string) {
	ev := map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	b, _ := json.Marshal(ev)
	_ = conn.WriteMessage(websocket.TextMessage, b)
}
