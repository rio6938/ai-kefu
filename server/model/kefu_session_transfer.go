package model

import "admin/server/internal/common"

// 会话转接类型
const (
	// KefuTransferTypeAssign 是分配, 会话第一次绑定客服
	KefuTransferTypeAssign = "assign"
	// KefuTransferTypeManual 是手动转接
	KefuTransferTypeManual = "transfer"
	// KefuTransferTypeAuto 是命中规则后的自动转接
	KefuTransferTypeAuto = "auto"
	// KefuTransferTypeDisable 是客服停用后的批量转出
	KefuTransferTypeDisable = "disable"
)

// KefuSessionTransfer 是会话转接记录，对应 kefu_session_transfer 表
type KefuSessionTransfer struct {
	// 转接记录id
	TransferID int64 `gorm:"column:transfer_id;primaryKey;autoIncrement;comment:转接记录id" json:"transferId"`
	// 会话id
	SessionID int64 `gorm:"column:session_id;not null;index;comment:会话id" json:"sessionId"`
	// 转接类型, assign 分配, transfer 手动转接, auto 规则转接, disable 停用转出
	TransferType string `gorm:"column:transfer_type;size:32;not null;comment:转接类型" json:"transferType"`
	// 转出客服id, 0 表示分配前没有客服
	FromAgentID int64 `gorm:"column:from_agent_id;not null;default:0;comment:转出客服id" json:"fromAgentId"`
	// 转入客服id, 0 表示转回排队
	ToAgentID int64 `gorm:"column:to_agent_id;not null;default:0;comment:转入客服id" json:"toAgentId"`
	// 转接原因
	Reason string `gorm:"column:reason;size:255;comment:转接原因" json:"reason"`
	// 操作人id, 0 表示系统自动处理
	OperatorID int64 `gorm:"column:operator_id;not null;default:0;comment:操作人id" json:"operatorId"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 转出客服名称, 不由数据库存储
	FromAgentName string `gorm:"-" json:"fromAgentName,omitempty"`
	// 转入客服名称, 不由数据库存储
	ToAgentName string `gorm:"-" json:"toAgentName,omitempty"`
}

// TableName 指定表名
func (KefuSessionTransfer) TableName() string {
	return "kefu_session_transfer"
}

// KefuSessionTransferParam 是转接记录的查询参数
type KefuSessionTransferParam struct {
	common.PageParam
	// 会话id
	SessionID *int64 `form:"sessionId"`
	// 转接类型
	TransferType string `form:"transferType"`
	// 转出客服id
	FromAgentID *int64 `form:"fromAgentId"`
	// 转入客服id
	ToAgentID *int64 `form:"toAgentId"`
}

// KefuTransferSuggestion 是规则命中的建议转接，工作台据此提示客服转接
type KefuTransferSuggestion struct {
	// 命中的转接关键词
	Keyword string `json:"keyword"`
	// 建议说明, 直接展示在工作台
	Reason string `json:"reason"`
}

// KefuSessionTransferForm 是转接会话的提交数据
type KefuSessionTransferForm struct {
	// 会话id
	SessionID int64 `json:"sessionId"`
	// 目标客服id
	ToAgentID int64 `json:"toAgentId"`
	// 转接原因
	Reason string `json:"reason"`
}
