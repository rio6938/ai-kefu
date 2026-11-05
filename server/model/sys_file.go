package model

import "admin/server/internal/common"

// SysFile 是文件上传记录，对应 sys_file 表
type SysFile struct {
	// id
	ID int64 `gorm:"column:id;primaryKey;autoIncrement;comment:id" json:"id"`
	// 文件名称
	Name string `gorm:"column:name;size:255;not null;comment:文件名称" json:"name"`
	// 文件存储路径
	Path string `gorm:"column:path;size:255;not null;comment:文件存储路径" json:"path"`
	// 文件大小
	Length int64 `gorm:"column:length;comment:文件大小" json:"length"`
	// 文件类型
	ContentType string `gorm:"column:content_type;size:128;comment:文件类型" json:"contentType"`
	// 上传人id
	CreateUserID int64 `gorm:"column:create_user_id;comment:上传人id" json:"createUserId"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 文件访问地址, 不由数据库存储
	URL string `gorm:"-" json:"url"`
	// 文件缩略图访问地址, 不由数据库存储
	Thumbnail string `gorm:"-" json:"thumbnail"`
	// 文件下载地址, 不由数据库存储
	DownloadURL string `gorm:"-" json:"downloadUrl"`
	// 上传人账号, 关联用户回填
	CreateUsername string `gorm:"-" json:"createUsername"`
	// 上传人名称, 关联用户回填
	CreateNickname string `gorm:"-" json:"createNickname"`
}

// TableName 指定表名
func (SysFile) TableName() string {
	return "sys_file"
}

// SysUserFile 是用户文件，对应 sys_user_file 表
type SysUserFile struct {
	// id
	ID int64 `gorm:"column:id;primaryKey;autoIncrement;comment:id" json:"id"`
	// 用户id
	UserID int64 `gorm:"column:user_id;not null;comment:用户id" json:"userId"`
	// 文件名称
	Name string `gorm:"column:name;size:255;not null;comment:文件名称" json:"name"`
	// 是否是文件夹, 0否, 1是
	IsDirectory int `gorm:"column:is_directory;not null;default:0;comment:是否是文件夹" json:"isDirectory"`
	// 上级id
	ParentID int64 `gorm:"column:parent_id;not null;default:0;comment:上级id" json:"parentId"`
	// 文件存储路径
	Path string `gorm:"column:path;size:255;comment:文件存储路径" json:"path"`
	// 文件大小
	Length int64 `gorm:"column:length;comment:文件大小" json:"length"`
	// 文件类型
	ContentType string `gorm:"column:content_type;size:128;comment:文件类型" json:"contentType"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 文件访问地址, 不由数据库存储
	URL string `gorm:"-" json:"url"`
	// 文件缩略图访问地址, 不由数据库存储
	Thumbnail string `gorm:"-" json:"thumbnail"`
	// 文件下载地址, 不由数据库存储
	DownloadURL string `gorm:"-" json:"downloadUrl"`
}

// TableName 指定表名
func (SysUserFile) TableName() string {
	return "sys_user_file"
}

// FileRecordParam 是文件上传记录的查询参数
type FileRecordParam struct {
	common.PageParam
	// 文件名称
	Name string `form:"name"`
	// 文件存储路径
	Path string `form:"path"`
	// 上传人名称
	CreateNickname string `form:"createNickname"`
}

// UserFileParam 是用户文件的查询参数
type UserFileParam struct {
	common.PageParam
	// 用户id
	UserID *int64 `form:"userId"`
	// 文件名称
	Name string `form:"name"`
	// 上级id
	ParentID *int64 `form:"parentId"`
	// 是否是文件夹
	IsDirectory *int `form:"isDirectory"`
}

// UserFileForm 是用户文件的编辑提交数据
type UserFileForm struct {
	// id
	ID int64 `json:"id"`
	// 文件名称
	Name string `json:"name"`
	// 是否是文件夹, 0否, 1是
	IsDirectory int `json:"isDirectory"`
	// 上级id
	ParentID int64 `json:"parentId"`
	// 文件存储路径
	Path string `json:"path"`
	// 文件大小
	Length int64 `json:"length"`
	// 文件类型
	ContentType string `json:"contentType"`
}
