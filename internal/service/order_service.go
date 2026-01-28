package service

import (
	"fmt"
	"pos-demo/internal/model"
)

// 偷个懒，复用 CheckoutService 的结构体，因为依赖一样
// 以后如果逻辑复杂了可以拆开
func (s *CheckoutService) Book(req model.BookingRequest) (int64, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	orderID, err := s.OrderRepo.CreateOrder(tx, req.CustomerName, req.Phone, "Pending")
	if err != nil {
		return 0, err
	}

	for _, itemReq := range req.Items {
		p, err := s.ProductRepo.FindByID(tx, itemReq.ID)
		if err != nil {
			continue
		} // 商品没了就跳过

		// 预订不扣库存，qty_picked = 0
		item := model.OrderItem{OrderID: int(orderID), ProductID: p.ID, ProductName: p.Name, Price: p.Price, QtyOrdered: itemReq.Qty, QtyPicked: 0}
		s.OrderRepo.CreateOrderItem(tx, item)
	}

	return orderID, tx.Commit()
}

func (s *CheckoutService) SearchOrders(query string) ([]model.Order, error) {
	orders, err := s.OrderRepo.SearchPendingOrders(query)
	if err != nil {
		return nil, err
	}

	// 填充明细
	for i := range orders {
		items, _ := s.OrderRepo.GetItemsByOrderID(orders[i].ID)
		orders[i].Items = items
	}
	return orders, nil
}

func (s *CheckoutService) Pickup(req model.PickupRequest) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ticketContent := "---- 提货小票 ----\n"

	for _, itemReq := range req.Items {
		// 1. 查明细
		orderItem, err := s.OrderRepo.GetItemByID(tx, itemReq.ItemID)
		if err != nil {
			return err
		}

		// 2. 查实物库存
		p, err := s.ProductRepo.FindByID(tx, orderItem.ProductID)
		if err != nil {
			return err
		}
		if p.Stock < itemReq.Qty {
			return fmt.Errorf("%s 库存不足", p.Name)
		}

		// 3. 扣减
		s.ProductRepo.DecreaseStock(tx, p.ID, itemReq.Qty)
		s.OrderRepo.UpdatePickedQty(tx, itemReq.ItemID, itemReq.Qty)

		ticketContent += fmt.Sprintf("%s\n   本次取:%d  (剩余:%d)\n", p.Name, itemReq.Qty, orderItem.QtyOrdered-orderItem.QtyPicked-itemReq.Qty)
	}

	// 4. 检查完结
	isComplete, _ := s.OrderRepo.CheckOrderComplete(tx, req.OrderID)
	if isComplete {
		s.OrderRepo.UpdateStatus(tx, req.OrderID, "Completed")
		ticketContent += "\n[该订单已全部提货完成]\n"
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	s.printAsync(ticketContent, 0, int64(req.OrderID), "提货")
	return nil
}
