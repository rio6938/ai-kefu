package repository

import (
	"gorm.io/gorm"

	"admin/server/model"
	"admin/server/pkg/db"
)

// KefuSessionTagRepository 是会话标签的数据访问
type KefuSessionTagRepository struct{}

// NewKefuSessionTagRepository 创建会话标签的数据访问
func NewKefuSessionTagRepository() *KefuSessionTagRepository {
	return &KefuSessionTagRepository{}
}

// ListBySessions 批量查询会话的标签，返回 session_id -> 标签编码列表
func (r *KefuSessionTagRepository) ListBySessions(
	sessionIDs []int64,
) (map[int64][]string, error) {
	result := make(map[int64][]string, len(sessionIDs))
	if len(sessionIDs) == 0 {
		return result, nil
	}

	var list []model.KefuSessionTag
	if err := db.DB.Model(&model.KefuSessionTag{}).
		Where("session_id in ?", sessionIDs).
		Order("tag_id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	for _, item := range list {
		result[item.SessionID] = append(result[item.SessionID], item.TagCode)
	}
	return result, nil
}

// Replace 覆盖式设置会话标签：在一个事务里先清空再写入，重复设置不产生重复行
func (r *KefuSessionTagRepository) Replace(
	sessionID int64, tags []string, userID int64,
) error {
	now := model.Now()
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).
			Delete(&model.KefuSessionTag{}).Error; err != nil {
			return err
		}
		if len(tags) == 0 {
			return nil
		}
		list := make([]model.KefuSessionTag, 0, len(tags))
		for _, code := range tags {
			list = append(list, model.KefuSessionTag{
				SessionID:    sessionID,
				TagCode:      code,
				CreateUserID: userID,
				CreateTime:   now,
			})
		}
		return tx.Create(&list).Error
	})
}
