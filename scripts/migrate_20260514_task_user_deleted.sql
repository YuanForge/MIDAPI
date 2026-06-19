-- 为 tasks 表添加 user_deleted 列，用于用户侧清除任务历史（软删除）
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS user_deleted BOOLEAN NOT NULL DEFAULT FALSE;

-- 为过滤提速
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_user_deleted ON tasks (user_id, user_deleted);
