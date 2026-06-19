package handler

import (
	"fanapi/internal/db"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type adminPaymentRecord struct {
	Source      string     `json:"source" xorm:"source"`
	SourceID    int64      `json:"source_id" xorm:"source_id"`
	ID          int64      `json:"id" xorm:"id"`
	UserID      int64      `json:"user_id" xorm:"user_id"`
	UserEmail   string     `json:"user_email" xorm:"user_email"`
	OutTradeNo  string     `json:"out_trade_no" xorm:"out_trade_no"`
	Amount      float64    `json:"amount" xorm:"amount"`
	Credits     int64      `json:"credits" xorm:"credits"`
	Status      string     `json:"status" xorm:"status"`
	TradeNo     string     `json:"trade_no" xorm:"trade_no"`
	PayFlat     int        `json:"pay_flat" xorm:"pay_flat"`
	PayChannel  string     `json:"pay_channel" xorm:"pay_channel"`
	PayFrom     string     `json:"pay_from" xorm:"pay_from"`
	EventTime   time.Time  `json:"event_time" xorm:"event_time"`
	CreatedAt   time.Time  `json:"created_at" xorm:"created_at"`
	PaidAt      *time.Time `json:"paid_at" xorm:"paid_at"`
	CardCode    string     `json:"card_code" xorm:"card_code"`
	Note        string     `json:"note" xorm:"note"`
	GrossProfit float64    `json:"gross_profit" xorm:"gross_profit"`
}

type adminPaymentSummary struct {
	GrossProfit        float64 `json:"gross_profit" xorm:"gross_profit"`
	PaymentGrossProfit float64 `json:"payment_gross_profit" xorm:"payment_gross_profit"`
	CardGrossProfit    float64 `json:"card_gross_profit" xorm:"card_gross_profit"`
	PaidCount          int64   `json:"paid_count" xorm:"paid_count"`
	CardCount          int64   `json:"card_count" xorm:"card_count"`
}

type adminPaymentDailySummary struct {
	Day                string  `json:"day" xorm:"day"`
	GrossProfit        float64 `json:"gross_profit" xorm:"gross_profit"`
	PaymentGrossProfit float64 `json:"payment_gross_profit" xorm:"payment_gross_profit"`
	CardGrossProfit    float64 `json:"card_gross_profit" xorm:"card_gross_profit"`
	Count              int64   `json:"count" xorm:"count"`
	CardCount          int64   `json:"card_count" xorm:"card_count"`
}

// GET /admin/payments  管理员查看支付订单与卡密兑换充值记录
func AdminListPaymentOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}

	unionSQL, args, err := adminPaymentUnionSQL(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	type countRow struct {
		Total int64 `xorm:"total"`
	}
	count := countRow{}
	if _, err := db.Engine.SQL("SELECT COUNT(*) AS total FROM ("+unionSQL+") AS records", args...).Get(&count); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var summary adminPaymentSummary
	summarySQL := `SELECT
		COALESCE(SUM(gross_profit), 0) AS gross_profit,
		COALESCE(SUM(CASE WHEN source = 'payment' THEN gross_profit ELSE 0 END), 0) AS payment_gross_profit,
		COALESCE(SUM(CASE WHEN source = 'card' THEN gross_profit ELSE 0 END), 0) AS card_gross_profit,
		COUNT(*) FILTER (WHERE gross_profit <> 0) AS paid_count,
		COUNT(*) FILTER (WHERE source = 'card') AS card_count
	FROM (` + unionSQL + `) AS records`
	if _, err := db.Engine.SQL(summarySQL, args...).Get(&summary); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var daily []adminPaymentDailySummary
	dailySQL := `SELECT
		TO_CHAR(DATE_TRUNC('day', event_time AT TIME ZONE 'Asia/Shanghai'), 'YYYY-MM-DD') AS day,
		COALESCE(SUM(gross_profit), 0) AS gross_profit,
		COALESCE(SUM(CASE WHEN source = 'payment' THEN gross_profit ELSE 0 END), 0) AS payment_gross_profit,
		COALESCE(SUM(CASE WHEN source = 'card' THEN gross_profit ELSE 0 END), 0) AS card_gross_profit,
		COUNT(*) FILTER (WHERE gross_profit <> 0) AS count,
		COUNT(*) FILTER (WHERE source = 'card') AS card_count
	FROM (` + unionSQL + `) AS records
	WHERE gross_profit <> 0
	GROUP BY DATE_TRUNC('day', event_time AT TIME ZONE 'Asia/Shanghai')
	ORDER BY day DESC
	LIMIT 31`
	if err := db.Engine.SQL(dailySQL, args...).Find(&daily); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsSQL := unionSQL + " ORDER BY event_time DESC NULLS LAST, source_id DESC LIMIT ? OFFSET ?"
	rowArgs := append([]interface{}{}, args...)
	rowArgs = append(rowArgs, size, (page-1)*size)
	var rows []adminPaymentRecord
	if err := db.Engine.SQL(rowsSQL, rowArgs...).Find(&rows); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders":  rows,
		"total":   count.Total,
		"summary": summary,
		"daily":   daily,
	})
}

