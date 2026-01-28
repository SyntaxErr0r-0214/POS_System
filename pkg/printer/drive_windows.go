//go:build windows

package printer

import (
	"fmt"

	"github.com/alexbrainman/printer"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type WindowsUSBPrinter struct {
	PrinterName string
}

func utf8ToGbk(s string) ([]byte, error) {
	result, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), []byte(s))
	return result, err
}

func (p *WindowsUSBPrinter) PrintTicket(content string) error {
	prt, err := printer.Open(p.PrinterName)
	if err != nil {
		return err
	}
	defer prt.Close()

	if err := prt.StartDocument("POS_Receipt", "RAW"); err != nil {
		return err
	}
	if err := prt.StartPage(); err != nil {
		return err
	}

	var data []byte
	// ... (这里省略具体的 ESC/POS 指令，保持和你之前的一模一样即可) ...
	// 为了节省篇幅，请把你之前验证成功的 ESC/POS 代码逻辑完整复制到这里
	// 记得加上 utf8ToGbk 的调用

	// 简单的示例占位，请用你的真实代码替换：
	body, _ := utf8ToGbk(content)
	data = append(data, body...)

	prt.Write(data)
	prt.EndPage()
	prt.EndDocument()
	return nil
}

func GetPrinter() Printer {
	// 你的收银机打印机名字
	name := "POS58"
	fmt.Printf("启用 Windows 打印机: [%s]\n", name)
	return &WindowsUSBPrinter{PrinterName: name}
}
