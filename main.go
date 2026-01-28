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

	// 2. 初始化打印机 (注意：pkg/printer 里的文件需要你自己从之前的地方拖进去)
	p := printer.GetPrinter()
	printer.SetPrinter(p)

	// 3. 依赖注入
	pRepo := repository.NewProductRepo(db)
	oRepo := repository.NewOrderRepo(db)

	svc := service.NewCheckoutService(db, pRepo, oRepo)

	pHandler := &handler.ProductHandler{Repo: pRepo}
	oHandler := &handler.OrderHandler{Service: svc}

	// 4. 路由
	http.Handle("/", http.FileServer(http.Dir("./static")))

	http.HandleFunc("/api/scan", pHandler.Scan)
	http.HandleFunc("/api/checkout", oHandler.Checkout)
	http.HandleFunc("/api/book", oHandler.Book)
	http.HandleFunc("/api/orders", oHandler.Search)
	http.HandleFunc("/api/pickup", oHandler.Pickup)

	log.Println("Start: http://localhost:8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
