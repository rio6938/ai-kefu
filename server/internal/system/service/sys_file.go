package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"admin/server/internal/common"
	"admin/server/internal/system/repository"
	"admin/server/model"
)

// 文件上传的默认配置
const (
	// UploadDir 是文件上传的根目录
	UploadDir = "uploads"
	// MaxUploadSize 是单个文件的最大字节数
	MaxUploadSize = 20 << 20
)

// 常见的业务错误
var (
	// ErrFileEmpty 是上传文件为空
	ErrFileEmpty = errors.New("上传文件不能为空")
	// ErrFileTooLarge 是上传文件超过大小限制
	ErrFileTooLarge = errors.New("文件大小不能超过 20MB")
	// ErrFileNameRequired 是文件名称不能为空
	ErrFileNameRequired = errors.New("文件名称不能为空")
	// ErrFileNotFound 是文件记录不存在
	ErrFileNotFound = errors.New("文件不存在")
	// ErrUserFileNotFound 是用户文件不存在
	ErrUserFileNotFound = errors.New("用户文件不存在")
)

// SysFileService 是文件上传记录与用户文件的业务处理
type SysFileService struct {
	repo *repository.SysFileRepository
}

// NewSysFileService 创建文件上传记录与用户文件的业务处理
func NewSysFileService() *SysFileService {
	return &SysFileService{repo: repository.NewSysFileRepository()}
}

// Save 保存上传的文件并写入上传记录
func (s *SysFileService) Save(fileName string, r io.Reader, size int64, contentType string, userID int64) (*model.SysFile, error) {
	if size <= 0 {
		return nil, ErrFileEmpty
	}
	if size > MaxUploadSize {
		return nil, ErrFileTooLarge
	}

	savedPath, err := saveToDisk(fileName, r)
	if err != nil {
		return nil, err
	}

	now := model.Now()
	record := &model.SysFile{
		Name:         fileName,
		Path:         savedPath,
		Length:       size,
		ContentType:  contentType,
		CreateUserID: userID,
		CreateTime:   now,
		UpdateTime:   now,
	}
	if err := s.repo.Create(record); err != nil {
		return nil, err
	}
	list := []model.SysFile{*record}
	s.fillExtra(list)
	*record = list[0]
	fillFileURL(record)
	return record, nil
}

// SaveBase64 保存 base64 编码的文件并写入上传记录
func (s *SysFileService) SaveBase64(data, fileName string, userID int64) (*model.SysFile, error) {
	if fileName == "" {
		fileName = "file"
	}

	content := data
	if idx := strings.Index(content, ";base64,"); idx >= 0 {
		content = content[idx+len(";base64,"):]
	}
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("base64 数据不正确: %w", err)
	}
	if len(decoded) == 0 {
		return nil, ErrFileEmpty
	}
	if int64(len(decoded)) > MaxUploadSize {
		return nil, ErrFileTooLarge
	}
	return s.Save(fileName, strings.NewReader(string(decoded)), int64(len(decoded)), guessContentType(fileName), userID)
}

// Page 分页查询文件记录
func (s *SysFileService) Page(param model.FileRecordParam, page common.PageParam) ([]model.SysFile, int64, error) {
	list, count, err := s.repo.Page(param, page)
	if err != nil {
		return nil, 0, err
	}
	s.fillExtra(list)
	for i := range list {
		fillFileURL(&list[i])
	}
	return list, count, nil
}

// List 查询文件记录列表
func (s *SysFileService) List(param model.FileRecordParam) ([]model.SysFile, error) {
	list, err := s.repo.List(param)
	if err != nil {
		return nil, err
	}
	s.fillExtra(list)
	for i := range list {
		fillFileURL(&list[i])
	}
	return list, nil
}

