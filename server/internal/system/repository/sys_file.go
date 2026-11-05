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
	// ErrFileNotFound 是文件记录不存在
	ErrFileNotFound = errors.New("文件不存在")
	// ErrUserFileNotFound 是用户文件不存在
	ErrUserFileNotFound = errors.New("用户文件不存在")
)

// SysFileRepository 是文件上传记录与用户文件的数据访问
type SysFileRepository struct{}

// NewSysFileRepository 创建文件上传记录与用户文件的数据访问
func NewSysFileRepository() *SysFileRepository {
	return &SysFileRepository{}
}

// ==== 文件上传记录 ====

// db 返回带 sys_file 条件的查询
func (r *SysFileRepository) db() *gorm.DB {
	return db.DB.Model(&model.SysFile{}).Where("deleted = 0")
}

// GetByID 按 id 查询文件记录
func (r *SysFileRepository) GetByID(id int64) (*model.SysFile, error) {
	file := new(model.SysFile)
	err := r.db().Where("id = ?", id).First(file).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrFileNotFound
	}
	if err != nil {
		return nil, err
	}
	return file, nil
}

// Page 分页查询文件记录
func (r *SysFileRepository) Page(param model.FileRecordParam, page common.PageParam) ([]model.SysFile, int64, error) {
	query := r.db()
	if param.Name != "" {
		query = query.Where("name like ?", "%"+param.Name+"%")
	}
	if param.Path != "" {
		query = query.Where("path like ?", "%"+param.Path+"%")
	}
	if param.CreateNickname != "" {
		query = query.Where("create_user_id in (?)",
			db.DB.Model(&model.SysUser{}).Select("user_id").
				Where("nickname like ?", "%"+param.CreateNickname+"%"),
		)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.SysFile
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "id desc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// List 查询文件记录列表
func (r *SysFileRepository) List(param model.FileRecordParam) ([]model.SysFile, error) {
	var list []model.SysFile
	query := r.db()
	if param.Name != "" {
		query = query.Where("name like ?", "%"+param.Name+"%")
	}
	if param.Path != "" {
		query = query.Where("path like ?", "%"+param.Path+"%")
	}
	if param.CreateNickname != "" {
		query = query.Where("create_user_id in (?)",
			db.DB.Model(&model.SysUser{}).Select("user_id").
				Where("nickname like ?", "%"+param.CreateNickname+"%"),
		)
	}
	err := query.Order("id desc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// Create 新增文件记录
func (r *SysFileRepository) Create(file *model.SysFile) error {
	return db.DB.Create(file).Error
}

// Remove 按 id 批量删除文件记录
func (r *SysFileRepository) Remove(ids []int64) ([]model.SysFile, error) {
	var list []model.SysFile
	if err := r.db().Where("id in ?", ids).Find(&list).Error; err != nil {
		return nil, err
	}
	err := db.DB.Model(&model.SysFile{}).Where("id in ?", ids).
		Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// ==== 用户文件 ====

// userFileDB 返回带 sys_user_file 条件的查询
func (r *SysFileRepository) userFileDB() *gorm.DB {
	return db.DB.Model(&model.SysUserFile{}).Where("deleted = 0")
}

// GetUserFileByID 按 id 查询用户文件
func (r *SysFileRepository) GetUserFileByID(id int64) (*model.SysUserFile, error) {
	file := new(model.SysUserFile)
	err := r.userFileDB().Where("id = ?", id).First(file).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserFileNotFound
	}
	if err != nil {
		return nil, err
	}
	return file, nil
}

// PageUserFiles 分页查询用户文件
func (r *SysFileRepository) PageUserFiles(param model.UserFileParam, page common.PageParam) ([]model.SysUserFile, int64, error) {
	query := r.userFileDB()
	if param.UserID != nil {
		query = query.Where("user_id = ?", *param.UserID)
	}
	if param.Name != "" {
		query = query.Where("name like ?", "%"+param.Name+"%")
	}
	if param.ParentID != nil {
		query = query.Where("parent_id = ?", *param.ParentID)
	}
	if param.IsDirectory != nil {
		query = query.Where("is_directory = ?", *param.IsDirectory)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.SysUserFile
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "create_time desc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// ListUserFiles 查询用户文件列表
func (r *SysFileRepository) ListUserFiles(param model.UserFileParam) ([]model.SysUserFile, error) {
	var list []model.SysUserFile
	query := r.userFileDB()
	if param.UserID != nil {
		query = query.Where("user_id = ?", *param.UserID)
	}
	if param.Name != "" {
		query = query.Where("name like ?", "%"+param.Name+"%")
	}
	if param.ParentID != nil {
		query = query.Where("parent_id = ?", *param.ParentID)
	}
	if param.IsDirectory != nil {
		query = query.Where("is_directory = ?", *param.IsDirectory)
	}
	err := query.Order("create_time desc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// CreateUserFile 新增用户文件
func (r *SysFileRepository) CreateUserFile(file *model.SysUserFile) error {
	return db.DB.Create(file).Error
}

// UpdateUserFile 修改用户文件
func (r *SysFileRepository) UpdateUserFile(file *model.SysUserFile) error {
	return db.DB.Save(file).Error
}

// RemoveUserFiles 按 id 批量软删除用户文件
func (r *SysFileRepository) RemoveUserFiles(ids []int64) error {
	return db.DB.Model(&model.SysUserFile{}).Where("id in ?", ids).
		Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error
}
