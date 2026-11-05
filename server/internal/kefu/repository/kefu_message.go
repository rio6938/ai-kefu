package repository

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"admin/server/model"
	"admin/server/pkg/db"
)

// 常见的业务错误
var (
	// ErrMessageNotFound 是消息不存在
	ErrMessageNotFound = errors.New("消息不存在")
)

// KefuMessageRepository 是会话消息的数据访问
type KefuMessageRepository struct{}

// NewKefuMessageRepository 创建会话消息的数据访问
func NewKefuMessageRepository() *KefuMessageRepository {
	return &KefuMessageRepository{}
}

// db 返回带 kefu_message 条件的查询
func (r *KefuMessageRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuMessage{}).Where("deleted = 0")
}

// GetByID 按 id 查询消息
func (r *KefuMessageRepository) GetByID(messageID int64) (*model.KefuMessage, error) {
	message := new(model.KefuMessage)
	err := r.db().Where("message_id = ?", messageID).First(message).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMessageNotFound
	}
	if err != nil {
		return nil, err
	}
	return message, nil
}

// LastBySessions 批量查询每个会话的最后一条消息，会话列表的摘要用它
func (r *KefuMessageRepository) LastBySessions(
	sessionIDs []int64,
) (map[int64]model.KefuMessage, error) {
	result := make(map[int64]model.KefuMessage, len(sessionIDs))
	if len(sessionIDs) == 0 {
		return result, nil
	}

	type sessionMax struct {
		// 会话id
		SessionID int64
		// 最后一条消息id
		MaxMessageID int64
	}
	var rows []sessionMax
	if err := db.DB.Model(&model.KefuMessage{}).
		Select("session_id, max(message_id) as max_message_id").
		Where("deleted = 0 and session_id in ?", sessionIDs).
		Group("session_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return result, nil
	}

	messageIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		messageIDs = append(messageIDs, row.MaxMessageID)
	}
	var list []model.KefuMessage
	if err := r.db().Where("message_id in ?", messageIDs).Find(&list).Error; err != nil {
		return nil, err
	}
	for _, message := range list {
		result[message.SessionID] = message
	}
	return result, nil
}

// SearchBySession 在会话内按关键词搜索消息正文，最多返回 50 条，按消息 id 倒序
func (r *KefuMessageRepository) SearchBySession(
	sessionID int64, keyword string, limit int,
) ([]model.KefuMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	var list []model.KefuMessage
	if err := r.db().
		Where("session_id = ? and content ilike ?", sessionID, "%"+keyword+"%").
		Where("content_type in ?", []string{
			model.KefuMessageTypeText, model.KefuMessageTypeAiAnswer,
		}).
		Order("message_id desc").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// GetByClientMsgNo 按 IM 幂等消息编号查询消息
func (r *KefuMessageRepository) GetByClientMsgNo(clientMsgNo string) (*model.KefuMessage, error) {
	message := new(model.KefuMessage)
	err := r.db().Where("client_msg_no = ?", clientMsgNo).First(message).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMessageNotFound
	}
	if err != nil {
		return nil, err
	}
	return message, nil
}

// CreateIfAbsent 按client_msg_no幂等写入消息，已存在时不插入并返回 false
func (r *KefuMessageRepository) CreateIfAbsent(message *model.KefuMessage) (bool, error) {
	result := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "client_msg_no"}},
		DoNothing: true,
	}).Create(message)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// Create 新增消息
func (r *KefuMessageRepository) Create(message *model.KefuMessage) error {
	return db.DB.Create(message).Error
}

// ListBySession 分页查询会话的消息，startMessageID 大于 0 时拉取更早的消息
func (r *KefuMessageRepository) ListBySession(
	sessionID int64, param model.KefuMessageParam,
) ([]model.KefuMessage, int64, error) {
	query := r.db().Where("session_id = ?", sessionID)
	if param.StartMessageID != nil && *param.StartMessageID > 0 {
		query = query.Where("message_id < ?", *param.StartMessageID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.KefuMessage
	// 消息按写入顺序正序返回，前端重连后据此恢复气泡顺序
	err := query.Order("message_id desc").Limit(param.PageSize()).
		Offset(param.Offset()).Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, count, nil
}

// ListRecent 查询会话最近的 limit 条消息，按写入顺序正序返回；
// AI 编排用它组装最近几轮对话上下文
func (r *KefuMessageRepository) ListRecent(sessionID int64, limit int) ([]model.KefuMessage, error) {
	if limit <= 0 {
		return nil, nil
	}
	var list []model.KefuMessage
	err := r.db().Where("session_id = ?", sessionID).
		Order("message_id desc").Limit(limit).Find(&list).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
}

// UpdateIMInfo 回填消息在 IM 侧的消息id与序号
func (r *KefuMessageRepository) UpdateIMInfo(messageID int64, imMessageID string, imMessageSeq int64) error {
	return db.DB.Model(&model.KefuMessage{}).Where("message_id = ?", messageID).
		Updates(map[string]any{
			"im_message_id":  imMessageID,
			"im_message_seq": imMessageSeq,
			"update_time":    model.Now(),
		}).Error
}

// UpdateContent 更新消息内容与状态，AI 回复结束时回填完整文本
func (r *KefuMessageRepository) UpdateContent(messageID int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["update_time"] = model.Now()
	return db.DB.Model(&model.KefuMessage{}).Where("message_id = ?", messageID).
		Updates(fields).Error
}

// UpdateAiReplyID 回填消息关联的 AI 回复id
func (r *KefuMessageRepository) UpdateAiReplyID(messageID int64, aiReplyID int64) error {
	return r.UpdateContent(messageID, map[string]any{"ai_reply_id": aiReplyID})
}

// Remove 软删除消息，IM 发送失败时撤销已落库的消息
func (r *KefuMessageRepository) Remove(messageID int64) error {
	return db.DB.Model(&model.KefuMessage{}).Where("message_id = ?", messageID).
		Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error
}

// GetByIDAny 按 id 查询消息，包含已软删除的记录，重发失败邮件时取回原文
func (r *KefuMessageRepository) GetByIDAny(messageID int64) (*model.KefuMessage, error) {
	message := new(model.KefuMessage)
	err := db.DB.Model(&model.KefuMessage{}).Where("message_id = ?", messageID).First(message).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMessageNotFound
	}
	if err != nil {
		return nil, err
	}
	return message, nil
}

// Restore 恢复被软删除的消息，重发成功后让消息重新出现在会话里
func (r *KefuMessageRepository) Restore(messageID int64) error {
	return db.DB.Model(&model.KefuMessage{}).Where("message_id = ?", messageID).
		Updates(map[string]any{"deleted": 0, "update_time": model.Now()}).Error
}
