package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"modern-pos/internal/handler"
	"modern-pos/internal/middleware"
	"modern-pos/internal/repository"
	"modern-pos/internal/service"
	"modern-pos/pkg/database"
	"modern-pos/pkg/printer"
	"strings"
	"syscall"
	"time"
)

//go:embed static/*
var embeddedStatic embed.FS

// getFileSystem 获取前端静态资源文件系统：优先检查磁盘 ./static 目录以支持二次定制修改，若不存在则降级使用二进制内置嵌入式资源
func getFileSystem() http.FileSystem {
	if _, err := os.Stat("./static"); err == nil {
		log.Println("[系统启动] 检测到本地 ./static 目录，优先使用磁盘外部静态资源（支持二次定制）")
		return http.Dir("./static")
	}
	log.Println("[系统启动] 未检测到外部 ./static 目录，自动启用二进制内置嵌入式静态资源")
	fsys, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		log.Fatal("[系统异常] 提取内置静态资源失败:", err)
	}
	return http.FS(fsys)
}

func main() {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	time.Local = loc

	db := database.Init()
	// 数据库连接将在优雅停机流程中安全释放

	p := printer.GetPrinter()
	printer.SetPrinter(p)

	pRepo := repository.NewProductRepo(db)
	oRepo := repository.NewOrderRepo(db)
	rRepo := repository.NewReportRepo(db)

	inventorySvc := service.NewInventoryService(pRepo, oRepo)
	checkoutSvc := service.NewCheckoutService(db, pRepo, oRepo)
	reportSvc := service.NewReportService(rRepo)
	backupSvc := service.NewBackupService(db)

	// 启动真正落地的定时热备份与完整性校验服务 (默认每 6 小时执行一次备份与校验，可由环境变量调整)
	backupInterval := 6 * time.Hour
	if intervalStr := os.Getenv("BACKUP_INTERVAL_HOURS"); intervalStr != "" {
		if hours, err := time.ParseDuration(intervalStr + "h"); err == nil && hours > 0 {
			backupInterval = hours
		}
	}
	backupSvc.StartScheduledBackup(backupInterval)

	pHandler := &handler.ProductHandler{Repo: pRepo, Inventory: inventorySvc}
	oHandler := &handler.OrderHandler{Service: checkoutSvc}
	rHandler := &handler.ReportHandler{Service: reportSvc}
	sysHandler := &handler.SystemHandler{DB: db, BackupSvc: backupSvc}
	sysHandler.ReloadDB = func() {
		newDB := database.Init()
		db = newDB // Update local variable for graceful shutdown
		pRepo.DB = newDB
		oRepo.DB = newDB
		rRepo.DB = newDB
		checkoutSvc.DB = newDB
		backupSvc.DB = newDB
		sysHandler.DB = newDB
	}

	http.Handle("/", http.FileServer(getFileSystem()))

	http.HandleFunc("/api/scan", pHandler.Scan)
	http.HandleFunc("/api/checkout", oHandler.Checkout)
	http.HandleFunc("/api/book", oHandler.Book)
	http.HandleFunc("/api/orders", oHandler.Search)
	http.HandleFunc("/api/pickup", oHandler.Pickup)
	http.HandleFunc("/api/order/update", oHandler.UpdateOrder)
	http.HandleFunc("/api/refund", oHandler.Refund)
	http.HandleFunc("/api/procurement", oHandler.GetProcurement)
	http.HandleFunc("/api/order/delete", oHandler.DeleteOrder)
	http.HandleFunc("/api/order/clear_history", oHandler.ClearHistory)

	http.HandleFunc("/api/inventory/search", pHandler.SearchProduct)
	http.HandleFunc("/api/inventory/list", pHandler.ListInventory)
	http.HandleFunc("/api/inventory/save", pHandler.AddOrUpdate)
	http.HandleFunc("/api/inventory/delete", pHandler.DeleteProduct)
	http.HandleFunc("/api/inventory/batch-delete", pHandler.BatchDelete)
	http.HandleFunc("/api/inventory/batch-category", pHandler.BatchUpdateCategory)
	http.HandleFunc("/api/inventory/procure", pHandler.Procure)

	http.HandleFunc("/api/report", rHandler.GetReport)
	http.HandleFunc("/api/reprint", oHandler.Reprint)
	http.HandleFunc("/api/refund/partial", oHandler.DoPartialRefund)

	// 高危系统管理接口绑定管理员权限校验中间件
	http.HandleFunc("/api/system/backup", handler.RequireAdmin(sysHandler.Backup))
	http.HandleFunc("/api/system/restore", handler.RequireAdmin(sysHandler.Restore))
	http.HandleFunc("/api/system/reset", handler.RequireAdmin(sysHandler.Reset))

	// 注册调试与测试数据填充路由（通过编译标签与环境变量在生产构建中彻底剔除）
	handler.RegisterDebugRoutes(pRepo, oRepo, inventorySvc)

	// 解析命令行参数与环境变量，配置安全监听网络接口
	var addrFlag string
	flag.StringVar(&addrFlag, "addr", "", "服务监听地址及端口 (例如 127.0.0.1:8080 或 192.168.1.100:8080)")
	flag.Parse()

	serverAddr := addrFlag
	if serverAddr == "" {
		serverAddr = os.Getenv("SERVER_ADDR")
	}
	if serverAddr == "" {
		host := os.Getenv("HOST")
		if host == "" {
			host = "127.0.0.1" // 默认绑定本机网络接口，防止生产环境误绑 0.0.0.0 暴露至外网
		}
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		serverAddr = fmt.Sprintf("%s:%s", host, port)
	}

	if strings.HasPrefix(serverAddr, "0.0.0.0") || strings.HasPrefix(serverAddr, ":") {
		log.Println("安全警告：当前服务绑定了所有网络接口 (0.0.0.0)，生产环境中请指定明确的网卡 IP 或 127.0.0.1，并严格配合操作系统防火墙或安全组规则")
	} else {
		log.Printf("安全启动：服务监听地址已配置至明确网络接口 %s，建议配合防火墙规则限制非信任网段访问\n", serverAddr)
	}

	log.Printf("系统环境准备完毕 (时区: Asia/Shanghai)，配置监听网络接口 %s\n", serverAddr)

	// 4. 组装全局中间件（执行顺序：Panic Recovery -> Logging -> 业务路由处理）
	var handler http.Handler = http.DefaultServeMux
	handler = middleware.Logging(handler)
	handler = middleware.Recovery(handler)

	// 5. 配置 HTTP 服务器安全超时时间，防止慢速攻击或长连接资源耗尽
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second, // 读取请求头和请求体的最大超时时间
		WriteTimeout: 30 * time.Second, // 响应回复的最大超时时间（为热备份导出和报表生成预留足够缓冲）
		IdleTimeout:  60 * time.Second, // 空闲长连接存活超时时间
	}

	// 6. 在独立协程中启动 HTTP 监听服务
	go func() {
		log.Printf("服务启动成功！收银台程序正在监听 %s\n", serverAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务异常终止: %v\n", err)
		}
	}()

	// 7. 监听操作系统停机信号，执行安全优雅关闭 (Graceful Shutdown)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("收到系统停机信号 (SIGINT/SIGTERM)，正在启动安全优雅关闭流程 (Graceful Shutdown)...")

	// 第一步：停止后台定时热备份服务，防止停机时并发生成新备份快照
	backupSvc.Stop()

	// 第二步：停止接收新请求，并等待已有请求（如结算、退款、导出）在 10 秒内安全履约完成
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP 服务优雅关闭超时或出现异常: %v\n", err)
	} else {
		log.Println("HTTP 服务已安全停止，所有活跃交易与请求已收尾完成。")
	}

	// 第三步：安全关闭数据库连接，确保所有 WAL 预写日志完全刷写并安全落盘
	if err := db.Close(); err != nil {
		log.Printf("关闭数据库连接失败: %v\n", err)
	} else {
		log.Println("数据库连接已安全释放，底层 SQLite 数据文件已安全落盘。")
	}

	log.Println("收银台系统已完成优雅停机安全退出。")
}
