package service

import (
	"database/sql"
	"fmt"
	"log"
	"modern-pos/internal/model"
	"modern-pos/internal/repository"
	"modern-pos/pkg/printer"
	"strings"
	"time"
)

// StoreName 店铺名称常量
const StoreName = "POS System"

type CheckoutService struct {
	DB          *sql.DB
	ProductRepo *repository.ProductRepo
	OrderRepo   *repository.OrderRepo
}

func NewCheckoutService(db *sql.DB, pRepo *repository.ProductRepo, oRepo *repository.OrderRepo) *CheckoutService {
	return &CheckoutService{DB: db, ProductRepo: pRepo, OrderRepo: oRepo}
}

// Checkout 实时结算 (现货交易)
func (s *CheckoutService) Checkout(req model.CheckoutRequest) error {
	if len(req.Items) == 0 {
		return fmt.Errorf("订单不能为空，请至少添加一件商品")
	}
	for _, itemReq := range req.Items {
		if itemReq.Qty <= 0 {
			return fmt.Errorf("商品数量必须大于0")
		}
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 创建订单 (Completed)
	orderID, err := s.OrderRepo.CreateOrder(tx, "散客", "", "Completed")
	if err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("   %s\n", StoreName))

	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("单号:#%d\n", orderID))
	sb.WriteString(fmt.Sprintf("时间:%s\n", time.Now().Format("06-01-02 15:04")))
	sb.WriteString("-------------------------------\n")
	sb.WriteString("商品名称         数量      金额\n")

	var totalPrice float64 = 0

	for _, itemReq := range req.Items {
		p, err := s.ProductRepo.FindByID(tx, itemReq.ID)
		if err != nil {
			return fmt.Errorf("商品ID %d 异常", itemReq.ID)
		}

		if p.Category == "临时" {
			if itemReq.Price > 0 {
				p.Price = itemReq.Price
				_, _ = tx.Exec("UPDATE products SET price = ? WHERE id = ?", p.Price, p.ID)
			}
		} else {
			if p.Stock < itemReq.Qty {
				return fmt.Errorf("商品 %s 库存不足(剩%d)", p.Name, p.Stock)
			}
			if err := s.ProductRepo.DecreaseStock(tx, p.ID, itemReq.Qty); err != nil {
				return err
			}
		}

		finalPrice := p.Price
		if itemReq.Price > 0 {
			finalPrice = itemReq.Price
		}

		item := model.OrderItem{
			OrderID: int(orderID), ProductID: p.ID, ProductName: p.Name,
			Price: finalPrice, QtyOrdered: itemReq.Qty, QtyPicked: itemReq.Qty, QtyPaid: itemReq.Qty, Unit: p.Unit,
		}
		if itemReq.Unit != "" {
			item.Unit = itemReq.Unit
		}
		if err := s.OrderRepo.CreateOrderItem(tx, item); err != nil {
			return err
		}

		subtotal := finalPrice * float64(itemReq.Qty)
		totalPrice += subtotal

		displayUnit := ""
		if item.Unit != "" {
			displayUnit = "/" + item.Unit
		}
		sb.WriteString(fmt.Sprintf("%s\n", p.Name))
		sb.WriteString(fmt.Sprintf(" %-7.2f%-3s x%-3d %8.2f\n", finalPrice, displayUnit, itemReq.Qty, subtotal))
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("总计金额:      RMB %8.2f\n", totalPrice))
	sb.WriteString(fmt.Sprintf("已收金额:      RMB %8.2f\n", totalPrice))
	sb.WriteString(fmt.Sprintf("未收金额:      RMB %8.2f\n", 0.00))
	sb.WriteString("-------------------------------\n")
	sb.WriteString("    谢谢惠顾，欢迎下次光临！\n\n\n\n")

	s.printAsync(sb.String())
	return nil
}

