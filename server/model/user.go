package model

// User 是前台用户，对应 user 表
type User struct {
	// id
	ID int64 `gorm:"column:id;primaryKey;autoIncrement;comment:id" json:"id"`
	// 账号
	Username string `gorm:"column:username;size:64;not null;uniqueIndex;comment:账号" json:"username"`
	// 密码，数据库存 bcrypt 哈希，不输出给前端
	Password string `gorm:"column:password;size:128;not null;comment:密码" json:"-"`
	// 昵称
	Nickname string `gorm:"column:nickname;size:64;comment:昵称" json:"nickname"`
	// 头像
	Avatar string `gorm:"column:avatar;size:255;comment:头像" json:"avatar"`
	// 手机号
	Phone string `gorm:"column:phone;size:32;comment:手机号" json:"phone"`
	// 邮箱
	Email string `gorm:"column:email;size:128;comment:邮箱" json:"email"`
	// 状态, 0正常, 1冻结
	Status int `gorm:"column:status;not null;default:0;comment:状态, 0正常, 1冻结" json:"status"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (User) TableName() string {
	return "user"
}
