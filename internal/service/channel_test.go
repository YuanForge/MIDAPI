package service

import (
	"testing"

	"fanapi/internal/model"
)

func TestStableChannelLessUsesHigherPriorityWhenPriceTied(t *testing.T) {
	channel22 := model.Channel{
		ID:          22,
		Priority:    98,
		BillingType: "token",
		BillingConfig: model.JSON{
			"input_price_per_1m_tokens":  2100000,
			"output_price_per_1m_tokens": 13000000,
		},
	}
	channel24 := model.Channel{
		ID:          24,
		Priority:    99,
		BillingType: "token",
		BillingConfig: model.JSON{
			"input_price_per_1m_tokens":  2100000,
			"output_price_per_1m_tokens": 13000000,
		},
	}

	if !stableChannelLess(channel24, channel22, "") {
		t.Fatalf("expected channel 24 with higher priority to sort before channel 22 at the same price")
	}
	if stableChannelLess(channel22, channel24, "") {
		t.Fatalf("expected channel 22 with lower priority not to sort before channel 24 at the same price")
	}
}

func TestChannelBasePriceForGroupUsesGroupPricing(t *testing.T) {
	channel := model.Channel{
		ID:          24,
		BillingType: "token",
		BillingConfig: model.JSON{
			"input_price_per_1m_tokens":  9000000,
			"output_price_per_1m_tokens": 9000000,
			"pricing_groups": map[string]interface{}{
				"vip": map[string]interface{}{
					"input_price_per_1m_tokens":  2000000,
					"output_price_per_1m_tokens": 10000000,
				},
			},
		},
	}

	if got := channelBasePriceForGroup(channel, "vip"); got != 12000000 {
		t.Fatalf("expected group price rank 12000000, got %v", got)
	}
}

func TestChannelRoutingKeyPrefersDisplayName(t *testing.T) {
	channel := model.Channel{
		Model:       "gpt-5.5",
		DisplayName: "gpt-5.5-cpa",
	}

	if got := ChannelRoutingKey(channel); got != "gpt-5.5-cpa" {
		t.Fatalf("expected display_name to be the routing key, got %q", got)
	}
}

func TestChannelRoutingKeyFallsBackToModel(t *testing.T) {
	channel := model.Channel{
		Model:       "gpt-5.5",
		DisplayName: "  ",
	}

	if got := ChannelRoutingKey(channel); got != "gpt-5.5" {
		t.Fatalf("expected model to be the routing key when display_name is empty, got %q", got)
	}
}
