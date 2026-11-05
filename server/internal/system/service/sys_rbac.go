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
	// ErrRoleCodeExists 是角色标识已存在
	ErrRoleCodeExists = errors.New("角色标识已经存在")
	// ErrRoleCodeRequired 是角色标识不能为空
	ErrRoleCodeRequired = errors.New("角色标识不能为空")
	// ErrRoleNameRequired 是角色名称不能为空
	ErrRoleNameRequired = errors.New("角色名称不能为空")
	// ErrMenuTitleRequired 是菜单名称不能为空
	ErrMenuTitleRequired = errors.New("菜单名称不能为空")
	// ErrMenuHasChildren 是菜单存在子级不能删除
	ErrMenuHasChildren = errors.New("存在子菜单，不能删除")
	// ErrMenuParentInvalid 是上级菜单不正确
	ErrMenuParentInvalid = errors.New("上级菜单不能是自己或自己的子菜单")
	// ErrAdminRoleCannotRemove 是超级管理员角色不能删除
	ErrAdminRoleCannotRemove = errors.New("超级管理员角色不能删除")
	// ErrRoleNotFound 是角色不存在
	ErrRoleNotFound = errors.New("角色不存在")
	// ErrMenuNotFound 是菜单不存在
	ErrMenuNotFound = errors.New("菜单不存在")
)

// SysRbacService 是角色、菜单与权限的业务处理
type SysRbacService struct {
	repo *repository.SysRbacRepository
}

// NewSysRbacService 创建角色、菜单与权限的业务处理
func NewSysRbacService() *SysRbacService {
	return &SysRbacService{repo: repository.NewSysRbacRepository()}
}

// ==== 角色 ====

// PageRoles 分页查询角色
func (s *SysRbacService) PageRoles(param model.RoleParam, page common.PageParam) ([]model.Role, int64, error) {
	return s.repo.PageRoles(param, page)
}

// ListRoles 查询角色列表
func (s *SysRbacService) ListRoles(param model.RoleParam) ([]model.Role, error) {
	return s.repo.ListRoles(param)
}

// AddRole 新增角色
func (s *SysRbacService) AddRole(form model.RoleForm) error {
	form.RoleCode = strings.TrimSpace(form.RoleCode)
	form.RoleName = strings.TrimSpace(form.RoleName)
	if form.RoleCode == "" {
		return ErrRoleCodeRequired
	}
	if form.RoleName == "" {
		return ErrRoleNameRequired
	}

	exists, err := s.repo.RoleCodeExists(form.RoleCode, 0)
	if err != nil {
		return err
	}
	if exists {
		return ErrRoleCodeExists
	}

	now := model.Now()
	return s.repo.CreateRole(&model.Role{
		RoleCode:   form.RoleCode,
		RoleName:   form.RoleName,
		Comments:   form.Comments,
		CreateTime: now,
		UpdateTime: now,
	})
}

// UpdateRole 修改角色
func (s *SysRbacService) UpdateRole(form model.RoleForm) error {
	role, err := s.repo.GetRoleByID(form.RoleID)
	if err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return ErrRoleNotFound
		}
		return err
	}

	form.RoleCode = strings.TrimSpace(form.RoleCode)
	form.RoleName = strings.TrimSpace(form.RoleName)
	if form.RoleCode == "" {
		return ErrRoleCodeRequired
	}
	if form.RoleName == "" {
		return ErrRoleNameRequired
	}

	exists, err := s.repo.RoleCodeExists(form.RoleCode, form.RoleID)
	if err != nil {
		return err
	}
	if exists {
		return ErrRoleCodeExists
	}

	role.RoleCode = form.RoleCode
	role.RoleName = form.RoleName
	role.Comments = form.Comments
	role.UpdateTime = model.Now()
	return s.repo.UpdateRole(role)
}

