package model

// SysUser 是系统用户，对应 sys_user 表
type SysUser struct {
	// 用户id
	UserID int64 `gorm:"column:user_id;primaryKey;autoIncrement;comment:用户id" json:"userId"`
	// 账号
	Username string `gorm:"column:username;size:64;not null;uniqueIndex;comment:账号" json:"username"`
	// 密码，数据库存 bcrypt 哈希，不输出给前端
	Password string `gorm:"column:password;size:128;not null;comment:密码" json:"-"`
	// 昵称
	Nickname string `gorm:"column:nickname;size:64;comment:昵称" json:"nickname"`
	// 头像
	Avatar string `gorm:"column:avatar;size:255;comment:头像" json:"avatar"`
	// 性别, 1男, 2女
	Sex string `gorm:"column:sex;size:4;comment:性别" json:"sex"`
	// 性别名称，不由数据库存储
	SexName string `gorm:"-" json:"sexName"`
	// 手机号
	Phone string `gorm:"column:phone;size:32;comment:手机号" json:"phone"`
	// 邮箱
	Email string `gorm:"column:email;size:128;comment:邮箱" json:"email"`
	// 出生日期
	Birthday string `gorm:"column:birthday;size:32;comment:出生日期" json:"birthday"`
	// 个人简介
	Introduction string `gorm:"column:introduction;size:255;comment:个人简介" json:"introduction"`
	// 状态, 0正常, 1冻结
	Status int `gorm:"column:status;not null;default:0;comment:状态, 0正常, 1冻结" json:"status"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 角色列表，按用户角色关联回填，不由数据库存储
	Roles []Role `gorm:"-" json:"roles"`
	// 权限列表，按角色菜单关联回填，不由数据库存储
	Authorities []Menu `gorm:"-" json:"authorities"`
}

// TableName 指定表名
func (SysUser) TableName() string {
	return "sys_user"
}

// FillExtra 填充不由数据库存储的展示字段，角色与权限由服务层按 rbac 回填
func (u *SysUser) FillExtra() {
	u.SexName = SexName(u.Sex)
}

// SexName 返回性别名称
func SexName(sex string) string {
	switch sex {
	case "1":
		return "男"
	case "2":
		return "女"
	default:
		return ""
	}
}
