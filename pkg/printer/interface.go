package printer

// 接口定义
type Printer interface {
	PrintTicket(content string) error
}

// 全局变量，存放当前打印机实例
var Current Printer

// 设置打印机实例
func SetPrinter(p Printer) {
	Current = p
}
