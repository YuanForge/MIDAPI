package handler

import (
	"fanapi/internal/model"
	"fanapi/internal/service"

	"context"
)

// selectNextChannel 为重试选择下一个渠道，排除已尝试过的渠道 ID。
// 稳定密钥使用价格升序候选列表；普通路由使用现有加权重试选择。
func selectNextChannel(ctx context.Context, routingModel string, excludeIDs []int64, stableChannels []model.Channel, requireResponses bool) *model.Channel {
	excluded := make(map[int64]bool, len(excludeIDs))
	for _, id := range excludeIDs {
		excluded[id] = true
	}

	if len(stableChannels) > 0 {
		for i := range stableChannels {
			if !excluded[stableChannels[i].ID] {
				ch := stableChannels[i]
				return &ch
			}
		}
		return nil
	}

	if routingModel == "" {
		return nil
	}

	var (
		ch  *model.Channel
		err error
	)
	if requireResponses {
		ch, err = service.SelectChannelByProtocol(ctx, routingModel, protocolResponses, excludeIDs...)
	} else {
		ch, err = service.SelectChannelByWeight(ctx, routingModel, excludeIDs...)
	}
	if err != nil {
		return nil
	}
	return ch
}
