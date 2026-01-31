package repository

import (
	"database/sql"
	"pos-demo/internal/model"
	"time"
)

type OrderRepo struct {
	DB *sql.DB
}

func NewOrderRepo(db *sql.DB) *OrderRepo {
	return &OrderRepo{DB: db}
}

// CreateOrder 创建主订单
func (r *OrderRepo) CreateOrder(tx *sql.Tx, customer string, phone string, status string) (int64, error) {
	res, err := tx.Exec("INSERT INTO orders (customer_name, phone, status) VALUES (?, ?, ?)", customer, phone, status)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CreateOrderItem 创建明细
func (r *OrderRepo) CreateOrderItem(tx *sql.Tx, item model.OrderItem) error {
	_, err := tx.Exec(`INSERT INTO order_items (order_id, product_id, product_name, price, qty_ordered, qty_picked) VALUES (?, ?, ?, ?, ?, ?)`,
		item.OrderID, item.ProductID, item.ProductName, item.Price, item.QtyOrdered, item.QtyPicked)
	return err
}

// GetOrders 通用订单查询 (核心升级)
// status: 'Pending' 或 'Completed'
// query: 搜索关键词 (ID, 姓名, 电话)
// GetOrders 获取订单列表
func (r *OrderRepo) GetOrders(status string, query string) ([]model.Order, error) {
	var sqlStr string
	var args []interface{}

	if status == "Completed" {
		// [修改] 历史记录包含: 完成、全退、部分退
		sqlStr = `SELECT id, customer_name, phone, status, created_at FROM orders WHERE status IN ('Completed', 'Refunded', 'Partial')`
	} else {
		sqlStr = `SELECT id, customer_name, phone, status, created_at FROM orders WHERE status = ?`
		args = append(args, status)
	}

	if query != "" {
		sqlStr += ` AND (customer_name LIKE ? OR phone LIKE ? OR CAST(id AS TEXT) LIKE ?)`
		likeQuery := "%" + query + "%"
		args = append(args, likeQuery, likeQuery, likeQuery)
	}

	sqlStr += ` ORDER BY id DESC LIMIT 50`

	rows, err := r.DB.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var o model.Order
		if err := rows.Scan(&o.ID, &o.CustomerName, &o.Phone, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

// GetItemsByOrderID 获取订单明细
// GetItemsByOrderID 获取订单明细 (带进价)
// GetItemsByOrderID 获取订单明细 (带进价和退款数)
func (r *OrderRepo) GetItemsByOrderID(orderID int) ([]model.OrderItem, error) {
	// [修改] 增加了 oi.qty_refunded
	query := `
		SELECT 
			oi.id, oi.order_id, oi.product_id, oi.product_name, 
			oi.price, oi.qty_ordered, oi.qty_picked,
			COALESCE(p.cost_price, 0),
			COALESCE(oi.qty_refunded, 0)
		FROM order_items oi
		LEFT JOIN products p ON oi.product_id = p.id
		WHERE oi.order_id = ?
	`
	rows, err := r.DB.Query(query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.OrderItem
	for rows.Next() {
		var i model.OrderItem
		// [修改] Scan 增加了 &i.QtyRefunded
		if err := rows.Scan(
			&i.ID, &i.OrderID, &i.ProductID, &i.ProductName,
			&i.Price, &i.QtyOrdered, &i.QtyPicked,
			&i.CostPrice, &i.QtyRefunded,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

// GetItemByID 获取单条明细
func (r *OrderRepo) GetItemByID(tx *sql.Tx, itemID int) (*model.OrderItem, error) {
	var i model.OrderItem
	err := tx.QueryRow(`SELECT id, product_id, product_name, qty_ordered, qty_picked FROM order_items WHERE id = ?`, itemID).Scan(&i.ID, &i.ProductID, &i.ProductName, &i.QtyOrdered, &i.QtyPicked)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

// UpdatePickedQty 更新已取数量
func (r *OrderRepo) UpdatePickedQty(tx *sql.Tx, itemID int, qty int) error {
	_, err := tx.Exec("UPDATE order_items SET qty_picked = qty_picked + ? WHERE id = ?", qty, itemID)
	return err
}

// CheckOrderComplete 检查订单是否全部取完
func (r *OrderRepo) CheckOrderComplete(tx *sql.Tx, orderID int) (bool, error) {
	var unpickedCount int
	// 统计 "订购量 > 已取量" 的条目数
	err := tx.QueryRow("SELECT COUNT(*) FROM order_items WHERE order_id = ? AND qty_picked < qty_ordered", orderID).Scan(&unpickedCount)
	if err != nil {
		return false, err
	}
	// 如果没有未取完的条目，说明完成了
	return unpickedCount == 0, nil
}

// UpdateStatus 更新订单状态
func (r *OrderRepo) UpdateStatus(tx *sql.Tx, orderID int, status string) error {
	_, err := tx.Exec("UPDATE orders SET status = ? WHERE id = ?", status, orderID)
	return err
}

// HasActiveOrders 检查商品是否有未完成订单
func (r *OrderRepo) HasActiveOrders(productID int) (bool, error) {
	var count int
	sqlStr := `SELECT COUNT(*) FROM order_items oi JOIN orders o ON oi.order_id = o.id WHERE oi.product_id = ? AND o.status = 'Pending'`
	err := r.DB.QueryRow(sqlStr, productID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UnlinkProduct 解除商品关联
func (r *OrderRepo) UnlinkProduct(productID int) error {
	_, err := r.DB.Exec("UPDATE order_items SET product_id = NULL WHERE product_id = ?", productID)
	return err
}

// CreateOrderWithTime (测试用) 创建指定时间的订单
func (r *OrderRepo) CreateOrderWithTime(tx *sql.Tx, customer string, phone string, status string, createTime time.Time) (int64, error) {
	// 注意：这里我们显式插入 created_at 字段
	res, err := tx.Exec("INSERT INTO orders (customer_name, phone, status, created_at) VALUES (?, ?, ?, ?)",
		customer, phone, status, createTime)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ProcurementItem 采购清单项
type ProcurementItem struct {
	ProductID    int64  `json:"product_id"` // 新增：需要ID来更新
	ProductName  string `json:"product_name"`
	CurrentStock int    `json:"current_stock"` // 新增：当前库存
	TotalNeeded  int    `json:"total_needed"`  // 订单总需求
}

// GetProcurementList 获取待采购清单 (聚合所有 Pending 订单的剩余需求 + 当前库存)
func (r *OrderRepo) GetProcurementList() ([]ProcurementItem, error) {
	// 逻辑升级：关联 products 表获取当前库存
	// 统计所有 Pending 订单中 (订购 - 已取) 的数量
	sqlStr := `
		SELECT p.id, p.name, p.stock, SUM(oi.qty_ordered - oi.qty_picked) as demand
		FROM order_items oi
		JOIN orders o ON oi.order_id = o.id
		JOIN products p ON oi.product_id = p.id
		WHERE o.status = 'Pending'
		GROUP BY p.id
		HAVING demand > 0
		ORDER BY demand DESC
	`
	rows, err := r.DB.Query(sqlStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ProcurementItem
	for rows.Next() {
		var item ProcurementItem
		if err := rows.Scan(&item.ProductID, &item.ProductName, &item.CurrentStock, &item.TotalNeeded); err != nil {
			continue
		}
		list = append(list, item)
	}
	return list, nil
}
