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
	err := r.DB.QueryRow("SELECT id, barcode, name, price, stock FROM products WHERE barcode = ?", code).Scan(&p.ID, &p.Barcode, &p.Name, &p.Price, &p.Stock)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// FindByID 根据ID查找 (支持事务)
func (r *ProductRepo) FindByID(tx *sql.Tx, id int) (*model.Product, error) {
	var p model.Product
	err := tx.QueryRow("SELECT id, name, barcode, price, stock FROM products WHERE id = ?", id).Scan(&p.ID, &p.Name, &p.Barcode, &p.Price, &p.Stock)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecreaseStock 扣减库存 (支持事务)
func (r *ProductRepo) DecreaseStock(tx *sql.Tx, productID int, qty int) error {
	_, err := tx.Exec("UPDATE products SET stock = stock - ? WHERE id = ?", qty, productID)
	return err
}
