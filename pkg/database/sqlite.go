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

	// 开启外键支持
	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		log.Fatal("无法开启外键支持:", err)
	}

	// --- 核心修改在这里 ---
	// 我们在 products 表里增加了一个 cost_price (进价) 字段
	sqlStmt := `
	CREATE TABLE IF NOT EXISTS products (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		barcode TEXT UNIQUE, 
		name TEXT, 
		price REAL, 
		cost_price REAL DEFAULT 0, -- <--- 新增了这一列
		stock INTEGER DEFAULT 0
	);
	
	CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		customer_name TEXT, 
		phone TEXT, 
		status TEXT DEFAULT 'Pending', 
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE TABLE IF NOT EXISTS order_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		order_id INTEGER, 
		product_id INTEGER, 
		product_name TEXT, 
		price REAL, 
		qty_ordered INTEGER, 
		qty_picked INTEGER DEFAULT 0, 
		FOREIGN KEY(order_id) REFERENCES orders(id), 
		FOREIGN KEY(product_id) REFERENCES products(id)
	);
	`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Fatal("创建表结构失败: ", err)
	}

	// 初始化测试数据 (也顺便更新了，加上了进价)
	var count int
	db.QueryRow("SELECT COUNT(*) FROM products").Scan(&count)
	return db
}
