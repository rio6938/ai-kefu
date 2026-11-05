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
	// ErrDictionaryNotFound 是字典不存在
	ErrDictionaryNotFound = errors.New("字典不存在")
	// ErrDictionaryDataNotFound 是字典数据不存在
	ErrDictionaryDataNotFound = errors.New("字典数据不存在")
)

// SysDictionaryRepository 是字典与字典数据的数据访问
type SysDictionaryRepository struct{}

// NewSysDictionaryRepository 创建字典与字典数据的数据访问
func NewSysDictionaryRepository() *SysDictionaryRepository {
	return &SysDictionaryRepository{}
}

// ==== 字典 ====

// db 返回带 sys_dictionary 条件的查询
func (r *SysDictionaryRepository) db() *gorm.DB {
	return db.DB.Model(&model.Dictionary{}).Where("deleted = 0")
}

// GetByID 按 id 查询字典
func (r *SysDictionaryRepository) GetByID(dictID int64) (*model.Dictionary, error) {
	dictionary := new(model.Dictionary)
	err := r.db().Where("dict_id = ?", dictID).First(dictionary).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDictionaryNotFound
	}
	if err != nil {
		return nil, err
	}
	return dictionary, nil
}

// GetByCode 按标识查询字典
func (r *SysDictionaryRepository) GetByCode(dictCode string) (*model.Dictionary, error) {
	dictionary := new(model.Dictionary)
	err := r.db().Where("dict_code = ?", dictCode).First(dictionary).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDictionaryNotFound
	}
	if err != nil {
		return nil, err
	}
	return dictionary, nil
}

// Page 分页查询字典
func (r *SysDictionaryRepository) Page(param model.DictionaryParam, page common.PageParam) ([]model.Dictionary, int64, error) {
	query := r.db()
	if param.DictCode != "" {
		query = query.Where("dict_code like ?", "%"+param.DictCode+"%")
	}
	if param.DictName != "" {
		query = query.Where("dict_name like ?", "%"+param.DictName+"%")
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.Dictionary
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "sort_number asc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// List 查询字典列表
func (r *SysDictionaryRepository) List(param model.DictionaryParam) ([]model.Dictionary, error) {
	var list []model.Dictionary
	query := r.db()
	if param.DictCode != "" {
		query = query.Where("dict_code like ?", "%"+param.DictCode+"%")
	}
	if param.DictName != "" {
		query = query.Where("dict_name like ?", "%"+param.DictName+"%")
	}
	err := query.Order("sort_number asc").Order("dict_id asc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// ExistsByCode 判断字典标识是否已存在
func (r *SysDictionaryRepository) ExistsByCode(dictCode string, excludeID int64) (bool, error) {
	query := r.db().Where("dict_code = ?", dictCode)
	if excludeID > 0 {
		query = query.Where("dict_id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create 新增字典
func (r *SysDictionaryRepository) Create(dictionary *model.Dictionary) error {
	return db.DB.Create(dictionary).Error
}

// Update 修改字典
func (r *SysDictionaryRepository) Update(dictionary *model.Dictionary) error {
	return db.DB.Save(dictionary).Error
}

// Remove 批量删除字典及其字典数据
func (r *SysDictionaryRepository) Remove(dictIDs []int64) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		now := model.Now()
		if err := tx.Model(&model.Dictionary{}).Where("dict_id in ?", dictIDs).
			Updates(map[string]any{"deleted": 1, "update_time": now}).Error; err != nil {
			return err
		}
		return tx.Model(&model.DictionaryData{}).Where("dict_id in ?", dictIDs).
			Updates(map[string]any{"deleted": 1, "update_time": now}).Error
	})
}

// ==== 字典数据 ====

// dataDB 返回带 sys_dictionary_data 条件的查询
func (r *SysDictionaryRepository) dataDB() *gorm.DB {
	return db.DB.Model(&model.DictionaryData{}).Where("deleted = 0")
}

// GetDataByID 按 id 查询字典数据
func (r *SysDictionaryRepository) GetDataByID(dictDataID int64) (*model.DictionaryData, error) {
	data := new(model.DictionaryData)
	err := r.dataDB().Where("dict_data_id = ?", dictDataID).First(data).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDictionaryDataNotFound
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// PageData 分页查询字典数据
func (r *SysDictionaryRepository) PageData(param model.DictionaryDataParam, page common.PageParam) ([]model.DictionaryData, int64, error) {
	query := r.dataDB()
	if param.DictID != nil {
		query = query.Where("dict_id = ?", *param.DictID)
	}
	if param.DictCode != "" {
		query = query.Where("dict_id = (?)",
			db.DB.Model(&model.Dictionary{}).Select("dict_id").Where("dict_code = ?", param.DictCode),
		)
	}
	if param.DictDataCode != "" {
		query = query.Where("dict_data_code like ?", "%"+param.DictDataCode+"%")
	}
	if param.DictDataName != "" {
		query = query.Where("dict_data_name like ?", "%"+param.DictDataName+"%")
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.DictionaryData
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "sort_number asc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// ListData 查询字典数据列表
func (r *SysDictionaryRepository) ListData(param model.DictionaryDataParam) ([]model.DictionaryData, error) {
	var list []model.DictionaryData
	query := r.dataDB()
	if param.DictID != nil {
		query = query.Where("dict_id = ?", *param.DictID)
	}
	if param.DictCode != "" {
		query = query.Where("dict_id = (?)",
			db.DB.Model(&model.Dictionary{}).Select("dict_id").Where("dict_code = ?", param.DictCode),
		)
	}
	if param.DictDataCode != "" {
		query = query.Where("dict_data_code like ?", "%"+param.DictDataCode+"%")
	}
	if param.DictDataName != "" {
		query = query.Where("dict_data_name like ?", "%"+param.DictDataName+"%")
	}
	err := query.Order("sort_number asc").Order("dict_data_id asc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// ExistsDataCode 判断字典数据标识是否已存在
func (r *SysDictionaryRepository) ExistsDataCode(dictID int64, code string, excludeID int64) (bool, error) {
	query := r.dataDB().Where("dict_id = ? and dict_data_code = ?", dictID, code)
	if excludeID > 0 {
		query = query.Where("dict_data_id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateData 新增字典数据
func (r *SysDictionaryRepository) CreateData(data *model.DictionaryData) error {
	return db.DB.Create(data).Error
}

// UpdateData 修改字典数据
func (r *SysDictionaryRepository) UpdateData(data *model.DictionaryData) error {
	return db.DB.Save(data).Error
}

// RemoveData 按 id 批量软删除字典数据
func (r *SysDictionaryRepository) RemoveData(dictDataIDs []int64) error {
	return db.DB.Model(&model.DictionaryData{}).
		Where("dict_data_id in ?", dictDataIDs).
		Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error
}
