package model

// KefuSessionTag 是会话标签关联，对应 kefu_session_tag 表；
// 标签编码取自字典 kefu_session_tag，覆盖式设置时直接替换整组关联
type KefuSessionTag struct {
	// 关联id
	TagID int64 `gorm:"column:tag_id;primaryKey;autoIncrement;comment:关联id" json:"tagId"`
	// 会话id
	SessionID int64 `gorm:"column:session_id;not null;uniqueIndex:uk_kefu_session_tag;comment:会话id" json:"sessionId"`
	// 标签编码
	TagCode string `gorm:"column:tag_code;size:64;not null;uniqueIndex:uk_kefu_session_tag;comment:标签编码" json:"tagCode"`
	// 创建人id
	CreateUserID int64 `gorm:"column:create_user_id;not null;default:0;comment:创建人id" json:"createUserId"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
}

// TableName 指定表名
func (KefuSessionTag) TableName() string {
	return "kefu_session_tag"
}
