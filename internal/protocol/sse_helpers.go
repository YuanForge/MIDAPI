package protocol

import (
	"strings"

	"github.com/google/uuid"
)

func intFromJSON(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	default:
		return 0
	}
}

// newShortID 生成短 ID（不含横线）供 Responses API 的 id 字段使用。
func newShortID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}
