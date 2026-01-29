package repository

import (
	"database/sql"
	"pos-demo/internal/model"
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
func (r *OrderRepo) GetOrders(status string, query string) ([]model.Order, error) {
	sqlStr := `SELECT id, customer_name, phone, status, created_at FROM orders WHERE status = ?`
	args := []interface{}{status}

	if query != "" {
		// 支持搜单号(纯数字) 或 姓名/电话
		sqlStr += ` AND (customer_name LIKE ? OR phone LIKE ? OR id = ?)`
		likeQuery := "%" + query + "%"
		args = append(args, likeQuery, likeQuery, query)
	}

	sqlStr += ` ORDER BY id DESC LIMIT 50` // 限制50条，避免卡顿

	rows, err := r.DB.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var o model.Order
		rows.Scan(&o.ID, &o.CustomerName, &o.Phone, &o.Status, &o.CreatedAt)
		orders = append(orders, o)
	}
	return orders, nil
}

// GetItemsByOrderID 获取订单明细
func (r *OrderRepo) GetItemsByOrderID(orderID int) ([]model.OrderItem, error) {
	rows, err := r.DB.Query(`SELECT id, product_id, product_name, price, qty_ordered, qty_picked FROM order_items WHERE order_id = ?`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.OrderItem
	for rows.Next() {
		var i model.OrderItem
		// 处理 product_id 可能为 NULL 的情况 (商品已删除)
		var nullPid sql.NullInt64
		err = rows.Scan(&i.ID, &nullPid, &i.ProductName, &i.Price, &i.QtyOrdered, &i.QtyPicked)
		if err == nil {
			i.ProductID = int(nullPid.Int64)
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
