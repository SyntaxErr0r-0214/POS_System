package handler

import (
	"database/sql"
	"io"
	"net/http"
	"os"
)

type SystemHandler struct {
	DB *sql.DB
}

// Backup 导出数据库文件
func (h *SystemHandler) Backup(w http.ResponseWriter, r *http.Request) {
	// 假设数据库文件就在当前目录下
	filename := "pos_data.db"

	file, err := os.Open(filename)
	if err != nil {
		http.Error(w, "无法读取数据库文件", 500)
		return
	}
	defer file.Close()

	// 设置响应头，让浏览器下载
	w.Header().Set("Content-Disposition", "attachment; filename=pos_data_backup.db")
	w.Header().Set("Content-Type", "application/octet-stream")

	io.Copy(w, file)
}

// Restore 恢复数据库文件
func (h *SystemHandler) Restore(w http.ResponseWriter, r *http.Request) {
	// 限制上传大小 (比如 50MB)
	r.ParseMultipartForm(50 << 20)

	file, _, err := r.FormFile("db_file")
	if err != nil {
		http.Error(w, "请上传有效的文件", 400)
		return
	}
	defer file.Close()

	// 警告：这里直接覆盖运行中的数据库文件。
	// 在 Windows 上可能会因为文件锁定而失败，在 Linux/Mac 上通常没问题。
	// 最严谨的做法是先关闭 DB 连接，覆盖后再重连。但为了代码简单，我们先尝试直接覆盖。
	// 如果覆盖成功，建议重启服务。

	dst, err := os.Create("pos_data.db")
	if err != nil {
		http.Error(w, "无法写入数据库文件 (可能正被占用，请先停止程序)", 500)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "写入失败", 500)
		return
	}

	w.Write([]byte("恢复成功！请务必重启程序以加载新数据。"))
}

// Reset 初始化清空系统所有数据
func (h *SystemHandler) Reset(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "未连接数据库", 500)
		return
	}
	tx, err := h.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer tx.Rollback()

	queries := []string{
		"DELETE FROM order_items",
		"DELETE FROM orders",
		"DELETE FROM products",
		"DELETE FROM sqlite_sequence",
	}
	for _, q := range queries {
		if _, err := tx.Exec(q); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// 释放空间
	h.DB.Exec("VACUUM")
	w.Write([]byte("系统已成功初始化！所有订单、商品及历史流水均已清空。"))
}
