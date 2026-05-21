-- 为 llm_logs 表添加 transport 字段，用于区分请求传输协议
-- 取值：ws（WebSocket）/ sse（Server-Sent Events）/ http（普通 HTTP）
ALTER TABLE llm_logs ADD COLUMN IF NOT EXISTS transport VARCHAR(8) NOT NULL DEFAULT '';
