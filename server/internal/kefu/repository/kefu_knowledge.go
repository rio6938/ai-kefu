package repository

import (
	"errors"

	"gorm.io/gorm"

	"admin/server/internal/common"
	"admin/server/model"
	"admin/server/pkg/db"
)

// ErrKnowledgeNotFound 是知识库文档不存在
var ErrKnowledgeNotFound = errors.New("知识库文档不存在")

// KefuKnowledgeRepository 是知识库文档的数据访问
type KefuKnowledgeRepository struct{}

// NewKefuKnowledgeRepository 创建知识库文档的数据访问
func NewKefuKnowledgeRepository() *KefuKnowledgeRepository {
	return &KefuKnowledgeRepository{}
}

// db 返回带 kefu_knowledge 条件的查询
func (r *KefuKnowledgeRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuKnowledge{}).Where("deleted = 0")
}

// GetByID 按 id 查询知识库文档
func (r *KefuKnowledgeRepository) GetByID(docID int64) (*model.KefuKnowledge, error) {
	record := new(model.KefuKnowledge)
	err := r.db().Where("doc_id = ?", docID).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrKnowledgeNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// Page 分页查询知识库文档
func (r *KefuKnowledgeRepository) Page(
	param model.KefuKnowledgeParam, page common.PageParam,
) ([]model.KefuKnowledge, int64, error) {
	query := r.db()
	if param.CategoryID != nil {
		query = query.Where("category_id = ?", *param.CategoryID)
	}
	if param.Title != "" {
		query = query.Where("title like ?", "%"+param.Title+"%")
	}
	if param.Status != nil {
		query = query.Where("status = ?", *param.Status)
	}
	if param.Source != "" {
		query = query.Where("source = ?", param.Source)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.KefuKnowledge
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "doc_id desc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// Create 新增知识库文档
func (r *KefuKnowledgeRepository) Create(record *model.KefuKnowledge) error {
	return db.DB.Create(record).Error
}

// UpdateFields 按字段更新知识库文档，只更新传入的字段
func (r *KefuKnowledgeRepository) UpdateFields(docID int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["update_time"] = model.Now()
	return db.DB.Model(&model.KefuKnowledge{}).Where("doc_id = ?", docID).
		Updates(fields).Error
}

// Remove 软删除知识库文档
func (r *KefuKnowledgeRepository) Remove(docID int64) error {
	return db.DB.Model(&model.KefuKnowledge{}).Where("doc_id = ?", docID).
		Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error
}
