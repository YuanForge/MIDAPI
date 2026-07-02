package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"fanapi/internal/config"
	"fanapi/internal/db"
	"fanapi/internal/model"
)

const (
	resellerPlatformChannelSource = "reseller_platform"
	resellerChannelSyncInterval   = 5 * time.Minute
)

type ResellerPlatformChannelSyncResult struct {
	Upserted int
	Disabled int
}

type resellerPlatformChannelPayload struct {
	Channels []resellerPlatformChannel `json:"channels"`
}

type resellerPlatformChannel struct {
	ID            int64                  `json:"id"`
	Name          string                 `json:"name"`
	RoutingModel  string                 `json:"routing_model"`
	ModelProvider string                 `json:"model_provider"`
	Type          string                 `json:"type"`
	Protocol      string                 `json:"protocol"`
	BillingType   string                 `json:"billing_type"`
	BillingConfig map[string]interface{} `json:"billing_config"`
	IconURL       string                 `json:"icon_url"`
	Description   string                 `json:"description"`
}

func IsResellerSiteMode(cfg *config.Config) bool {
	return cfg != nil && strings.EqualFold(strings.TrimSpace(cfg.App.Mode), "reseller_site")
}

func StartResellerPlatformChannelSyncer(ctx context.Context, cfg *config.Config) {
	if !IsResellerSiteMode(cfg) || !cfg.PlatformAPI.PriceSyncEnabled {
		return
	}
	go func() {
		ticker := time.NewTicker(resellerChannelSyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, err := SyncResellerPlatformChannels(ctx, cfg)
				if err != nil {
					log.Printf("[reseller-sync] channel sync failed: %v", err)
					continue
				}
				log.Printf("[reseller-sync] channel sync ok: upserted=%d disabled=%d", result.Upserted, result.Disabled)
			}
		}
	}()
}

func SyncResellerPlatformChannels(ctx context.Context, cfg *config.Config) (ResellerPlatformChannelSyncResult, error) {
	var result ResellerPlatformChannelSyncResult
	if !IsResellerSiteMode(cfg) {
		return result, nil
	}
	if !cfg.PlatformAPI.PriceSyncEnabled {
		return result, nil
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.PlatformAPI.BaseURL), "/")
	apiKey := strings.TrimSpace(cfg.PlatformAPI.Key)
	if baseURL == "" || apiKey == "" {
		return result, fmt.Errorf("platform_api.base_url and platform_api.key are required")
	}

	channels, err := fetchResellerPlatformChannels(ctx, baseURL, apiKey)
	if err != nil {
		return result, err
	}

	var existing []model.Channel
	if err := db.Engine.Context(ctx).Find(&existing); err != nil {
		return result, fmt.Errorf("load local channels: %w", err)
	}
	existingByPlatformID := make(map[int64]model.Channel)
	for _, ch := range existing {
		if channelSource(ch) != resellerPlatformChannelSource {
			continue
		}
		if platformID := channelPlatformID(ch); platformID > 0 {
			existingByPlatformID[platformID] = ch
		}
	}

	seenPlatformIDs := make(map[int64]struct{}, len(channels))
	changed := make([]model.Channel, 0, len(channels))
	for _, platformChannel := range channels {
		if platformChannel.ID <= 0 {
			continue
		}
		localChannel, ok := buildResellerProxyChannel(platformChannel, baseURL, apiKey, resellerProfitRatio(cfg))
		if !ok {
			continue
		}
		seenPlatformIDs[platformChannel.ID] = struct{}{}
		if current, found := existingByPlatformID[platformChannel.ID]; found {
			localChannel.ID = current.ID
			if _, err := db.Engine.Context(ctx).ID(current.ID).Cols(
				"name", "model", "type", "base_url", "method", "headers", "timeout_ms",
				"request_script", "response_script", "query_url", "query_method", "query_timeout_ms",
				"query_script", "billing_type", "billing_config", "key_pool_id", "protocol",
				"error_script", "auth_type", "auth_param_name", "auth_region", "auth_service",
				"passthrough_headers", "passthrough_body", "weight", "priority", "is_active",
				"groups", "display_name", "model_provider", "icon_url", "description",
			).Update(&localChannel); err != nil {
				return result, fmt.Errorf("update reseller channel %d: %w", current.ID, err)
			}
			changed = append(changed, current, localChannel)
		} else {
			if _, err := db.Engine.Context(ctx).Insert(&localChannel); err != nil {
				return result, fmt.Errorf("insert reseller channel %q: %w", localChannel.Name, err)
			}
			changed = append(changed, localChannel)
		}
		result.Upserted++
	}

	for _, ch := range existing {
		source := channelSource(ch)
		platformID := channelPlatformID(ch)
		_, stillAllowed := seenPlatformIDs[platformID]
		if ch.IsActive && (source != resellerPlatformChannelSource || platformID <= 0 || !stillAllowed) {
			if _, err := db.Engine.Context(ctx).ID(ch.ID).Cols("is_active").Update(&model.Channel{IsActive: false}); err != nil {
				return result, fmt.Errorf("disable local channel %d: %w", ch.ID, err)
			}
			ch.IsActive = false
			changed = append(changed, ch)
			result.Disabled++
		}
	}
	if len(changed) > 0 {
		InvalidateChannelRouteCaches(ctx, changed...)
	}
	return result, nil
}

