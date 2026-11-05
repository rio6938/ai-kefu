package service

import (
	"errors"
	"strings"

	"admin/server/internal/common"
	"admin/server/internal/system/repository"
	"admin/server/model"
)

// 常见的业务错误
var (
	// ErrDictCodeRequired 是字典标识不能为空
	ErrDictCodeRequired = errors.New("字典标识不能为空")
	// ErrDictNameRequired 是字典名称不能为空
	ErrDictNameRequired = errors.New("字典名称不能为空")
	// ErrDictCodeExists 是字典标识已存在
	ErrDictCodeExists = errors.New("字典标识已经存在")
	// ErrDictDataCodeRequired 是字典数据标识不能为空
	ErrDictDataCodeRequired = errors.New("字典数据标识不能为空")
	// ErrDictDataNameRequired 是字典数据名称不能为空
	ErrDictDataNameRequired = errors.New("字典数据名称不能为空")
	// ErrDictDataCodeExists 是字典数据标识已存在
	ErrDictDataCodeExists = errors.New("字典数据标识已经存在")
	// ErrDictionaryNotFound 是字典不存在
	ErrDictionaryNotFound = errors.New("字典不存在")
	// ErrDictionaryDataNotFound 是字典数据不存在
	ErrDictionaryDataNotFound = errors.New("字典数据不存在")
)

// SysDictionaryService 是字典与字典数据的业务处理
type SysDictionaryService struct {
	repo *repository.SysDictionaryRepository
}

// NewSysDictionaryService 创建字典与字典数据的业务处理
func NewSysDictionaryService() *SysDictionaryService {
	return &SysDictionaryService{repo: repository.NewSysDictionaryRepository()}
}

// Page 分页查询字典
func (s *SysDictionaryService) Page(param model.DictionaryParam, page common.PageParam) ([]model.Dictionary, int64, error) {
	return s.repo.Page(param, page)
}

// List 查询字典列表
func (s *SysDictionaryService) List(param model.DictionaryParam) ([]model.Dictionary, error) {
	return s.repo.List(param)
}

// Add 新增字典
func (s *SysDictionaryService) Add(form model.DictionaryForm) error {
	form.DictCode = strings.TrimSpace(form.DictCode)
	form.DictName = strings.TrimSpace(form.DictName)
	if form.DictCode == "" {
		return ErrDictCodeRequired
	}
	if form.DictName == "" {
		return ErrDictNameRequired
	}

	exists, err := s.repo.ExistsByCode(form.DictCode, 0)
	if err != nil {
		return err
	}
	if exists {
		return ErrDictCodeExists
	}

	now := model.Now()
	return s.repo.Create(&model.Dictionary{
		DictCode:   form.DictCode,
		DictName:   form.DictName,
		SortNumber: form.SortNumber,
		Comments:   form.Comments,
		CreateTime: now,
		UpdateTime: now,
	})
}

// Update 修改字典
func (s *SysDictionaryService) Update(form model.DictionaryForm) error {
	dictionary, err := s.repo.GetByID(form.DictID)
	if err != nil {
		if errors.Is(err, repository.ErrDictionaryNotFound) {
			return ErrDictionaryNotFound
		}
		return err
	}

	form.DictCode = strings.TrimSpace(form.DictCode)
	form.DictName = strings.TrimSpace(form.DictName)
	if form.DictCode == "" {
		return ErrDictCodeRequired
	}
	if form.DictName == "" {
		return ErrDictNameRequired
	}

	exists, err := s.repo.ExistsByCode(form.DictCode, form.DictID)
	if err != nil {
		return err
	}
	if exists {
		return ErrDictCodeExists
	}

	dictionary.DictCode = form.DictCode
	dictionary.DictName = form.DictName
	dictionary.SortNumber = form.SortNumber
	dictionary.Comments = form.Comments
	dictionary.UpdateTime = model.Now()
	return s.repo.Update(dictionary)
}

// Remove 按 id 删除字典
func (s *SysDictionaryService) Remove(dictID int64) error {
	if _, err := s.repo.GetByID(dictID); err != nil {
		if errors.Is(err, repository.ErrDictionaryNotFound) {
			return ErrDictionaryNotFound
		}
		return err
	}
	return s.repo.Remove([]int64{dictID})
}

