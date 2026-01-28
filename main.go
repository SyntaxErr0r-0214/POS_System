package main

import (
	"log"
	"net/http"
	"pos-demo/internal/handler"
	"pos-demo/internal/repository"
	"pos-demo/internal/service"
	"pos-demo/pkg/database"
	"pos-demo/pkg/printer"
)

func main() {
	// 1. 底层初始化
	db := database.Init()
	defer db.Close()

	// 2. 初始化打印机
	p := printer.GetPrinter()
	printer.SetPrinter(p)

	// 3. 依赖注入
	pRepo := repository.NewProductRepo(db)
	oRepo := repository.NewOrderRepo(db)

	// 初始化业务服务
	checkoutSvc := service.NewCheckoutService(db, pRepo, oRepo)
	inventorySvc := service.NewInventoryService(pRepo) // <--- 【新增】库存服务

	// 初始化 Handler
	// 注意：pHandler 现在被注入了 InventoryService
	pHandler := &handler.ProductHandler{
		Repo:      pRepo,
		Inventory: inventorySvc, // <--- 【新增】注入进去
	}
	oHandler := &handler.OrderHandler{Service: checkoutSvc}

	// 4. 路由注册
	http.Handle("/", http.FileServer(http.Dir("./static")))

	// 收银相关接口
	http.HandleFunc("/api/scan", pHandler.Scan)
	http.HandleFunc("/api/checkout", oHandler.Checkout)
	http.HandleFunc("/api/book", oHandler.Book)
	http.HandleFunc("/api/orders", oHandler.Search)
	http.HandleFunc("/api/pickup", oHandler.Pickup)

	// 库存管理相关接口 (新增)
	http.HandleFunc("/api/inventory/list", pHandler.ListInventory) // <--- 【新增】列表
	http.HandleFunc("/api/inventory/save", pHandler.AddOrUpdate)   // <--- 【新增】保存

	log.Println("Start: http://localhost:8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
