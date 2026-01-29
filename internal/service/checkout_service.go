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

		// 2. [新增] 查当前库存
		// 注意：我们通过 OrderItem 里的 ProductID 去查最新的 Product 表
		product, err := s.ProductRepo.FindByID(tx, int(orderItem.ProductID))
		if err != nil {
			return fmt.Errorf("找不到商品信息，可能已被删除")
		}

		// 3. [新增] 严格校验：库存必须足够！
		// 如果是临时商品，库存为0，这里就会拦截报错，强制你去【采购入库】
		if product.Stock < pickItem.Qty {
			return fmt.Errorf("【%s】库存不足(剩%d)，请先完成采购入库", product.Name, product.Stock)
		}

		// 4. [新增] 扣减库存 (因为 Book 阶段没扣，现在必须扣)
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
