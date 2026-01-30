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

	// --- 1. 商品表 (新增 category) ---
	sqlStmt := `
	CREATE TABLE IF NOT EXISTS products (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		barcode TEXT UNIQUE, 
		name TEXT, 
		category TEXT DEFAULT '未分类',  -- <--- 新增这一列
		price REAL, 
		cost_price REAL DEFAULT 0, 
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

	// 初始化测试数据 (为了演示分类，我们预设两个不同分类的商品)
	var count int
	db.QueryRow("SELECT COUNT(*) FROM products").Scan(&count)
	if count == 0 {
		// 注意：这里的 SQL 参数多了 category
		db.Exec("INSERT INTO products (barcode, name, category, price, cost_price, stock) VALUES (?, ?, ?, ?, ?, ?)",
			"123456", "可口可乐", "饮料", 3.00, 2.20, 100)
		db.Exec("INSERT INTO products (barcode, name, category, price, cost_price, stock) VALUES (?, ?, ?, ?, ?, ?)",
			"888888", "精品香蕉", "水果", 5.50, 3.00, 50)
	}
	return db
}
