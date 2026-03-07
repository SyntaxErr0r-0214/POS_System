package main

import (
	"log"
	"net/http"
	"pos-demo/internal/handler"
	"pos-demo/internal/repository"
	"pos-demo/internal/service"
	"pos-demo/pkg/database"
	"pos-demo/pkg/printer"
	"time"
)

func main() {
	// 1. 设置时区
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	time.Local = loc

	// 2. 初始化数据库
	db := database.Init()
	defer db.Close()

	p := printer.GetPrinter()
	printer.SetPrinter(p)

	// 3. 依赖注入
	pRepo := repository.NewProductRepo(db)
	oRepo := repository.NewOrderRepo(db)
	rRepo := repository.NewReportRepo(db)

	inventorySvc := service.NewInventoryService(pRepo, oRepo)
	checkoutSvc := service.NewCheckoutService(db, pRepo, oRepo)
	reportSvc := service.NewReportService(rRepo)

	pHandler := &handler.ProductHandler{Repo: pRepo, Inventory: inventorySvc}
	oHandler := &handler.OrderHandler{Service: checkoutSvc}
	rHandler := &handler.ReportHandler{Service: reportSvc}
	sysHandler := &handler.SystemHandler{}

	// 👇【新增】测试数据生成器 Handler
	testHandler := &handler.TestHandler{
		PRepo:     pRepo,
		ORepo:     oRepo,
		Inventory: inventorySvc,
	}

	// 4. 路由注册
	http.Handle("/", http.FileServer(http.Dir("./static")))

	// 业务接口
	http.HandleFunc("/api/scan", pHandler.Scan)
	http.HandleFunc("/api/checkout", oHandler.Checkout)
	http.HandleFunc("/api/book", oHandler.Book)
	http.HandleFunc("/api/orders", oHandler.Search)
	http.HandleFunc("/api/pickup", oHandler.Pickup)
	// 👇 新增这一行
	http.HandleFunc("/api/refund", oHandler.Refund)
	// 👇 新增：采购清单接口
	http.HandleFunc("/api/procurement", oHandler.GetProcurement)

	http.HandleFunc("/api/inventory/search", pHandler.SearchProduct)
	http.HandleFunc("/api/inventory/list", pHandler.ListInventory)
	http.HandleFunc("/api/inventory/save", pHandler.AddOrUpdate)
	http.HandleFunc("/api/inventory/delete", pHandler.DeleteProduct)
	http.HandleFunc("/api/inventory/batch-delete", pHandler.BatchDelete)           // 👈 新增这一行
	http.HandleFunc("/api/inventory/batch-category", pHandler.BatchUpdateCategory) // 👈 新增这一行
	// 👇 新增：批量采购入库接口
	http.HandleFunc("/api/inventory/procure", pHandler.Procure)

	http.HandleFunc("/api/report", rHandler.GetReport)
	http.HandleFunc("/api/reprint", oHandler.Reprint) // 👈 新增这一行
	http.HandleFunc("/api/refund/partial", oHandler.DoPartialRefund)

	// 系统管理接口
	http.HandleFunc("/api/system/backup", sysHandler.Backup)
	http.HandleFunc("/api/system/restore", sysHandler.Restore)

	// 👇【新增】测试数据生成接口 (浏览器访问即可生成)
	http.HandleFunc("/api/debug/seed", testHandler.SeedData)

	log.Println("Start: http://localhost:8080 (TimeZone: Asia/Shanghai)")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