func adminPaymentUnionSQL(c *gin.Context) (string, []interface{}, error) {
	paymentWhere := []string{"1=1"}
	cardWhere := []string{"cards.status = 'used'", "cards.used_at IS NOT NULL"}
	paymentArgs := make([]interface{}, 0, 8)
	cardArgs := make([]interface{}, 0, 8)

	if s := c.Query("status"); s != "" {
		paymentWhere = append(paymentWhere, "payment_orders.status = ?")
		paymentArgs = append(paymentArgs, s)
		if s != "paid" {
			cardWhere = append(cardWhere, "1=0")
		}
	}
	if uid := c.Query("user_id"); uid != "" {
		paymentWhere = append(paymentWhere, "payment_orders.user_id = ?")
		paymentArgs = append(paymentArgs, uid)
		cardWhere = append(cardWhere, "cards.used_by = ?")
		cardArgs = append(cardArgs, uid)
	}
	if email := strings.TrimSpace(c.Query("email")); email != "" {
		likeEmail := "%" + email + "%"
		paymentWhere = append(paymentWhere, "users.email ILIKE ?")
		paymentArgs = append(paymentArgs, likeEmail)
		cardWhere = append(cardWhere, "users.email ILIKE ?")
		cardArgs = append(cardArgs, likeEmail)
	}
	if pf := c.Query("pay_flat"); pf != "" {
		paymentWhere = append(paymentWhere, "payment_orders.pay_flat = ?")
		paymentArgs = append(paymentArgs, pf)
		cardWhere = append(cardWhere, "1=0")
	}
	if pc := c.Query("pay_channel"); pc != "" {
		if pc == "card" {
			paymentWhere = append(paymentWhere, "1=0")
		} else {
			paymentWhere = append(paymentWhere, "payment_orders.pay_channel = ?")
			paymentArgs = append(paymentArgs, pc)
			cardWhere = append(cardWhere, "1=0")
		}
	}

	startAt, err := parseDateTime(c.Query("start_at"), false)
	if err != nil {
		return "", nil, fmt.Errorf("start_at 时间格式错误")
	}
	if !startAt.IsZero() {
		paymentWhere = append(paymentWhere, "payment_orders.created_at >= ?")
		paymentArgs = append(paymentArgs, startAt)
		cardWhere = append(cardWhere, "cards.used_at >= ?")
		cardArgs = append(cardArgs, startAt)
	}
	endAt, err := parseDateTime(c.Query("end_at"), true)
	if err != nil {
		return "", nil, fmt.Errorf("end_at 时间格式错误")
	}
	if !endAt.IsZero() {
		paymentWhere = append(paymentWhere, "payment_orders.created_at <= ?")
		paymentArgs = append(paymentArgs, endAt)
		cardWhere = append(cardWhere, "cards.used_at <= ?")
		cardArgs = append(cardArgs, endAt)
	}

	sql := fmt.Sprintf(`
SELECT
	'payment'::text AS source,
	payment_orders.id AS source_id,
	payment_orders.id AS id,
	payment_orders.user_id AS user_id,
	COALESCE(users.email, '') AS user_email,
	payment_orders.out_trade_no AS out_trade_no,
	payment_orders.amount::float8 AS amount,
	payment_orders.credits AS credits,
	payment_orders.status AS status,
	payment_orders.trade_no AS trade_no,
	payment_orders.pay_flat AS pay_flat,
	payment_orders.pay_channel AS pay_channel,
	payment_orders.pay_from AS pay_from,
	payment_orders.created_at AS event_time,
	payment_orders.created_at AS created_at,
	payment_orders.paid_at AS paid_at,
	''::text AS card_code,
	payment_orders.pro_name AS note,
	(CASE
		WHEN payment_orders.status = 'paid' THEN payment_orders.amount
		WHEN payment_orders.status = 'refunded' THEN -payment_orders.amount
		ELSE 0
	END)::float8 AS gross_profit
FROM payment_orders
LEFT JOIN users ON users.id = payment_orders.user_id
WHERE %s
UNION ALL
SELECT
	'card'::text AS source,
	cards.id AS source_id,
	cards.id AS id,
	cards.used_by AS user_id,
	COALESCE(users.email, '') AS user_email,
	cards.code AS out_trade_no,
	(cards.credits::float8 / 1000000.0) AS amount,
	cards.credits AS credits,
	'paid'::text AS status,
	''::text AS trade_no,
	0 AS pay_flat,
	'card'::text AS pay_channel,
	'redeem'::text AS pay_from,
	cards.used_at AS event_time,
	cards.used_at AS created_at,
	cards.used_at AS paid_at,
	cards.code AS card_code,
	cards.note AS note,
	(cards.credits::float8 / 1000000.0) AS gross_profit
FROM cards
LEFT JOIN users ON users.id = cards.used_by
WHERE %s`, strings.Join(paymentWhere, " AND "), strings.Join(cardWhere, " AND "))

	args := append([]interface{}{}, paymentArgs...)
	args = append(args, cardArgs...)
	return sql, args, nil
}
