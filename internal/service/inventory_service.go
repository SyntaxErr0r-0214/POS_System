package service

import (
	"errors"
	"fmt"
	"modern-pos/internal/model"
	"modern-pos/internal/repository"
	"time"
)

type InventoryService struct {
	Repo      *repository.ProductRepo
	OrderRepo *repository.OrderRepo
}

func NewInventoryService(pRepo *repository.ProductRepo, oRepo *repository.OrderRepo) *InventoryService {
	return &InventoryService{Repo: pRepo, OrderRepo: oRepo}
}

// GetList 获取列表
func (s *InventoryService) GetList(query string) ([]model.Product, error) {
	if query == "" {
		return s.Repo.GetAll()
	}
	return s.Repo.SearchInventory(query)
}

// CheckDuplicateResult 查重结果结构体
type CheckDuplicateResult struct {
	IsDuplicate bool
	ExistingID  int
	Message     string
}

// AddProduct 新增商品并进行重名及条码校验
func (s *InventoryService) AddProduct(p model.Product) (CheckDuplicateResult, error) {
	if p.Barcode == "" {
		p.Barcode = fmt.Sprintf("A%s", time.Now().Format("20060102150405"))
	}

	if exist, _ := s.Repo.FindByBarcode(p.Barcode); exist != nil {
		return CheckDuplicateResult{IsDuplicate: true, ExistingID: exist.ID, Message: "条码已存在"}, nil
	}

	if p.Name == "" {
		return CheckDuplicateResult{}, errors.New("商品名称不能为空")
	}
	unitToCheck := p.Unit
	if unitToCheck == "" {
		unitToCheck = "个"
	}
	if exist, _ := s.Repo.FindByNameAndUnit(p.Name, unitToCheck); exist != nil {
		return CheckDuplicateResult{IsDuplicate: true, ExistingID: exist.ID, Message: fmt.Sprintf("商品名称 '%s' (单位: %s) 已存在", p.Name, unitToCheck)}, nil
	}

	err := s.Repo.Create(p)
	return CheckDuplicateResult{IsDuplicate: false}, err
}

// EditProduct 编辑商品信息
func (s *InventoryService) EditProduct(p model.Product) error {
	if p.ID == 0 {
		return errors.New("商品ID丢失")
	}
	if p.Barcode != "" {
		exist, _ := s.Repo.FindByBarcode(p.Barcode)
		if exist != nil && exist.ID != p.ID {
			return errors.New("修改后的条码与其他商品冲突")
		}
	}
	return s.Repo.Update(p)
}

// DeleteProduct 删除商品
func (s *InventoryService) DeleteProduct(id int) error {
	if id <= 0 {
		return errors.New("无效ID")
	}
	hasPending, err := s.OrderRepo.HasActiveOrders(id)
	if err != nil {
		return err
	}
	if hasPending {
		return errors.New("无法删除：该商品存在于【未完成的预订订单】中")
	}
	if err := s.OrderRepo.UnlinkProduct(id); err != nil {
		return errors.New("解除历史关联失败")
	}
	return s.Repo.Delete(id)
}

// BatchDelete 批量删除商品
func (s *InventoryService) BatchDelete(ids []int) error {
	if len(ids) == 0 {
		return errors.New("未选择任何商品")
	}

	for _, id := range ids {
		hasPending, err := s.OrderRepo.HasActiveOrders(id)
		if err != nil {
			return err
		}
		if hasPending {
			return fmt.Errorf("无法批量删除：商品(ID:%d)存在于未完成的预订订单中。请先取消勾选该商品。", id)
		}
	}

	for _, id := range ids {
		if err := s.OrderRepo.UnlinkProduct(id); err != nil {
			return fmt.Errorf("商品(ID:%d)解除历史关联失败", id)
		}
	}

	return s.Repo.BatchDelete(ids)
}

// BatchUpdateCategory 批量更新分类
func (s *InventoryService) BatchUpdateCategory(ids []int, category string) error {
	if len(ids) == 0 {
		return errors.New("未选择任何商品")
	}
	return s.Repo.BatchUpdateCategory(ids, category)
}
