package handler

import (
	"fanapi/internal/model"
)

// selectNextChannel 为重试选择下一个渠道，排除已尝试过的渠道 ID。
// 仅稳定密钥（stableChannels 非空）支持兜底重试，按价格升序列表顺序选取下一个未尝试的渠道。
// 低价密钥不做跨渠道重试，直接返回 nil。
func selectNextChannel(_ map[string]any, excludeIDs []int64, stableChannels []model.Channel) *model.Channel {
	if len(stableChannels) == 0 {
		return nil
	}

	excluded := make(map[int64]bool, len(excludeIDs))
	for _, id := range excludeIDs {
		excluded[id] = true
	}

	for i := range stableChannels {
		if !excluded[stableChannels[i].ID] {
			ch := stableChannels[i]
			return &ch
		}
	}
	return nil
}
