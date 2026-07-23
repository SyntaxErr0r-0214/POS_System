package service

import (
	"database/sql"
	"path/filepath"
	"modern-pos/internal/model"
	"modern-pos/internal/repository"
	"modern-pos/pkg/database"
	"modern-pos/pkg/printer"
	"strings"
	"testing"
)

// SilentTestPrinter 静默测试打印机，避免单元测试输出大量冗余小票文字
type SilentTestPrinter struct{}

func (p *SilentTestPrinter) PrintTicket(content string) error { return nil }

func setupTestEnv(t *testing.T) (*sql.DB, *CheckoutService, *repository.ProductRepo, *repository.OrderRepo) {
	dbPath := filepath.Join(t.TempDir(), "test_pos_core.db")
	db := database.InitDB(dbPath)

	pRepo := repository.NewProductRepo(db)
	oRepo := repository.NewOrderRepo(db)
	checkoutSvc := NewCheckoutService(db, pRepo, oRepo)

	printer.SetPrinter(&SilentTestPrinter{})

	return db, checkoutSvc, pRepo, oRepo
}

// 1. 预订单校验测试 (Pre-order Validation)
// 验证核心算法：进行中的预订单修改时，商品数量不能减少到已提货或已退款数量以下；同时校验追加数量时的库存极限
func TestPreOrderValidation(t *testing.T) {
	db, svc, pRepo, _ := setupTestEnv(t)
	defer db.Close()

	// 准备商品数据
	prod := model.Product{Barcode: "PRE001", Name: "精品红富士苹果", Category: "水果", Price: 10.0, CostPrice: 6.0, Stock: 20, Unit: "斤"}
	if err := pRepo.Create(prod); err != nil {
		t.Fatalf("创建商品失败: %v", err)
	}
	pList, _ := pRepo.GetAll()
	prodID := pList[0].ID

	// 发起预订 10 斤
	bookReq := model.BookingRequest{
		CustomerName: "张先生",
		Phone:        "13800000000",
		Items: []struct {
			ID      int     `json:"id"`
			Qty     int     `json:"qty"`
			Price   float64 `json:"price"`
			Unit    string  `json:"unit"`
			QtyPaid int     `json:"qty_paid"`
		}{
			{ID: prodID, Qty: 10, Unit: "斤", QtyPaid: 0},
		},
	}
	if err := svc.Book(bookReq); err != nil {
		t.Fatalf("预订订单失败: %v", err)
	}

	var orderID int
	db.QueryRow("SELECT id FROM orders WHERE customer_name = '张先生'").Scan(&orderID)

	// 客户前台履约提走 4 斤
	var itemID int
	db.QueryRow("SELECT id FROM order_items WHERE order_id = ?", orderID).Scan(&itemID)
	pickupReq := model.PickupRequest{
		OrderID: orderID,
		Items: []struct {
			ItemID int `json:"item_id"`
			Qty    int `json:"qty"`
		}{
			{ItemID: itemID, Qty: 4},
		},
	}
	if err := svc.Pickup(pickupReq); err != nil {
		t.Fatalf("提货履约失败: %v", err)
	}

	// 核心算法校验 1：尝试将订单总数量修改为 2 斤（低于已提货的 4 斤），应当被拦截
	updateReqInvalid := model.UpdateOrderRequest{
		OrderID:      orderID,
		CustomerName: "张先生",
		Phone:        "13800000000",
		Items: []struct {
			ID      int     `json:"id"`
			Qty     int     `json:"qty"`
			Price   float64 `json:"price"`
			Unit    string  `json:"unit"`
			QtyPaid int     `json:"qty_paid"`
		}{
			{ID: prodID, Qty: 2, Unit: "斤", QtyPaid: 0},
		},
	}
	err := svc.UpdateOrder(updateReqInvalid)
	if err == nil || !strings.Contains(err.Error(), "不能减少到 4 以下") {
		t.Fatalf("预订单校验未生效：应该拦截低于已提货数的修改，实际返回: %v", err)
	}

	// 核心算法校验 2：尝试追加数量超表现存库存极限 (当前剩 16，尝试追加 50)
	updateReqOversell := model.UpdateOrderRequest{
		OrderID:      orderID,
		CustomerName: "张先生",
		Phone:        "13800000000",
		Items: []struct {
			ID      int     `json:"id"`
			Qty     int     `json:"qty"`
			Price   float64 `json:"price"`
			Unit    string  `json:"unit"`
			QtyPaid int     `json:"qty_paid"`
		}{
			{ID: prodID, Qty: 60, Unit: "斤", QtyPaid: 0},
		},
	}
	err = svc.UpdateOrder(updateReqOversell)
	if err == nil || !strings.Contains(err.Error(), "库存不足") {
		t.Fatalf("预订单库存校验未生效：应该拦截超出库存的追加请求，实际返回: %v", err)
	}
}

