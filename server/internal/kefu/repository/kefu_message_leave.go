package repository

import (
	"errors"

	"gorm.io/gorm"

	"admin/server/internal/common"
	"admin/server/model"
	"admin/server/pkg/db"
)

// 常见的业务错误
var (
	// ErrMessageLeaveNotFound 是留言不存在
	ErrMessageLeaveNotFound = errors.New("留言不存在")
)

// KefuMessageLeaveRepository 是留言的数据访问
type KefuMessageLeaveRepository struct{}

// NewKefuMessageLeaveRepository 创建留言的数据访问
func NewKefuMessageLeaveRepository() *KefuMessageLeaveRepository {
	return &KefuMessageLeaveRepository{}
}

// db 返回带 kefu_message_leave 条件的查询
func (r *KefuMessageLeaveRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuMessageLeave{}).Where("deleted = 0")
}

// Create 新增留言
func (r *KefuMessageLeaveRepository) Create(leave *model.KefuMessageLeave) error {
	return db.DB.Create(leave).Error
}

// GetByID 按 id 查询留言
func (r *KefuMessageLeaveRepository) GetByID(leaveID int64) (*model.KefuMessageLeave, error) {
	leave := new(model.KefuMessageLeave)
	err := r.db().Where("leave_id = ?", leaveID).First(leave).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMessageLeaveNotFound
	}
	if err != nil {
		return nil, err
	}
	return leave, nil
}

// Page 分页查询留言
func (r *KefuMessageLeaveRepository) Page(
	param model.KefuMessageLeaveParam, page common.PageParam,
) ([]model.KefuMessageLeave, int64, error) {
	query := r.db()
	if param.Status != nil {
		query = query.Where("status = ?", *param.Status)
	}
	if param.ChannelCode != "" {
		query = query.Where("channel_code = ?", param.ChannelCode)
	}
	if param.ContactType != "" {
		query = query.Where("contact_type = ?", param.ContactType)
	}
	if param.Keyword != "" {
		keyword := "%" + param.Keyword + "%"
		query = query.Where("name like ? or contact like ?", keyword, keyword)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.KefuMessageLeave
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "leave_id desc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// MarkHandled 标记留言为已处理，已处理的不再重复处理，返回是否写入成功
func (r *KefuMessageLeaveRepository) MarkHandled(
	leaveID, userID int64, remark string, now model.Time,
) (bool, error) {
	result := db.DB.Model(&model.KefuMessageLeave{}).
		Where("leave_id = ? and deleted = 0 and status = ?",
			leaveID, model.KefuMessageLeaveStatusPending).
		Updates(map[string]any{
			"status":         model.KefuMessageLeaveStatusHandled,
			"handle_user_id": userID,
			"handle_time":    now,
			"remark":         remark,
			"update_time":    now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