// Book 预订 (静默模式)
func (s *CheckoutService) Book(req model.BookingRequest) error {
	if len(req.Items) == 0 {
		return fmt.Errorf("订单不能为空，请至少添加一件商品")
	}
	for _, itemReq := range req.Items {
		if itemReq.Qty <= 0 {
			return fmt.Errorf("商品数量必须大于0")
		}
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 创建订单 (Pending)
	orderID, err := s.OrderRepo.CreateOrder(tx, req.CustomerName, req.Phone, "Pending")
	if err != nil {
		return err
	}

	var hasPaid bool
	var paidTotal float64
	type paidEntry struct {
		Name  string
		Unit  string
		Price float64
		Qty   int
	}
	var paidItems []paidEntry

	for _, itemReq := range req.Items {
		p, err := s.ProductRepo.FindByID(tx, itemReq.ID)
		if err != nil {
			return fmt.Errorf("商品ID %d 异常", itemReq.ID)
		}

		// 如果是临时商品，使用前端传来的初始定价，并同步回商品表，以供入库单界面查看
		if p.Category == "临时" && itemReq.Price > 0 {
			p.Price = itemReq.Price
			_, _ = tx.Exec("UPDATE products SET price = ? WHERE id = ?", p.Price, p.ID)
		}

		qtyPickedNow := itemReq.QtyPaid
		if qtyPickedNow > itemReq.Qty {
			qtyPickedNow = itemReq.Qty
		}

		unit := itemReq.Unit
		if unit == "" {
			unit = p.Unit
		}

		if p.Category != "临时" {
			// 取消物理库存预扣减，仅作为意向单
		}

		if qtyPickedNow > 0 {
			hasPaid = true
			sub := p.Price * float64(qtyPickedNow)
			paidTotal += sub
			paidItems = append(paidItems, paidEntry{Name: p.Name, Unit: unit, Price: p.Price, Qty: qtyPickedNow})
		}

		item := model.OrderItem{
			OrderID: int(orderID), ProductID: p.ID, ProductName: p.Name,
			Price: p.Price, QtyOrdered: itemReq.Qty, QtyPicked: 0, QtyPaid: itemReq.QtyPaid, Unit: unit,
		}
		if err := s.OrderRepo.CreateOrderItem(tx, item); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// 只有当存在预付款并提货时才打印小票
	if hasPaid {
		var sb strings.Builder
		sb.WriteString("-------------------------------\n")
		sb.WriteString(fmt.Sprintf("   %s\n", StoreName))
		sb.WriteString("-------------------------------\n")
		sb.WriteString(fmt.Sprintf("单号:#%d\n", orderID))
		sb.WriteString(fmt.Sprintf("时间:%s\n", time.Now().Format("06-01-02 15:04")))
		if req.CustomerName != "" {
			sb.WriteString(fmt.Sprintf("客户:%s\n", req.CustomerName))
		}
		sb.WriteString("-------------------------------\n")
		for _, pe := range paidItems {
			sb.WriteString(fmt.Sprintf("%s\n", pe.Name))
			displayUnit := ""
			if pe.Unit != "" {
				displayUnit = "/" + pe.Unit
			}
			sub := pe.Price * float64(pe.Qty)
			sb.WriteString(fmt.Sprintf(" %-7.2f%-3s x%-3d %8.2f\n", pe.Price, displayUnit, pe.Qty, sub))
		}
		sb.WriteString("-------------------------------\n")
		sb.WriteString(fmt.Sprintf("本次实收:      RMB %8.2f\n", paidTotal))
		sb.WriteString("-------------------------------\n")
		sb.WriteString("\n\n\n")
		s.printAsync(sb.String())
	}
	return nil
}

// UpdateOrder 修改进行中的预订单
func (s *CheckoutService) UpdateOrder(req model.UpdateOrderRequest) error {
	if len(req.Items) == 0 {
		return fmt.Errorf("订单明细不能为空")
	}
	for _, itemReq := range req.Items {
		if itemReq.Qty <= 0 {
			return fmt.Errorf("商品数量必须大于0")
		}
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 获取原订单及明细
	order, err := s.OrderRepo.GetOrderByIDTx(tx, req.OrderID)
	if err != nil {
		return err
	}
	if order.Status != "Pending" {
		return fmt.Errorf("只能修改进行中的订单")
	}

	oldItems, err := s.OrderRepo.GetItemsByOrderIDTx(tx, req.OrderID)
	if err != nil {
		return err
	}

	// 将旧明细转为 map 方便对比
	oldItemMap := make(map[int]model.OrderItem)
	for _, item := range oldItems {
		oldItemMap[int(item.ProductID)] = item
	}

	// 2. 对比新明细
	newItemIDs := make(map[int]bool)

	for _, itemReq := range req.Items {
		newItemIDs[itemReq.ID] = true

		oldItem, exists := oldItemMap[itemReq.ID]
		if exists {
			// 商品已存在，可能修改了数量或已付款数
			// 校验数量不能低于已提走+已退款的数量
			minRequiredQty := oldItem.QtyPicked + oldItem.QtyRefunded
			if itemReq.Qty < minRequiredQty {
				return fmt.Errorf("商品【%s】的数量不能减少到 %d 以下 (已提/退部分无法被删减)", oldItem.ProductName, minRequiredQty)
			}

			if p, err := s.ProductRepo.FindByID(tx, itemReq.ID); err == nil && p.Category == "临时" && itemReq.Price > 0 {
				p.Price = itemReq.Price
				_, _ = tx.Exec("UPDATE products SET price = ? WHERE id = ?", p.Price, p.ID)
				// 同步订单明细价格
				_, _ = tx.Exec("UPDATE order_items SET price = ? WHERE id = ?", p.Price, oldItem.ID)
			}

			// 取消修改时的物理库存多退少补
			diffQty := itemReq.Qty - oldItem.QtyOrdered
			if diffQty != 0 {
				_, err := s.ProductRepo.FindByID(tx, itemReq.ID)
				if err != nil {
					return err
				}
			}

			// 付款数量不能低于已取数量
			newQtyPaid := itemReq.QtyPaid
			if newQtyPaid < oldItem.QtyPicked {
				newQtyPaid = oldItem.QtyPicked
			}

			if itemReq.Qty != oldItem.QtyOrdered || newQtyPaid != oldItem.QtyPaid {
				if err := s.OrderRepo.UpdateOrderItemQtyAndPaid(tx, oldItem.ID, itemReq.Qty, newQtyPaid); err != nil {
					return err
				}
			}
		} else {
			// 全新商品，直接扣库存并新增行
			p, err := s.ProductRepo.FindByID(tx, itemReq.ID)
			if err != nil {
				return fmt.Errorf("新增商品异常")
			}
			if p.Category == "临时" {
				if itemReq.Price > 0 {
					p.Price = itemReq.Price
					_, _ = tx.Exec("UPDATE products SET price = ? WHERE id = ?", p.Price, p.ID)
				}
			} else {
				// 新增商品不扣减物理库存
			}

			item := model.OrderItem{
				OrderID: req.OrderID, ProductID: p.ID, ProductName: p.Name,
				Price: p.Price, QtyOrdered: itemReq.Qty, QtyPicked: 0, QtyPaid: itemReq.QtyPaid, Unit: p.Unit,
			}
			if err := s.OrderRepo.CreateOrderItem(tx, item); err != nil {
				return err
			}
		}
	}

	// 3. 处理被删除的旧明细
	for productID, oldItem := range oldItemMap {
		if !newItemIDs[productID] {
			if oldItem.QtyPicked > 0 || oldItem.QtyRefunded > 0 {
				return fmt.Errorf("商品【%s】已经产生过提货或退款记录，不能直接删除。请将其修改为已发生数量。", oldItem.ProductName)
			}
			// 取消退还未提货部分库存，因为本来就没有扣减
			if err := s.OrderRepo.DeleteOrderItem(tx, oldItem.ID); err != nil {
				return err
			}
		}
	}

	// 4. 更新订单基本信息
	if err := s.OrderRepo.UpdateOrderInfo(tx, req.OrderID, req.CustomerName, req.Phone); err != nil {
		return err
	}

	return tx.Commit()
}

// Pickup 提货 (履约) - 58mm 防溢出版
func (s *CheckoutService) Pickup(req model.PickupRequest) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	pickedMap := make(map[int]int)

	for _, pickItem := range req.Items {
		if pickItem.Qty <= 0 {
			continue
		}

		orderItem, err := s.OrderRepo.GetItemByID(tx, pickItem.ItemID)
		if err != nil {
			return err
		}

		pickedMap[pickItem.ItemID] = pickItem.Qty

		product, err := s.ProductRepo.FindByID(tx, int(orderItem.ProductID))
		if err != nil {
			return fmt.Errorf("找不到商品信息，可能已被删除")
		}

		if orderItem.Price == 0 && product.Price > 0 {
			_, err := tx.Exec("UPDATE order_items SET price = ? WHERE id = ?", product.Price, orderItem.ID)
			if err != nil {
				return fmt.Errorf("同步价格失败: %v", err)
			}
		}

		// 提货时检查并扣减物理库存
		if product.Stock < pickItem.Qty {
			return fmt.Errorf("商品【%s】尚未完成采购入库或库存不足 (需:%d，当前库存:%d)", product.Name, pickItem.Qty, product.Stock)
		}
		if err := s.ProductRepo.DecreaseStock(tx, int(orderItem.ProductID), pickItem.Qty); err != nil {
			return err
		}
		newPickedQty := orderItem.QtyPicked + pickItem.Qty
		if orderItem.QtyOrdered-newPickedQty < 0 {
			return fmt.Errorf("商品【%s】提货量超出剩余量", orderItem.ProductName)
		}

		if err := s.OrderRepo.UpdatePickedQty(tx, pickItem.ItemID, pickItem.Qty); err != nil {
			return err
		}
	}

	isComplete, err := s.OrderRepo.CheckOrderComplete(tx, req.OrderID)
	if err != nil {
		return err
	}

	if isComplete {
		if err := s.OrderRepo.UpdateStatus(tx, req.OrderID, "Completed"); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if len(pickedMap) == 0 {
		return nil
	}

	allItems, err := s.OrderRepo.GetItemsByOrderID(req.OrderID)
	if err != nil {
		log.Printf("获取订单明细失败，无法打印: %v", err)
		return nil
	}

	var sb strings.Builder
	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("   %s\n", StoreName))

	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("订单:#%d\n", req.OrderID))
	sb.WriteString(fmt.Sprintf("提货:%s\n", time.Now().Format("06-01-02 15:04")))
	sb.WriteString("-------------------------------\n")

	var pickTotal float64
	for _, item := range allItems {
		thisTimeQty, exists := pickedMap[item.ID]
		if !exists || thisTimeQty <= 0 {
			continue
		}
		subtotal := item.Price * float64(thisTimeQty)
		pickTotal += subtotal

		sb.WriteString(fmt.Sprintf("%s\n", item.ProductName))
		displayUnit := ""
		if item.Unit != "" {
			displayUnit = "/" + item.Unit
		}
		sb.WriteString(fmt.Sprintf(" %-7.2f%-3s x%-3d %8.2f\n", item.Price, displayUnit, thisTimeQty, subtotal))
	}

	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("本次提货:      RMB %8.2f\n", pickTotal))
	sb.WriteString("-------------------------------\n")

	if isComplete {
		sb.WriteString("     ★ 订单已完成 ★\n")
	} else {
		sb.WriteString("     >>> 订单未完 <<<\n")
	}
	sb.WriteString("\n\n\n")

	s.printAsync(sb.String())
	return nil
}

