package model

import "time"

// LLMLog records each LLM request with upstream/client payloads, usage, and settle status.
type LLMLog struct {
	ID                     int64     `xorm:"pk autoincr 'id'" json:"id"`
	UserID                 int64     `xorm:"notnull index 'user_id'" json:"user_id"`
	ChannelID              int64     `xorm:"'channel_id'" json:"channel_id"`
	APIKeyID               int64     `xorm:"'api_key_id'" json:"api_key_id"`
	CorrID                 string    `xorm:"index 'corr_id'" json:"corr_id"`
	Model                  string    `xorm:"default('') 'model'" json:"model"`
	InputPricePer1MTokens  *int64    `xorm:"'input_price_per_1m_tokens' null" json:"input_price_per_1m_tokens,omitempty"`
	OutputPricePer1MTokens *int64    `xorm:"'output_price_per_1m_tokens' null" json:"output_price_per_1m_tokens,omitempty"`
	IsStream               bool      `xorm:"'is_stream'" json:"is_stream"`
	Transport              string    `xorm:"default('') 'transport'" json:"transport,omitempty"`
	UpstreamURL            string    `xorm:"text default('') 'upstream_url'" json:"upstream_url"`
	UpstreamMethod         string    `xorm:"default('POST') 'upstream_method'" json:"upstream_method"`
	UpstreamHeaders        JSON      `xorm:"jsonb 'upstream_headers'" json:"upstream_headers,omitempty"`
	UpstreamRequest        JSON      `xorm:"jsonb 'upstream_request'" json:"upstream_request"`
	ClientRequest          JSON      `xorm:"jsonb 'client_request'" json:"client_request,omitempty"`
	UpstreamStatus         int       `xorm:"default(0) 'upstream_status'" json:"upstream_status"`
	UpstreamResponse       JSON      `xorm:"jsonb 'upstream_response'" json:"upstream_response,omitempty"`
	ClientResponse         JSON      `xorm:"jsonb 'client_response'" json:"client_response,omitempty"`
	Usage                  JSON      `xorm:"jsonb 'usage'" json:"usage,omitempty"`
	Status                 string    `xorm:"notnull default('pending') 'status'" json:"status"`
	ErrorMsg               string    `xorm:"text 'error_msg'" json:"error_msg,omitempty"`
	CreatedAt              time.Time `xorm:"created 'created_at'" json:"created_at"`
	UpdatedAt              time.Time `xorm:"updated 'updated_at'" json:"updated_at"`
}

func (*LLMLog) TableName() string { return "llm_logs" }
