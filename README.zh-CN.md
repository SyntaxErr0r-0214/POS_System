<h1 align = "center">极简智能收银台系统 - 技术架构与设计规范</h1>

<div align = "center">

[![WeChat](https://img.shields.io/badge/WeChat-Connect-07c160?logo=wechat&logoColor=white)](docs/images/WeChat.png)
[![Telegram](https://img.shields.io/badge/Telegram-Connect-07c160?logo=telegram&logoColor=white)](docs/images/Telegram.png)
[![WhatsApp](https://img.shields.io/badge/WhatsApp-Connect-07c160?logo=whatsapp&logoColor=white)](docs/images/WhatsApp.png)
[![Line](https://img.shields.io/badge/Line-Connect-07c160?logo=line&logoColor=white)](docs/images/Line.png)

![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?logo=go&logoColor=white)
![Database](https://img.shields.io/badge/Database-SQLite3%20(Pure%20Go)-003B57?logo=sqlite&logoColor=white)
![Frontend](https://img.shields.io/badge/Frontend-Vanilla%20JS%20%7C%20HTML5%20%7C%20CSS3-E34F26?logo=html5&logoColor=white)
![Architecture](https://img.shields.io/badge/Architecture-RESTful%20%7C%20Single%20Binary-4A154B)
![License](https://img.shields.io/badge/License-Apache-blue.svg)

</div>

本系统是一款基于 Go 语言和 SQLite3 嵌入式数据库构建的轻量级、高性能、无依赖现代收银与进销存管理系统。系统放弃了传统的重型 Web 框架和复杂的前端构建工具链，采用经典的单体分层架构与纯粹的 DOM/RESTful API 交互模式，专注于零售与特产门店在称重计价、多维度预付款追踪、临时商品零价挂单以及订单防呆修改等复杂业务场景下的底层高可靠性与计算精确性。

---

## 1. 系统总体架构与设计哲学

在软件工程设计上，本系统奉行“极简主义”与“高内聚、低耦合”的架构原则，追求极致的运行效率与零维护心智负担：

1. **去依赖化后端 (Dependency-Free Backend)**：后端完全基于 Go 1.20+ 标准库的 `net/http` 包构建纯 RESTful API，不引入任何第三方 HTTP 路由框架（如 Gin、Echo 等），避免了额外的框架开销与依赖冲突，确保了程序的高运行效率与极简编译体积。
2. **纯 Go 驱动嵌入式数据库 (Pure Go SQLite3)**：数据持久化层采用 `modernc.org/sqlite` 驱动。该驱动基于 C99-to-Go 转译技术，使 Go 在操作 SQLite3 数据库时完全不需要开启 CGO (C-Go Interoperability)。这彻底消除了 CGO 编译时的底层 GCC/Clang 依赖，使得系统可以在任何开发机上无缝跨平台交叉编译（例如在 macOS 下一键编译 Windows 原生静态二进制文件）。
3. **零构建前端 (Zero-Build Frontend)**：前端界面直接基于 Vanilla JavaScript、原生 CSS Variables 与 HTML5 构建，排除了 Node.js、Webpack、Vite 或 npm 等复杂的编译打包流程。系统运行仅需提供静态排版文件，降低了系统运维和二次定制的门槛。

---

## 2. 软件分层架构与代码组织

系统代码严格遵循经典的分层架构 (Layered Architecture)，从软件工程角度实现职责分离 (Separation of Concerns)，确保了核心业务逻辑层不受外部接口协议和存储介质变化的影响：

```
+-----------------------------------------------------------------------+
|                       HTTP 接口层 (Handler Layer)                      |
|   OrderHandler   |   ProductHandler   |   ReportHandler  |   System   |
+-----------------------------------------------------------------------+
                                   | (接口调用 / DTO 传递)
                                   v
+-----------------------------------------------------------------------+
|                      业务逻辑层 (Service Layer)                        |
|   CheckoutService   |   InventoryService   |   ReportService          |
|   [预订单反向装载]    |   [防提货倒挂算法]     |    [零价同步匹配]          |
+-----------------------------------------------------------------------+
                                   | (依赖注入 / 接口解耦)
                                   v
+-----------------------------------------------------------------------+
|                     数据访问层 (Repository Layer)                      |
|   OrderRepo      |   ProductRepo      |   ReportRepo                  |
+-----------------------------------------------------------------------+
                                   | (原生态 SQL / 事务 Tx)
                                   v
+-----------------------------------------------------------------------+
|                    底层基础设施层 (Infrastructure Layer)                |
|   SQLite3 Database Engine    |    Windows RAW winspool.drv Printer    |
+-----------------------------------------------------------------------+
```

### 2.1 接口层 (Handler Layer)
位于 `internal/handler` 目录下，负责 HTTP 请求的接收、参数解析、数据校验以及 JSON 响应的格式化。各 Handler 不直接操作数据库，而是通过结构体成员引用依赖的业务 Logic/Service 接口。

### 2.2 业务逻辑层 (Service Layer)
位于 `internal/service` 目录下，承载了系统的核心商业逻辑与安全校验。
* **依赖注入 (Dependency Injection)**：在系统启动入口 `main.go` 中，向各 Service 装配具体实现的 Repository 实例。这种松耦合的设计使得系统在未来做单元测试（Mock）或切换存储引擎时，无需修改业务逻辑代码。
* **并发与异步解耦 (Concurrency & Async Decoupling)**：对于硬件 IO 操作（如热敏打印机小票打印），Service 层采用 Go 协程 (`go printAsync`) 进行异步解耦处理。即使用户底层硬件处于等待或响应缓慢状态，主处理线程也能够做到全异步非阻塞，立即返回 HTTP 状态结果并完成数据库事务提交，保障前端收银台的高响应度。

### 2.3 数据访问层 (Repository Layer)
位于 `internal/repository` 目录下，负责所有 SQL 语句的构造、执行与结果集映射。该层对上层屏蔽了底层 SQLite 语句的复杂性，负责执行联表深度查询、分页、多字段关键词检索以及复杂聚合统计。

### 2.4 底层硬件与操作系统适配层 (Infrastructure Layer)
位于 `pkg/printer` 目录下，负责操作系统外设和底层资源调度。
* **打印驱动抽象接口**：声明了全局标准接口 `Printer` 及其核心方法 `PrintTicket(content string) error`。
* **Windows API 深度封装**：对于 Windows 环境下常见的 POS-58 热敏打印机，系统没有调用系统顶层的重型驱动图形化绘制，而是通过 Go `syscall` 和 `unsafe.Pointer` 直接调用 Windows 核心动态链接库 `winspool.drv` 中的 `OpenPrinterW`、`StartDocPrinterW`、`WritePrinter` 等核心 API。该模块在将 Go 内部的标准 UTF-8 字符串转换为打印机原生支持的 GBK 编码后，直接以 RAW 模式向打印机端口发送原始字节流与 ESC/POS 硬件切纸指令，实现了低延迟和硬件兼容性。

---

## 3. 核心算法与进阶业务机制

系统为了解决实际实体门面经营中的数据一致性、防呆除错与价格实时响应等难题，实现了多项进阶核心算法：

### 3.1 预订单“反向装载”与“防呆校验”算法 (Reverse Loading & Safe Modification Algorithm)
在POS业务中，已经挂单（Pending）的预订单，用户可能在付款前多次对商品数量、单价或定金进行调整。为此系统设计了“防倒挂校验的反向装载”算法：

1. **内存快照与反向映射**：当用户请求修改某历史预订单时，系统从数据库中提取该订单在 `order_items` 表中的所有行，将其还原并格式化为内存中的标准购物车对象 (`cart`)。系统同时将当前全局状态标注为该订单的“编辑锁定态”。
2. **安全库存双向补偿机制**：
   * 在执行改单提交时，系统实时比对新购物车中各商品的数量与原订单中对应商品的数量差异 (`delta = new_qty - old_qty`)。
   * 若 `delta > 0`（追加订购），系统向上层调用库存排他排查，验证主库存是否充裕。若充裕，则在主表中再扣减 `delta` 数量的库存；若不足，立刻拦截事务并抛出提示。
   * 若 `delta < 0`（减少订购），系统自动将剩余差异差额释放并补回主商品库的真实库存中。
3. **防提货数量倒挂算法 (Pick-up Safety Check)**：在改单与部分退款逻辑中，系统对每一行商品严格执行公式校验：
   $$\text{ValidQty} \ge (\text{QtyPicked} - \text{QtyRefunded})$$
   一旦收银员试图修改或退除的数量导致订购量低于客户此前已经提走（且尚未退款）的数量，底层业务服务将立即触发安全异常。此校验从根本上杜绝了“货物已被取走，但账单被删减或退款”的资金倒挂风险。

### 3.2 临时商品“零价挂单”与“价格自动同步”算法 (Temporary Product Auto-Sync)
针对生鲜或到货新批次尚未完成后台入库定价，但前台急需销售开单的场景，系统设计了价格延迟决断与自动补偿映射机制：

1. **零价占位与内存标记**：允许前台在购物车中临时录入尚未定价的商品（进价与现价默认初始化为 0），生成挂单并锁定对应库存与客户属性。
2. **入库触发机制**：当后台管理人员在进货入库模块 (`Procure`) 对该临时商品补充进价和现价并保存后，系统的更新事务在完成持久化后，主动派发库存与价格刷新指令。
3. **自动同步与联动补全算法**：前端接收到更新信号后，执行内存购物车与挂单池匹配遍历。对于每一个处于预订挂单状态或当前购物车中商品 ID 匹配、且历史价格为 0、最新价格大于 0 的商品，系统立即在内存中自动补全最新单价与进价，并调用 `calcMargin()` 和总计重新计算渲染。整个同步全自动化完成，彻底清除了收银台在繁忙时手动翻找历史订单改价的繁琐工序。

### 3.3 颗粒度到单品的付款与提货追踪机制 (Item-Level Tracking)
不同于普通收银系统仅对“整单”进行“已付/未付”的二元判定，本系统在数据结构层面引入了颗粒度细化到**每个独立 SKU 商品**的四重计数跟踪：
* **`qty_ordered`**：初始订购总数
* **`qty_picked`**：已提货核销数量
* **`qty_paid`**：已结算预付数量
* **`qty_refunded`**：已完成退货数量

**提货与付款防漏算法**：当客户对预订单进行分批提货操作 (`Pickup`) 时，若提货清单中的某些商品在早先挂单时并未勾选预付标记，系统在执行 `qty_picked += delta` 更新的同时，自动将此商品的 `qty_paid` 同步更新至不小于提货数量的值。该设计在逻辑底层保证了任何发生物理提货的商品，在其属性上必然被标记为“已结算”，防范了业务流程中可能出现的漏收款隐患。

### 3.4 系统级原子化数据清空与重置算法 (Atomic System Reset)
为适应系统在上线部署调试完毕后切换至商业正式营业，或旧版本整体重置的业务需求，系统实现了原子化重置接口 (`/api/system/reset`)：
1. **排他事务开启**：执行 `db.BeginTx` 获取独占锁，防止在清空过程中有外来并发写入。
2. **外键逆向清除**：严格按照关系型数据库的外键约束顺序，依次执行：
   * `DELETE FROM order_items;`
   * `DELETE FROM orders;`
   * `DELETE FROM products;`
3. **自增主键序列归零**：系统针对 SQLite 底层系统表执行 `DELETE FROM sqlite_sequence WHERE name IN ('products', 'orders', 'order_items');`。这保证了重置后，新的商品编号与销售单号准确无误地自数字 `1` 顺序递增。
4. **物理空间回收**：事务成功提交后，底层数据库自动执行 `VACUUM;` 指令。该指令重新整理 SQLite 底层的页结构并释放已经被删除记录占用的物理磁盘扇区，保持数据库文件体积小巧与查询高效。

---

## 4. 数据库设计与实体关系模型 (ER Model)

系统底层核心持久化结构以三张主体表为支撑，支持完备的索引与关联计算：

### 4.1 商品库存表 (`products`)
* **`id`** (INTEGER PRIMARY KEY AUTOINCREMENT)：商品内部流水自增 ID。
* **`barcode`** (TEXT UNIQUE)：商品条形码/电子秤扫码标记，在数据库层面创建唯一索引，加速扫描检索。
* **`name`** (TEXT NOT NULL)：商品标准名称。
* **`category`** (TEXT NOT NULL)：商品所属品类，支持批量修改聚合。
* **`cost_price`** (REAL NOT NULL)：商品进货成本价，用于核算实时毛利率与营收净利。
* **`price`** (REAL NOT NULL)：商品前台零售售价。
* **`stock`** (INTEGER NOT NULL)：当前可用物理库存（支持根据订单自动上下浮动与预扣除）。
* **`unit`** (TEXT NOT NULL)：商品计量单位（如：个、斤、箱、克）。

### 4.2 订单主表 (`orders`)
* **`id`** (INTEGER PRIMARY KEY AUTOINCREMENT)：系统订单全局唯一流水编号。
* **`daily_seq`** (INTEGER NOT NULL)：每日排队排号流水单号。系统按天为周期归零并自动重新自增运算，提供符合实际门店场景的简短排号。
* **`customer_name`** (TEXT)：预订客户姓名。
* **`phone`** (TEXT)：预订客户联系电话，支持前台基于电话号码前缀的快速模糊检索。
* **`total_amount`** (REAL NOT NULL)：该订单订购商品应收总额。
* **`paid_amount`** (REAL NOT NULL)：该订单实际已预付/已结算金额。
* **`status`** (TEXT NOT NULL)：状态枚举：`Pending` (预订挂单)、`Completed` (实时结算完成)、`Refunded` (已全单退款)、`Partial` (部分退货/发生部分退款)。
* **`created_at`** / **`updated_at`** (DATETIME)：事务创建时间戳与最后修改时间戳。

### 4.3 订单商品明细表 (`order_items`)
作为 `orders` 与 `products` 之间的多对多映射载体，同时承担了保存历史瞬间价格与快照追踪的重任：
* **`id`** (INTEGER PRIMARY KEY AUTOINCREMENT)：明细行 ID。
* **`order_id`** (INTEGER NOT NULL, FOREIGN KEY -> orders.id)：关联主表 ID。
* **`product_id`** (INTEGER NOT NULL, FOREIGN KEY -> products.id)：关联商品 ID。
* **`product_name`** (TEXT NOT NULL)：订购快照名称（避免后续商品改名影响历史小票）。
* **`price`** (REAL NOT NULL) / **`cost_price`** (REAL NOT NULL)：下单瞬间的零售价与进货价快照。
* **`qty_ordered`** / **`qty_picked`** / **`qty_paid`** / **`qty_refunded`** (REAL NOT NULL)：数量状态流转四元组。

---

## 5. 编译、跨平台构建与部署规范

由于本系统消除了对外界运行环境与 C 语言编译器的依赖，您可以直接通过 Go 工具链执行跨平台部署与静态二进制生成。

### 5.1 本地编译与运行
在准备好 Go 1.20 及以上开发环境后，进入项目根目录执行以下命令：

```bash
# 同步并验证 Go Modules 依赖 (主要为 modernc.org/sqlite 纯 Go 数据库驱动)
go mod tidy

# 编译当前操作系统的可执行静态二进制程序
go build -o modern-pos main.go

# 启动收银台服务器
./modern-pos
```

启动程序后，服务器会默认安全绑定本机网络接口监听 `127.0.0.1:8080`（防止生产环境误绑 `0.0.0.0` 暴露至外网）。使用任意现代浏览器访问 `http://127.0.0.1:8080` 即可登入并操作收银系统。对于生产环境部署，建议指定明确网卡 IP 并严格配合防火墙或安全组规则限制访问。

### 5.2 交叉编译与免命令行窗口部署 (Windows POS 原生支持)
为了适应绝大多数实体门店采用的 Windows 操作台环境，您可以通过环境变量跨平台一键编译出无需控制台窗口的纯原生 Windows 应用程序：

```bash
# 在 Linux / macOS 下交叉编译 Windows X86-64 架构程序
# -ldflags="-s -w -H windowsgui" 的作用：
#   -s -w : 去除调试符号，大幅压缩编译生成的二进制 EXE 文件体积
#   -H windowsgui : 隐藏 Windows 启动时的黑框命令提示符窗口，使程序静默在后台作为服务运行
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o modern-pos.exe main.go
```

将生成的 `modern-pos.exe` 和 `static` 目录放置同一文件夹下，在店面电脑上双击即可直接运行，无需在目标主机上安装任何 Go 环境、Node 运行库或数据库软件。

### 5.3 数据库高可靠性与备份还原策略
系统运行时，会在工作目录下自动生成并操作 `pos_data.db` 文件作为嵌入式 SQLite3 物理存储介质：
* **WAL 并发预写模式**：系统在初始化时显式开启 `PRAGMA journal_mode=WAL`（预写日志模式）与 `PRAGMA busy_timeout=5000`（5秒忙等待超时）。在高并发交易与批量报表查询同时发生时，彻底解决读写锁冲突，杜绝 `database is locked` 异常。
* **定时热备份与完整性校验**：内置 `BackupService` 后台定时任务，默认每 6 小时自动执行一次底层原子热备份。备份使用 SQLite 零阻塞原子指令 `VACUUM INTO` 生成快照，并在生成后立即打开快照执行 `PRAGMA integrity_check` 深度完整性校验，确认无误后自动保存至 `./backups` 目录并清理超过 30 天的历史快照；如校验发现快照损坏，立刻安全销毁，确保存档文件 100% 可恢复。
* **系统高危操作鉴权与行数审计**：对于 `/api/system/backup`、`/api/system/restore` 和 `/api/system/reset` 高危运维接口，强制应用 `RequireAdmin` 权限校验中间件（经由 `subtle.ConstantTimeCompare` 防时序攻击）；在订单扣货与退货底层 Repository SQL 执行中，对受影响行数 (`RowsAffected`) 进行严格验证，防止多用户并发下的库存超扣或提货量倒挂。

### 5.4 稳定性保障、可观测性与 CI/CD 规范
* **全量可观测性审计 (Logging Middleware)**：系统拦截并记录每一次 HTTP 调用的微秒级响应耗时与真实状态码；针对 `/api/checkout`、`/api/refund`、`/api/inventory/*`、`/api/system/*` 等关键收银与库管路径，自动输出专属 `[业务审计]` 日志。
* **异常自愈与安全保护 (Panic Recovery & Server Timeouts)**：通过全局 `Recovery` 中间件拦截所有未处理的运行时异常（Panic）并保存堆栈追踪，在发生局部故障时向客户端返回安全友好的 500 提示，主程序进程不会崩溃，保障其他收银台继续营业；同时对底层 `http.Server` 严格配置 `ReadTimeout (15s)`、`WriteTimeout (30s)` 与 `IdleTimeout (60s)`，彻底防范慢速攻击与长连接句柄耗尽。
* **优雅停机机制 (Graceful Shutdown)**：系统后台监听操作系统停机信号 (`SIGINT`/`SIGTERM`)。在收到停机指令后，首先优雅停止定时热备份任务，防止触发新写入；随后通过 `srv.Shutdown(ctx)` 为当前活跃的交易和下载预留 10 秒收尾履约时间；最后安全关闭 `sql.DB` 驱动，确保所有预写日志完全落盘。
* **持续集成与自动交叉编译 (GitHub Actions CI/CD)**：项目配置了完整的 `.github/workflows/ci.yml` 自动化流水线。每次提交或 Pull Request 均自动启动环境执行代码规范检查、单元测试及并发竞态校验 (`go test -race ./...`)；通过自动化构建矩阵，每次发版能够同时针对 Linux (amd64)、Windows (amd64 GUI 版)、macOS (Intel amd64 与 Apple Silicon arm64) 编译原生可执行文件并作为 Release 归档。

---

## 6. 开源协议与致谢

本软件及相关源代码架构遵循 Apache 2.0 License 自由开源协议。我们鼓励开发者、集成商和从业者在商业或非商业经营活动中对本系统进行定制开发、私有化部署以及二次分发。

在引用、借鉴本项目的底层软件架构、核心防呆转载算法或前端设计范式时，请在产品文档或致谢部分标注原作者致谢：**[Ju1ian SyntaxErr0r Zhang]**。感谢对本项目的系统功能完善、深度测试与商业逻辑架构设计推演。