// ReprintTicket 补打 (58mm 防溢出版)
func (s *CheckoutService) ReprintTicket(orderID int) error {
	var customerName, phone, createdAt string
	err := s.DB.QueryRow("SELECT customer_name, phone, created_at FROM orders WHERE id = ?", orderID).Scan(&customerName, &phone, &createdAt)
	if err != nil {
		return fmt.Errorf("查询订单失败: %v", err)
	}

	items, err := s.OrderRepo.GetItemsByOrderID(orderID)
	if err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("   %s\n", StoreName))

	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("单号:#%d\n", orderID))
	if len(createdAt) > 16 {
		createdAt = createdAt[:16]
	}
	sb.WriteString(fmt.Sprintf("下单:%s\n", createdAt))
	sb.WriteString(fmt.Sprintf("补打:%s\n", time.Now().Format("06-01-02 15:04")))
	if customerName != "散客" {
		sb.WriteString(fmt.Sprintf("客户:%s\n", customerName))
		// 电话单独一行，防止名字太长挤下来
		sb.WriteString(fmt.Sprintf("电话:%s\n", phone))
	}
	sb.WriteString("-------------------------------\n")
	sb.WriteString("状态(订/提/剩)           金额\n")
	sb.WriteString("-------------------------------\n")

	var totalAmount float64
	var totalPaidAmount float64
	for _, item := range items {
		remaining := item.QtyOrdered - item.QtyPicked - item.QtyRefunded
		if remaining < 0 {
			remaining = 0
		}

		subtotal := item.Price * float64(item.QtyOrdered-item.QtyRefunded)
		totalAmount += subtotal

		paidQty := item.QtyPaid
		effectiveQty := item.QtyOrdered - item.QtyRefunded
		if paidQty > effectiveQty {
			paidQty = effectiveQty
		}
		totalPaidAmount += item.Price * float64(paidQty)

		sb.WriteString(fmt.Sprintf("%s\n", item.ProductName))

		statusStr := fmt.Sprintf("订%d提%d剩%d", item.QtyOrdered-item.QtyRefunded, item.QtyPicked, remaining)
		if item.QtyRefunded > 0 {
			statusStr += fmt.Sprintf("(退%d)", item.QtyRefunded)
		}

		// 同样使用 %-13s 的安全宽度
		sb.WriteString(fmt.Sprintf("  %-13s %8.2f\n", statusStr, subtotal))
	}

	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("总计金额:      RMB %8.2f\n", totalAmount))
	sb.WriteString(fmt.Sprintf("已收金额:      RMB %8.2f\n", totalPaidAmount))
	unpaidAmount := totalAmount - totalPaidAmount
	if unpaidAmount < 0 {
		unpaidAmount = 0
	}
	sb.WriteString(fmt.Sprintf("未收金额:      RMB %8.2f\n", unpaidAmount))
	sb.WriteString("-------------------------------\n")
	sb.WriteString("      (此票据为补打副本)\n\n\n\n")

	s.printAsync(sb.String())
	return nil
}

