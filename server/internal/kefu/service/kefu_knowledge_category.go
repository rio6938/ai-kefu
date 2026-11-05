package service

import (
	"errors"
	"strings"

	"admin/server/internal/kefu/repository"
	"admin/server/model"
)

// 常见的业务错误
var (
	// ErrCategoryNotFound 是文档库分类不存在
	ErrCategoryNotFound = repository.ErrKnowledgeCategoryNotFound
	// ErrCategoryNameRequired 是分类名称不能为空
	ErrCategoryNameRequired = errors.New("分类名称不能为空")
	// ErrCategoryNameExists 是分类名称已存在
	ErrCategoryNameExists = errors.New("分类名称已经存在")
	// ErrCategoryStatusInvalid 是分类状态不正确
	ErrCategoryStatusInvalid = errors.New("分类状态不正确")
	// ErrCategoryInUse 是分类下还有文档，不能删除
	ErrCategoryInUse = errors.New("该分类下还有文档，不能删除")
)

// KefuKnowledgeCategoryService 是文档库分类的业务处理
type KefuKnowledgeCategoryService struct {
	// 文档库分类的数据访问
	categoryRepository *repository.KefuKnowledgeCategoryRepository
}

// NewKefuKnowledgeCategoryService 创建文档库分类的业务处理
func NewKefuKnowledgeCategoryService() *KefuKnowledgeCategoryService {
	return &KefuKnowledgeCategoryService{
		categoryRepository: repository.NewKefuKnowledgeCategoryRepository(),
	}
}

// Page 分页查询文档库分类
func (s *KefuKnowledgeCategoryService) Page(
	param model.KefuKnowledgeCategoryParam,
) ([]model.KefuKnowledgeCategory, int64, error) {
	param.Name = strings.TrimSpace(param.Name)
	return s.categoryRepository.Page(param, param.PageParam)
}

// List 查询启用中的文档库分类，下拉选择时使用
func (s *KefuKnowledgeCategoryService) List() ([]model.KefuKnowledgeCategory, error) {
	return s.categoryRepository.List()
}

// GetEnabled 查询启用中的文档库分类，知识库上传与修改前校验分类可用
func (s *KefuKnowledgeCategoryService) GetEnabled(
	categoryID int64,
) (*model.KefuKnowledgeCategory, error) {
	record, err := s.categoryRepository.GetByID(categoryID)
	if err != nil {
		return nil, err
	}
	if record.Status != model.KefuKnowledgeCategoryStatusNormal {
		return nil, ErrCategoryNotFound
	}
	return record, nil
}

// Add 新增文档库分类
func (s *KefuKnowledgeCategoryService) Add(form model.KefuKnowledgeCategoryForm) error {
	if err := normalizeCategoryForm(&form); err != nil {
		return err
	}

	exists, err := s.categoryRepository.ExistsByName(form.Name, 0)
	if err != nil {
		return err
	}
	if exists {
		return ErrCategoryNameExists
	}

	now := model.Now()
	return s.categoryRepository.Create(&model.KefuKnowledgeCategory{
		Name:       form.Name,
		SortNumber: form.SortNumber,
		Status:     form.Status,
		CreateTime: now,
		UpdateTime: now,
	})
}

// Update 修改文档库分类
func (s *KefuKnowledgeCategoryService) Update(form model.KefuKnowledgeCategoryForm) error {
	record, err := s.categoryRepository.GetByID(form.CategoryID)
	if err != nil {
		return err
	}
	if err := normalizeCategoryForm(&form); err != nil {
		return err
	}

	exists, err := s.categoryRepository.ExistsByName(form.Name, form.CategoryID)
	if err != nil {
		return err
	}
	if exists {
		return ErrCategoryNameExists
	}

	record.Name = form.Name
	record.SortNumber = form.SortNumber
	record.Status = form.Status
	record.UpdateTime = model.Now()
	return s.categoryRepository.Update(record)
}

// UpdateStatus 修改文档库分类状态
func (s *KefuKnowledgeCategoryService) UpdateStatus(categoryID int64, status int) error {
	if _, err := s.categoryRepository.GetByID(categoryID); err != nil {
		return err
	}
	if status != model.KefuKnowledgeCategoryStatusNormal &&
		status != model.KefuKnowledgeCategoryStatusDisabled {
		return ErrCategoryStatusInvalid
	}
	return s.categoryRepository.UpdateStatus(categoryID, status)
}

// Remove 删除文档库分类，分类下还有文档时不允许删除
func (s *KefuKnowledgeCategoryService) Remove(categoryID int64) error {
	if _, err := s.categoryRepository.GetByID(categoryID); err != nil {
		return err
	}
	count, err := s.categoryRepository.CountKnowledge(categoryID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrCategoryInUse
	}
	return s.categoryRepository.Remove(categoryID)
}

// normalizeCategoryForm 校验并归一化文档库分类的提交数据
func normalizeCategoryForm(form *model.KefuKnowledgeCategoryForm) error {
	form.Name = strings.TrimSpace(form.Name)
	if form.Name == "" {
		return ErrCategoryNameRequired
	}
	if form.Status != model.KefuKnowledgeCategoryStatusNormal &&
		form.Status != model.KefuKnowledgeCategoryStatusDisabled {
		return ErrCategoryStatusInvalid
	}
	return nil
}
