package service

import (
	"database/sql"
	"fmt"
	"log"
	"pos-demo/internal/model"
	"pos-demo/internal/repository"
	"pos-demo/pkg/printer"
	"strings"
	"time"
)

// StoreName 店铺名称常量
const StoreName = "万康生态食品团购超市"

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
	// [58mm] 分割线控制在31个字符，防止溢出
	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("   %s\n", StoreName))
	sb.WriteString("          [销售小票]\n")
	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("单号:#%d\n", orderID))
	sb.WriteString(fmt.Sprintf("时间:%s\n", time.Now().Format("06-01-02 15:04")))
	sb.WriteString("-------------------------------\n")
	// [58mm] 紧凑表头
	sb.WriteString("商品名称         数量      金额\n")

	var totalPrice float64 = 0

	for _, itemReq := range req.Items {
		p, err := s.ProductRepo.FindByID(tx, itemReq.ID)
		if err != nil {
			return fmt.Errorf("商品ID %d 异常", itemReq.ID)
		}

		if p.Stock < itemReq.Qty {
			return fmt.Errorf("商品 %s 库存不足(剩%d)", p.Name, p.Stock)
		}

		if err := s.ProductRepo.DecreaseStock(tx, p.ID, itemReq.Qty); err != nil {
			return err
		}

		finalPrice := p.Price
		if itemReq.Price >= 0 {
			finalPrice = itemReq.Price
		}

		item := model.OrderItem{
			OrderID: int(orderID), ProductID: p.ID, ProductName: p.Name,
			Price: finalPrice, QtyOrdered: itemReq.Qty, QtyPicked: itemReq.Qty, Unit: p.Unit,
		}
		if itemReq.Unit != "" {
			item.Unit = itemReq.Unit // 优先使用请求中传来的单位（针对临时商品）
		}
		if err := s.OrderRepo.CreateOrderItem(tx, item); err != nil {
			return err
		}

		subtotal := finalPrice * float64(itemReq.Qty)
		totalPrice += subtotal

		// [58mm] 双行模式，严格控制宽度
		// 如果有单位，则在商品名称后附加单位展示
		displayUnit := ""
		if item.Unit != "" {
			displayUnit = "/" + item.Unit
		}
		sb.WriteString(fmt.Sprintf("%s\n", p.Name))
		// 缩进1格 | 单价(7位) | x数量 | 总价(8位)
		// 示例:  5.00/个   x2      10.00
		sb.WriteString(fmt.Sprintf(" %-7.2f%-3s x%-3d %8.2f\n", finalPrice, displayUnit, itemReq.Qty, subtotal))
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("合计:          RMB %.2f\n", totalPrice))
	sb.WriteString("-------------------------------\n")
	sb.WriteString("    谢谢惠顾，欢迎下次光临！\n\n\n\n")

	s.printAsync(sb.String())
	return nil
}

