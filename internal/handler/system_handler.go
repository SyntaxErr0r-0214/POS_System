package handler

import (
	"database/sql"
	"io"
	"log"
	"net/http"
	"os"
	"modern-pos/internal/service"
)

type SystemHandler struct {
	DB        *sql.DB
	BackupSvc *service.BackupService
	ReloadDB  func()
}

// Backup 实时执行热备份与完整性校验，并向管理员流式导出校验通过的备份文件
func (h *SystemHandler) Backup(w http.ResponseWriter, r *http.Request) {
	if h.BackupSvc == nil {
		http.Error(w, "后台备份与校验服务未初始化", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=pos_data_backup.db")
	w.Header().Set("Content-Type", "application/octet-stream")

	if err := h.BackupSvc.WriteBackupToStream(w); err != nil {
		log.Printf("热备份导出异常: %v", err)
		http.Error(w, "热备份或完整性校验失败: "+err.Error(), http.StatusInternalServerError)
	}
}

// Restore 恢复数据库文件（恢复前执行深度完整性与核心业务表结构校验）
func (h *SystemHandler) Restore(w http.ResponseWriter, r *http.Request) {
	if h.BackupSvc == nil {
		http.Error(w, "后台备份与校验服务未初始化", http.StatusInternalServerError)
		return
	}

	r.ParseMultipartForm(50 << 20)

	file, _, err := r.FormFile("db_file")
	if err != nil {
		http.Error(w, "请上传有效的数据库文件", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 1. 将上传的文件流存入临时待校验文件
	tmpRestorePath := "pos_data_restore.tmp"
	defer os.Remove(tmpRestorePath)

	tmpFile, err := os.Create(tmpRestorePath)
	if err != nil {
		http.Error(w, "无法写入临时校验文件: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(tmpFile, file); err != nil {
		tmpFile.Close()
		http.Error(w, "读取上传文件流失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tmpFile.Close()

	// 2. 执行严苛的 SQLite 完整性与核心业务表结构校验 (ValidateBackup)
	if err := h.BackupSvc.ValidateBackup(tmpRestorePath); err != nil {
		log.Printf("数据恢复拦截：上传的文件校验未通过: %v", err)
		http.Error(w, "恢复失败！上传的文件非有效 SQLite 数据库或已损坏/核心业务表缺失："+err.Error(), http.StatusBadRequest)
		return
	}

	// 3. 校验通过，安全覆盖原业务数据库文件
	if h.DB != nil {
		h.DB.Close()
	}

	if err := os.Rename(tmpRestorePath, "pos_data.db"); err != nil {
		// 备用方案：通过流拷贝覆盖
		src, errOpen := os.Open(tmpRestorePath)
		if errOpen != nil {
			http.Error(w, "覆盖数据库文件失败: "+errOpen.Error(), http.StatusInternalServerError)
			return
		}
		defer src.Close()

		dst, errCreate := os.Create("pos_data.db")
		if errCreate != nil {
			http.Error(w, "覆盖数据库文件失败 (文件正被系统占用): "+errCreate.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()
		if _, errCopy := io.Copy(dst, src); errCopy != nil {
			http.Error(w, "文件覆盖中途失败: "+errCopy.Error(), http.StatusInternalServerError)
			return
		}
	}

	if h.ReloadDB != nil {
		h.ReloadDB()
	}

	w.Write([]byte("数据库恢复及校验成功！系统已自动重载最新业务数据。"))
}

// Reset 初始化清空系统数据
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
	h.DB.Exec("VACUUM")
	w.Write([]byte("系统已成功初始化！所有订单、商品及历史流水均已清空。"))
}
