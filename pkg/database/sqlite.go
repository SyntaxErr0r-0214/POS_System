package database

import (
	"database/sql"
	"log"
	"os"

	_ "modernc.org/sqlite" // 使用纯 Go 的 SQLite 驱动
)

func Init() *sql.DB {
	dbPath := "pos_data.db"

	// 确保文件存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		file, err := os.Create(dbPath)
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	// 1. 创建基础表结构
	createTables := `
	CREATE TABLE IF NOT EXISTS products (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		barcode TEXT,
		name TEXT,
		category TEXT,
		price REAL,
		cost_price REAL DEFAULT 0,
		stock INTEGER
	);
	CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		customer_name TEXT,
		phone TEXT,
		status TEXT, 
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS order_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_id INTEGER,
		product_id INTEGER,
		product_name TEXT,
		price REAL,
		qty_ordered INTEGER,
		qty_picked INTEGER,
		FOREIGN KEY(order_id) REFERENCES orders(id)
	);
	`
	if _, err := db.Exec(createTables); err != nil {
		log.Fatal(err)
	}

	// 2. [关键升级] 检查并添加 qty_refunded 列 (部分退款支持)
	// 这一步保证了旧数据也能兼容，不用删库
	var count int
	err = db.QueryRow("SELECT count(*) FROM pragma_table_info('order_items') WHERE name='qty_refunded'").Scan(&count)
	if count == 0 {
		log.Println("正在升级数据库: 添加 qty_refunded 列...")
		_, err = db.Exec("ALTER TABLE order_items ADD COLUMN qty_refunded INTEGER DEFAULT 0")
		if err != nil {
			log.Fatal("升级数据库失败:", err)
		}
	}

	log.Println("Database initialized successfully.")
	return db
}
