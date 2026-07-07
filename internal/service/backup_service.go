package service

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// BackupService 负责落地数据库热备份、完整性校验与定时备份任务。
type BackupService struct {
	DB            *sql.DB
	BackupDir     string
	RetentionDays int
	ticker        *time.Ticker
	stopChan      chan struct{}
}

// NewBackupService 创建备份服务实例，默认备份目录为 ./backups，默认保留期 30 天。
func NewBackupService(db *sql.DB) *BackupService {
	dir := os.Getenv("BACKUP_DIR")
	if dir == "" {
		dir = "./backups"
	}
	return &BackupService{
		DB:            db,
		BackupDir:     dir,
		RetentionDays: 30,
		stopChan:      make(chan struct{}),
	}
}

// StartScheduledBackup 启动真正有效的后台定时热备份与校验服务。
// 自动在后台周期性执行备份、完整性验证及过期文件清理。
func (s *BackupService) StartScheduledBackup(interval time.Duration) {
	if err := os.MkdirAll(s.BackupDir, 0755); err != nil {
		log.Printf("创建数据备份目录失败: %v", err)
		return
	}

	log.Printf("数据可靠性保障：定时热备份与完整性校验服务已启动，保存路径 [%s]，备份周期 [%v]，历史备份保留期 [%d 天]", s.BackupDir, interval, s.RetentionDays)

	s.ticker = time.NewTicker(interval)
	go func() {
		// 首次启动时延迟 1 分钟执行一次初始热备份与检验
		select {
		case <-time.After(1 * time.Minute):
			if _, err := s.PerformBackup(); err != nil {
				log.Printf("自动热备份执行异常: %v", err)
			}
			s.CleanExpiredBackups()
		case <-s.stopChan:
			log.Println("后台定时热备份与校验服务已停止。")
			return
		}

		for {
			select {
			case <-s.ticker.C:
				if _, err := s.PerformBackup(); err != nil {
					log.Printf("定时热备份与完整性校验失败: %v", err)
				}
				s.CleanExpiredBackups()
			case <-s.stopChan:
				log.Println("后台定时热备份与校验服务已优雅安全停止。")
				return
			}
		}
	}()
}

// Stop 优雅停止定时热备份服务。
func (s *BackupService) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	if s.stopChan != nil {
		close(s.stopChan)
	}
}

// PerformBackup 执行真正的 SQLite 热备份（VACUUM INTO）并自动进行完整性校验（PRAGMA integrity_check）。
func (s *BackupService) PerformBackup() (string, error) {
	if err := os.MkdirAll(s.BackupDir, 0755); err != nil {
		return "", fmt.Errorf("创建备份目录失败: %v", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupFileName := fmt.Sprintf("pos_backup_%s.db", timestamp)
	backupPath := filepath.Join(s.BackupDir, backupFileName)

	// 1. 使用 SQLite 原生 VACUUM INTO 执行原子热备份（不阻塞生产环境任何并发读写交易）
	_ = os.Remove(backupPath)
	query := fmt.Sprintf("VACUUM INTO '%s'", backupPath)
	if _, err := s.DB.Exec(query); err != nil {
		return "", fmt.Errorf("执行热备份指令 (VACUUM INTO) 失败: %v", err)
	}

	// 2. 备份文件深度完整性校验 (Backup Validation)
	if err := s.ValidateBackup(backupPath); err != nil {
		// 校验未通过，立刻安全销毁损坏的临时备份文件
		_ = os.Remove(backupPath)
		return "", fmt.Errorf("备份完整性校验未通过，已安全丢弃该无效备份: %v", err)
	}

	log.Printf("数据保护：热备份及完整性校验成功，文件存档至 [%s]", backupPath)
	return backupPath, nil
}

// ValidateBackup 打开备份数据库文件并执行 PRAGMA integrity_check 深度校验。
func (s *BackupService) ValidateBackup(backupPath string) error {
	info, err := os.Stat(backupPath)
	if err != nil || info.Size() == 0 {
		return fmt.Errorf("备份文件不存在或大小为 0")
	}

	backupDB, err := sql.Open("sqlite", backupPath)
	if err != nil {
		return fmt.Errorf("无法连接至备份数据库文件: %v", err)
	}
	defer backupDB.Close()

	// 执行 SQLite 内置深度完整性校验
	var integrityResult string
	err = backupDB.QueryRow("PRAGMA integrity_check;").Scan(&integrityResult)
	if err != nil {
		return fmt.Errorf("执行 PRAGMA integrity_check 查询异常: %v", err)
	}
	if integrityResult != "ok" {
		return fmt.Errorf("数据库底层结构完整性受损，校验返回: %s", integrityResult)
	}

	// 进一步验证收银系统核心业务表是否存在
	var tableCount int
	err = backupDB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('products', 'orders', 'order_items')").Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("校验核心业务表结构失败: %v", err)
	}
	if tableCount < 3 {
		return fmt.Errorf("备份数据不完整：核心业务表缺失（仅检测到 %d 个表）", tableCount)
	}

	return nil
}

// CleanExpiredBackups 自动清理超出保留期的历史备份文件。
func (s *BackupService) CleanExpiredBackups() {
	files, err := os.ReadDir(s.BackupDir)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -s.RetentionDays)
	var removedCount int

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		info, err := f.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(s.BackupDir, f.Name())
			if err := os.Remove(path); err == nil {
				removedCount++
			}
		}
	}

	if removedCount > 0 {
		log.Printf("数据维护：已自动清理 %d 个超过 %d 天保留期的历史备份文件", removedCount, s.RetentionDays)
	}
}

// WriteBackupToStream 执行临时热备份与完整性校验，并将通过校验的备份文件流式输出给请求方。
func (s *BackupService) WriteBackupToStream(w io.Writer) error {
	backupPath, err := s.PerformBackup()
	if err != nil {
		return err
	}
	// 接口导出完成后自动清理本次临时的热备份文件（定期存档由后台定时任务负责）
	defer os.Remove(backupPath)

	file, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("打开校验通过的备份文件失败: %v", err)
	}
	defer file.Close()

	_, err = io.Copy(w, file)
	return err
}
