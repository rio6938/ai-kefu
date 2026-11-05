package repository

import (
	"admin/server/model"
	"admin/server/pkg/db"
)

// KefuDictionaryRepository 是客服模块读取系统字典的数据访问；
// 敏感词、转接关键词等字典由系统模块维护，这里只做只读查询
type KefuDictionaryRepository struct{}

// NewKefuDictionaryRepository 创建字典的数据访问
func NewKefuDictionaryRepository() *KefuDictionaryRepository {
	return &KefuDictionaryRepository{}
}

// ListCodes 返回字典下启用中的字典数据标识，按排序号升序；
// 字典不存在时返回空列表，不返回错误
func (r *KefuDictionaryRepository) ListCodes(dictCode string) ([]string, error) {
	var codes []string
	err := db.DB.Table("sys_dictionary_data AS d").
		Joins("join sys_dictionary AS c on c.dict_id = d.dict_id").
		Where("c.dict_code = ? and c.deleted = 0 and d.deleted = 0", dictCode).
		Order("d.sort_number asc, d.dict_data_id asc").
		Pluck("d.dict_data_code", &codes).Error
	if err != nil {
		return nil, err
	}
	return codes, nil
}

// ListItems 返回字典下启用中的字典数据（标识 + 名称），
// 建议转接的提示文案需要用到字典名称
func (r *KefuDictionaryRepository) ListItems(dictCode string) ([]model.DictionaryData, error) {
	var list []model.DictionaryData
	err := db.DB.Table("sys_dictionary_data AS d").
		Select("d.*").
		Joins("join sys_dictionary AS c on c.dict_id = d.dict_id").
		Where("c.dict_code = ? and c.deleted = 0 and d.deleted = 0", dictCode).
		Order("d.sort_number asc, d.dict_data_id asc").
		Scan(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}
