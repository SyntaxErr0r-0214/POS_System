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

	p := printer.GetPrinter()
	printer.SetPrinter(p)

	// 2. 依赖注入
	pRepo := repository.NewProductRepo(db)
	oRepo := repository.NewOrderRepo(db)

	// --- 核心修改在这里 ---
	// InventoryService 现在需要 pRepo 和 oRepo 两个参数
	inventorySvc := service.NewInventoryService(pRepo, oRepo)
	checkoutSvc := service.NewCheckoutService(db, pRepo, oRepo)
	// ---------------------

	pHandler := &handler.ProductHandler{
		Repo:      pRepo,
		Inventory: inventorySvc,
	}
	oHandler := &handler.OrderHandler{Service: checkoutSvc}

	// 3. 路由
	http.Handle("/", http.FileServer(http.Dir("./static")))

	http.HandleFunc("/api/scan", pHandler.Scan)
	http.HandleFunc("/api/checkout", oHandler.Checkout)
	http.HandleFunc("/api/book", oHandler.Book)
	http.HandleFunc("/api/orders", oHandler.Search)
	http.HandleFunc("/api/pickup", oHandler.Pickup)

	http.HandleFunc("/api/product/search", pHandler.SearchProduct)
	http.HandleFunc("/api/inventory/list", pHandler.ListInventory)
	http.HandleFunc("/api/inventory/save", pHandler.AddOrUpdate)
	http.HandleFunc("/api/inventory/delete", pHandler.DeleteProduct)

	log.Println("Start: http://localhost:8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
