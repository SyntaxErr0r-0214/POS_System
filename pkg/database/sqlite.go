package database

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

func Init() *sql.DB {
	db, err := sql.Open("sqlite", "./pos_data.db")
	if err != nil {
		log.Fatal(err)
	}

	db.Exec("PRAGMA foreign_keys = ON;")

	// 简单的建表逻辑，和之前一样
	sqlStmt := `
	CREATE TABLE IF NOT EXISTS products (id INTEGER PRIMARY KEY AUTOINCREMENT, barcode TEXT UNIQUE, name TEXT, price REAL, stock INTEGER DEFAULT 0);
	CREATE TABLE IF NOT EXISTS orders (id INTEGER PRIMARY KEY AUTOINCREMENT, customer_name TEXT, phone TEXT, status TEXT DEFAULT 'Pending', created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
	CREATE TABLE IF NOT EXISTS order_items (id INTEGER PRIMARY KEY AUTOINCREMENT, order_id INTEGER, product_id INTEGER, product_name TEXT, price REAL, qty_ordered INTEGER, qty_picked INTEGER DEFAULT 0, FOREIGN KEY(order_id) REFERENCES orders(id), FOREIGN KEY(product_id) REFERENCES products(id));
	`
	db.Exec(sqlStmt)

	// 初始化测试数据
	var count int
	db.QueryRow("SELECT COUNT(*) FROM products").Scan(&count)
	if count == 0 {
		db.Exec("INSERT INTO products (barcode, name, price, stock) VALUES (?, ?, ?, ?)", "123456", "可口可乐", 3.00, 100)
		db.Exec("INSERT INTO products (barcode, name, price, stock) VALUES (?, ?, ?, ?)", "888888", "卫龙", 5.50, 50)
	}
	return db
}