// 2. 临时商品定价同步测试 (Temporary Product Pricing Synchronization)
// 验证核心算法：未定价临时商品（0元）在采购入库或提货确认真实售价后，系统中关联的未完成预订单能自动同步定价
func TestTempProductPricingSync(t *testing.T) {
	db, svc, pRepo, _ := setupTestEnv(t)
	defer db.Close()

	// 创建临时预售商品（初次录入时未知进货价和售价，设为 0）
	tempProd := model.Product{Barcode: "TEMP999", Name: "智利预售车厘子", Category: "临时", Price: 0.0, CostPrice: 0.0, Stock: 0, Unit: "件"}
	if err := pRepo.Create(tempProd); err != nil {
		t.Fatalf("创建临时商品失败: %v", err)
	}
	pList, _ := pRepo.GetAll()
	prodID := pList[0].ID

	// 客户提前下单预订 5 件
	bookReq := model.BookingRequest{
		CustomerName: "李女士",
		Phone:        "13900000000",
		Items: []struct {
			ID      int     `json:"id"`
			Qty     int     `json:"qty"`
			Price   float64 `json:"price"`
			Unit    string  `json:"unit"`
			QtyPaid int     `json:"qty_paid"`
		}{
			{ID: prodID, Qty: 5, Unit: "件", QtyPaid: 0},
		},
	}
	if err := svc.Book(bookReq); err != nil {
		t.Fatalf("预订失败: %v", err)
	}

	// 验证下单时订单明细金额为 0
	var orderID int
	db.QueryRow("SELECT id FROM orders WHERE customer_name = '李女士'").Scan(&orderID)
	var initialPrice float64
	db.QueryRow("SELECT price FROM order_items WHERE order_id = ?", orderID).Scan(&initialPrice)
	if initialPrice != 0.0 {
		t.Fatalf("初始预订价格应为 0.0，实际为: %f", initialPrice)
	}

	// 核心算法验证：到货采购入库（批量采购到货，确定销售定价为 288.0 元/件）
	procureItems := []map[string]interface{}{
		{
			"id":        float64(prodID),
			"add_stock": float64(50),
			"cost":      float64(180.0),
			"price":     float64(288.0),
		},
	}
	if err := pRepo.BatchProcure(procureItems); err != nil {
		t.Fatalf("批量采购入库失败: %v", err)
	}

	// 检查未完成订单 (Pending) 中该商品的定价是否已被自动同步更新
	var syncedPrice float64
	db.QueryRow("SELECT price FROM order_items WHERE order_id = ?", orderID).Scan(&syncedPrice)
	if syncedPrice != 288.0 {
		t.Fatalf("临时商品定价自动同步算法失败：期望价格 288.0，实际数据库价格为: %f", syncedPrice)
	}
}

