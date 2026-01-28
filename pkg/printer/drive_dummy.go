//go:build !windows

package printer

import "fmt"

type ConsolePrinter struct{}

func (p *ConsolePrinter) PrintTicket(content string) error {
	fmt.Println(">> [Mac模拟打印] 内容如下:")
	fmt.Println(content)
	return nil
}

// GetPrinter 返回当前系统的打印机实现
func GetPrinter() Printer {
	fmt.Println("启用 Mac 模拟打印机")
	return &ConsolePrinter{}
}
