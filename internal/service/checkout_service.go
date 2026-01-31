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

// Checkout 实时结算 (现货交易：必须严查库存)
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
	// --- 销售小票头部 ---
	sb.WriteString("**************************\n")
	sb.WriteString(fmt.Sprintf("   %s\n", StoreName))
	sb.WriteString("       [销售小票]\n")
	sb.WriteString("**************************\n")
	sb.WriteString(fmt.Sprintf("单号: #%d\n", orderID))
	sb.WriteString(fmt.Sprintf("时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString("--------------------------\n")
	sb.WriteString("商品          单价   数量   金额\n")

	var totalPrice float64 = 0

	for _, itemReq := range req.Items {
		p, err := s.ProductRepo.FindByID(tx, itemReq.ID)
		if err != nil {
			return fmt.Errorf("商品ID %d 异常", itemReq.ID)
		}

		// 严查库存：现货交易必须有货
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

		// 存入数据库
		item := model.OrderItem{
			OrderID: int(orderID), ProductID: p.ID, ProductName: p.Name,
			Price: finalPrice, QtyOrdered: itemReq.Qty, QtyPicked: itemReq.Qty,
		}
		if err := s.OrderRepo.CreateOrderItem(tx, item); err != nil {
			return err
		}

		subtotal := finalPrice * float64(itemReq.Qty)
		totalPrice += subtotal

		sb.WriteString(fmt.Sprintf("%-12s\n", p.Name))
		sb.WriteString(fmt.Sprintf("          %6.2f   x%-3d %6.2f\n", finalPrice, itemReq.Qty, subtotal))
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// --- 销售小票尾部 ---
	sb.WriteString("--------------------------\n")
	sb.WriteString(fmt.Sprintf("合计金额:      RMB %.2f\n", totalPrice))
	sb.WriteString("--------------------------\n")
	sb.WriteString("    谢谢惠顾，欢迎下次光临！\n\n\n\n")

	s.printAsync(sb.String())
	return nil
}

// Book 预订 (期货交易：完全不查库存，也不扣库存)
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
		// 这里只查商品信息，不查库存，不扣减
		p, err := s.ProductRepo.FindByID(tx, itemReq.ID)
		if err != nil {
			return fmt.Errorf("商品ID %d 异常", itemReq.ID)
		}

		item := model.OrderItem{
			OrderID: int(orderID), ProductID: p.ID, ProductName: p.Name,
			Price: p.Price, QtyOrdered: itemReq.Qty, QtyPicked: 0,
		}
		if err := s.OrderRepo.CreateOrderItem(tx, item); err != nil {
			return err
		}
	}
	// 预订成功只入库不打印，或者你可以自己加打印逻辑
	return tx.Commit()
}

