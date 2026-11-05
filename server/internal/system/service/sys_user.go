package service

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"admin/server/internal/common"
	"admin/server/internal/system/repository"
	"admin/server/model"
	"admin/server/pkg/jwt"
)

// 密码哈希使用的 bcrypt 强度
const bcryptCost = 10

// adminUsername 是内置管理员账号，与初始化数据时的 admin 账号保持一致
const adminUsername = "admin"

// 常见的业务错误
var (
	// ErrPasswordIncorrect 是密码错误
	ErrPasswordIncorrect = errors.New("密码错误")
	// ErrUserDisabled 是账号被冻结
	ErrUserDisabled = errors.New("账号已被冻结")
	// ErrUsernameExists 是账号已存在
	ErrUsernameExists = errors.New("账号已经存在")
	// ErrPasswordRequired 是密码不能为空
	ErrPasswordRequired = errors.New("密码不能为空")
	// ErrUserNotFound 是用户不存在
	ErrUserNotFound = errors.New("用户不存在")
	// ErrUserProtectedRemove 是受保护的管理员账号不允许删除
	ErrUserProtectedRemove = errors.New("受保护的管理员账号不允许删除")
	// ErrUserProtectedDisable 是内置管理员账号不允许冻结
	ErrUserProtectedDisable = errors.New("内置管理员账号不允许冻结")
	// ErrUserProtectedPassword 是受保护的管理员账号不允许重置密码
	ErrUserProtectedPassword = errors.New("受保护的管理员账号不允许重置密码")
	// ErrUserProtectedUpdate 是受保护的管理员账号不允许修改账号、冻结或去掉超级管理员角色
	ErrUserProtectedUpdate = errors.New("受保护的管理员账号不允许修改账号、冻结或去掉超级管理员角色")
)

// SysUserLoginResult 是登录成功返回的数据
type SysUserLoginResult struct {
	// token
	AccessToken string `json:"access_token"`
	// 用户信息
	User *model.SysUser `json:"user"`
}

// SysUserService 是系统用户的业务处理
type SysUserService struct {
	repo *repository.SysUserRepository
	rbac *repository.SysRbacRepository
}

// NewSysUserService 创建系统用户的业务处理
func NewSysUserService() *SysUserService {
	return &SysUserService{
		repo: repository.NewSysUserRepository(),
		rbac: repository.NewSysRbacRepository(),
	}
}

// Login 校验账号密码并签发 token
func (s *SysUserService) Login(username, password string) (*SysUserLoginResult, error) {
	user, err := s.repo.GetByUsername(username)
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

	token, err := jwt.Generate(user.UserID, user.Username)
	if err != nil {
		return nil, err
	}

	s.fillRbac(user)
	return &SysUserLoginResult{AccessToken: token, User: user}, nil
}

// GetByID 按 id 查询用户，并填充角色与权限
func (s *SysUserService) GetByID(userID int64) (*model.SysUser, error) {
	user, err := s.repo.GetByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	s.fillRbac(user)
	return user, nil
}

// Page 分页查询用户
func (s *SysUserService) Page(param model.SysUserParam, page common.PageParam) ([]model.SysUser, int64, error) {
	list, count, err := s.repo.Page(param, page)
	if err != nil {
		return nil, 0, err
	}
	for i := range list {
		s.fillRbac(&list[i])
	}
	return list, count, nil
}

// List 查询用户列表
func (s *SysUserService) List(param model.SysUserParam) ([]model.SysUser, error) {
	list, err := s.repo.List(param)
	if err != nil {
		return nil, err
	}
	for i := range list {
		s.fillRbac(&list[i])
	}
	return list, nil
}

