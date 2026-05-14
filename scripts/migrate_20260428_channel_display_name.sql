-- 为渠道表新增自定义展示名称字段
-- display_name 留空时，用户端以 model 字段作为展示名和分组依据
ALTER TABLE channels ADD COLUMN IF NOT EXISTS display_name VARCHAR(255) NOT NULL DEFAULT '';
