package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"modern-pos/internal/model"
	"modern-pos/internal/repository"
	"modern-pos/internal/service"
	"modern-pos/pkg/database"
	"testing"
)

// 4. 系统重置与初始化测试 (System Reset)
// 验证核心算法：调用 /api/system/reset 能够在一个原子事务中清空所有商品、订单及订单明细表，并重置自增 ID 序列
func TestSystemReset(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_pos_reset.db")
	db := database.InitDB(dbPath)
	defer db.Close()

	pRepo := repository.NewProductRepo(db)
	oRepo := repository.NewOrderRepo(db)

	// 插入前置测试脏数据
	pRepo.Create(model.Product{Barcode: "R001", Name: "待清空商品", Price: 10.0, Stock: 100})
	tx, _ := db.Begin()
	oRepo.CreateOrder(tx, "张三", "13800000000", "Completed")
	tx.Commit()

	var prodCount, orderCount int
	db.QueryRow("SELECT COUNT(*) FROM products").Scan(&prodCount)
	db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&orderCount)
	if prodCount == 0 || orderCount == 0 {
		t.Fatalf("测试前置准备失败：脏数据未能插入")
	}

	// 构造系统重置请求
	sysHandler := &SystemHandler{DB: db, BackupSvc: service.NewBackupService(db)}
	req := httptest.NewRequest("POST", "/api/system/reset", nil)
	w := httptest.NewRecorder()

	sysHandler.Reset(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("系统重置接口期望响应码 200，实际返回: %d", resp.StatusCode)
	}

	// 验证所有核心业务表是否被彻底清空
	db.QueryRow("SELECT COUNT(*) FROM products").Scan(&prodCount)
	db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&orderCount)
	var itemCount int
	db.QueryRow("SELECT COUNT(*) FROM order_items").Scan(&itemCount)

	if prodCount != 0 || orderCount != 0 || itemCount != 0 {
		t.Fatalf("系统重置核心算法失败：表内仍残留业务数据 (products=%d, orders=%d, items=%d)", prodCount, orderCount, itemCount)
	}

	var seqCount int
	db.QueryRow("SELECT COUNT(*) FROM sqlite_sequence WHERE name IN ('products', 'orders', 'order_items')").Scan(&seqCount)
	if seqCount != 0 {
		t.Fatalf("系统重置序列算法失败：sqlite_sequence 未被完全重置")
	}
}

// 5. 热备份与完整性校验测试 (Backup Validation & Integrity)
// 验证备份与校验服务是否能够通过 VACUUM INTO 导出有效快照，并在恢复前成功识别与拦截损坏的数据库文件
func TestBackupAndRestoreValidation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_pos_backup.db")
	db := database.InitDB(dbPath)
	defer db.Close()

	backupDir := filepath.Join(t.TempDir(), "backups")
	backupSvc := &service.BackupService{
		DB:            db,
		BackupDir:     backupDir,
		RetentionDays: 7,
	}

	// 1. 验证热备份导出与完整性校验
	backupPath, err := backupSvc.PerformBackup()
	if err != nil {
		t.Fatalf("执行热备份失败: %v", err)
	}

	// 验证备份文件深度完整性校验是否成功
	if err := backupSvc.ValidateBackup(backupPath); err != nil {
		t.Fatalf("合法备份文件完整性校验未通过: %v", err)
	}

	// 2. 验证损坏/无效数据库上传是否能够被识别与拦截
	corruptPath := filepath.Join(t.TempDir(), "corrupt.db")
	if err := os.WriteFile(corruptPath, []byte("this is not a valid sqlite database file!!!"), 0644); err != nil {
		t.Fatalf("生成损坏文件失败: %v", err)
	}

	err = backupSvc.ValidateBackup(corruptPath)
	if err == nil {
		t.Fatalf("完整性安全校验防线失效：未能识别并拦截损坏的非 SQLite 文件！")
	}
}
