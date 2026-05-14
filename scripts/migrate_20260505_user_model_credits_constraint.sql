-- 补丁：为 user_model_credits 补加唯一约束（若表已存在但约束缺失时使用）
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uq_user_model_credits'
          AND conrelid = 'user_model_credits'::regclass
    ) THEN
        ALTER TABLE user_model_credits
            ADD CONSTRAINT uq_user_model_credits UNIQUE (user_id, model_name);
    END IF;
END $$;
