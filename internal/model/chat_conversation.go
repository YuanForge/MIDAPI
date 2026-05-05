package model

import "time"

// ChatConversation 存储用户在 Playground 中的对话历史。
type ChatConversation struct {
	ID        int64     `xorm:"pk autoincr 'id'" json:"id"`
	UserID    int64     `xorm:"notnull index 'user_id'" json:"user_id"`
	Title     string    `xorm:"notnull default('') 'title'" json:"title"`
	Model     string    `xorm:"notnull default('') 'model'" json:"model"`
	Messages  JSON      `xorm:"jsonb 'messages'" json:"messages"` // []Message
	CreatedAt time.Time `xorm:"created 'created_at'" json:"created_at"`
	UpdatedAt time.Time `xorm:"updated 'updated_at'" json:"updated_at"`
}

func (*ChatConversation) TableName() string { return "chat_conversations" }
