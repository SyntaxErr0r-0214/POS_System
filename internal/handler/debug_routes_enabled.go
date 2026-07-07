//go:build debug

package handler

import (
	"log"
	"net/http"
	"os"
	"modern-pos/internal/repository"
	"modern-pos/internal/service"
)

// RegisterDebugRoutes 在调试编译模式（-tags debug）下启动测试数据填充与调试接口。
// 即使在调试编译模式下，也会执行环境变量二次校验，并在挂载路由时强制应用管理员权限校验中间件。
func RegisterDebugRoutes(pRepo *repository.ProductRepo, oRepo *repository.OrderRepo, inventorySvc *service.InventoryService) {
	// 环境变量二次安全防线：如果在运行时明确设置了生产环境标志，则强制拦截调试路由的注册
	if os.Getenv("ENV") == "production" || os.Getenv("PROD") == "true" {
		log.Println("安全防护：检测到生产环境变量配置 (ENV=production 或 PROD=true)，已拦截调试与测试数据填充路由的挂载")
		return
	}

	testHandler := &TestHandler{
		PRepo:     pRepo,
		ORepo:     oRepo,
		Inventory: inventorySvc,
	}

	// 为调试数据填充接口绑定管理员权限校验中间件
	http.HandleFunc("/api/debug/seed", RequireAdmin(testHandler.SeedData))
	log.Println("调试模式启用：已挂载测试数据填充路由 /api/debug/seed（已开启管理员权限校验）")
}