// Pickup 提货 (履约：关键修复点！！！)
// Pickup 提货 (履约)
func (s *CheckoutService) Pickup(req model.PickupRequest) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var sb strings.Builder
	// ... 打印头 ...
	sb.WriteString("**************************\n")
	sb.WriteString(fmt.Sprintf("   %s\n", StoreName))
	sb.WriteString("       [预订提货单]\n")
	sb.WriteString("**************************\n")
	sb.WriteString(fmt.Sprintf("订单号: #%d\n", req.OrderID))
	sb.WriteString(fmt.Sprintf("提货时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString("--------------------------\n")
	sb.WriteString("商品名称       本次提取 / 剩余\n")
	sb.WriteString("--------------------------\n")

	for _, pickItem := range req.Items {
		// 1. 获取订单明细
		orderItem, err := s.OrderRepo.GetItemByID(tx, pickItem.ItemID)
		if err != nil {
			return err
		}

		// 2. 查当前商品库的最新信息 (库存、价格)
		product, err := s.ProductRepo.FindByID(tx, int(orderItem.ProductID))
		if err != nil {
			return fmt.Errorf("找不到商品信息，可能已被删除")
		}

		// ★★★★★ [核心修复] 价格同步逻辑 ★★★★★
		// 如果订单里的价格是 0 (说明是临时挂单)，但商品库里现在有价格了 (说明已采购)
		// 那么在提货这一刻，把订单里的价格修正为最新售价！
		if orderItem.Price == 0 && product.Price > 0 {
			// 1. 更新数据库里的订单明细价格
			// 这里假设你没有专门的 UpdateOrderItemPrice 方法，我们直接用 SQL
			_, err := tx.Exec("UPDATE order_items SET price = ? WHERE id = ?", product.Price, orderItem.ID)
			if err != nil {
				return fmt.Errorf("同步价格失败: %v", err)
			}
			// 2. 更新内存里的价格，以便下面打印小票时显示正确金额
			orderItem.Price = product.Price
		}
		// ★★★★★★★★★★★★★★★★★★★★★★★★★★★

		// 3. 严格校验：库存必须足够！
		if product.Stock < pickItem.Qty {
			return fmt.Errorf("【%s】库存不足(剩%d)，请先完成采购入库", product.Name, product.Stock)
		}

		// 4. 扣减库存
		if err := s.ProductRepo.DecreaseStock(tx, int(orderItem.ProductID), pickItem.Qty); err != nil {
			return err
		}

		// 5. 更新订单已取数量
		newPickedQty := orderItem.QtyPicked + pickItem.Qty
		remainingQty := orderItem.QtyOrdered - newPickedQty

		if remainingQty < 0 {
			return fmt.Errorf("提货量超出剩余量")
		}

		if err := s.OrderRepo.UpdatePickedQty(tx, pickItem.ItemID, pickItem.Qty); err != nil {
			return err
		}

		sb.WriteString(fmt.Sprintf("%-14s\n", orderItem.ProductName))
		// 这里打印的价格已经是同步后的 product.Price 了
		sb.WriteString(fmt.Sprintf("              取 x%-3d (剩 %d)\n", pickItem.Qty, remainingQty))
	}

	// 检查是否全部取完
	isComplete, err := s.OrderRepo.CheckOrderComplete(tx, req.OrderID)
	if err != nil {
		return err
	}

	sb.WriteString("--------------------------\n")
	if isComplete {
		// 只有全部取完，才把订单状态改成 Completed
		if err := s.OrderRepo.UpdateStatus(tx, req.OrderID, "Completed"); err != nil {
			return err
		}
		sb.WriteString("  ★ 该订单已全部提货完成 ★\n")
	} else {
		sb.WriteString("  >>> 订单未完，请妥善保管 <<<\n")
	}
	sb.WriteString("\n\n\n")

	if err := tx.Commit(); err != nil {
		return err
	}

	s.printAsync(sb.String())
	return nil
}

// printAsync
func (s *CheckoutService) printAsync(content string) {
	go func() {
		if err := printer.Current.PrintTicket(content); err != nil {
			log.Println("打印失败:", err)
		}
	}()
}

// RefundOrder 订单退款
func (s *CheckoutService) RefundOrder(orderID int) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	// 关键：这行代码保证了如果中间出任何错，事务会自动取消，释放数据库锁
	defer tx.Rollback()

	// 1. 检查订单状态
	var status string
	err = tx.QueryRow("SELECT status FROM orders WHERE id = ?", orderID).Scan(&status)
	if err != nil {
		return err
	}

	if status == "Refunded" {
		return fmt.Errorf("该订单已退款")
	}
	if status != "Completed" {
		return fmt.Errorf("只有已完成的订单才能退款")
	}

	// 2. 获取订单明细
	items, err := s.OrderRepo.GetItemsByOrderID(orderID)
	if err != nil {
		return err
	}

	// 3. 归还库存
	for _, item := range items {
		qtyToReturn := item.QtyPicked
		if qtyToReturn > 0 {
			// 调用 ProductRepo 的加库存方法
			if err := s.ProductRepo.UpdateStock(tx, int64(item.ProductID), qtyToReturn); err != nil {
				return err
			}
		}
	}

	// 4. 更新订单状态
	if err := s.OrderRepo.UpdateStatus(tx, orderID, "Refunded"); err != nil {
		return err
	}

	// 关键：最后必须提交事务
	return tx.Commit()
}

