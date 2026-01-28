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

// Checkout 即时结算
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
		p, err := s.ProductRepo.FindByID(tx, itemReq.ID)
		if err != nil {
			return fmt.Errorf("商品ID %d 异常", itemReq.ID)
		}

		if p.Stock < itemReq.Qty {
			return fmt.Errorf("商品 %s 库存不足", p.Name)
		}

		if err := s.ProductRepo.DecreaseStock(tx, p.ID, itemReq.Qty); err != nil {
			return err
		}

		item := model.OrderItem{OrderID: int(orderID), ProductID: p.ID, ProductName: p.Name, Price: p.Price, QtyOrdered: itemReq.Qty, QtyPicked: itemReq.Qty}
		if err := s.OrderRepo.CreateOrderItem(tx, item); err != nil {
			return err
		}

		subtotal := p.Price * float64(itemReq.Qty)
		totalPrice += subtotal
		ticketContent += fmt.Sprintf("%s\n           x%d   %.2f\n", p.Name, itemReq.Qty, subtotal)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	s.printAsync(ticketContent, totalPrice, orderID, "销售")
	return nil
}

func (s *CheckoutService) printAsync(content string, total float64, orderID int64, title string) {
	content += "----------------------\n"
	if total > 0 {
		content += fmt.Sprintf("合计:          %.2f\n", total)
		content += "----------------------\n"
	}
	content += fmt.Sprintf("单号: #%d\n", orderID)
	content += time.Now().Format("2006-01-02 15:04:05") + "\n\n\n"
	go func() {
		if err := printer.Current.PrintTicket(content); err != nil {
			log.Println("打印失败:", err)
		}
	}()
}