func fetchResellerPlatformChannels(ctx context.Context, baseURL, apiKey string) ([]resellerPlatformChannel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/reseller/platform/channels", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request platform channels: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("platform channels returned HTTP %d", resp.StatusCode)
	}
	var payload resellerPlatformChannelPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode platform channels: %w", err)
	}
	return payload.Channels, nil
}

func buildResellerProxyChannel(upstream resellerPlatformChannel, baseURL, apiKey string, profitRatio float64) (model.Channel, bool) {
	routingModel := strings.TrimSpace(upstream.RoutingModel)
	if routingModel == "" {
		return model.Channel{}, false
	}
	channelType := strings.ToLower(strings.TrimSpace(upstream.Type))
	if channelType == "" {
		channelType = "llm"
	}
	protocol := strings.ToLower(strings.TrimSpace(upstream.Protocol))
	if protocol == "" {
		protocol = "openai"
	}
	billingType := strings.TrimSpace(upstream.BillingType)
	if billingType == "" {
		billingType = "custom"
	}

	billingConfig := prepareResellerBillingConfig(upstream.ID, upstream.BillingConfig, profitRatio)
	ch := model.Channel{
		Name:           firstNonEmptyString(upstream.Name, routingModel),
		Model:          routingModel,
		Type:           channelType,
		BaseURL:        resellerPlatformEndpoint(baseURL, channelType, protocol),
		Method:         http.MethodPost,
		Headers:        model.JSON{"Authorization": "Bearer " + apiKey, "Content-Type": "application/json"},
		TimeoutMs:      resellerProxyTimeout(channelType),
		QueryMethod:    http.MethodGet,
		QueryTimeoutMs: 30000,
		BillingType:    billingType,
		BillingConfig:  billingConfig,
		Protocol:       protocol,
		AuthType:       "bearer",
		Weight:         1,
		IsActive:       true,
		DisplayName:    routingModel,
		ModelProvider:  strings.TrimSpace(upstream.ModelProvider),
		IconURL:        strings.TrimSpace(upstream.IconURL),
		Description:    strings.TrimSpace(upstream.Description),
		Groups:         model.JSONStrings{},
	}
	if channelType != "llm" {
		ch.ResponseScript = resellerProxySubmitScript
		ch.QueryURL = baseURL + "/v1/tasks/{id}"
		ch.QueryScript = resellerProxyQueryScript
	}
	return ch, true
}

func prepareResellerBillingConfig(platformID int64, cfg map[string]interface{}, profitRatio float64) model.JSON {
	out := deepCopyMap(cfg)
	copyPlatformPriceToCost(out)
	markupPriceFields(out, profitRatio)
	out["source"] = resellerPlatformChannelSource
	out["platform_channel_id"] = platformID
	out["profit_ratio"] = profitRatio
	return model.JSON(out)
}

func deepCopyMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(src))
	for key, value := range src {
		out[key] = deepCopyValue(value)
	}
	return out
}

func deepCopyValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return deepCopyMap(typed)
	case model.JSON:
		return deepCopyMap(map[string]interface{}(typed))
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i := range typed {
			out[i] = deepCopyValue(typed[i])
		}
		return out
	default:
		return value
	}
}

func copyPlatformPriceToCost(cfg map[string]interface{}) {
	copyNumberField(cfg, "input_price_per_1m_tokens", "input_cost_per_1m_tokens")
	copyNumberField(cfg, "output_price_per_1m_tokens", "output_cost_per_1m_tokens")
	copyNumberField(cfg, "cache_creation_price_per_1m_tokens", "cache_creation_cost_per_1m_tokens")
	copyNumberField(cfg, "cache_read_price_per_1m_tokens", "cache_read_cost_per_1m_tokens")
	copyNumberField(cfg, "base_price", "base_cost")
	copyNumberField(cfg, "default_size_price", "default_size_cost")
	copyNumberField(cfg, "price_per_second", "cost_per_second")
	copyNumberField(cfg, "price_per_call", "cost_per_call")
	copyNumberField(cfg, "price_per_count", "cost_per_call")
	if raw, ok := cfg["size_prices"]; ok {
		cfg["size_costs"] = deepCopyValue(raw)
	}
}

func copyNumberField(cfg map[string]interface{}, from, to string) {
	if _, exists := cfg[to]; exists {
		return
	}
	if n, ok := configNumberForSync(cfg[from]); ok {
		cfg[to] = n
	}
}

