package service

import (
	"errors"
	"pos-demo/internal/model"
	"pos-demo/internal/repository"
)

type InventoryService struct {
	Repo      *repository.ProductRepo
	OrderRepo *repository.OrderRepo // <--- 新增依赖
}

// 注意：这里参数变了，多传一个 orderRepo
func NewInventoryService(pRepo *repository.ProductRepo, oRepo *repository.OrderRepo) *InventoryService {
	return &InventoryService{
		Repo:      pRepo,
		OrderRepo: oRepo,
	}
}

// GetList 获取列表
func (s *InventoryService) GetList(query string) ([]model.Product, error) {
	if query == "" {
		return s.Repo.GetAll()
	}
	return s.Repo.SearchInventory(query)
}

// AddProduct 入库
func (s *InventoryService) AddProduct(p model.Product) error {
	if p.Barcode == "" || p.Name == "" {
		return errors.New("条码和名称必填")
	}
	exist, _ := s.Repo.FindByBarcode(p.Barcode)
	if exist != nil {
		return errors.New("该条码已存在")
	}
	return s.Repo.Create(p)
}

// EditProduct 编辑
func (s *InventoryService) EditProduct(p model.Product) error {
	if p.ID == 0 {
		return errors.New("商品ID丢失")
	}
	return s.Repo.Update(p)
}

// DeleteProduct 删除商品 (核心逻辑更新)
func (s *InventoryService) DeleteProduct(id int) error {
	if id <= 0 {
		return errors.New("无效ID")
	}

	// 1. 检查是否有“进行中(Pending)”的预订订单
	hasPending, err := s.OrderRepo.HasActiveOrders(id)
	if err != nil {
		return err
	}
	if hasPending {
		// 如果有预订，直接拒绝删除，并告知前端
		return errors.New("无法删除：该商品存在于【未完成的预订订单】中，请先处理订单！")
	}

	// 2. 如果只有“已完成(Completed)”的历史订单，就切断关联
	// 把 order_items 表里这个商品的 ID 设为 NULL，保留名称和价格记录
	if err := s.OrderRepo.UnlinkProduct(id); err != nil {
		return errors.New("解除历史关联失败")
	}

	// 3. 彻底删除商品档案
	return s.Repo.Delete(id)
}