// Book 预订 (静默模式)
func (s *CheckoutService) Book(req model.BookingRequest) error {
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

	for _, itemReq := range req.Items {
		p, err := s.ProductRepo.FindByID(tx, itemReq.ID)
		if err != nil {
			return fmt.Errorf("商品ID %d 异常", itemReq.ID)
		}

		item := model.OrderItem{
			OrderID: int(orderID), ProductID: p.ID, ProductName: p.Name,
			Price: p.Price, QtyOrdered: itemReq.Qty, QtyPicked: 0, Unit: itemReq.Unit, // 预订商品带上单位
		}
		if item.Unit == "" {
			item.Unit = p.Unit
		}
		if err := s.OrderRepo.CreateOrderItem(tx, item); err != nil {
			return err
		}
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

		if product.Stock < pickItem.Qty {
			return fmt.Errorf("【%s】库存不足(剩%d)", product.Name, product.Stock)
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

	// --- 打印 ---
	allItems, err := s.OrderRepo.GetItemsByOrderID(req.OrderID)
	if err != nil {
		log.Printf("获取订单明细失败，无法打印: %v", err)
		return nil
	}

	var sb strings.Builder
	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("   %s\n", StoreName))
	sb.WriteString("       [预订提货单]\n")
	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("订单:#%d\n", req.OrderID))
	sb.WriteString(fmt.Sprintf("提货:%s\n", time.Now().Format("06-01-02 15:04")))
	sb.WriteString("-------------------------------\n")
	// [58mm] 极简表头
	sb.WriteString("状态(订/提/剩)           金额\n")
	sb.WriteString("-------------------------------\n")

	var totalAmount float64

	for _, item := range allItems {
		remaining := item.QtyOrdered - item.QtyPicked - item.QtyRefunded
		if remaining < 0 {
			remaining = 0
		}

		subtotal := item.Price * float64(item.QtyOrdered)
		totalAmount += subtotal

		sb.WriteString(fmt.Sprintf("%s\n", item.ProductName))

		var statusStr string
		thisTimePickQty, exists := pickedMap[item.ID]

		if exists && thisTimePickQty > 0 {
			// 极简写法：[取2]剩1
			statusStr = fmt.Sprintf("[取%d]剩%d", thisTimePickQty, remaining)
		} else {
			// 极简写法：订3提2剩1
			statusStr = fmt.Sprintf("订%d提%d剩%d", item.QtyOrdered, item.QtyPicked, remaining)
		}

		// ★★★ 核心修复：大幅减少 padding ★★★
		// %-13s: 预留13个位置给汉字状态串(汉字视觉宽，实际短，容易撑开)
		// %8.2f: 预留8个位置给价格
		// 缩进2格
		// 总视觉宽度估算: 2 + (10~14) + 8 = ~24-28 (安全范围32)
		sb.WriteString(fmt.Sprintf("  %-13s %8.2f\n", statusStr, subtotal))
	}

	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("总额:          RMB %.2f\n", totalAmount))
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
	sb.WriteString("     [补打小票/Reprint]\n")
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
	for _, item := range items {
		remaining := item.QtyOrdered - item.QtyPicked - item.QtyRefunded
		if remaining < 0 {
			remaining = 0
		}

		subtotal := item.Price * float64(item.QtyOrdered)
		totalAmount += subtotal

		sb.WriteString(fmt.Sprintf("%s\n", item.ProductName))

		statusStr := fmt.Sprintf("订%d提%d剩%d", item.QtyOrdered, item.QtyPicked, remaining)
		if item.QtyRefunded > 0 {
			statusStr += fmt.Sprintf("(退%d)", item.QtyRefunded)
		}

		// 同样使用 %-13s 的安全宽度
		sb.WriteString(fmt.Sprintf("  %-13s %8.2f\n", statusStr, subtotal))
	}

	sb.WriteString("-------------------------------\n")
	sb.WriteString(fmt.Sprintf("总额:          RMB %.2f\n", totalAmount))
	sb.WriteString("-------------------------------\n")
	sb.WriteString("      (此票据为补打副本)\n\n\n\n")

	s.printAsync(sb.String())
	return nil
}

// PartialRefundRequest (保持不变)
type PartialRefundRequest struct {
	OrderID int `json:"order_id"`
	Items   []struct {
		ItemID int `json:"item_id"`
		Qty    int `json:"qty"`
	} `json:"items"`
}

// PartialRefund (保持不变)
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

		if _, err := tx.Exec("UPDATE order_items SET qty_refunded = qty_refunded + ? WHERE id = ?", refundItem.Qty, refundItem.ItemID); err != nil {
			return err
		}

		if err := s.ProductRepo.UpdateStock(tx, pid, refundItem.Qty); err != nil {
			return err
		}
	}

	row := tx.QueryRow("SELECT SUM(qty_picked), SUM(qty_refunded) FROM order_items WHERE order_id = ?", req.OrderID)
	if err := row.Scan(&totalItemsOrdered, &totalItemsRefundedBefore); err != nil {
		return err
	}

	newStatus := "Partial"
	if totalItemsRefundedBefore == totalItemsOrdered {
		newStatus = "Refunded"
	}

	if err := s.OrderRepo.UpdateStatus(tx, req.OrderID, newStatus); err != nil {
		return err
	}

	return tx.Commit()
}

// RefundOrder (保持不变)
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

	items, err := s.OrderRepo.GetItemsByOrderID(orderID)
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
