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
		stock INTEGER,
		unit TEXT DEFAULT '个'
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
		unit TEXT,
		FOREIGN KEY(order_id) REFERENCES orders(id)
	);
	`
	if _, err := db.Exec(createTables); err != nil {
		log.Fatal(err)
	}

	// 2. 检查并添加 qty_refunded 列 (部分退款支持)
	var count int
	err = db.QueryRow("SELECT count(*) FROM pragma_table_info('order_items') WHERE name='qty_refunded'").Scan(&count)
	if count == 0 {
		log.Println("正在升级数据库: 添加 qty_refunded 列...")
		_, err = db.Exec("ALTER TABLE order_items ADD COLUMN qty_refunded INTEGER DEFAULT 0")
		if err != nil {
			log.Fatal("升级数据库失败:", err)
		}
	}

	// 3. [新增] 检查并添加 daily_seq 列 (每日流水号)
	var countSeq int
	err = db.QueryRow("SELECT count(*) FROM pragma_table_info('orders') WHERE name='daily_seq'").Scan(&countSeq)
	if countSeq == 0 {
		log.Println("正在升级数据库: 添加 daily_seq 列...")
		// 旧数据默认设为 0
		_, err = db.Exec("ALTER TABLE orders ADD COLUMN daily_seq INTEGER DEFAULT 0")
		if err != nil {
			log.Fatal("升级数据库(daily_seq)失败:", err)
		}
	}

	// 4. [新增] 检查并添加 products.unit 列
	var countPUnit int
	err = db.QueryRow("SELECT count(*) FROM pragma_table_info('products') WHERE name='unit'").Scan(&countPUnit)
	if countPUnit == 0 {
		log.Println("正在升级数据库: 添加 products.unit 列...")
		_, err = db.Exec("ALTER TABLE products ADD COLUMN unit TEXT DEFAULT '个'")
		if err != nil {
			log.Fatal("升级数据库(products.unit)失败:", err)
		}
	}

	// 5. [新增] 检查并添加 order_items.unit 列
	var countOUnit int
	err = db.QueryRow("SELECT count(*) FROM pragma_table_info('order_items') WHERE name='unit'").Scan(&countOUnit)
	if countOUnit == 0 {
		log.Println("正在升级数据库: 添加 order_items.unit 列...")
		_, err = db.Exec("ALTER TABLE order_items ADD COLUMN unit TEXT DEFAULT '个'")
		if err != nil {
			log.Fatal("升级数据库(order_items.unit)失败:", err)
		}
	}

	log.Println("Database initialized successfully.")
	return db
}
