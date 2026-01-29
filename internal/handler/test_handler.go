package handler

import (
	"fmt"
	"math/rand"
	"net/http"
	"pos-demo/internal/model"
	"pos-demo/internal/repository"
	"pos-demo/internal/service"
	"time"
)

type TestHandler struct {
	PRepo     *repository.ProductRepo
	ORepo     *repository.OrderRepo
	Inventory *service.InventoryService
}

// SeedData 一键生成测试数据
func (h *TestHandler) SeedData(w http.ResponseWriter, r *http.Request) {
	// 1. 预设商品库 (如果库存为空，就加这些)
	products := []model.Product{
		{Barcode: "1001", Name: "农夫山泉 550ml", Price: 2.00, CostPrice: 1.20, Stock: 100},
		{Barcode: "1002", Name: "可口可乐 330ml", Price: 3.00, CostPrice: 2.20, Stock: 200},
		{Barcode: "1003", Name: "精品五花肉 (kg)", Price: 35.00, CostPrice: 28.00, Stock: 50},
		{Barcode: "1004", Name: "东北大米 10kg", Price: 65.00, CostPrice: 45.00, Stock: 20},
		{Barcode: "1005", Name: "海天生抽 500ml", Price: 9.90, CostPrice: 6.50, Stock: 60},
		{Barcode: "1006", Name: "卫龙辣条", Price: 5.00, CostPrice: 2.50, Stock: 500},
		{Barcode: "1007", Name: "有机西红柿 (kg)", Price: 8.50, CostPrice: 4.00, Stock: 30},
		{Barcode: "1008", Name: "纯牛奶 1L", Price: 12.00, CostPrice: 9.50, Stock: 40},
		{Barcode: "1009", Name: "乐事薯片", Price: 7.50, CostPrice: 4.50, Stock: 80},
		{Barcode: "1010", Name: "洁柔抽纸 (提)", Price: 19.90, CostPrice: 14.00, Stock: 100},
	}

	// 批量插入商品
	for _, p := range products {
		// 简单查重：如果条码不存在才插入
		if exist, _ := h.PRepo.FindByBarcode(p.Barcode); exist == nil {
			h.PRepo.Create(p)
		}
	}

	// 重新获取所有商品（我们要用到它们的真实ID）
	allProducts, _ := h.PRepo.GetAll()
	if len(allProducts) == 0 {
		w.Write([]byte("错误：商品库为空"))
		return
	}

	// 2. 生成过去 7 天的订单
	tx, _ := h.ORepo.DB.Begin()
	defer tx.Commit()

	count := 0
	// 遍历过去 7 天 (从 7天前 到 昨天)
	for i := 7; i >= 0; i-- {
		// 生成当天的随机时间
		dayBase := time.Now().AddDate(0, 0, -i)

		// 每天生成 3-8 个订单
		ordersPerDay := rand.Intn(6) + 3

		for j := 0; j < ordersPerDay; j++ {
			// 随机时间：上午9点 ~ 晚上9点
			randomHour := rand.Intn(12) + 9
			orderTime := time.Date(dayBase.Year(), dayBase.Month(), dayBase.Day(), randomHour, rand.Intn(60), 0, 0, time.Local)

			// 创建历史订单
			orderID, _ := h.ORepo.CreateOrderWithTime(tx, "散客", "", "Completed", orderTime)

			// 每个订单随机买 1-5 种商品
			itemsCount := rand.Intn(5) + 1
			for k := 0; k < itemsCount; k++ {
				// 随机挑一个商品
				p := allProducts[rand.Intn(len(allProducts))]
				qty := rand.Intn(3) + 1 // 买 1-3 个

				// 插入明细
				item := model.OrderItem{
					OrderID: int(orderID), ProductID: p.ID, ProductName: p.Name,
					Price: p.Price, QtyOrdered: qty, QtyPicked: qty,
				}
				h.ORepo.CreateOrderItem(tx, item)
			}
			count++
		}
	}

	w.Write([]byte(fmt.Sprintf("✅ 成功！已补全商品库，并生成了 %d 条历史订单数据（覆盖过去7天）。\n现在请去【营收报表】刷新查看！", count)))
}
