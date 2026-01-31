//go:build windows

package printer

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// ★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★
// ⚠️ 务必修改！去 Win7 电脑的【设备和打印机】看你的打印机名字
// 必须一字不差，建议复制粘贴过来
const PrinterName = "POS-58"

// ★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★★

type WindowsPrinter struct{}

// GetPrinter [核心修复]：必须要有这个函数，main.go 才能调用
func GetPrinter() Printer {
	return &WindowsPrinter{}
}

func (p *WindowsPrinter) PrintTicket(content string) error {
	log.Printf("正在尝试打印到: %s", PrinterName)
	return rawPrint(PrinterName, content)
}

// --- 以下是 Windows API 调用的底层封装 ---

var (
	modwinspool  = syscall.NewLazyDLL("winspool.drv")
	openPrinter  = modwinspool.NewProc("OpenPrinterW")
	startDoc     = modwinspool.NewProc("StartDocPrinterW")
	startPage    = modwinspool.NewProc("StartPagePrinter")
	writePrinter = modwinspool.NewProc("WritePrinter")
	endPage      = modwinspool.NewProc("EndPagePrinter")
	endDoc       = modwinspool.NewProc("EndDocPrinter")
	closePrinter = modwinspool.NewProc("ClosePrinter")
)

type DOC_INFO_1 struct {
	pDocName    *uint16
	pOutputFile *uint16
	pDatatype   *uint16
}

// rawPrint 直接向打印机发送原始数据 (RAW Mode)
func rawPrint(printerName, data string) error {
	// 1. 编码转换：Go(UTF-8) -> 打印机(GBK)
	gbkData, err := utf8ToGbk(data)
	if err != nil {
		return err
	}

	// 2. 打开打印机句柄
	namePtr, _ := syscall.UTF16PtrFromString(printerName)
	var hPrinter syscall.Handle
	r1, _, err := openPrinter.Call(
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(unsafe.Pointer(&hPrinter)),
		0,
	)
	if r1 == 0 {
		return fmt.Errorf("打开打印机失败: %v (请检查【设备和打印机】里的名字是否完全一致)", err)
	}
	defer closePrinter.Call(uintptr(hPrinter))

	// 3. 开始文档
	docNamePtr, _ := syscall.UTF16PtrFromString("POS Receipt")
	dataTypePtr, _ := syscall.UTF16PtrFromString("RAW")
	di := DOC_INFO_1{
		pDocName:    docNamePtr,
		pOutputFile: nil,
		pDatatype:   dataTypePtr,
	}
	r1, _, err = startDoc.Call(uintptr(hPrinter), 1, uintptr(unsafe.Pointer(&di)))
	if r1 == 0 {
		return fmt.Errorf("StartDoc 失败: %v", err)
	}
	defer endDoc.Call(uintptr(hPrinter))

	// 4. 开始页
	startPage.Call(uintptr(hPrinter))
	defer endPage.Call(uintptr(hPrinter))

	// 5. 构造最终数据 (内容 + ESC/POS 切纸指令)
	finalData := append(gbkData, []byte{0x0A, 0x0A, 0x0A, 0x0A, 0x1D, 0x56, 0x42, 0x00}...)

	// 6. 写入数据
	var written uint32
	r1, _, err = writePrinter.Call(
		uintptr(hPrinter),
		uintptr(unsafe.Pointer(&finalData[0])),
		uintptr(len(finalData)),
		uintptr(unsafe.Pointer(&written)),
	)
	if r1 == 0 {
		return fmt.Errorf("写入打印机失败: %v", err)
	}

	return nil
}

// 辅助函数：UTF-8 转 GBK
func utf8ToGbk(s string) ([]byte, error) {
	reader := transform.NewReader(strings.NewReader(s), simplifiedchinese.GBK.NewEncoder())
	return ioutil.ReadAll(reader)
}
