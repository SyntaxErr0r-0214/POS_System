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

// CreateOrder 创建主订单 (自动计算每日流水号)
func (r *OrderRepo) CreateOrder(tx *sql.Tx, customer string, phone string, status string) (int64, error) {
	// 1. 计算今日流水号
	// 逻辑：查找今天(00:00:00以后)最大的 daily_seq，然后 +1
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).Format("2006-01-02 15:04:05")

	var maxSeq sql.NullInt64
	// 查询今天已经存在的最大序号
	err := tx.QueryRow("SELECT MAX(daily_seq) FROM orders WHERE created_at >= ?", todayStart).Scan(&maxSeq)
	if err != nil {
		return 0, err
	}

	newSeq := 1
	if maxSeq.Valid {
		newSeq = int(maxSeq.Int64) + 1
	}

	// 2. 插入订单 (带上 daily_seq)
	res, err := tx.Exec("INSERT INTO orders (customer_name, phone, status, daily_seq) VALUES (?, ?, ?, ?)", customer, phone, status, newSeq)
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

// GetOrders 获取订单列表
// status: 状态
// query: 关键词
// dateFilter: 日期 "YYYY-MM-DD" (空字符串表示不限日期)
func (r *OrderRepo) GetOrders(status string, query string, dateFilter string) ([]model.Order, error) {
	var sqlStr string
	var args []interface{}

	// 基础 SQL，包含 daily_seq
	sqlStr = `SELECT id, daily_seq, customer_name, phone, status, created_at FROM orders WHERE 1=1`

	// 1. 状态筛选
	if status == "Completed" {
		sqlStr += ` AND status IN ('Completed', 'Refunded', 'Partial')`
	} else {
		sqlStr += ` AND status = ?`
		args = append(args, status)
	}

	// 2. 日期筛选 (新增)
	if dateFilter != "" {
		// SQLite 字符串匹配 "YYYY-MM-DD%"
		sqlStr += ` AND created_at LIKE ?`
		args = append(args, dateFilter+"%")
	}

	// 3. 关键词搜索
	if query != "" {
		sqlStr += ` AND (customer_name LIKE ? OR phone LIKE ? OR CAST(id AS TEXT) LIKE ?)`
		likeQuery := "%" + query + "%"
		args = append(args, likeQuery, likeQuery, likeQuery)
	}

	// 4. 排序
	sqlStr += ` ORDER BY id DESC`

	// 5. 数量限制逻辑 (关键修改)
	// 如果是 Pending (进行中)，不加 LIMIT，显示全部
	// 如果是 Completed (历史)，且没有选日期，也没有搜关键词，才限制 100 条防止卡顿
	// 如果选了日期，说明用户想看那天的全部，也不限制
	if status == "Completed" && dateFilter == "" && query == "" {
		sqlStr += ` LIMIT 100`
	}

	rows, err := r.DB.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var o model.Order
		var dailySeq sql.NullInt64 // 处理旧数据可能为 NULL 的情况

		if err := rows.Scan(&o.ID, &dailySeq, &o.CustomerName, &o.Phone, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		o.DailySeq = int(dailySeq.Int64)
		orders = append(orders, o)
	}
	return orders, nil
}

// GetItemsByOrderID 获取订单明细 (带进价和退款数)
func (r *OrderRepo) GetItemsByOrderID(orderID int) ([]model.OrderItem, error) {
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
	err := tx.QueryRow("SELECT COUNT(*) FROM order_items WHERE order_id = ? AND qty_picked < qty_ordered", orderID).Scan(&unpickedCount)
	if err != nil {
		return false, err
	}
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

// CreateOrderWithTime (测试用)
func (r *OrderRepo) CreateOrderWithTime(tx *sql.Tx, customer string, phone string, status string, createTime time.Time) (int64, error) {
	res, err := tx.Exec("INSERT INTO orders (customer_name, phone, status, created_at) VALUES (?, ?, ?, ?)",
		customer, phone, status, createTime)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ProcurementItem 采购清单项
type ProcurementItem struct {
	ProductID    int64  `json:"product_id"`
	ProductName  string `json:"product_name"`
	CurrentStock int    `json:"current_stock"`
	TotalNeeded  int    `json:"total_needed"`
}

// GetProcurementList 获取待采购清单
func (r *OrderRepo) GetProcurementList() ([]ProcurementItem, error) {
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
