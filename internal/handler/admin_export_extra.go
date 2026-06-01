package handler

import (
	"encoding/csv"
	"fanapi/internal/db"
	"fanapi/internal/model"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// GET /admin/exports
func ListExportTasks(c *gin.Context) {
	adminID := getAdminID(c)
	var tasks []model.ExportTask
	db.Engine.Where("created_by=?", adminID).OrderBy("created_at DESC").Limit(50).Find(&tasks)
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

// POST /admin/exports  创建导出任务（异步，直接标记为 pending）
func CreateExportTask(c *gin.Context) {
	var req struct {
		Name   string     `json:"name"`
		Type   string     `json:"type"`
		Params model.JSON `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	expires := time.Now().Add(24 * time.Hour)
	task := &model.ExportTask{
		Name:      req.Name,
		Type:      req.Type,
		Params:    req.Params,
		Status:    "pending",
		CreatedBy: getAdminID(c),
		ExpiresAt: &expires,
	}
	if _, err := db.Engine.Insert(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 在后台 goroutine 中执行导出
	go runExportTask(task.ID, c.Request.Host)
	c.JSON(http.StatusCreated, task)
}

// runExportTask 在后台执行导出任务，生成 CSV 文件。
func runExportTask(taskID int64, _ string) {
	fail := func(msg string) {
		db.Engine.ID(taskID).Cols("status", "error_msg").Update(&model.ExportTask{
			Status:   "failed",
			ErrorMsg: msg,
		})
	}

	var task model.ExportTask
	if found, _ := db.Engine.ID(taskID).Get(&task); !found {
		return
	}

	// 标记为处理中
	db.Engine.ID(taskID).Cols("status").Update(&model.ExportTask{Status: "processing"})

	// 根据类型执行查询并构建 CSV 行
	var headers []string
	var rows [][]string

	switch task.Type {
	case "transactions":
		type row struct {
			ID        int64     `xorm:"id"`
			UserID    int64     `xorm:"user_id"`
			Type      string    `xorm:"type"`
			Credits   int64     `xorm:"credits"`
			Cost      int64     `xorm:"cost"`
			CorrID    string    `xorm:"corr_id"`
			CreatedAt time.Time `xorm:"created_at"`
		}
		var records []row
		db.Engine.Table("billing_transactions").OrderBy("id DESC").Limit(100000).Find(&records)
		headers = []string{"ID", "用户ID", "类型", "积分变动(CNY)", "成本(CNY)", "关联ID", "时间"}
		for _, r := range records {
			rows = append(rows, []string{
				fmt.Sprintf("%d", r.ID),
				fmt.Sprintf("%d", r.UserID),
				r.Type,
				fmt.Sprintf("%.6f", float64(r.Credits)/1_000_000),
				fmt.Sprintf("%.6f", float64(r.Cost)/1_000_000),
				r.CorrID,
				r.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
	case "billing":
		type row struct {
			ID        int64     `xorm:"id"`
			UserID    int64     `xorm:"user_id"`
			Type      string    `xorm:"type"`
			Credits   int64     `xorm:"credits"`
			Cost      int64     `xorm:"cost"`
			CorrID    string    `xorm:"corr_id"`
			CreatedAt time.Time `xorm:"created_at"`
		}
		var records []row
		db.Engine.Table("billing_transactions").OrderBy("id DESC").Limit(100000).Find(&records)
		headers = []string{"ID", "用户ID", "类型", "积分变动(CNY)", "成本(CNY)", "关联ID", "时间"}
		for _, r := range records {
			rows = append(rows, []string{
				fmt.Sprintf("%d", r.ID),
				fmt.Sprintf("%d", r.UserID),
				r.Type,
				fmt.Sprintf("%.6f", float64(r.Credits)/1_000_000),
				fmt.Sprintf("%.6f", float64(r.Cost)/1_000_000),
				r.CorrID,
				r.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
	case "users":
		var records []model.User
		db.Engine.Table("users").OrderBy("id ASC").Limit(100000).Find(&records)
		headers = []string{"ID", "用户名", "邮箱", "余额(CNY)", "角色", "状态", "注册时间"}
		for _, r := range records {
			email := ""
			if r.Email != nil {
				email = *r.Email
			}
			status := "正常"
			if !r.IsActive {
				status = "已冻结"
			}
			rows = append(rows, []string{
				fmt.Sprintf("%d", r.ID),
				r.Username,
				email,
				fmt.Sprintf("%.6f", float64(r.Balance)/1_000_000),
				r.Role,
				status,
				r.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
	case "payments":
		type row struct {
			ID         int64     `xorm:"id"`
			UserID     int64     `xorm:"user_id"`
			OutTradeNo string    `xorm:"out_trade_no"`
			Amount     float64   `xorm:"amount"`
			Credits    int64     `xorm:"credits"`
			Status     string    `xorm:"status"`
			PayChannel string    `xorm:"pay_channel"`
			PayFlat    int       `xorm:"pay_flat"`
			CreatedAt  time.Time `xorm:"created_at"`
		}
		var records []row
		db.Engine.Table("payment_orders").OrderBy("id DESC").Limit(100000).Find(&records)
		headers = []string{"ID", "用户ID", "订单号", "金额(元)", "到账金额(¥)", "状态", "支付渠道", "时间"}
		for _, r := range records {
			statusZH := r.Status
			switch r.Status {
			case "paid":
				statusZH = "已支付"
			case "pending":
				statusZH = "待支付"
			case "failed":
				statusZH = "失败"
			case "refunded":
				statusZH = "已退款"
			}
			payChannelZH := r.PayChannel
			switch r.PayChannel {
			case "wechat":
				payChannelZH = "微信支付"
			case "alipay":
				payChannelZH = "支付宝"
			case "epay":
				payChannelZH = "Epay"
			case "shouqianba_wechat":
				payChannelZH = "收钱吧-微信"
			case "shouqianba_alipay":
				payChannelZH = "收钱吧-支付宝"
			case "":
				switch r.PayFlat {
				case 1:
					payChannelZH = "微信支付"
				case 2:
					payChannelZH = "支付宝"
				default:
					payChannelZH = "-"
				}
			}
			rows = append(rows, []string{
				fmt.Sprintf("%d", r.ID),
				fmt.Sprintf("%d", r.UserID),
				r.OutTradeNo,
				fmt.Sprintf("%.2f", r.Amount),
				fmt.Sprintf("%.2f", float64(r.Credits)/1_000_000),
				statusZH,
				payChannelZH,
				r.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
	case "llm_logs":
		type row struct {
			ID        int64     `xorm:"id"`
			UserID    int64     `xorm:"user_id"`
			ChannelID int64     `xorm:"channel_id"`
			Model     string    `xorm:"model"`
			Status    string    `xorm:"status"`
			CorrID    string    `xorm:"corr_id"`
			ErrorMsg  string    `xorm:"error_msg"`
			CreatedAt time.Time `xorm:"created_at"`
		}
		var records []row
		db.Engine.Table("llm_logs").OrderBy("id DESC").Limit(100000).Find(&records)
		headers = []string{"ID", "用户ID", "渠道ID", "模型", "状态", "CorrID", "错误信息", "时间"}
		for _, r := range records {
			rows = append(rows, []string{
				fmt.Sprintf("%d", r.ID),
				fmt.Sprintf("%d", r.UserID),
				fmt.Sprintf("%d", r.ChannelID),
				r.Model,
				r.Status,
				r.CorrID,
				r.ErrorMsg,
				r.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
	default:
		fail("不支持的导出类型: " + task.Type)
		return
	}

	// 写入 CSV 文件
	exportDir := filepath.Join("uploads", "exports")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		fail("创建导出目录失败: " + err.Error())
		return
	}
	filename := fmt.Sprintf("export_%d_%d.csv", taskID, time.Now().Unix())
	fullPath := filepath.Join(exportDir, filename)
	f, err := os.Create(fullPath)
	if err != nil {
		fail("创建文件失败: " + err.Error())
		return
	}
	// UTF-8 BOM 让 Excel 正常显示中文
	f.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(f)
	w.Write(headers)
	w.WriteAll(rows)
	w.Flush()
	f.Close()

	if err := w.Error(); err != nil {
		fail("写入 CSV 失败: " + err.Error())
		return
	}

	info, _ := os.Stat(fullPath)
	fileSize := int64(0)
	if info != nil {
		fileSize = info.Size()
	}
	fileURL := fmt.Sprintf("/uploads/exports/%s", filename)

	db.Engine.ID(taskID).Cols("status", "progress", "file_url", "file_size").Update(&model.ExportTask{
		Status:   "done",
		Progress: 100,
		FileURL:  fileURL,
		FileSize: fileSize,
	})
}
