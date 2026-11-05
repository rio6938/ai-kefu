package repository

import (
	"errors"

	"gorm.io/gorm"

	"admin/server/model"
	"admin/server/pkg/db"
)

// ErrUploadFileNotFound 是知识库关联的上传文件不存在
var ErrUploadFileNotFound = errors.New("上传的文件不存在")

// KefuFileRepository 是知识库入库与访客附件读取上传文件记录的数据访问；
// sys_file 由系统模块维护，这里只做只读查询与访客附件的过期清理
type KefuFileRepository struct{}

// NewKefuFileRepository 创建上传文件记录的数据访问
func NewKefuFileRepository() *KefuFileRepository {
	return &KefuFileRepository{}
}

// GetByID 按 id 查询上传文件记录
func (r *KefuFileRepository) GetByID(fileID int64) (*model.SysFile, error) {
	record := new(model.SysFile)
	err := db.DB.Model(&model.SysFile{}).
		Where("id = ? and deleted = 0", fileID).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUploadFileNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// ListExpiredVisitorFiles 分页查询过期的访客附件记录，
// 只取 create_user_id = 0 的访客上传，客服与后台上传的文件不动
func (r *KefuFileRepository) ListExpiredVisitorFiles(
	before model.Time, limit int,
) ([]model.SysFile, error) {
	var list []model.SysFile
	err := db.DB.Model(&model.SysFile{}).
		Where("create_user_id = 0 and deleted = 0 and create_time < ?", before).
		Order("id asc").Limit(limit).Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// SoftDeleteByIDs 批量软删除上传文件记录
func (r *KefuFileRepository) SoftDeleteByIDs(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return db.DB.Model(&model.SysFile{}).Where("id in ?", ids).
		Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error
}
