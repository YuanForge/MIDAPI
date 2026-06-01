package handler

import (
	"fanapi/internal/db"
	"fanapi/internal/model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

// GET /admin/cards/batches  批次列表
func ListCardBatches(c *gin.Context) {
	engine := db.Engine
	type batchRow struct {
		ID        int64     `json:"id" xorm:"id"`
		BatchID   string    `json:"batch_id" xorm:"batch_id"`
		Note      string    `json:"note" xorm:"note"`
		Credits   int64     `json:"credits" xorm:"credits"`
		Count     int       `json:"count" xorm:"count"`
		CreatedBy int64     `json:"created_by" xorm:"created_by"`
		CreatedAt time.Time `json:"created_at" xorm:"created_at"`
		Used      int       `json:"used" xorm:"used"`
	}
	var rows []batchRow
	_ = engine.SQL(
		`SELECT cb.*, COUNT(c.id) FILTER (WHERE c.status='used') AS used
		 FROM card_batches cb
		 LEFT JOIN cards c ON c.batch_id = cb.batch_id
		 GROUP BY cb.id ORDER BY cb.created_at DESC LIMIT 100`,
	).Find(&rows)

	// 历史兼容：旧数据可能仅存在 cards.batch_id，而 card_batches 为空。
	if len(rows) == 0 {
		_ = engine.SQL(
			`SELECT
				MIN(c.id) AS id,
				c.batch_id AS batch_id,
				MAX(c.note) AS note,
				MAX(c.credits) AS credits,
				COUNT(*) AS count,
				0 AS created_by,
				MIN(c.created_at) AS created_at,
				COUNT(*) FILTER (WHERE c.status='used') AS used
			 FROM cards c
			 WHERE c.batch_id IS NOT NULL AND c.batch_id <> ''
			 GROUP BY c.batch_id
			 ORDER BY MIN(c.created_at) DESC
			 LIMIT 100`,
		).Find(&rows)
	}
	c.JSON(http.StatusOK, gin.H{"batches": rows})
}

// POST /admin/cards/:id/void  作废单张卡密
func VoidCard(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式错误"})
		return
	}
	engine := db.Engine
	var card model.Card
	if found, err := engine.ID(id).Get(&card); err != nil || !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "卡密不存在"})
		return
	}
	if card.Status == "used" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "已使用的卡密不可作废"})
		return
	}
	engine.ID(id).Update(&model.Card{Status: "voided"})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /admin/cards/batches/:batch_id/void  批量作废整批次未用卡密
func VoidCardBatch(c *gin.Context) {
	batchID := c.Param("batch_id")
	engine := db.Engine

	// 优先按 card_batch_id（数字）关联查询；兼容旧数据（batch_id 字符串）
	if batchIDInt, err := strconv.ParseInt(batchID, 10, 64); err == nil {
		// 新数据：按 card_batch_id 作废
		res, err := engine.Exec(
			"UPDATE cards SET status='voided' WHERE card_batch_id=$1 AND status='unused'", batchIDInt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		n, _ := res.RowsAffected()
		c.JSON(http.StatusOK, gin.H{"ok": true, "voided": n})
	} else {
		// 旧数据：按 batch_id（字符串）作废
		res, err := engine.Exec(
			"UPDATE cards SET status='voided' WHERE batch_id=$1 AND status='unused'", batchID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		n, _ := res.RowsAffected()
		c.JSON(http.StatusOK, gin.H{"ok": true, "voided": n})
	}
}
