package service

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"admin/server/internal/common"
	"admin/server/internal/user/repository"
	"admin/server/model"
	"admin/server/pkg/jwt"
)

// 密码哈希使用的 bcrypt 强度，与 system 模块保持一致
const bcryptCost = 10

// 常见的业务错误
var (
	// ErrUserNotFound 是用户不存在
	ErrUserNotFound = errors.New("用户不存在")
	// ErrUsernameExists 是账号已存在
	ErrUsernameExists = errors.New("账号已经存在")
	// ErrUserDisabled 是账号被冻结
	ErrUserDisabled = errors.New("账号已被冻结")
	// ErrPasswordIncorrect 是账号或密码错误
	ErrPasswordIncorrect = errors.New("账号或密码错误")
	// ErrPasswordRequired 是密码不能为空
	ErrPasswordRequired = errors.New("密码不能为空")
	// ErrNicknameRequired 是昵称不能为空
	ErrNicknameRequired = errors.New("昵称不能为空")
)

// UserLoginResult 是登录成功返回的数据
type UserLoginResult struct {
	// token
	AccessToken string `json:"access_token"`
	// 用户信息
	User *model.User `json:"user"`
}

// UserService 是前台用户的业务处理
type UserService struct {
	repo *repository.UserRepository
}

// NewUserService 创建前台用户的业务处理
func NewUserService() *UserService {
	return &UserService{
		repo: repository.NewUserRepository(),
	}
}

// Register 注册前台用户，返回创建的用户，账号重复时返回 ErrUsernameExists
func (s *UserService) Register(username, password, nickname string) (*model.User, error) {
	username = strings.TrimSpace(username)
	nickname = strings.TrimSpace(nickname)
	if strings.TrimSpace(password) == "" {
		return nil, ErrPasswordRequired
	}
	if nickname == "" {
		return nil, ErrNicknameRequired
	}

	exists, err := s.repo.ExistsByUsername(username, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameExists
	}

	hashed, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	now := model.Now()
	user := &model.User{
		Username:   username,
		Password:   hashed,
		Nickname:   nickname,
		Status:     0,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := s.repo.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

// Login 校验账号密码并签发前台用户 token
func (s *UserService) Login(username, password string) (*UserLoginResult, error) {
	user, err := s.repo.GetByUsername(strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrPasswordIncorrect
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrPasswordIncorrect
	}
	if user.Status != 0 {
		return nil, ErrUserDisabled
	}

	token, err := jwt.GenerateUser(user.ID, user.Username)
	if err != nil {
		return nil, err
	}
	return &UserLoginResult{AccessToken: token, User: user}, nil
}

// GetByID 按 id 查询用户
func (s *UserService) GetByID(id int64) (*model.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

// Page 分页查询用户
func (s *UserService) Page(param model.UserParam, page common.PageParam) ([]model.User, int64, error) {
	return s.repo.Page(param, page)
}

// Add 新增用户
func (s *UserService) Add(form model.UserForm) error {
	if strings.TrimSpace(form.Password) == "" {
		return ErrPasswordRequired
	}
	if strings.TrimSpace(form.Nickname) == "" {
		return ErrNicknameRequired
	}

	exists, err := s.repo.ExistsByUsername(form.Username, 0)
	if err != nil {
		return err
	}
	if exists {
		return ErrUsernameExists
	}

	hashed, err := hashPassword(form.Password)
	if err != nil {
		return err
	}

	now := model.Now()
	return s.repo.Create(&model.User{
		Username:   form.Username,
		Password:   hashed,
		Nickname:   form.Nickname,
		Avatar:     form.Avatar,
		Phone:      form.Phone,
		Email:      form.Email,
		Status:     form.Status,
		CreateTime: now,
		UpdateTime: now,
	})
}

// Update 修改用户
func (s *UserService) Update(form model.UserForm) error {
	user, err := s.repo.GetByID(form.ID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	if strings.TrimSpace(form.Nickname) == "" {
		return ErrNicknameRequired
	}

	exists, err := s.repo.ExistsByUsername(form.Username, form.ID)
	if err != nil {
		return err
	}
	if exists {
		return ErrUsernameExists
	}

	user.Username = form.Username
	user.Nickname = form.Nickname
	user.Avatar = form.Avatar
	user.Phone = form.Phone
	user.Email = form.Email
	user.Status = form.Status
	user.UpdateTime = model.Now()
	return s.repo.Update(user)
}

// Remove 按 id 批量删除用户
func (s *UserService) Remove(ids []int64) error {
	return s.repo.Remove(ids)
}

// UpdateStatus 修改用户状态
func (s *UserService) UpdateStatus(id int64, status int) error {
	if _, err := s.repo.GetByID(id); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	return s.repo.UpdateFields(id, map[string]any{
		"status":      status,
		"update_time": model.Now(),
	})
}

// UpdatePassword 重置用户密码
func (s *UserService) UpdatePassword(id int64, password string) error {
	if strings.TrimSpace(password) == "" {
		return ErrPasswordRequired
	}
	if _, err := s.repo.GetByID(id); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	hashed, err := hashPassword(password)
	if err != nil {
		return err
	}
	return s.repo.UpdateFields(id, map[string]any{
		"password":    hashed,
		"update_time": model.Now(),
	})
}

// ExistsByUsername 判断账号是否已存在
func (s *UserService) ExistsByUsername(username string, excludeID int64) (bool, error) {
	return s.repo.ExistsByUsername(username, excludeID)
}

// hashPassword 生成 bcrypt 哈希
func hashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}
