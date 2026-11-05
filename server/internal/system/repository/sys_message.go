package repository

import (
	"gorm.io/gorm"

	"admin/server/internal/common"
	"admin/server/model"
	"admin/server/pkg/db"
)

// SysMessageRepository 是我的消息的数据访问
type SysMessageRepository struct{}

// NewSysMessageRepository 创建我的消息的数据访问
func NewSysMessageRepository() *SysMessageRepository {
	return &SysMessageRepository{}
}

// db 返回带 sys_message 条件的查询，只查指定接收人自己的消息
func (r *SysMessageRepository) db(userID int64) *gorm.DB {
	return db.DB.Model(&model.SysMessage{}).
		Where("deleted = 0 and user_id = ?", userID)
}

// Page 分页查询我的消息
func (r *SysMessageRepository) Page(userID int64, param model.MessageParam, page common.PageParam) ([]model.SysMessage, int64, error) {
	query := r.db(userID)
	if param.Type != nil {
		query = query.Where("type = ?", *param.Type)
	}
	if param.Title != "" {
		query = query.Where("title like ?", "%"+param.Title+"%")
	}
	if param.Status != nil {
		query = query.Where("status = ?", *param.Status)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.SysMessage
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "id desc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// CountUnread 统计指定接收人的通知未读数量与待办未完成数量
func (r *SysMessageRepository) CountUnread(userID int64) (int64, int64, error) {
	var noticeCount int64
	err := r.db(userID).
		Where("type = ? and status = ?", model.MessageTypeNotice, model.MessageStatusUnread).
		Count(&noticeCount).Error
	if err != nil {
		return 0, 0, err
	}

	var todoCount int64
	err = r.db(userID).
		Where("type = ? and status = ?", model.MessageTypeTodo, model.MessageStatusUnread).
		Count(&todoCount).Error
	if err != nil {
		return 0, 0, err
	}
	return noticeCount, todoCount, nil
}

// UpdateStatus 按 id 批量修改我的消息状态
func (r *SysMessageRepository) UpdateStatus(userID int64, ids []int64, status int) error {
	return db.DB.Model(&model.SysMessage{}).
		Where("deleted = 0 and user_id = ? and id in ?", userID, ids).
		Updates(map[string]any{"status": status, "update_time": model.Now()}).Error
}

// Remove 按 id 批量软删除我的消息
func (r *SysMessageRepository) Remove(userID int64, ids []int64) error {
	return db.DB.Model(&model.SysMessage{}).
		Where("deleted = 0 and user_id = ? and id in ?", userID, ids).
		Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error
}