// ReprintTicket 补打小票
func (s *CheckoutService) ReprintTicket(orderID int) error {
	// 这里不需要开启事务，因为只是读取数据
	// 1. 查询订单基础信息
	var customerName, phone, createdAt string
	err := s.DB.QueryRow("SELECT customer_name, phone, created_at FROM orders WHERE id = ?", orderID).Scan(&customerName, &phone, &createdAt)
	if err != nil {
		return fmt.Errorf("查询订单失败: %v", err)
	}

	// 2. 查询订单明细
	items, err := s.OrderRepo.GetItemsByOrderID(orderID)
	if err != nil {
		return err
	}

	// 3. 拼装小票内容
	var sb strings.Builder
	sb.WriteString("**************************\n")
	sb.WriteString(fmt.Sprintf("   %s\n", StoreName))
	sb.WriteString("     [补打小票/Reprint]\n") // 明显的标记
	sb.WriteString("**************************\n")
	sb.WriteString(fmt.Sprintf("单号: #%d\n", orderID))
	sb.WriteString(fmt.Sprintf("下单: %s\n", createdAt))
	sb.WriteString(fmt.Sprintf("补打: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	if customerName != "散客" {
		sb.WriteString(fmt.Sprintf("客户: %s (%s)\n", customerName, phone))
	}
	sb.WriteString("--------------------------\n")
	sb.WriteString("商品          单价   数量   金额\n")

	var total float64
	for _, item := range items {
		subtotal := item.Price * float64(item.QtyOrdered)
		total += subtotal
		sb.WriteString(fmt.Sprintf("%-12s\n", item.ProductName))
		sb.WriteString(fmt.Sprintf("          %6.2f   x%-3d %6.2f\n", item.Price, item.QtyOrdered, subtotal))
	}

	sb.WriteString("--------------------------\n")
	sb.WriteString(fmt.Sprintf("合计金额:      RMB %.2f\n", total))
	sb.WriteString("--------------------------\n")
	sb.WriteString("      (此票据为补打副本)\n\n\n\n")

	// 4. 发送打印
	s.printAsync(sb.String())
	return nil
}

// PartialRefundRequest 部分退款请求参数
type PartialRefundRequest struct {
	OrderID int `json:"order_id"`
	Items   []struct {
		ItemID int `json:"item_id"` // order_item 的 id
		Qty    int `json:"qty"`     // 要退多少个
	} `json:"items"`
}

// PartialRefund 处理部分退款
func (s *CheckoutService) PartialRefund(req PartialRefundRequest) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 验证订单状态
	var status string
	if err := tx.QueryRow("SELECT status FROM orders WHERE id = ?", req.OrderID).Scan(&status); err != nil {
		return err
	}
	if status == "Refunded" {
		return fmt.Errorf("该订单已全额退款")
	}

	totalItemsOrdered := 0
	totalItemsRefundedBefore := 0
	currentRefundQty := 0

	// 2. 遍历要退款的商品
	for _, refundItem := range req.Items {
		if refundItem.Qty <= 0 {
			continue
		}

		// 查当前明细状态
		var pid int64
		var picked, refunded int
		err := tx.QueryRow("SELECT product_id, qty_picked, qty_refunded FROM order_items WHERE id = ?", refundItem.ItemID).Scan(&pid, &picked, &refunded)
		if err != nil {
			return err
		}

		// 校验：不能退超过买的数量
		if refunded+refundItem.Qty > picked {
			return fmt.Errorf("退款数量超出购买量")
		}

		// A. 更新明细里的退款数
		if _, err := tx.Exec("UPDATE order_items SET qty_refunded = qty_refunded + ? WHERE id = ?", refundItem.Qty, refundItem.ItemID); err != nil {
			return err
		}

		// B. 库存回滚 (把东西加回去)
		if err := s.ProductRepo.UpdateStock(tx, pid, refundItem.Qty); err != nil {
			return err
		}

		currentRefundQty += refundItem.Qty
	}

	// 3. 判断订单新状态 (Partial 还是 Refunded?)
	// 统计这单总共买了多少，总共退了多少
	row := tx.QueryRow("SELECT SUM(qty_picked), SUM(qty_refunded) FROM order_items WHERE order_id = ?", req.OrderID)
	if err := row.Scan(&totalItemsOrdered, &totalItemsRefundedBefore); err != nil {
		return err
	}

	newStatus := "Partial"
	if totalItemsRefundedBefore == totalItemsOrdered {
		newStatus = "Refunded" // 如果全部退完了，状态改为全退
	}

	if err := s.OrderRepo.UpdateStatus(tx, req.OrderID, newStatus); err != nil {
		return err
	}

	return tx.Commit()
}
