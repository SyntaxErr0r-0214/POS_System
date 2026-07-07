//go:build !debug

package handler

import (
	"log"
	"modern-pos/internal/repository"
	"modern-pos/internal/service"
)

// RegisterDebugRoutes 在生产构建模式（无 debug 编译标签）下为空实现，
// 确保测试处理类与调试路由彻底剔出二进制文件，不在正式路由中上线。
func RegisterDebugRoutes(pRepo *repository.ProductRepo, oRepo *repository.OrderRepo, inventorySvc *service.InventoryService) {
	log.Println("安全构建：当前为生产构建版本（未开启 debug 编译标签），已彻底剔除 /api/debug/* 调试与数据填充路由")
}
