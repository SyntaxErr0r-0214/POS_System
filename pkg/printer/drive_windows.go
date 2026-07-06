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

// PrinterName 打印机名称，需与系统中配置的打印机完全一致
const PrinterName = "POS-58"

type WindowsPrinter struct{}

// GetPrinter 返回 Windows 下的打印机实现
func GetPrinter() Printer {
	return &WindowsPrinter{}
}

func (p *WindowsPrinter) PrintTicket(content string) error {
	log.Printf("正在尝试打印到: %s", PrinterName)
	return rawPrint(PrinterName, content)
}

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
	gbkData, err := utf8ToGbk(data)
	if err != nil {
		return err
	}

	namePtr, _ := syscall.UTF16PtrFromString(printerName)
	var hPrinter syscall.Handle
	r1, _, err := openPrinter.Call(
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(unsafe.Pointer(&hPrinter)),
		0,
	)
	if r1 == 0 {
		return fmt.Errorf("打开打印机失败: %v", err)
	}
	defer closePrinter.Call(uintptr(hPrinter))

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

	startPage.Call(uintptr(hPrinter))
	defer endPage.Call(uintptr(hPrinter))

	finalData := append(gbkData, []byte{0x0A, 0x0A, 0x0A, 0x0A, 0x1D, 0x56, 0x42, 0x00}...)

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

// utf8ToGbk 将 UTF-8 编码转化为 GBK 编码
func utf8ToGbk(s string) ([]byte, error) {
	reader := transform.NewReader(strings.NewReader(s), simplifiedchinese.GBK.NewEncoder())
	return ioutil.ReadAll(reader)
}
