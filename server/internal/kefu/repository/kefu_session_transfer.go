package repository

import (
	"gorm.io/gorm"

	"admin/server/model"
	"admin/server/pkg/db"
)

// KefuSessionTransferRepository 是会话转接记录的数据访问
type KefuSessionTransferRepository struct{}

// NewKefuSessionTransferRepository 创建会话转接记录的数据访问
func NewKefuSessionTransferRepository() *KefuSessionTransferRepository {
	return &KefuSessionTransferRepository{}
}

// db 返回带 kefu_session_transfer 条件的查询
func (r *KefuSessionTransferRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuSessionTransfer{}).Where("deleted = 0")
}

// CountSince 统计会话在指定时间点之后的转接次数，用于 24 小时转接上限
func (r *KefuSessionTransferRepository) CountSince(
	sessionID int64, since model.Time,
) (int64, error) {
	var count int64
	err := r.db().
		Where("session_id = ? and transfer_type <> ? and create_time >= ?",
			sessionID, model.KefuTransferTypeAssign, since.Time).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// LastTransferTimes 返回客服最近一次分配到会话的时间，
// 分配排序用「最近分配时间最早」做次要条件，没有记录时不在返回结果里
func (r *KefuSessionTransferRepository) LastTransferTimes(
	agentIDs []int64,
) (map[int64]model.Time, error) {
	result := make(map[int64]model.Time)
	if len(agentIDs) == 0 {
		return result, nil
	}

	type lastTransfer struct {
		// 客服id
		ToAgentID int64
		// 最近一次转接时间
		LastTime model.Time
	}
	var list []lastTransfer
	err := r.db().
		Select("to_agent_id, max(create_time) as last_time").
		Where("to_agent_id in ?", agentIDs).
		Group("to_agent_id").Scan(&list).Error
	if err != nil {
		return nil, err
	}
	for _, item := range list {
		result[item.ToAgentID] = item.LastTime
	}
	return result, nil
}

// Create 新增转接记录
func (r *KefuSessionTransferRepository) Create(record *model.KefuSessionTransfer) error {
	return db.DB.Create(record).Error
}

// ListBySession 查询会话的全部转接记录，会话详情按时间顺序回放
func (r *KefuSessionTransferRepository) ListBySession(
	sessionID int64,
) ([]model.KefuSessionTransfer, error) {
	var list []model.KefuSessionTransfer
	if err := r.db().Where("session_id = ?", sessionID).
		Order("transfer_id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// TransferSession 在一个事务里改写会话的客服归属并写入转接记录，
// 避免出现"会话已换人但没留痕"或反过来的中间状态
func (r *KefuSessionTransferRepository) TransferSession(
	sessionID int64, fields map[string]any, record *model.KefuSessionTransfer,
) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		fields["update_time"] = model.Now()
		if err := tx.Model(&model.KefuSession{}).Where("session_id = ?", sessionID).
			Updates(fields).Error; err != nil {
			return err
		}
		return tx.Create(record).Error
	})
}