// RemoveRoles 按 id 批量删除角色
func (s *SysRbacService) RemoveRoles(roleIDs []int64) error {
	for _, roleID := range roleIDs {
		role, err := s.repo.GetRoleByID(roleID)
		if err != nil {
			if errors.Is(err, repository.ErrRoleNotFound) {
				return ErrRoleNotFound
			}
			return err
		}
		if role.RoleCode == model.RoleCodeAdmin {
			return ErrAdminRoleCannotRemove
		}
	}
	return s.repo.RemoveRoles(roleIDs)
}

// ListRoleMenus 查询角色已授权的菜单树
func (s *SysRbacService) ListRoleMenus(roleID int64) ([]model.Menu, error) {
	if _, err := s.repo.GetRoleByID(roleID); err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}

	menus, err := s.repo.ListMenus(model.MenuParam{})
	if err != nil {
		return nil, err
	}
	granted, err := s.repo.ListRoleMenus(roleID)
	if err != nil {
		return nil, err
	}

	grantedSet := make(map[int64]bool, len(granted))
	for _, menuID := range granted {
		grantedSet[menuID] = true
	}
	for i := range menus {
		checked := grantedSet[menus[i].MenuID]
		menus[i].Checked = &checked
	}
	// 查询全部菜单后再转为树，避免授权菜单的父级缺失
	full, err := s.allMenus()
	if err != nil {
		return nil, err
	}
	checkedMap := make(map[int64]bool, len(menus))
	for i := range menus {
		checkedMap[menus[i].MenuID] = *menus[i].Checked
	}
	for i := range full {
		checked := checkedMap[full[i].MenuID]
		full[i].Checked = &checked
	}
	return BuildMenuTree(full), nil
}

// UpdateRoleMenus 覆盖角色的菜单授权
func (s *SysRbacService) UpdateRoleMenus(roleID int64, menuIDs []int64) error {
	if _, err := s.repo.GetRoleByID(roleID); err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return ErrRoleNotFound
		}
		return err
	}
	return s.repo.ReplaceRoleMenus(roleID, menuIDs)
}

// ==== 菜单 ====

// PageMenus 分页查询菜单
func (s *SysRbacService) PageMenus(param model.MenuParam, page common.PageParam) ([]model.Menu, int64, error) {
	return s.repo.PageMenus(param, page)
}

// ListMenus 查询菜单树
func (s *SysRbacService) ListMenus(param model.MenuParam) ([]model.Menu, error) {
	list, err := s.repo.ListMenus(param)
	if err != nil {
		return nil, err
	}
	return BuildMenuTree(list), nil
}

// AddMenu 新增菜单
func (s *SysRbacService) AddMenu(form model.MenuForm) error {
	form.Title = strings.TrimSpace(form.Title)
	if form.Title == "" {
		return ErrMenuTitleRequired
	}

	now := model.Now()
	return s.repo.CreateMenu(&model.Menu{
		ParentID:   form.ParentID,
		Title:      form.Title,
		Path:       form.Path,
		Component:  form.Component,
		MenuType:   form.MenuType,
		SortNumber: form.SortNumber,
		Authority:  form.Authority,
		Icon:       form.Icon,
		Hide:       form.Hide,
		Meta:       form.Meta,
		CreateTime: now,
		UpdateTime: now,
	})
}

// UpdateMenu 修改菜单
func (s *SysRbacService) UpdateMenu(form model.MenuForm) error {
	menu, err := s.repo.GetMenuByID(form.MenuID)
	if err != nil {
		if errors.Is(err, repository.ErrMenuNotFound) {
			return ErrMenuNotFound
		}
		return err
	}

	form.Title = strings.TrimSpace(form.Title)
	if form.Title == "" {
		return ErrMenuTitleRequired
	}
	if form.ParentID != 0 && s.isDescendant(form.ParentID, form.MenuID) {
		return ErrMenuParentInvalid
	}

	menu.ParentID = form.ParentID
	menu.Title = form.Title
	menu.Path = form.Path
	menu.Component = form.Component
	menu.MenuType = form.MenuType
	menu.SortNumber = form.SortNumber
	menu.Authority = form.Authority
	menu.Icon = form.Icon
	menu.Hide = form.Hide
	menu.Meta = form.Meta
	menu.UpdateTime = model.Now()
	return s.repo.UpdateMenu(menu)
}

