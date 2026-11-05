package repository

import (
	"errors"

	"gorm.io/gorm"

	"admin/server/model"
	"admin/server/pkg/db"
)

// 常见的业务错误
var (
	// ErrVisitorNotFound 是访客不存在
	ErrVisitorNotFound = errors.New("访客不存在")
)

// KefuVisitorRepository 是访客的数据访问
type KefuVisitorRepository struct{}

// NewKefuVisitorRepository 创建访客的数据访问
func NewKefuVisitorRepository() *KefuVisitorRepository {
	return &KefuVisitorRepository{}
}

// db 返回带 kefu_visitor 条件的查询
func (r *KefuVisitorRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuVisitor{}).Where("deleted = 0")
}

// GetByID 按 id 查询访客
func (r *KefuVisitorRepository) GetByID(visitorID int64) (*model.KefuVisitor, error) {
	visitor := new(model.KefuVisitor)
	err := r.db().Where("visitor_id = ?", visitorID).First(visitor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrVisitorNotFound
	}
	if err != nil {
		return nil, err
	}
	return visitor, nil
}

// GetByKey 按访客标识查询访客
func (r *KefuVisitorRepository) GetByKey(visitorKey string) (*model.KefuVisitor, error) {
	visitor := new(model.KefuVisitor)
	err := r.db().Where("visitor_key = ?", visitorKey).First(visitor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrVisitorNotFound
	}
	if err != nil {
		return nil, err
	}
	return visitor, nil
}

// ListByIDs 按 id 批量查询访客，会话列表回填访客信息时使用
func (r *KefuVisitorRepository) ListByIDs(visitorIDs []int64) ([]model.KefuVisitor, error) {
	if len(visitorIDs) == 0 {
		return nil, nil
	}
	var list []model.KefuVisitor
	if err := r.db().Where("visitor_id in ?", visitorIDs).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Create 新增访客
func (r *KefuVisitorRepository) Create(visitor *model.KefuVisitor) error {
	return db.DB.Create(visitor).Error
}

// Update 修改访客
func (r *KefuVisitorRepository) Update(visitor *model.KefuVisitor) error {
	return db.DB.Save(visitor).Error
}
