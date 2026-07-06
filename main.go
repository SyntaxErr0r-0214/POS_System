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
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	time.Local = loc

	db := database.Init()
	defer db.Close()

	p := printer.GetPrinter()
	printer.SetPrinter(p)

	pRepo := repository.NewProductRepo(db)
	oRepo := repository.NewOrderRepo(db)
	rRepo := repository.NewReportRepo(db)

	inventorySvc := service.NewInventoryService(pRepo, oRepo)
	checkoutSvc := service.NewCheckoutService(db, pRepo, oRepo)
	reportSvc := service.NewReportService(rRepo)

	pHandler := &handler.ProductHandler{Repo: pRepo, Inventory: inventorySvc}
	oHandler := &handler.OrderHandler{Service: checkoutSvc}
	rHandler := &handler.ReportHandler{Service: reportSvc}
	sysHandler := &handler.SystemHandler{DB: db}
	testHandler := &handler.TestHandler{
		PRepo:     pRepo,
		ORepo:     oRepo,
		Inventory: inventorySvc,
	}

	http.Handle("/", http.FileServer(http.Dir("./static")))

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

	http.HandleFunc("/api/system/backup", sysHandler.Backup)
	http.HandleFunc("/api/system/restore", sysHandler.Restore)
	http.HandleFunc("/api/system/reset", sysHandler.Reset)

	http.HandleFunc("/api/debug/seed", testHandler.SeedData)

	log.Println("Start: http://localhost:8080 (TimeZone: Asia/Shanghai)")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
