package service

import (
	"errors"
	"pos-demo/internal/model"
	"pos-demo/internal/repository"
)

type InventoryService struct {
	Repo *repository.ProductRepo
}

func NewInventoryService(repo *repository.ProductRepo) *InventoryService {
	return &InventoryService{Repo: repo}
}

// GetList 获取列表
func (s *InventoryService) GetList() ([]model.Product, error) {
	return s.Repo.GetAll()
}

// AddProduct 入库
func (s *InventoryService) AddProduct(p model.Product) error {
	if p.Barcode == "" || p.Name == "" {
		return errors.New("条码和名称必填")
	}
	// 查重
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