// isDescendant 判断 parentID 是否是 menuID 的子级，避免把父级挂到自己的子级下
func (s *SysRbacService) isDescendant(parentID, menuID int64) bool {
	currentID := parentID
	for currentID != 0 {
		if currentID == menuID {
			return true
		}
		menu, err := s.repo.GetMenuByID(currentID)
		if err != nil {
			return false
		}
		currentID = menu.ParentID
	}
	return false
}

// RemoveMenu 按 id 删除菜单
func (s *SysRbacService) RemoveMenu(menuID int64) error {
	if _, err := s.repo.GetMenuByID(menuID); err != nil {
		if errors.Is(err, repository.ErrMenuNotFound) {
			return ErrMenuNotFound
		}
		return err
	}

	count, err := s.repo.CountChildren(menuID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrMenuHasChildren
	}
	return s.repo.RemoveMenu([]int64{menuID})
}

// allMenus 查询全部未删除菜单（含按钮）
func (s *SysRbacService) allMenus() ([]model.Menu, error) {
	return s.repo.ListMenus(model.MenuParam{})
}

// ==== 用户角色与权限 ====

// UserRoles 查询用户的角色
func (s *SysRbacService) UserRoles(userID int64) ([]model.Role, error) {
	return s.repo.ListRolesByUserID(userID)
}

// UserAuthorities 查询用户的菜单权限
func (s *SysRbacService) UserAuthorities(userID int64) ([]model.Menu, error) {
	return s.repo.ListMenusByUserID(userID)
}

// SetUserRoles 覆盖用户的角色
func (s *SysRbacService) SetUserRoles(userID int64, roleIDs []int64) error {
	return s.repo.ReplaceUserRoles(userID, roleIDs)
}

// LoadAuthorities 查询用户是否为超级管理员以及其拥有的权限标识集合，
// 供权限中间件校验接口权限使用；超级管理员不需要再查询菜单
func (s *SysRbacService) LoadAuthorities(userID int64) (map[string]struct{}, bool, error) {
	roles, err := s.repo.ListRolesByUserID(userID)
	if err != nil {
		return nil, false, err
	}
	if hasAdminRole(roles) {
		return nil, true, nil
	}

	// 菜单中同时包含菜单与按钮上的权限标识
	menus, err := s.repo.ListMenusByUserID(userID)
	if err != nil {
		return nil, false, err
	}
	codes := make(map[string]struct{}, len(menus))
	for _, menu := range menus {
		if menu.Authority != "" {
			codes[menu.Authority] = struct{}{}
		}
	}
	return codes, false, nil
}

// BuildMenuTree 把平铺的菜单列表构建成树
func BuildMenuTree(list []model.Menu) []model.Menu {
	if len(list) == 0 {
		return []model.Menu{}
	}

	// 记录列表中存在的菜单id，父级不在列表中的节点作为顶级节点
	exists := make(map[int64]bool, len(list))
	for _, menu := range list {
		exists[menu.MenuID] = true
	}

	childrenMap := make(map[int64][]model.Menu, len(list))
	roots := make([]model.Menu, 0, len(list))
	for _, menu := range list {
		menu.Children = nil
		if menu.ParentID == 0 || !exists[menu.ParentID] {
			roots = append(roots, menu)
			continue
		}
		childrenMap[menu.ParentID] = append(childrenMap[menu.ParentID], menu)
	}

	var attach func(menus []model.Menu) []model.Menu
	attach = func(menus []model.Menu) []model.Menu {
		for i := range menus {
			children := childrenMap[menus[i].MenuID]
			if len(children) > 0 {
				menus[i].Children = attach(children)
			}
		}
		return menus
	}
	return attach(roots)
}
