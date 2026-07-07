package database

import (
	"database/sql"
	"log"
	"os"

	_ "modernc.org/sqlite" // 使用纯 Go 的 SQLite 驱动
)

// Init 初始化默认数据库连接与自动表迁移
func Init() *sql.DB {
	return InitDB("pos_data.db")
}

// InitDB 根据指定路径初始化数据库连接与自动表迁移（支持单元测试传入自定义临时路径）
func InitDB(dbPath string) *sql.DB {

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

	// 开启 WAL 日志模式及高并发参数配置，杜绝多并发下的 database is locked 错误并提升读写性能
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA foreign_keys = ON;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			log.Printf("配置数据库并发参数失败 [%s]: %v", p, err)
		}
	}

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

	var count int
	err = db.QueryRow("SELECT count(*) FROM pragma_table_info('order_items') WHERE name='qty_refunded'").Scan(&count)
	if count == 0 {
		log.Println("正在升级数据库: 添加 qty_refunded 列...")
		_, err = db.Exec("ALTER TABLE order_items ADD COLUMN qty_refunded INTEGER DEFAULT 0")
		if err != nil {
			log.Fatal("升级数据库失败:", err)
		}
	}

	var countSeq int
	err = db.QueryRow("SELECT count(*) FROM pragma_table_info('orders') WHERE name='daily_seq'").Scan(&countSeq)
	if countSeq == 0 {
		log.Println("正在升级数据库: 添加 daily_seq 列...")
		_, err = db.Exec("ALTER TABLE orders ADD COLUMN daily_seq INTEGER DEFAULT 0")
		if err != nil {
			log.Fatal("升级数据库(daily_seq)失败:", err)
		}
	}

	var countPUnit int
	err = db.QueryRow("SELECT count(*) FROM pragma_table_info('products') WHERE name='unit'").Scan(&countPUnit)
	if countPUnit == 0 {
		log.Println("正在升级数据库: 添加 products.unit 列...")
		_, err = db.Exec("ALTER TABLE products ADD COLUMN unit TEXT DEFAULT '个'")
		if err != nil {
			log.Fatal("升级数据库(products.unit)失败:", err)
		}
	}

	var countOUnit int
	err = db.QueryRow("SELECT count(*) FROM pragma_table_info('order_items') WHERE name='unit'").Scan(&countOUnit)
	if countOUnit == 0 {
		log.Println("正在升级数据库: 添加 order_items.unit 列...")
		_, err = db.Exec("ALTER TABLE order_items ADD COLUMN unit TEXT DEFAULT '个'")
		if err != nil {
			log.Fatal("升级数据库(order_items.unit)失败:", err)
		}
	}

	var countQP int
	err = db.QueryRow("SELECT count(*) FROM pragma_table_info('order_items') WHERE name='qty_paid'").Scan(&countQP)
	if countQP == 0 {
		log.Println("正在升级数据库: 添加 order_items.qty_paid 列...")
		_, err = db.Exec("ALTER TABLE order_items ADD COLUMN qty_paid INTEGER DEFAULT 0")
		if err != nil {
			log.Fatal("升级数据库(order_items.qty_paid)失败:", err)
		}
	}

	log.Println("Database initialized successfully.")
	return db
}
