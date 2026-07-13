*Read this in other languages: [English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md)

---

<h1 align = "center">

Modern POS System - Technical Architecture and Design Specifications

[![WeChat](https://img.shields.io/badge/WeChat-Connect-07c160?logo=wechat&logoColor=white)](docs/images/WeChat.png)
[![Telegram](https://img.shields.io/badge/Telegram-Connect-07c160?logo=telegram&logoColor=white)](docs/images/Telegram.png)
[![WhatsApp](https://img.shields.io/badge/WhatsApp-Connect-07c160?logo=whatsapp&logoColor=white)](docs/images/WhatsApp.png)
[![Line](https://img.shields.io/badge/Line-Connect-07c160?logo=line&logoColor=white)](docs/images/Line.png)

![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?logo=go&logoColor=white)
![Database](https://img.shields.io/badge/Database-SQLite3%20(Pure%20Go)-003B57?logo=sqlite&logoColor=white)
![Frontend](https://img.shields.io/badge/Frontend-Vanilla%20JS%20%7C%20HTML5%20%7C%20CSS3-E34F26?logo=html5&logoColor=white)
![Architecture](https://img.shields.io/badge/Architecture-RESTful%20%7C%20Single%20Binary-4A154B)
![License](https://img.shields.io/badge/License-MIT-blue.svg)

</div>

This system is a lightweight, high-performance, and dependency-free modern point-of-sale (POS) and inventory management system built with Go and an embedded SQLite3 database. The system abandons traditional heavy Web frameworks and complex frontend build toolchains, adopting a classic monolithic layered architecture with a pure DOM/RESTful API interaction model. It focuses on underlying high reliability and computational accuracy in complex business scenarios for retail and specialty stores, such as weight-based pricing, multi-dimensional prepayment tracking, zero-price pending orders for temporary products, and fail-safe order modifications.

---

## 1. System Overall Architecture and Design Philosophy

In terms of software engineering design, this system adheres to the architectural principles of "minimalism" and "high cohesion, low coupling," pursuing ultimate operational efficiency and zero maintenance cognitive load:

1. **Dependency-Free Backend**: The backend is built entirely on the `net/http` package of the Go 1.20+ standard library, functioning as a pure RESTful API without introducing any third-party HTTP routing frameworks (such as Gin, Echo, etc.). This avoids additional framework overhead and dependency conflicts, ensuring high operational efficiency and an extremely minimal compiled binary size.
2. **Pure Go SQLite3 Database**: The data persistence layer uses the `modernc.org/sqlite` driver. This driver is based on C99-to-Go transpilation technology, allowing Go to operate the SQLite3 database entirely without enabling CGO (C-Go Interoperability). This completely eliminates the underlying GCC/Clang dependencies during CGO compilation, enabling the system to be seamlessly cross-compiled across platforms on any development machine (e.g., one-click compiling a native static Windows binary on macOS).
3. **Zero-Build Frontend**: The frontend interface is built directly with Vanilla JavaScript, native CSS Variables, and HTML5, eliminating complex compilation and bundling processes involving Node.js, Webpack, Vite, or npm. The system only requires static layout files to run, lowering the threshold for system operation, maintenance, and secondary customization.

---

## 2. Software Layered Architecture and Code Organization

The system code strictly follows a classic Layered Architecture, achieving Separation of Concerns from a software engineering perspective. This ensures that the core business logic layer remains unaffected by changes in external interface protocols and storage media:

```
+-----------------------------------------------------------------------+
|                       HTTP Handler Layer                              |
|   OrderHandler   |   ProductHandler   |   ReportHandler  |   System   |
+-----------------------------------------------------------------------+
                                   | (Interface Calls / DTO Passing)
                                   v
+-----------------------------------------------------------------------+
|                         Service Layer                                 |
|   CheckoutService   |   InventoryService   |   ReportService          |
|  [Reverse Loading]  |  [Fail-Safe Pickup]  |   [Zero-Price Sync]      |
+-----------------------------------------------------------------------+
                                   | (Dependency Injection / Decoupling)
                                   v
+-----------------------------------------------------------------------+
|                        Repository Layer                               |
|   OrderRepo      |   ProductRepo      |   ReportRepo                  |
+-----------------------------------------------------------------------+
                                   | (Native SQL / Transactions Tx)
                                   v
+-----------------------------------------------------------------------+
|                      Infrastructure Layer                             |
|   SQLite3 Database Engine    |    Windows RAW winspool.drv Printer    |
+-----------------------------------------------------------------------+
```

### 2.1 Handler Layer
Located in the `internal/handler` directory, this layer is responsible for receiving HTTP requests, parsing parameters, validating data, and formatting JSON responses. The Handlers do not interact with the database directly; instead, they reference the dependent business Logic/Service interfaces through struct members.

### 2.2 Service Layer
Located in the `internal/service` directory, this layer houses the system's core commercial logic and security validations.
* **Dependency Injection**: In the system startup entry point `main.go`, specific Repository implementation instances are assembled and injected into each Service. This loosely coupled design ensures that the system can undergo unit testing (Mocking) or switch storage engines in the future without modifying the business logic code.
* **Concurrency & Async Decoupling**: For hardware IO operations (such as thermal receipt printing), the Service layer utilizes Go Goroutines (`go printAsync`) for asynchronous decoupling. Even if the user's underlying hardware is waiting or responding slowly, the main processing thread remains fully asynchronous and non-blocking, immediately returning the HTTP status result and completing the database transaction commit to ensure high responsiveness at the frontend POS.

### 2.3 Repository Layer
Located in the `internal/repository` directory, this layer is responsible for constructing, executing, and mapping result sets for all SQL statements. It shields the upper layers from the complexity of underlying SQLite statements, handling deep join queries, pagination, multi-field keyword retrieval, and complex aggregate statistics.

### 2.4 Infrastructure Layer
Located in the `pkg/printer` directory, this layer is responsible for operating system peripherals and underlying resource scheduling.
* **Printer Driver Abstraction Interface**: Declares the global standard interface `Printer` and its core method `PrintTicket(content string) error`.
* **Deep Encapsulation of Windows APIs**: For POS-58 thermal printers common in Windows environments, the system does not invoke the heavy top-level system graphical drawing drivers. Instead, it directly calls core APIs like `OpenPrinterW`, `StartDocPrinterW`, and `WritePrinter` in the Windows core dynamic link library `winspool.drv` using Go `syscall` and `unsafe.Pointer`. After converting Go's internal standard UTF-8 strings to the GBK encoding natively supported by the printer, this module sends raw byte streams and ESC/POS hardware paper-cutting commands directly to the printer port in RAW mode, achieving low latency and maximum hardware compatibility.

---

## 3. Core Algorithms and Advanced Business Mechanisms

To resolve practical physical store management challenges such as data consistency, fail-safe error prevention, and real-time price response, the system implements several advanced core algorithms:

### 3.1 Reverse Loading & Safe Modification Algorithm
In POS operations, a user may repeatedly adjust product quantities, unit prices, or deposits for pending orders before final payment. Therefore, the system incorporates a "Fail-Safe Reverse Loading" algorithm:

1. **Memory Snapshot and Reverse Mapping**: When a user requests to modify a historical pending order, the system extracts all rows belonging to that order from the `order_items` table in the database, reconstructing and formatting them into a standard shopping cart object (`cart`) in memory. The system simultaneously marks the global state as "Edit Locked" for this order.
2. **Bidirectional Safety Inventory Compensation Mechanism**:
   * Upon submitting the modified order, the system calculates the difference in quantity for each product between the new cart and the original order (`delta = new_qty - old_qty`) in real-time.
   * If `delta > 0` (additional order), the system requests an exclusive inventory check to verify if the main stock is sufficient. If sufficient, it deducts the `delta` quantity from the main table; if insufficient, it immediately intercepts the transaction and throws a prompt.
   * If `delta < 0` (reduced order), the system automatically releases the difference and restores the true inventory in the main product database.
3. **Fail-Safe Pick-up Quantity Algorithm**: In modification and partial refund logic, the system strictly applies a formula validation to every product row:
   $$\text{ValidQty} \ge (\text{QtyPicked} - \text{QtyRefunded})$$
   If a cashier attempts to modify or refund a quantity that causes the ordered amount to drop below what the customer has already picked up (and not yet refunded), the underlying business service immediately triggers a security exception. This validation fundamentally eliminates the financial risk of "goods picked up, but bill deleted or refunded."

### 3.2 Temporary Product Auto-Sync & Zero-Price Pricing Algorithm
For scenarios where fresh produce or new arrival batches have not yet been priced and entered into the backend inventory, but the frontend urgently needs to create a sales order, the system features a deferred pricing and automatic compensation mapping mechanism:

1. **Zero-Price Placeholder and Memory Marking**: Allows the frontend to temporarily enter unpriced products into the shopping cart (cost and current price default to 0), generating a pending order and locking the corresponding inventory and customer attributes.
2. **Stock-In Trigger Mechanism**: When backend management supplements the cost and current price for this temporary product in the procurement module (`Procure`) and saves it, the system's update transaction actively dispatches an inventory and price refresh command after persistence is complete.
3. **Auto-Sync and Linkage Completion Algorithm**: Upon receiving the update signal, the frontend executes a matching traversal between the memory shopping cart and the pending order pool. For every product matching the ID, currently in a pending state or in the active cart, with a historical price of 0 and a new price greater than 0, the system instantly auto-completes the latest unit price and cost price in memory, and triggers `calcMargin()` and total recalculation rendering. This entire synchronization is fully automated, eliminating the tedious process of manually digging through historical orders to adjust prices at a busy checkout counter.

### 3.3 Item-Level Tracking and Payment Mechanism
Unlike standard POS systems that only make a binary "Paid/Unpaid" determination for the "entire order," this system introduces a four-tiered tracking count at the data structure level **for each independent SKU product**:
* **`qty_ordered`**: Initial total ordered quantity
* **`qty_picked`**: Quantity picked up/verified
* **`qty_paid`**: Quantity prepaid/settled
* **`qty_refunded`**: Quantity returned/refunded

**Pick-up and Payment Leak-Proof Algorithm**: When a customer performs batch pick-ups (`Pickup`) against a pending order, if certain products in the pick-up list were not marked as prepaid during the initial ordering, the system, while executing the `qty_picked += delta` update, automatically syncs the `qty_paid` for that product to a value no less than the pick-up quantity. This design logically ensures at the base level that any product physically picked up is necessarily marked as "settled," preventing the risk of uncollected payments in the business workflow.

### 3.4 Atomic System Reset Algorithm
To accommodate business needs for switching to formal commercial operation after deployment and debugging, or for complete resets of legacy versions, the system implements an atomic reset API (`/api/system/reset`):
1. **Exclusive Transaction Initiation**: Executes `db.BeginTx` to acquire an exclusive lock, preventing external concurrent writes during the clearing process.
2. **Reverse Foreign Key Clearance**: Strictly following relational database foreign key constraint orders, it sequentially executes:
   * `DELETE FROM order_items;`
   * `DELETE FROM orders;`
   * `DELETE FROM products;`
3. **Auto-Increment Primary Key Sequence Reset**: The system executes `DELETE FROM sqlite_sequence WHERE name IN ('products', 'orders', 'order_items');` against underlying SQLite system tables. This ensures that after a reset, new product IDs and sales order numbers accurately auto-increment sequentially starting from `1`.
4. **Physical Space Reclamation**: After a successful transaction commit, the underlying database automatically executes the `VACUUM;` command. This rebuilds the underlying SQLite page structure and releases physical disk sectors occupied by deleted records, keeping the database file size small and queries highly efficient.

---

## 4. Database Design and ER Model

The system's core persistence structure is supported by three main tables, allowing for comprehensive indexing and relational computation:

### 4.1 Product Inventory Table (`products`)
* **`id`** (INTEGER PRIMARY KEY AUTOINCREMENT): Internal sequential auto-increment ID.
* **`barcode`** (TEXT UNIQUE): Barcode / electronic scale scan marker, creating a unique index at the database level to accelerate scan retrieval.
* **`name`** (TEXT NOT NULL): Standard product name.
* **`category`** (TEXT NOT NULL): Product category, supporting batch modification and aggregation.
* **`cost_price`** (REAL NOT NULL): Product cost price, used to calculate real-time gross margin and net revenue.
* **`price`** (REAL NOT NULL): Frontend retail price.
* **`stock`** (INTEGER NOT NULL): Currently available physical inventory (supports automatic floating and pre-deduction based on orders).
* **`unit`** (TEXT NOT NULL): Unit of measurement (e.g., piece, kg, box, gram).

### 4.2 Order Master Table (`orders`)
* **`id`** (INTEGER PRIMARY KEY AUTOINCREMENT): System global unique sequential order ID.
* **`daily_seq`** (INTEGER NOT NULL): Daily queue sequence number. The system resets and auto-increments this daily, providing short queue numbers suitable for real-world store scenarios.
* **`customer_name`** (TEXT): Booking customer's name.
* **`phone`** (TEXT): Booking customer's phone number, supporting fast fuzzy retrieval on the frontend based on phone number prefixes.
* **`total_amount`** (REAL NOT NULL): Total receivable amount for the products ordered.
* **`paid_amount`** (REAL NOT NULL): Actual prepaid/settled amount for this order.
* **`status`** (TEXT NOT NULL): Status enumeration: `Pending`, `Completed` (real-time settlement finished), `Refunded` (full order refunded), `Partial` (partial return/refund occurred).
* **`created_at`** / **`updated_at`** (DATETIME): Transaction creation and last modification timestamps.

### 4.3 Order Item Details Table (`order_items`)
Acts as the many-to-many mapping carrier between `orders` and `products`, while also taking on the crucial role of preserving historical instantaneous prices and snapshot tracking:
* **`id`** (INTEGER PRIMARY KEY AUTOINCREMENT): Detail row ID.
* **`order_id`** (INTEGER NOT NULL, FOREIGN KEY -> orders.id): Associated master table ID.
* **`product_id`** (INTEGER NOT NULL, FOREIGN KEY -> products.id): Associated product ID.
* **`product_name`** (TEXT NOT NULL): Order snapshot name (prevents future product name changes from affecting historical receipts).
* **`price`** (REAL NOT NULL) / **`cost_price`** (REAL NOT NULL): Retail and cost price snapshots at the exact moment of order placement.
* **`qty_ordered`** / **`qty_picked`** / **`qty_paid`** / **`qty_refunded`** (REAL NOT NULL): The four-tuple defining the quantity status flow.

---

## 5. Compilation, Cross-Platform Build, and Deployment Specifications

Because this system eliminates dependencies on external runtime environments and C language compilers, you can execute cross-platform deployments and static binary generation directly via the Go toolchain.

### 5.1 Local Compilation and Execution
After preparing a Go 1.20+ development environment, navigate to the project root directory and run the following commands:

```bash
# Sync and verify Go Modules dependencies (primarily the modernc.org/sqlite pure Go driver)
go mod tidy

# Compile an executable static binary for the current OS
go build -o modern-pos main.go

# Start the POS server
./modern-pos
```

Once started, the server safely binds to the local network interface by default, listening on `127.0.0.1:8080` (preventing accidental exposure to the public internet by binding to `0.0.0.0` in a production environment). Use any modern browser to visit `http://127.0.0.1:8080` to log in and operate the POS system. For production deployments, it is recommended to specify the exact network adapter IP and strictly restrict access using firewalls or security group rules.

### 5.2 Cross-Compilation and GUI-less Deployment (Windows POS Native Support)
To accommodate the Windows workstation environments used in the vast majority of physical stores, you can use environment variables to compile a pure native Windows application without a console window in a single click:

```bash
# Cross-compile a Windows X86-64 architecture executable from Linux / macOS
# The purpose of -ldflags="-s -w -H windowsgui":
#   -s -w : Strips debugging symbols, significantly compressing the generated EXE file size
#   -H windowsgui : Hides the black command prompt window upon Windows startup, letting the program run silently in the background as a service
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o modern-pos.exe main.go
```

Place the generated `modern-pos.exe` and the `static` directory in the same folder. Double-click to run it directly on the store computer without installing any Go environment, Node runtime libraries, or database software on the target machine.

### 5.3 Database High Reliability and Backup/Restore Strategy
During runtime, the system automatically creates and operates the `pos_data.db` file in the working directory as the embedded SQLite3 physical storage medium:
* **WAL Concurrency Mode**: Upon initialization, the system explicitly enables `PRAGMA journal_mode=WAL` (Write-Ahead Logging) and `PRAGMA busy_timeout=5000` (5-second busy wait timeout). This completely resolves read/write lock conflicts and eliminates `database is locked` exceptions when high-concurrency transactions and batch report queries occur simultaneously.
* **Scheduled Hot Backups and Integrity Checks**: Features a built-in `BackupService` background cron task that defaults to an atomic hot backup every 6 hours. The backup utilizes the SQLite zero-blocking atomic command `VACUUM INTO` to generate a snapshot. Immediately after generation, it opens the snapshot to execute a deep `PRAGMA integrity_check`. Once verified, it is automatically saved to the `./backups` directory, and historical snapshots older than 30 days are cleaned up. If corruption is found during validation, the snapshot is safely destroyed immediately, ensuring archive files are 100% restorable.
* **High-Risk Operation Authentication and Row Auditing**: High-risk operational APIs like `/api/system/backup`, `/api/system/restore`, and `/api/system/reset` strictly apply a `RequireAdmin` permission validation middleware (via `subtle.ConstantTimeCompare` to prevent timing attacks). During the execution of underlying Repository SQL for order deductions and refunds, the number of affected rows (`RowsAffected`) is strictly verified to prevent inventory over-deduction or pick-up quantity inversion under high multi-user concurrency.

### 5.4 Stability Assurance, Observability, and CI/CD Specifications
* **Comprehensive Observability Auditing (Logging Middleware)**: The system intercepts and logs the microsecond-level response time and actual status code of every HTTP call. It automatically outputs exclusive `[Business Audit]` logs for critical checkout and inventory paths such as `/api/checkout`, `/api/refund`, `/api/inventory/*`, and `/api/system/*`.
* **Self-Healing and Security Protection (Panic Recovery & Server Timeouts)**: A global `Recovery` middleware intercepts all unhandled runtime exceptions (Panics) and saves stack traces. In the event of a localized failure, it returns a safe, user-friendly 500 error to the client while the main application process avoids crashing, ensuring other POS terminals continue operating. The underlying `http.Server` is strictly configured with `ReadTimeout (15s)`, `WriteTimeout (30s)`, and `IdleTimeout (60s)` to completely safeguard against slowloris attacks and connection handle exhaustion.
* **Graceful Shutdown**: The system listens for OS shutdown signals (`SIGINT`/`SIGTERM`) in the background. Upon receiving a shutdown command, it first gracefully stops scheduled hot backup tasks to prevent triggering new writes; then it grants a 10-second wrap-up window via `srv.Shutdown(ctx)` for currently active transactions and downloads; finally, it safely closes the `sql.DB` driver, ensuring all write-ahead logs are fully flushed to disk.
* **Continuous Integration and Automated Cross-Compilation (GitHub Actions CI/CD)**: The project is configured with a complete `.github/workflows/ci.yml` automated pipeline. Every commit or Pull Request automatically spins up environments to run code linting, unit tests, and concurrency race checks (`go test -race ./...`). Through an automated build matrix, every release can simultaneously compile native executable files for Linux (amd64), Windows (amd64 GUI version), and macOS (Intel amd64 and Apple Silicon arm64) to be archived as Release assets.

---

## 6. Open Source License and Acknowledgments

This software and its related source code architecture operate under the Apache 2.0 License, a free and open-source license. We encourage developers, integrators, and practitioners to customize, privately deploy, and secondarily distribute this system in commercial or non-commercial operational activities.

When referencing or adapting the underlying software architecture, core fail-safe algorithms, or frontend design paradigms of this project, please credit the original author in the product documentation or acknowledgments section: **[Ju1ian SyntaxErr0r Zhang]**. Thank you for contributing to the functional refinement, deep testing, and commercial logical architecture deductions of this project.
