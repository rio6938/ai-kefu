package repository

import (
	"admin/server/model"
	"admin/server/pkg/db"
)

// KefuNoticeRepository 是客服模块发送站内消息的数据访问，
// 转接、分配、待接管提醒都通过它写 sys_message
type KefuNoticeRepository struct{}

// NewKefuNoticeRepository 创建站内消息的数据访问
func NewKefuNoticeRepository() *KefuNoticeRepository {
	return &KefuNoticeRepository{}
}

// Send 给管理员用户发一条通知，同一用户批量发送时逐条写入
func (r *KefuNoticeRepository) Send(userIDs []int64, title, content string) error {
	if len(userIDs) == 0 {
		return nil
	}

	now := model.Now()
	list := make([]model.SysMessage, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		list = append(list, model.SysMessage{
			UserID:     userID,
			Type:       model.MessageTypeNotice,
			Title:      title,
			Content:    content,
			Status:     model.MessageStatusUnread,
			CreateTime: now,
			UpdateTime: now,
		})
	}
	if len(list) == 0 {
		return nil
	}
	return db.DB.Create(&list).Error
}