// Add 新增用户
func (s *SysUserService) Add(form model.SysUserForm) error {
	if strings.TrimSpace(form.Password) == "" {
		return ErrPasswordRequired
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
	user := &model.SysUser{
		Username:     form.Username,
		Password:     hashed,
		Nickname:     form.Nickname,
		Avatar:       form.Avatar,
		Sex:          form.Sex,
		Phone:        form.Phone,
		Email:        form.Email,
		Birthday:     form.Birthday,
		Introduction: form.Introduction,
		Status:       form.Status,
		CreateTime:   now,
		UpdateTime:   now,
	}
	if err := s.repo.Create(user); err != nil {
		return err
	}
	return s.saveRoles(user.UserID, form.Roles)
}

// Update 修改用户
func (s *SysUserService) Update(form model.SysUserForm) error {
	user, err := s.repo.GetByID(form.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	exists, err := s.repo.ExistsByUsername(form.Username, form.UserID)
	if err != nil {
		return err
	}
	if exists {
		return ErrUsernameExists
	}

	// 受保护的管理员账号不允许冻结、改账号名或去掉超级管理员角色
	if s.isProtectedUser(user) {
		keepsAdmin, err := s.keepsAdminRole(form.Roles)
		if err != nil {
			return err
		}
		if form.Status != 0 || form.Username != user.Username || !keepsAdmin {
			return ErrUserProtectedUpdate
		}
	}

	user.Username = form.Username
	user.Nickname = form.Nickname
	user.Avatar = form.Avatar
	user.Sex = form.Sex
	user.Phone = form.Phone
	user.Email = form.Email
	user.Birthday = form.Birthday
	user.Introduction = form.Introduction
	user.Status = form.Status
	user.UpdateTime = model.Now()
	if err := s.repo.Update(user); err != nil {
		return err
	}
	return s.saveRoles(user.UserID, form.Roles)
}

// Remove 按 id 批量删除用户，批量中含受保护的管理员账号时整单拒绝
func (s *SysUserService) Remove(userIDs []int64) error {
	users, err := s.repo.ListByIDs(userIDs)
	if err != nil {
		return err
	}
	for i := range users {
		if s.isProtectedUser(&users[i]) {
			return ErrUserProtectedRemove
		}
	}
	return s.repo.Remove(userIDs)
}

// UpdateStatus 修改用户状态，只保护内置管理员账号，其他超级管理员账号允许冻结和解冻
func (s *SysUserService) UpdateStatus(userID int64, status int) error {
	user, err := s.repo.GetByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	if status != 0 && user.Username == adminUsername {
		return ErrUserProtectedDisable
	}
	return s.repo.UpdateFields(userID, map[string]any{
		"status":      status,
		"update_time": model.Now(),
	})
}

// UpdatePassword 重置用户密码，受保护的管理员账号不允许重置
func (s *SysUserService) UpdatePassword(userID int64, password string) error {
	if strings.TrimSpace(password) == "" {
		return ErrPasswordRequired
	}
	user, err := s.repo.GetByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	if s.isProtectedUser(user) {
		return ErrUserProtectedPassword
	}

	hashed, err := hashPassword(password)
	if err != nil {
		return err
	}
	return s.repo.UpdateFields(userID, map[string]any{
		"password":    hashed,
		"update_time": model.Now(),
	})
}

// ExistsByUsername 判断账号是否已存在
func (s *SysUserService) ExistsByUsername(username string, excludeID int64) (bool, error) {
	return s.repo.ExistsByUsername(username, excludeID)
}

// UpdateInfo 修改当前登录用户的个人信息
func (s *SysUserService) UpdateInfo(userID int64, form model.SysUserInfoForm) (*model.SysUser, error) {
	user, err := s.repo.GetByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	user.Nickname = form.Nickname
	user.Sex = form.Sex
	user.Phone = form.Phone
	user.Email = form.Email
	user.Introduction = form.Introduction
	user.UpdateTime = model.Now()
	if err := s.repo.Update(user); err != nil {
		return nil, err
	}

	s.fillRbac(user)
	return user, nil
}

// UpdateOwnPassword 修改当前登录用户的密码，需要校验原密码
func (s *SysUserService) UpdateOwnPassword(userID int64, oldPassword, password string) error {
	if strings.TrimSpace(password) == "" {
		return ErrPasswordRequired
	}

	user, err := s.repo.GetByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return ErrPasswordIncorrect
	}

	hashed, err := hashPassword(password)
	if err != nil {
		return err
	}
	return s.repo.UpdateFields(userID, map[string]any{
		"password":    hashed,
		"update_time": model.Now(),
	})
}

// isProtectedUser 判断账号是否为受保护的管理员账号，
// 内置 admin 账号与拥有超级管理员角色的账号都不允许删除和重置密码
func (s *SysUserService) isProtectedUser(user *model.SysUser) bool {
	if user.Username == adminUsername {
		return true
	}
	roles, err := s.rbac.ListRolesByUserID(user.UserID)
	if err != nil {
		return false
	}
	return hasAdminRole(roles)
}

// hasAdminRole 判断角色列表中是否有超级管理员角色
func hasAdminRole(roles []model.Role) bool {
	for _, role := range roles {
		if role.RoleCode == model.RoleCodeAdmin {
			return true
		}
	}
	return false
}

// keepsAdminRole 判断提交的角色中是否仍保留超级管理员角色，
// 前端角色选择只回传 roleId，角色标识一律以数据库中的角色为准
func (s *SysUserService) keepsAdminRole(roles []model.Role) (bool, error) {
	for _, role := range roles {
		if role.RoleID == 0 {
			continue
		}
		dbRole, err := s.rbac.GetRoleByID(role.RoleID)
		if err != nil {
			if errors.Is(err, repository.ErrRoleNotFound) {
				continue
			}
			return false, err
		}
		if dbRole.RoleCode == model.RoleCodeAdmin {
			return true, nil
		}
	}
	return false, nil
}

// hashPassword 生成密码哈希
func hashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// fillRbac 填充用户的角色与菜单权限
func (s *SysUserService) fillRbac(user *model.SysUser) {
	user.FillExtra()
	roles, err := s.rbac.ListRolesByUserID(user.UserID)
	if err == nil {
		user.Roles = roles
	}
	menus, err := s.rbac.ListMenusByUserID(user.UserID)
	if err == nil {
		user.Authorities = menus
	}
}

// saveRoles 按表单保存用户的角色
func (s *SysUserService) saveRoles(userID int64, roles []model.Role) error {
	roleIDs := make([]int64, 0, len(roles))
	for _, role := range roles {
		if role.RoleID > 0 {
			roleIDs = append(roleIDs, role.RoleID)
		}
	}
	return s.rbac.ReplaceUserRoles(userID, roleIDs)
}

// ListByIDs 按 id 批量查询用户
func (s *SysUserService) ListByIDs(userIDs []int64) ([]model.SysUser, error) {
	return s.repo.ListByIDs(userIDs)
}