// PartialRefundRequest 部分退款请求
type PartialRefundRequest struct {
	OrderID int `json:"order_id"`
	Items   []struct {
		ItemID int `json:"item_id"`
		Qty    int `json:"qty"`
	} `json:"items"`
}

// PartialRefund 处理部分退款
func (s *CheckoutService) PartialRefund(req PartialRefundRequest) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	if err := tx.QueryRow("SELECT status FROM orders WHERE id = ?", req.OrderID).Scan(&status); err != nil {
		return err
	}
	if status == "Refunded" {
		return fmt.Errorf("该订单已全额退款")
	}

	totalItemsOrdered := 0
	totalItemsRefundedBefore := 0

	for _, refundItem := range req.Items {
		if refundItem.Qty <= 0 {
			continue
		}

		var pid int64
		var picked, refunded int
		err := tx.QueryRow("SELECT product_id, qty_picked, qty_refunded FROM order_items WHERE id = ?", refundItem.ItemID).Scan(&pid, &picked, &refunded)
		if err != nil {
			return err
		}

		if refunded+refundItem.Qty > picked {
			return fmt.Errorf("退款数量超出购买量")
		}

		res, err := tx.Exec("UPDATE order_items SET qty_refunded = qty_refunded + ? WHERE id = ? AND qty_refunded + ? <= qty_picked", refundItem.Qty, refundItem.ItemID, refundItem.Qty)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return fmt.Errorf("退款处理失败：并发操作下退款总数超出了商品实际可退量 (明细ID: %d)", refundItem.ItemID)
		}

		if err := s.ProductRepo.UpdateStock(tx, pid, refundItem.Qty); err != nil {
			return err
		}
	}

	row := tx.QueryRow("SELECT SUM(qty_picked), SUM(qty_refunded) FROM order_items WHERE order_id = ?", req.OrderID)
	if err := row.Scan(&totalItemsOrdered, &totalItemsRefundedBefore); err != nil {
		return err
	}

	var unpickedCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM order_items WHERE order_id = ? AND qty_picked < qty_ordered", req.OrderID).Scan(&unpickedCount)
	if err != nil {
		return err
	}

	newStatus := "Partial"
	if unpickedCount > 0 {
		newStatus = "Pending"
	} else if totalItemsRefundedBefore == totalItemsOrdered {
		newStatus = "Refunded"
	}

	if err := s.OrderRepo.UpdateStatus(tx, req.OrderID, newStatus); err != nil {
		return err
	}

	return tx.Commit()
}

