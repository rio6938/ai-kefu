package model

import "admin/server/internal/common"

// 角色标识
const (
	// RoleCodeAdmin 是超级管理员角色标识，拥有全部菜单权限
	RoleCodeAdmin = "admin"
)

// Role 是系统角色，对应 sys_role 表
type Role struct {
	// 角色id
	RoleID int64 `gorm:"column:role_id;primaryKey;autoIncrement;comment:角色id" json:"roleId"`
	// 角色标识
	RoleCode string `gorm:"column:role_code;size:64;not null;uniqueIndex;comment:角色标识" json:"roleCode"`
	// 角色名称
	RoleName string `gorm:"column:role_name;size:64;not null;comment:角色名称" json:"roleName"`
	// 备注
	Comments string `gorm:"column:comments;size:255;comment:备注" json:"comments"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 用户id, 查询用户角色时回填
	UserID *int64 `gorm:"-" json:"userId"`
	// 是否已授权
	Checked *bool `gorm:"-" json:"checked,omitempty"`
}

// TableName 指定表名
func (Role) TableName() string {
	return "sys_role"
}

// Menu 是系统菜单与权限，对应 sys_menu 表
type Menu struct {
	// 菜单id
	MenuID int64 `gorm:"column:menu_id;primaryKey;autoIncrement;comment:菜单id" json:"menuId"`
	// 上级id, 0是顶级
	ParentID int64 `gorm:"column:parent_id;not null;default:0;comment:上级id" json:"parentId"`
	// 菜单名称
	Title string `gorm:"column:title;size:64;not null;comment:菜单名称" json:"title"`
	// 路由地址
	Path string `gorm:"column:path;size:255;comment:路由地址" json:"path"`
	// 组件地址
	Component string `gorm:"column:component;size:255;comment:组件地址" json:"component"`
	// 菜单类型, 0菜单, 1按钮
	MenuType int `gorm:"column:menu_type;not null;default:0;comment:菜单类型, 0菜单, 1按钮" json:"menuType"`
	// 排序号
	SortNumber int `gorm:"column:sort_number;not null;default:0;comment:排序号" json:"sortNumber"`
	// 权限标识
	Authority string `gorm:"column:authority;size:128;comment:权限标识" json:"authority"`
	// 菜单图标
	Icon string `gorm:"column:icon;size:64;comment:菜单图标" json:"icon"`
	// 是否隐藏, 0否, 1是
	Hide int `gorm:"column:hide;not null;default:0;comment:是否隐藏" json:"hide"`
	// 路由元信息
	Meta string `gorm:"column:meta;size:512;comment:路由元信息" json:"meta"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 子菜单, 查询菜单树时回填
	Children []Menu `gorm:"-" json:"children,omitempty"`
	// 权限树回显选中状态
	Checked *bool `gorm:"-" json:"checked,omitempty"`
}

// TableName 指定表名
func (Menu) TableName() string {
	return "sys_menu"
}

// RoleMenu 是角色与菜单的关联，对应 sys_role_menu 表
type RoleMenu struct {
	// 角色id
	RoleID int64 `gorm:"column:role_id;primaryKey;comment:角色id" json:"roleId"`
	// 菜单id
	MenuID int64 `gorm:"column:menu_id;primaryKey;comment:菜单id" json:"menuId"`
}

// TableName 指定表名
func (RoleMenu) TableName() string {
	return "sys_role_menu"
}

// UserRole 是用户与角色的关联，对应 sys_user_role 表
type UserRole struct {
	// 用户id
	UserID int64 `gorm:"column:user_id;primaryKey;comment:用户id" json:"userId"`
	// 角色id
	RoleID int64 `gorm:"column:role_id;primaryKey;comment:角色id" json:"roleId"`
}

// TableName 指定表名
func (UserRole) TableName() string {
	return "sys_user_role"
}

// RoleParam 是角色的查询参数
type RoleParam struct {
	common.PageParam
	// 角色名称
	RoleName string `form:"roleName"`
	// 角色标识
	RoleCode string `form:"roleCode"`
	// 备注
	Comments string `form:"comments"`
}

// RoleForm 是角色的编辑提交数据
type RoleForm struct {
	// 角色id
	RoleID int64 `json:"roleId"`
	// 角色标识
	RoleCode string `json:"roleCode"`
	// 角色名称
	RoleName string `json:"roleName"`
	// 备注
	Comments string `json:"comments"`
}

// MenuParam 是菜单的查询参数
type MenuParam struct {
	common.PageParam
	// 菜单名称
	Title string `form:"title"`
	// 路由地址
	Path string `form:"path"`
	// 权限标识
	Authority string `form:"authority"`
	// 上级id
	ParentID *int64 `form:"parentId"`
}

// MenuForm 是菜单的编辑提交数据
type MenuForm struct {
	// 菜单id
	MenuID int64 `json:"menuId"`
	// 上级id, 0是顶级
	ParentID int64 `json:"parentId"`
	// 菜单名称
	Title string `json:"title"`
	// 路由地址
	Path string `json:"path"`
	// 组件地址
	Component string `json:"component"`
	// 菜单类型, 0菜单, 1按钮
	MenuType int `json:"menuType"`
	// 排序号
	SortNumber int `json:"sortNumber"`
	// 权限标识
	Authority string `json:"authority"`
	// 菜单图标
	Icon string `json:"icon"`
	// 是否隐藏, 0否, 1是
	Hide int `json:"hide"`
	// 路由元信息
	Meta string `json:"meta"`
}
