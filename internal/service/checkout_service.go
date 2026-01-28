package service

import (
	"database/sql"
	"fmt"
	"log"
	"pos-demo/internal/model"
	"pos-demo/internal/repository"
	"pos-demo/pkg/printer"
	"time"
)

type CheckoutService struct {
	DB          *sql.DB
	ProductRepo *repository.ProductRepo
	OrderRepo   *repository.OrderRepo
}

func NewCheckoutService(db *sql.DB, pRepo *repository.ProductRepo, oRepo *repository.OrderRepo) *CheckoutService {
	return &CheckoutService{DB: db, ProductRepo: pRepo, OrderRepo: oRepo}
}

// Checkout 即时结算 (支持改价)
func (s *CheckoutService) Checkout(req model.CheckoutRequest) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	orderID, err := s.OrderRepo.CreateOrder(tx, "散客", "", "Completed")
	if err != nil {
		return err
	}

	var ticketContent = "---- 销售小票 ----\n"
	var totalPrice float64 = 0

	for _, itemReq := range req.Items {
		// 1. 查商品基础信息
		p, err := s.ProductRepo.FindByID(tx, itemReq.ID)
		if err != nil {
			return fmt.Errorf("商品ID %d 异常", itemReq.ID)
		}

		// 2. 校验库存
		if p.Stock < itemReq.Qty {
			return fmt.Errorf("商品 %s 库存不足", p.Name)
		}

		// 3. 扣减库存
		if err := s.ProductRepo.DecreaseStock(tx, p.ID, itemReq.Qty); err != nil {
			return err
		}

		// --- 核心修改：确定最终价格 ---
		// 默认使用商品原价
		finalPrice := p.Price

		// 这里的逻辑是：我们信任前端传来的价格。
		// 前端代码里，如果不改价，传的就是原价；如果改了，传的就是新价格。
		// 只要前端传了有效价格(>=0)，我们就以这个价格为准。
		if itemReq.Price >= 0 {
			finalPrice = itemReq.Price
		}
		// -------------------------

		// 4. 记录订单明细 (使用 finalPrice)
		item := model.OrderItem{
			OrderID:     int(orderID),
			ProductID:   p.ID,
			ProductName: p.Name,
			Price:       finalPrice, // ⚠️ 这里存入实际销售价
			QtyOrdered:  itemReq.Qty,
			QtyPicked:   itemReq.Qty,
		}
		if err := s.OrderRepo.CreateOrderItem(tx, item); err != nil {
			return err
		}

		// 5. 计算小计 (使用 finalPrice)
		subtotal := finalPrice * float64(itemReq.Qty)
		totalPrice += subtotal

		// 6. 拼凑小票内容 (显示单价，方便核对)
		// 格式：商品名
		//       @单价  x数量   小计
		ticketContent += fmt.Sprintf("%s\n", p.Name)
		ticketContent += fmt.Sprintf("   @%.2f  x%d   %.2f\n", finalPrice, itemReq.Qty, subtotal)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	s.printAsync(ticketContent, totalPrice, orderID, "销售")
	return nil
}

// printAsync 异步打印，不阻塞主流程
func (s *CheckoutService) printAsync(content string, total float64, orderID int64, title string) {
	content += "----------------------\n"
	if total > 0 {
		content += fmt.Sprintf("合计:          %.2f\n", total)
		content += "----------------------\n"
	}
	content += fmt.Sprintf("单号: #%d\n", orderID)
	content += time.Now().Format("2006-01-02 15:04:05") + "\n\n\n"

	// 启动一个 goroutine 去打印
	go func() {
		if err := printer.Current.PrintTicket(content); err != nil {
			log.Println("打印失败:", err)
		}
	}()
}