// RefundOrder 处理全单退款
func (s *CheckoutService) RefundOrder(orderID int) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRow("SELECT status FROM orders WHERE id = ?", orderID).Scan(&status)
	if err != nil {
		return err
	}

	if status == "Refunded" {
		return fmt.Errorf("该订单已退款")
	}

	items, err := s.OrderRepo.GetItemsByOrderIDTx(tx, orderID)
	if err != nil {
		return err
	}

	for _, item := range items {
		qtyToReturn := item.QtyPicked - item.QtyRefunded
		if qtyToReturn > 0 {
			if err := s.ProductRepo.UpdateStock(tx, int64(item.ProductID), qtyToReturn); err != nil {
				return err
			}
			_, err := tx.Exec("UPDATE order_items SET qty_refunded = qty_picked WHERE id = ?", item.ID)
			if err != nil {
				return err
			}
		}
	}

	if err := s.OrderRepo.UpdateStatus(tx, orderID, "Refunded"); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *CheckoutService) printAsync(content string) {
	go func() {
		if err := printer.Current.PrintTicket(content); err != nil {
			log.Println("打印失败:", err)
		}
	}()
}

// DeleteOrder 删除订单
func (s *CheckoutService) DeleteOrder(orderID int) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 删除前无需退还库存（仅当有被扣库存需要退，但由于现在是提货时才扣，所以只有Refund才退库存）
	_, err = s.OrderRepo.GetItemsByOrderIDTx(tx, orderID)

	if err := s.OrderRepo.DeleteOrder(tx, orderID); err != nil {
		return err
	}
	return tx.Commit()
}

// ClearHistoryOrders 清空历史记录
func (s *CheckoutService) ClearHistoryOrders() error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.OrderRepo.ClearHistoryOrders(tx); err != nil {
		return err
	}
	return tx.Commit()
}
