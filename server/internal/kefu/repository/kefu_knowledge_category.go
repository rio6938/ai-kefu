package repository

import (
	"errors"

	"gorm.io/gorm"

	"admin/server/internal/common"
	"admin/server/model"
	"admin/server/pkg/db"
)

// ErrKnowledgeCategoryNotFound 是文档库分类不存在
var ErrKnowledgeCategoryNotFound = errors.New("文档库分类不存在")

// KefuKnowledgeCategoryRepository 是文档库分类的数据访问
type KefuKnowledgeCategoryRepository struct{}

// NewKefuKnowledgeCategoryRepository 创建文档库分类的数据访问
func NewKefuKnowledgeCategoryRepository() *KefuKnowledgeCategoryRepository {
	return &KefuKnowledgeCategoryRepository{}
}

// db 返回带 kefu_knowledge_category 条件的查询
func (r *KefuKnowledgeCategoryRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuKnowledgeCategory{}).Where("deleted = 0")
}

// GetByID 按 id 查询文档库分类
func (r *KefuKnowledgeCategoryRepository) GetByID(categoryID int64) (*model.KefuKnowledgeCategory, error) {
	record := new(model.KefuKnowledgeCategory)
	err := r.db().Where("category_id = ?", categoryID).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrKnowledgeCategoryNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// Page 分页查询文档库分类
func (r *KefuKnowledgeCategoryRepository) Page(
	param model.KefuKnowledgeCategoryParam, page common.PageParam,
) ([]model.KefuKnowledgeCategory, int64, error) {
	query := r.db()
	if param.Name != "" {
		query = query.Where("name like ?", "%"+param.Name+"%")
	}
	if param.Status != nil {
		query = query.Where("status = ?", *param.Status)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.KefuKnowledgeCategory
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "sort_number asc, category_id asc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// List 查询启用中的文档库分类，下拉选择时使用
func (r *KefuKnowledgeCategoryRepository) List() ([]model.KefuKnowledgeCategory, error) {
	var list []model.KefuKnowledgeCategory
	if err := r.db().Where("status = ?", model.KefuKnowledgeCategoryStatusNormal).
		Order("sort_number asc, category_id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListByIDs 按 id 批量查询文档库分类，回填分类名称时使用
func (r *KefuKnowledgeCategoryRepository) ListByIDs(
	categoryIDs []int64,
) ([]model.KefuKnowledgeCategory, error) {
	if len(categoryIDs) == 0 {
		return nil, nil
	}
	var list []model.KefuKnowledgeCategory
	if err := r.db().Where("category_id in ?", categoryIDs).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ExistsByName 判断分类名称是否已存在，excludeID 为修改时的自身 id
func (r *KefuKnowledgeCategoryRepository) ExistsByName(
	name string, excludeID int64,
) (bool, error) {
	query := r.db().Where("name = ?", name)
	if excludeID > 0 {
		query = query.Where("category_id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create 新增文档库分类
func (r *KefuKnowledgeCategoryRepository) Create(record *model.KefuKnowledgeCategory) error {
	return db.DB.Create(record).Error
}

// Update 修改文档库分类
func (r *KefuKnowledgeCategoryRepository) Update(record *model.KefuKnowledgeCategory) error {
	return db.DB.Save(record).Error
}

// UpdateStatus 修改文档库分类状态
func (r *KefuKnowledgeCategoryRepository) UpdateStatus(categoryID int64, status int) error {
	return db.DB.Model(&model.KefuKnowledgeCategory{}).
		Where("category_id = ?", categoryID).
		Updates(map[string]any{"status": status, "update_time": model.Now()}).Error
}

// Remove 软删除文档库分类
func (r *KefuKnowledgeCategoryRepository) Remove(categoryID int64) error {
	return db.DB.Model(&model.KefuKnowledgeCategory{}).
		Where("category_id = ?", categoryID).
		Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error
}

// CountKnowledge 统计分类下引用中的文档数，删除分类前判断是否被引用
func (r *KefuKnowledgeCategoryRepository) CountKnowledge(categoryID int64) (int64, error) {
	var count int64
	err := db.DB.Model(&model.KefuKnowledge{}).
		Where("category_id = ? and deleted = 0", categoryID).Count(&count).Error
	return count, err
}
