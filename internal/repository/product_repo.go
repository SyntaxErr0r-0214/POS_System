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
	err := r.DB.QueryRow("SELECT id, barcode, name, price, cost_price, stock FROM products WHERE barcode = ?", code).Scan(&p.ID, &p.Barcode, &p.Name, &p.Price, &p.CostPrice, &p.Stock)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// FindByID 根据ID查找
func (r *ProductRepo) FindByID(tx *sql.Tx, id int) (*model.Product, error) {
	var p model.Product
	err := tx.QueryRow("SELECT id, name, barcode, price, cost_price, stock FROM products WHERE id = ?", id).Scan(&p.ID, &p.Name, &p.Barcode, &p.Price, &p.CostPrice, &p.Stock)
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
	rows, err := r.DB.Query("SELECT id, barcode, name, price, cost_price, stock FROM products ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []model.Product
	for rows.Next() {
		var p model.Product
		rows.Scan(&p.ID, &p.Barcode, &p.Name, &p.Price, &p.CostPrice, &p.Stock)
		products = append(products, p)
	}
	return products, nil
}

// SearchInventory 库存搜索
func (r *ProductRepo) SearchInventory(query string) ([]model.Product, error) {
	param := "%" + query + "%"
	rows, err := r.DB.Query("SELECT id, barcode, name, price, cost_price, stock FROM products WHERE name LIKE ? OR barcode LIKE ? ORDER BY id DESC", param, param)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Product
	for rows.Next() {
		var p model.Product
		rows.Scan(&p.ID, &p.Barcode, &p.Name, &p.Price, &p.CostPrice, &p.Stock)
		list = append(list, p)
	}
	return list, nil
}

// Search 联想搜索
func (r *ProductRepo) Search(query string) ([]model.Product, error) {
	param := "%" + query + "%"
	rows, err := r.DB.Query("SELECT id, barcode, name, price, cost_price, stock FROM products WHERE name LIKE ? OR barcode LIKE ? ORDER BY id DESC LIMIT 10", param, param)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Product
	for rows.Next() {
		var p model.Product
		rows.Scan(&p.ID, &p.Barcode, &p.Name, &p.Price, &p.CostPrice, &p.Stock)
		list = append(list, p)
	}
	return list, nil
}

// Create 新增
func (r *ProductRepo) Create(p model.Product) error {
	_, err := r.DB.Exec("INSERT INTO products (barcode, name, price, cost_price, stock) VALUES (?, ?, ?, ?, ?)",
		p.Barcode, p.Name, p.Price, p.CostPrice, p.Stock)
	return err
}

// Update 更新
func (r *ProductRepo) Update(p model.Product) error {
	_, err := r.DB.Exec("UPDATE products SET name=?, price=?, cost_price=?, stock=? WHERE id=?",
		p.Name, p.Price, p.CostPrice, p.Stock, p.ID)
	return err
}

// Delete 删除商品 (新增)
func (r *ProductRepo) Delete(id int) error {
	_, err := r.DB.Exec("DELETE FROM products WHERE id = ?", id)
	return err
}
