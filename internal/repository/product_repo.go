package repository

import (
	"database/sql"
	"pos-demo/internal/model"
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
	err := r.DB.QueryRow("SELECT id, barcode, name, category, price, cost_price, stock FROM products WHERE barcode = ?", code).Scan(&p.ID, &p.Barcode, &p.Name, &p.Category, &p.Price, &p.CostPrice, &p.Stock)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// FindByID 根据ID查找
func (r *ProductRepo) FindByID(tx *sql.Tx, id int) (*model.Product, error) {
	var p model.Product
	err := tx.QueryRow("SELECT id, name, barcode, category, price, cost_price, stock FROM products WHERE id = ?", id).Scan(&p.ID, &p.Name, &p.Barcode, &p.Category, &p.Price, &p.CostPrice, &p.Stock)
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
	rows, err := r.DB.Query("SELECT id, barcode, name, category, price, cost_price, stock FROM products ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// [修复] 显式初始化为空切片，确保返回 [] 而不是 null
	products := make([]model.Product, 0)

	for rows.Next() {
		var p model.Product
		rows.Scan(&p.ID, &p.Barcode, &p.Name, &p.Category, &p.Price, &p.CostPrice, &p.Stock)
		products = append(products, p)
	}
	return products, nil
}

// SearchInventory 库存搜索 (支持搜分类名)
func (r *ProductRepo) SearchInventory(query string) ([]model.Product, error) {
	param := "%" + query + "%"
	rows, err := r.DB.Query("SELECT id, barcode, name, category, price, cost_price, stock FROM products WHERE name LIKE ? OR barcode LIKE ? OR category LIKE ? ORDER BY id DESC", param, param, param)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// [修复] 显式初始化为空切片
	list := make([]model.Product, 0)

	for rows.Next() {
		var p model.Product
		rows.Scan(&p.ID, &p.Barcode, &p.Name, &p.Category, &p.Price, &p.CostPrice, &p.Stock)
		list = append(list, p)
	}
	return list, nil
}

// Search 联想搜索
func (r *ProductRepo) Search(query string) ([]model.Product, error) {
	param := "%" + query + "%"
	rows, err := r.DB.Query("SELECT id, barcode, name, category, price, cost_price, stock FROM products WHERE name LIKE ? OR barcode LIKE ? OR category LIKE ? ORDER BY id DESC LIMIT 10", param, param, param)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// [修复] 显式初始化为空切片
	list := make([]model.Product, 0)

	for rows.Next() {
		var p model.Product
		rows.Scan(&p.ID, &p.Barcode, &p.Name, &p.Category, &p.Price, &p.CostPrice, &p.Stock)
		list = append(list, p)
	}
	return list, nil
}

// Create 新增 (写入 Category)
func (r *ProductRepo) Create(p model.Product) error {
	_, err := r.DB.Exec("INSERT INTO products (barcode, name, category, price, cost_price, stock) VALUES (?, ?, ?, ?, ?, ?)",
		p.Barcode, p.Name, p.Category, p.Price, p.CostPrice, p.Stock)
	return err
}

// Update 更新 (写入 Category)
func (r *ProductRepo) Update(p model.Product) error {
	_, err := r.DB.Exec("UPDATE products SET name=?, category=?, price=?, cost_price=?, stock=? WHERE id=?",
		p.Name, p.Category, p.Price, p.CostPrice, p.Stock, p.ID)
	return err
}

// Delete 删除商品
func (r *ProductRepo) Delete(id int) error {
	_, err := r.DB.Exec("DELETE FROM products WHERE id = ?", id)
	return err
}

// FindByName 根据名称查找
func (r *ProductRepo) FindByName(name string) (*model.Product, error) {
	var p model.Product
	err := r.DB.QueryRow("SELECT id, barcode, name, category, price, cost_price, stock FROM products WHERE name = ?", name).Scan(&p.ID, &p.Barcode, &p.Name, &p.Category, &p.Price, &p.CostPrice, &p.Stock)
	if err != nil {
		return nil, err
	}
	return &p, nil
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
	}
	return tx.Commit()
}

// UpdateStock 更新库存 (加库存/减库存通用版)
// qty > 0 表示加库存 (退货/入库)，qty < 0 表示减库存
func (r *ProductRepo) UpdateStock(tx *sql.Tx, productID int64, qty int) error {
	_, err := tx.Exec("UPDATE products SET stock = stock + ? WHERE id = ?", qty, productID)
	return err
}
