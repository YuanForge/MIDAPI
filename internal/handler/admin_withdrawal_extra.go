package handler

import (
	"fanapi/internal/db"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

// POST /admin/withdrawals/:id/proof  上传打款凭证
func AdminUploadWithdrawalProof(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式错误"})
		return
	}
	var req struct {
		ProofURL  string `json:"proof_url"`
		ProofNote string `json:"proof_note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 兼容未执行迁移的生产库：按需补齐字段，避免上传凭证时报列不存在。
	_, _ = db.Engine.Exec("ALTER TABLE withdraw_requests ADD COLUMN IF NOT EXISTS proof_url TEXT NOT NULL DEFAULT ''")
	_, _ = db.Engine.Exec("ALTER TABLE withdraw_requests ADD COLUMN IF NOT EXISTS proof_note TEXT NOT NULL DEFAULT ''")
	_, err = db.Engine.Exec(
		"UPDATE withdraw_requests SET proof_url=$1, proof_note=$2, updated_at=NOW() WHERE id=$3",
		req.ProofURL, req.ProofNote, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
