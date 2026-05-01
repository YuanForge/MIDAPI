package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const larkWebhook = "https://open.larksuite.com/open-apis/bot/v2/hook/a367d5fd-3a7c-4c73-b8ed-be22e19b4c32"

// SendLarkChannelDisabled 通知运营：渠道因余额不足被停用
func SendLarkChannelDisabled(channelName string, channelID int64, reason string) error {
	content := fmt.Sprintf(
		"渠道【%s】(ID: %d) 因余额不足已被自动停用。\n原因: %s\n请及时处理。",
		channelName, channelID, reason,
	)
	card := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"config": map[string]interface{}{
				"wide_screen_mode": true,
			},
			"header": map[string]interface{}{
				"template": "red",
				"title": map[string]interface{}{
					"content": "⚠️ 渠道自动停用通知",
					"tag":     "plain_text",
				},
			},
			"elements": []interface{}{
				map[string]interface{}{
					"tag": "div",
					"text": map[string]interface{}{
						"content": content,
						"tag":     "lark_md",
					},
				},
			},
		},
	}

	body, err := json.Marshal(card)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(larkWebhook, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Lark通知失败: %s", resp.Status)
	}
	return nil
}
