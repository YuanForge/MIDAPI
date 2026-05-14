-- 为 billing_transactions 表添加 model_credit_charged 列
-- 记录每笔消费中扣除的专属模型积分数量（0 表示全部来自通用余额）
ALTER TABLE billing_transactions ADD COLUMN IF NOT EXISTS model_credit_charged BIGINT NOT NULL DEFAULT 0;