// PageData 分页查询字典数据
func (s *SysDictionaryService) PageData(param model.DictionaryDataParam, page common.PageParam) ([]model.DictionaryData, int64, error) {
	list, count, err := s.repo.PageData(param, page)
	if err != nil {
		return nil, 0, err
	}
	s.fillDataExtra(list)
	return list, count, nil
}

// ListData 查询字典数据列表
func (s *SysDictionaryService) ListData(param model.DictionaryDataParam) ([]model.DictionaryData, error) {
	list, err := s.repo.ListData(param)
	if err != nil {
		return nil, err
	}
	s.fillDataExtra(list)
	return list, nil
}

// fillDataExtra 填充字典数据所属字典的标识与名称
func (s *SysDictionaryService) fillDataExtra(list []model.DictionaryData) {
	if len(list) == 0 {
		return
	}
	dictionaries, err := s.repo.List(model.DictionaryParam{})
	if err != nil {
		return
	}
	codes := make(map[int64]model.Dictionary, len(dictionaries))
	for _, dictionary := range dictionaries {
		codes[dictionary.DictID] = dictionary
	}
	for i := range list {
		dictionary, ok := codes[list[i].DictID]
		if !ok {
			continue
		}
		list[i].DictCode = dictionary.DictCode
		list[i].DictName = dictionary.DictName
	}
}

// AddData 新增字典数据
func (s *SysDictionaryService) AddData(form model.DictionaryDataForm) error {
	form.DictDataCode = strings.TrimSpace(form.DictDataCode)
	form.DictDataName = strings.TrimSpace(form.DictDataName)
	if form.DictDataCode == "" {
		return ErrDictDataCodeRequired
	}
	if form.DictDataName == "" {
		return ErrDictDataNameRequired
	}
	if _, err := s.repo.GetByID(form.DictID); err != nil {
		if errors.Is(err, repository.ErrDictionaryNotFound) {
			return ErrDictionaryNotFound
		}
		return err
	}

	exists, err := s.repo.ExistsDataCode(form.DictID, form.DictDataCode, 0)
	if err != nil {
		return err
	}
	if exists {
		return ErrDictDataCodeExists
	}

	now := model.Now()
	return s.repo.CreateData(&model.DictionaryData{
		DictID:       form.DictID,
		DictDataCode: form.DictDataCode,
		DictDataName: form.DictDataName,
		SortNumber:   form.SortNumber,
		Comments:     form.Comments,
		CreateTime:   now,
		UpdateTime:   now,
	})
}

// UpdateData 修改字典数据
func (s *SysDictionaryService) UpdateData(form model.DictionaryDataForm) error {
	data, err := s.repo.GetDataByID(form.DictDataID)
	if err != nil {
		if errors.Is(err, repository.ErrDictionaryDataNotFound) {
			return ErrDictionaryDataNotFound
		}
		return err
	}

	form.DictDataCode = strings.TrimSpace(form.DictDataCode)
	form.DictDataName = strings.TrimSpace(form.DictDataName)
	if form.DictDataCode == "" {
		return ErrDictDataCodeRequired
	}
	if form.DictDataName == "" {
		return ErrDictDataNameRequired
	}

	exists, err := s.repo.ExistsDataCode(form.DictID, form.DictDataCode, form.DictDataID)
	if err != nil {
		return err
	}
	if exists {
		return ErrDictDataCodeExists
	}

	data.DictID = form.DictID
	data.DictDataCode = form.DictDataCode
	data.DictDataName = form.DictDataName
	data.SortNumber = form.SortNumber
	data.Comments = form.Comments
	data.UpdateTime = model.Now()
	return s.repo.UpdateData(data)
}

// RemoveData 按 id 删除字典数据
func (s *SysDictionaryService) RemoveData(dictDataID int64) error {
	if _, err := s.repo.GetDataByID(dictDataID); err != nil {
		if errors.Is(err, repository.ErrDictionaryDataNotFound) {
			return ErrDictionaryDataNotFound
		}
		return err
	}
	return s.repo.RemoveData([]int64{dictDataID})
}

// RemoveDataBatch 按 id 批量删除字典数据
func (s *SysDictionaryService) RemoveDataBatch(dictDataIDs []int64) error {
	return s.repo.RemoveData(dictDataIDs)
}