// fillExtra 填充上传人信息
func (s *SysFileService) fillExtra(list []model.SysFile) {
	if len(list) == 0 {
		return
	}
	userIDs := make([]int64, 0, len(list))
	seen := make(map[int64]bool, len(list))
	for _, file := range list {
		if file.CreateUserID == 0 || seen[file.CreateUserID] {
			continue
		}
		seen[file.CreateUserID] = true
		userIDs = append(userIDs, file.CreateUserID)
	}
	if len(userIDs) == 0 {
		return
	}

	users, err := NewSysUserService().ListByIDs(userIDs)
	if err != nil {
		return
	}
	names := make(map[int64]model.SysUser, len(users))
	for _, user := range users {
		names[user.UserID] = user
	}
	for i := range list {
		user, ok := names[list[i].CreateUserID]
		if !ok {
			continue
		}
		list[i].CreateUsername = user.Username
		list[i].CreateNickname = user.Nickname
	}
}

// Remove 按 id 批量删除文件记录及磁盘文件
func (s *SysFileService) Remove(ids []int64) error {
	list, err := s.repo.Remove(ids)
	if err != nil {
		return err
	}
	for _, file := range list {
		_ = os.Remove(filepath.Join(UploadDir, filepath.FromSlash(file.Path)))
	}
	return nil
}

// ==== 用户文件 ====

// PageUserFiles 分页查询用户文件
func (s *SysFileService) PageUserFiles(param model.UserFileParam, page common.PageParam) ([]model.SysUserFile, int64, error) {
	list, count, err := s.repo.PageUserFiles(param, page)
	if err != nil {
		return nil, 0, err
	}
	for i := range list {
		fillUserFileURL(&list[i])
	}
	return list, count, nil
}

// ListUserFiles 查询用户文件列表
func (s *SysFileService) ListUserFiles(param model.UserFileParam) ([]model.SysUserFile, error) {
	list, err := s.repo.ListUserFiles(param)
	if err != nil {
		return nil, err
	}
	for i := range list {
		fillUserFileURL(&list[i])
	}
	return list, nil
}

// AddUserFile 新增用户文件
func (s *SysFileService) AddUserFile(userID int64, form model.UserFileForm) error {
	form.Name = strings.TrimSpace(form.Name)
	if form.Name == "" {
		return ErrFileNameRequired
	}

	now := model.Now()
	return s.repo.CreateUserFile(&model.SysUserFile{
		UserID:      userID,
		Name:        form.Name,
		IsDirectory: form.IsDirectory,
		ParentID:    form.ParentID,
		Path:        form.Path,
		Length:      form.Length,
		ContentType: form.ContentType,
		CreateTime:  now,
		UpdateTime:  now,
	})
}

// UpdateUserFile 修改用户文件
func (s *SysFileService) UpdateUserFile(form model.UserFileForm) error {
	file, err := s.repo.GetUserFileByID(form.ID)
	if err != nil {
		if errors.Is(err, repository.ErrUserFileNotFound) {
			return ErrUserFileNotFound
		}
		return err
	}

	form.Name = strings.TrimSpace(form.Name)
	if form.Name == "" {
		return ErrFileNameRequired
	}

	file.Name = form.Name
	file.ParentID = form.ParentID
	file.UpdateTime = model.Now()
	return s.repo.UpdateUserFile(file)
}

// RemoveUserFiles 按 id 批量删除用户文件
func (s *SysFileService) RemoveUserFiles(ids []int64) error {
	return s.repo.RemoveUserFiles(ids)
}

// saveToDisk 把文件写入 UploadDir/日期/随机名 下，返回相对路径
func saveToDisk(fileName string, r io.Reader) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	dir := time.Now().Format("20060102")
	relative := path.Join(dir, hex.EncodeToString(buf)+ext)
	fullPath := filepath.Join(UploadDir, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", err
	}

	out, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, r); err != nil {
		return "", err
	}
	return relative, nil
}

// guessContentType 按扩展名推断文件类型
func guessContentType(fileName string) string {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

// fillFileURL 填充文件访问地址
func fillFileURL(file *model.SysFile) {
	file.URL = "/api/file/raw/" + file.Path
	file.Thumbnail = "/api/file/thumbnail/" + file.Path
	file.DownloadURL = "/api/file/download/" + file.Path
}

// fillUserFileURL 填充用户文件访问地址
func fillUserFileURL(file *model.SysUserFile) {
	if file.Path == "" {
		return
	}
	file.URL = "/api/file/raw/" + file.Path
	file.Thumbnail = "/api/file/thumbnail/" + file.Path
	file.DownloadURL = "/api/file/download/" + file.Path
}