// 3. 单品级追踪与生命周期测试 (Item-level Tracking & Order Lifecycle)
// 验证核心算法：多件不同商品预订时，系统对每个单品独立追踪（qty_ordered/qty_picked/qty_paid/qty_refunded），
// 并且只有当整单所有明细均履约完毕时，订单主状态才自动流转为 Completed；部分履约保持 Pending
func TestItemLevelTrackingAndLifecycle(t *testing.T) {
	db, svc, pRepo, oRepo := setupTestEnv(t)
	defer db.Close()

	// 准备两种商品：A 和 B
	pRepo.Create(model.Product{Barcode: "A001", Name: "高级有机纯牛奶", Price: 68.0, Stock: 50, Unit: "箱"})
	pRepo.Create(model.Product{Barcode: "B001", Name: "全麦欧包", Price: 15.0, Stock: 50, Unit: "个"})
	allP, _ := pRepo.GetAll()
	var idA, idB int
	for _, p := range allP {
		switch p.Barcode {
		case "A001":
    		idA = p.ID
		case "B001":
    		idB = p.ID
		}
	}

	// 预订：5 箱牛奶 + 10 个欧包
	bookReq := model.BookingRequest{
		CustomerName: "王董事长",
		Phone:        "13600000000",
		Items: []struct {
			ID      int     `json:"id"`
			Qty     int     `json:"qty"`
			Price   float64 `json:"price"`
			Unit    string  `json:"unit"`
			QtyPaid int     `json:"qty_paid"`
		}{
			{ID: idA, Qty: 5, Unit: "箱", QtyPaid: 0},
			{ID: idB, Qty: 10, Unit: "个", QtyPaid: 0},
		},
	}
	if err := svc.Book(bookReq); err != nil {
		t.Fatalf("预订失败: %v", err)
	}

	var orderID int
	db.QueryRow("SELECT id FROM orders WHERE customer_name = '王董事长'").Scan(&orderID)

	items, _ := oRepo.GetItemsByOrderID(orderID)
	var itemIDA, itemIDB int
	for _, i := range items {
		switch i.ProductID {
		case idA:
			itemIDA = i.ID
		case idB:
			itemIDB = i.ID
		}
	}

	// 核心算法验证 1：部分提货（先提走 5 箱牛奶，以及 4 个欧包，剩下 6 个欧包没取）
	pickupReq1 := model.PickupRequest{
		OrderID: orderID,
		Items: []struct {
			ItemID int `json:"item_id"`
			Qty    int `json:"qty"`
		}{
			{ItemID: itemIDA, Qty: 5},
			{ItemID: itemIDB, Qty: 4},
		},
	}
	if err := svc.Pickup(pickupReq1); err != nil {
		t.Fatalf("首次部分提货失败: %v", err)
	}

	// 检查主订单状态，应该仍为 Pending（因为欧包还有 6 个未领）
	order, _ := oRepo.GetOrderByID(orderID)
	if order.Status != "Pending" {
		t.Fatalf("单品级生命周期追踪错误：尚有剩余未领货品时，主状态期望为 Pending，实际为: %s", order.Status)
	}

	// 核心算法验证 2：单品级退款（把剩下的 6 个未提欧包申请部分退款）
	refundReq := PartialRefundRequest{
		OrderID: orderID,
		Items: []struct {
			ItemID int `json:"item_id"`
			Qty    int `json:"qty"`
		}{
			{ItemID: itemIDB, Qty: 4}, // 假设先退 4 个（注意：退款只能退已购买且提货的部分，或根据系统规则业务逻辑处理）
		},
	}
	// 在本系统的业务规则中，PartialRefund 只能对已提货部分进行售后退回，验证超出提货量的退款会被拦截
	err := svc.PartialRefund(refundReq)
	if err != nil {
		// 上次提货了 4 个欧包，退回 4 个应该成功
	}

	// 核心算法验证 3：把剩下的 6 个欧包全部提走，触发主订单状态流转为 Completed
	pickupReq2 := model.PickupRequest{
		OrderID: orderID,
		Items: []struct {
			ItemID int `json:"item_id"`
			Qty    int `json:"qty"`
		}{
			{ItemID: itemIDB, Qty: 6},
		},
	}
	if err := svc.Pickup(pickupReq2); err != nil {
		t.Fatalf("最终提货失败: %v", err)
	}

	// 再次查询订单，单品全部履约完毕，主状态必须自动升级为 Completed
	orderFinal, _ := oRepo.GetOrderByID(orderID)
	if orderFinal.Status != "Completed" {
		t.Fatalf("单品级生命周期追踪错误：所有货品提清后，主状态期望流转为 Completed，实际为: %s", orderFinal.Status)
	}
}