func markupPriceFields(cfg map[string]interface{}, ratio float64) {
	for _, key := range []string{
		"input_price_per_1m_tokens",
		"output_price_per_1m_tokens",
		"cache_creation_price_per_1m_tokens",
		"cache_read_price_per_1m_tokens",
		"base_price",
		"default_size_price",
		"price_per_second",
		"price_per_call",
		"price_per_count",
	} {
		markupNumberField(cfg, key, ratio)
	}
	if raw, ok := cfg["size_prices"]; ok {
		if values, ok := numericMapForSync(raw); ok {
			for key, value := range values {
				if n, ok := configNumberForSync(value); ok {
					values[key] = markupCredits(n, ratio)
				}
			}
			cfg["size_prices"] = values
		}
	}
}

func markupNumberField(cfg map[string]interface{}, key string, ratio float64) {
	if n, ok := configNumberForSync(cfg[key]); ok {
		cfg[key] = markupCredits(n, ratio)
	}
}

func configNumberForSync(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case uint64:
		if typed > math.MaxInt64 {
			return 0, false
		}
		return int64(typed), true
	case uint:
		return int64(typed), true
	case uint32:
		return int64(typed), true
	case float64:
		return int64(math.Ceil(typed)), true
	case float32:
		return int64(math.Ceil(float64(typed))), true
	case json.Number:
		if i, err := typed.Int64(); err == nil {
			return i, true
		}
		if f, err := typed.Float64(); err == nil {
			return int64(math.Ceil(f)), true
		}
	case string:
		var number json.Number = json.Number(strings.TrimSpace(typed))
		if i, err := number.Int64(); err == nil {
			return i, true
		}
		if f, err := number.Float64(); err == nil {
			return int64(math.Ceil(f)), true
		}
	}
	return 0, false
}

func numericMapForSync(raw interface{}) (map[string]interface{}, bool) {
	switch typed := raw.(type) {
	case map[string]interface{}:
		return typed, true
	case model.JSON:
		return map[string]interface{}(typed), true
	default:
		return nil, false
	}
}

func markupCredits(value int64, ratio float64) int64 {
	if value <= 0 {
		return value
	}
	if ratio < 1 {
		ratio = 1
	}
	return int64(math.Ceil(float64(value) * ratio))
}

func resellerPlatformEndpoint(baseURL, channelType, protocol string) string {
	switch channelType {
	case "image":
		return baseURL + "/v1/image"
	case "video":
		return baseURL + "/v1/video"
	case "audio":
		return baseURL + "/v1/audio"
	case "music":
		return baseURL + "/v1/music"
	}
	switch protocol {
	case "claude":
		return baseURL + "/v1/messages"
	case "gemini":
		return baseURL + "/v1/gemini"
	case "responses":
		return baseURL + "/v1/responses"
	case "realtime":
		return baseURL + "/v1/realtime"
	default:
		return baseURL + "/v1/chat/completions"
	}
}

func resellerProxyTimeout(channelType string) int64 {
	if channelType == "llm" {
		return 60000
	}
	return 120000
}

func resellerProfitRatio(cfg *config.Config) float64 {
	ratio := cfg.ResellerSite.ProfitRatio
	if ratio < 1 {
		ratio = 1
	}
	return ratio
}

func channelSource(ch model.Channel) string {
	if ch.BillingConfig == nil {
		return ""
	}
	source, _ := ch.BillingConfig["source"].(string)
	return strings.TrimSpace(source)
}

func channelPlatformID(ch model.Channel) int64 {
	if ch.BillingConfig == nil {
		return 0
	}
	id, ok := configNumberForSync(ch.BillingConfig["platform_channel_id"])
	if !ok {
		return 0
	}
	return id
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

const resellerProxySubmitScript = `function mapResponse(output) {
  if (!output) {
    return { status: 3, msg: 'platform submit failed' };
  }
  var taskID = output.task_id || output.id;
  if (taskID) {
    return { status: 1, upstream_task_id: String(taskID), msg: output.msg || 'submitted' };
  }
  if (output.status === 2 || output.code === 200) {
    return output;
  }
  return { status: 3, msg: output.error || output.msg || 'platform task id missing' };
}`

const resellerProxyQueryScript = `function mapResponse(output) {
  if (!output) {
    return { status: 1, code: 150, msg: 'processing', progress: 0 };
  }
  if (output.status === 3 || output.code === 500) {
    return { status: 3, code: 500, msg: output.msg || output.error || 'platform task failed' };
  }
  if (output.status === 2 || output.code === 200) {
    var out = output.result || {};
    for (var k in output) {
      if (out[k] === undefined && k !== 'result' && k !== 'request') {
        out[k] = output[k];
      }
    }
    if (out.status === undefined) out.status = 2;
    if (out.code === undefined) out.code = 200;
    return out;
  }
  return {
    status: 1,
    code: 150,
    msg: output.msg || 'processing',
    progress: output.progress || 0
  };
}`
