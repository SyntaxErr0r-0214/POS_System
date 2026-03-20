package repository

import (
	"database/sql"
	"fmt"
	"pos-demo/internal/model"
	"strings"
)

type ProductRepo struct {
	DB *sql.DB
}

func NewProductRepo(db *sql.DB) *ProductRepo {
	return &ProductRepo{DB: db}
}

// FindByBarcode 根据条码查找
func (r *ProductRepo) FindByBarcode(code string) (*model.Product, error) {
	var p model.Product
	err := r.DB.QueryRow("SELECT id, barcode, name, category, price, cost_price, stock, unit FROM products WHERE barcode = ?", code).Scan(&p.ID, &p.Barcode, &p.Name, &p.Category, &p.Price, &p.CostPrice, &p.Stock, &p.Unit)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// FindByID 根据ID查找
func (r *ProductRepo) FindByID(tx *sql.Tx, id int) (*model.Product, error) {
	var p model.Product
	err := tx.QueryRow("SELECT id, name, barcode, category, price, cost_price, stock, unit FROM products WHERE id = ?", id).Scan(&p.ID, &p.Name, &p.Barcode, &p.Category, &p.Price, &p.CostPrice, &p.Stock, &p.Unit)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecreaseStock 扣减库存
func (r *ProductRepo) DecreaseStock(tx *sql.Tx, productID int, qty int) error {
	_, err := tx.Exec("UPDATE products SET stock = stock - ? WHERE id = ?", qty, productID)
	return err
}

// GetAll 获取所有商品
func (r *ProductRepo) GetAll() ([]model.Product, error) {
	rows, err := r.DB.Query("SELECT id, barcode, name, category, price, cost_price, stock, unit FROM products ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// [修复] 显式初始化为空切片，确保返回 [] 而不是 null
	products := make([]model.Product, 0)

	for rows.Next() {
		var p model.Product
		rows.Scan(&p.ID, &p.Barcode, &p.Name, &p.Category, &p.Price, &p.CostPrice, &p.Stock, &p.Unit)
		products = append(products, p)
	}
	return products, nil
}

// SearchInventory 库存搜索 (支持搜分类名)
func (r *ProductRepo) SearchInventory(query string) ([]model.Product, error) {
	param := "%" + query + "%"
	rows, err := r.DB.Query("SELECT id, barcode, name, category, price, cost_price, stock, unit FROM products WHERE name LIKE ? OR barcode LIKE ? OR category LIKE ? ORDER BY id DESC", param, param, param)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// [修复] 显式初始化为空切片
	list := make([]model.Product, 0)

	for rows.Next() {
		var p model.Product
		rows.Scan(&p.ID, &p.Barcode, &p.Name, &p.Category, &p.Price, &p.CostPrice, &p.Stock, &p.Unit)
		list = append(list, p)
	}
	return list, nil
}

// Search 联想搜索
func (r *ProductRepo) Search(query string) ([]model.Product, error) {
	param := "%" + query + "%"
	rows, err := r.DB.Query("SELECT id, barcode, name, category, price, cost_price, stock, unit FROM products WHERE name LIKE ? OR barcode LIKE ? OR category LIKE ? ORDER BY id DESC LIMIT 10", param, param, param)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// [修复] 显式初始化为空切片
	list := make([]model.Product, 0)

	for rows.Next() {
		var p model.Product
		rows.Scan(&p.ID, &p.Barcode, &p.Name, &p.Category, &p.Price, &p.CostPrice, &p.Stock, &p.Unit)
		list = append(list, p)
	}
	return list, nil
}

// Create 新增 (写入 Category)
func (r *ProductRepo) Create(p model.Product) error {
	if p.Unit == "" {
		p.Unit = "个"
	}
	_, err := r.DB.Exec("INSERT INTO products (barcode, name, category, price, cost_price, stock, unit) VALUES (?, ?, ?, ?, ?, ?, ?)",
		p.Barcode, p.Name, p.Category, p.Price, p.CostPrice, p.Stock, p.Unit)
	return err
}

// Update 更新 (写入 Category)
func (r *ProductRepo) Update(p model.Product) error {
	if p.Unit == "" {
		p.Unit = "个"
	}
	_, err := r.DB.Exec("UPDATE products SET name=?, category=?, price=?, cost_price=?, stock=?, unit=? WHERE id=?",
		p.Name, p.Category, p.Price, p.CostPrice, p.Stock, p.Unit, p.ID)
	return err
}

// Delete 删除商品
func (r *ProductRepo) Delete(id int) error {
	_, err := r.DB.Exec("DELETE FROM products WHERE id = ?", id)
	return err
}

// FindByNameAndUnit 根据名称和单位查找
func (r *ProductRepo) FindByNameAndUnit(name string, unit string) (*model.Product, error) {
	var p model.Product
	err := r.DB.QueryRow("SELECT id, barcode, name, category, price, cost_price, stock, unit FROM products WHERE name = ? AND unit = ?", name, unit).Scan(&p.ID, &p.Barcode, &p.Name, &p.Category, &p.Price, &p.CostPrice, &p.Stock, &p.Unit)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// BatchUpdateCategory 批量更新分类
func (r *ProductRepo) BatchUpdateCategory(ids []int, category string) error {
	if len(ids) == 0 {
		return nil
	}

	// Create an IN clause with the correct number of placeholders
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)
	args[0] = category
	for i, id := range ids {
		placeholders[i] = "?"
		args[i+1] = id
	}
	query := fmt.Sprintf("UPDATE products SET category = ? WHERE id IN (%s)", strings.Join(placeholders, ","))

	_, err := r.DB.Exec(query, args...)
	return err
}

// BatchDelete 批量删除商品
func (r *ProductRepo) BatchDelete(ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	inClause := strings.Join(placeholders, ",")

	// Delete from products
	delQuery := fmt.Sprintf("DELETE FROM products WHERE id IN (%s)", inClause)
	if _, err := tx.Exec(delQuery, args...); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// BatchProcure 批量采购入库
func (r *ProductRepo) BatchProcure(items []map[string]interface{}) error {
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("UPDATE products SET stock = stock + ?, cost_price = ?, price = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		id := int64(item["id"].(float64))
		addStock := int(item["add_stock"].(float64))
		cost := item["cost"].(float64)
		price := item["price"].(float64)

		if _, err := stmt.Exec(addStock, cost, price, id); err != nil {
			tx.Rollback()
			return err
		}

		// 同步价格到待处理订单中价格为0的明细（临时商品采购后同步售价）
		if price > 0 {
			if _, err := tx.Exec(`UPDATE order_items SET price = ? WHERE product_id = ? AND price = 0
				AND order_id IN (SELECT id FROM orders WHERE status = 'Pending')`, price, id); err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	return tx.Commit()
}

// UpdateStock 更新库存 (加库存/减库存通用版)
// qty > 0 表示加库存 (退货/入库)，qty < 0 表示减库存
func (r *ProductRepo) UpdateStock(tx *sql.Tx, productID int64, qty int) error {
	_, err := tx.Exec("UPDATE products SET stock = stock + ? WHERE id = ?", qty, productID)
	return err
}
