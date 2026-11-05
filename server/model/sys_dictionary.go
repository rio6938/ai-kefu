package model

import "admin/server/internal/common"

// Dictionary 是字典，对应 sys_dictionary 表
type Dictionary struct {
	// 字典id
	DictID int64 `gorm:"column:dict_id;primaryKey;autoIncrement;comment:字典id" json:"dictId"`
	// 字典标识
	DictCode string `gorm:"column:dict_code;size:64;not null;uniqueIndex;comment:字典标识" json:"dictCode"`
	// 字典名称
	DictName string `gorm:"column:dict_name;size:64;not null;comment:字典名称" json:"dictName"`
	// 排序号
	SortNumber int `gorm:"column:sort_number;not null;default:0;comment:排序号" json:"sortNumber"`
	// 备注
	Comments string `gorm:"column:comments;size:255;comment:备注" json:"comments"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (Dictionary) TableName() string {
	return "sys_dictionary"
}

// DictionaryData 是字典数据，对应 sys_dictionary_data 表
type DictionaryData struct {
	// 字典数据id
	DictDataID int64 `gorm:"column:dict_data_id;primaryKey;autoIncrement;comment:字典数据id" json:"dictDataId"`
	// 字典id
	DictID int64 `gorm:"column:dict_id;not null;comment:字典id" json:"dictId"`
	// 字典数据标识
	DictDataCode string `gorm:"column:dict_data_code;size:64;not null;comment:字典数据标识" json:"dictDataCode"`
	// 字典数据名称
	DictDataName string `gorm:"column:dict_data_name;size:128;not null;comment:字典数据名称" json:"dictDataName"`
	// 排序号
	SortNumber int `gorm:"column:sort_number;not null;default:0;comment:排序号" json:"sortNumber"`
	// 备注
	Comments string `gorm:"column:comments;size:255;comment:备注" json:"comments"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 字典标识, 关联字典回填
	DictCode string `gorm:"-" json:"dictCode"`
	// 字典名称, 关联字典回填
	DictName string `gorm:"-" json:"dictName"`
}

// TableName 指定表名
func (DictionaryData) TableName() string {
	return "sys_dictionary_data"
}

// DictionaryParam 是字典的查询参数
type DictionaryParam struct {
	common.PageParam
	// 字典标识
	DictCode string `form:"dictCode"`
	// 字典名称
	DictName string `form:"dictName"`
}

// DictionaryForm 是字典的编辑提交数据
type DictionaryForm struct {
	// 字典id
	DictID int64 `json:"dictId"`
	// 字典标识
	DictCode string `json:"dictCode"`
	// 字典名称
	DictName string `json:"dictName"`
	// 排序号
	SortNumber int `json:"sortNumber"`
	// 备注
	Comments string `json:"comments"`
}

// DictionaryDataParam 是字典数据的查询参数
type DictionaryDataParam struct {
	common.PageParam
	// 字典id
	DictID *int64 `form:"dictId"`
	// 字典标识
	DictCode string `form:"dictCode"`
	// 字典数据标识
	DictDataCode string `form:"dictDataCode"`
	// 字典数据名称
	DictDataName string `form:"dictDataName"`
}

// DictionaryDataForm 是字典数据的编辑提交数据
type DictionaryDataForm struct {
	// 字典数据id
	DictDataID int64 `json:"dictDataId"`
	// 字典id
	DictID int64 `json:"dictId"`
	// 字典数据标识
	DictDataCode string `json:"dictDataCode"`
	// 字典数据名称
	DictDataName string `json:"dictDataName"`
	// 排序号
	SortNumber int `json:"sortNumber"`
	// 备注
	Comments string `json:"comments"`
}
