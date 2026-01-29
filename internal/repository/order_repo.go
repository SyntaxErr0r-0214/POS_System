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

// SearchPendingOrders 搜索未完成订单
func (r *OrderRepo) SearchPendingOrders(query string) ([]model.Order, error) {
	rows, err := r.DB.Query(`SELECT id, customer_name, phone, status, created_at FROM orders WHERE status = 'Pending' AND (customer_name LIKE ? OR phone LIKE ?) ORDER BY id DESC`, "%"+query+"%", "%"+query+"%")
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

// GetItemsByOrderID 获取订单下的商品
func (r *OrderRepo) GetItemsByOrderID(orderID int) ([]model.OrderItem, error) {
	rows, err := r.DB.Query(`SELECT id, product_id, product_name, price, qty_ordered, qty_picked FROM order_items WHERE order_id = ?`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.OrderItem
	for rows.Next() {
		var i model.OrderItem
		// 注意：这里 product_id 可能是 NULL (如果商品被删了)，所以我们要用 sql.NullInt64 或者简单处理
		// 为了简单，如果数据库是 NULL，Scan 到 int 会报错。
		// 我们这里做一个小技巧：COALESCE(product_id, 0) 把 NULL 转成 0
		err = rows.Scan(&i.ID, &i.ProductID, &i.ProductName, &i.Price, &i.QtyOrdered, &i.QtyPicked)
		if err != nil {
			// 如果扫描失败，尝试容错处理（通常是因为 NULL）
			var nullPid sql.NullInt64
			rows.Scan(&i.ID, &nullPid, &i.ProductName, &i.Price, &i.QtyOrdered, &i.QtyPicked)
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

// UpdatePickedQty 更新已提货数量
func (r *OrderRepo) UpdatePickedQty(tx *sql.Tx, itemID int, qty int) error {
	_, err := tx.Exec("UPDATE order_items SET qty_picked = qty_picked + ? WHERE id = ?", qty, itemID)
	return err
}

// CheckOrderComplete 检查订单是否全部完成
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

// --- 👇 新增的两个关键方法 👇 ---

// HasActiveOrders 检查该商品是否存在于未完成的订单中
func (r *OrderRepo) HasActiveOrders(productID int) (bool, error) {
	var count int
	// 联表查询：查 order_items 里的商品，且该订单的状态是 Pending
	sqlStr := `
		SELECT COUNT(*) 
		FROM order_items oi 
		JOIN orders o ON oi.order_id = o.id 
		WHERE oi.product_id = ? AND o.status = 'Pending'
	`
	err := r.DB.QueryRow(sqlStr, productID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UnlinkProduct 将历史订单中的 product_id 设为 NULL (解除关联)
func (r *OrderRepo) UnlinkProduct(productID int) error {
	// 只有把 product_id 设为 NULL，删除 products 表里的记录才不会报错
	_, err := r.DB.Exec("UPDATE order_items SET product_id = NULL WHERE product_id = ?", productID)
	return err
}
