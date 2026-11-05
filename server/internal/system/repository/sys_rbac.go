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
	// ErrRoleNotFound 是角色不存在
	ErrRoleNotFound = errors.New("角色不存在")
	// ErrMenuNotFound 是菜单不存在
	ErrMenuNotFound = errors.New("菜单不存在")
)

// SysRbacRepository 是角色、菜单与权限的数据访问
type SysRbacRepository struct{}

// NewSysRbacRepository 创建角色、菜单与权限的数据访问
func NewSysRbacRepository() *SysRbacRepository {
	return &SysRbacRepository{}
}

// ==== 角色 ====

// roleDB 返回带 sys_role 条件的查询
func (r *SysRbacRepository) roleDB() *gorm.DB {
	return db.DB.Model(&model.Role{}).Where("deleted = 0")
}

// GetRoleByID 按 id 查询角色
func (r *SysRbacRepository) GetRoleByID(roleID int64) (*model.Role, error) {
	role := new(model.Role)
	err := r.roleDB().Where("role_id = ?", roleID).First(role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoleNotFound
	}
	if err != nil {
		return nil, err
	}
	return role, nil
}

// PageRoles 分页查询角色
func (r *SysRbacRepository) PageRoles(param model.RoleParam, page common.PageParam) ([]model.Role, int64, error) {
	query := r.roleDB()
	if param.RoleName != "" {
		query = query.Where("role_name like ?", "%"+param.RoleName+"%")
	}
	if param.RoleCode != "" {
		query = query.Where("role_code like ?", "%"+param.RoleCode+"%")
	}
	if param.Comments != "" {
		query = query.Where("comments like ?", "%"+param.Comments+"%")
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.Role
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "role_id asc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// ListRoles 查询角色列表
func (r *SysRbacRepository) ListRoles(param model.RoleParam) ([]model.Role, error) {
	var list []model.Role
	query := r.roleDB()
	if param.RoleName != "" {
		query = query.Where("role_name like ?", "%"+param.RoleName+"%")
	}
	if param.RoleCode != "" {
		query = query.Where("role_code like ?", "%"+param.RoleCode+"%")
	}
	if err := query.Order("role_id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// RoleCodeExists 判断角色标识是否已存在
func (r *SysRbacRepository) RoleCodeExists(roleCode string, excludeID int64) (bool, error) {
	query := r.roleDB().Where("role_code = ?", roleCode)
	if excludeID > 0 {
		query = query.Where("role_id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateRole 新增角色
func (r *SysRbacRepository) CreateRole(role *model.Role) error {
	return db.DB.Create(role).Error
}

// UpdateRole 修改角色
func (r *SysRbacRepository) UpdateRole(role *model.Role) error {
	return db.DB.Save(role).Error
}

// RemoveRoles 按 id 批量删除角色
func (r *SysRbacRepository) RemoveRoles(roleIDs []int64) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Role{}).Where("role_id in ?", roleIDs).
			Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id in ?", roleIDs).Delete(&model.RoleMenu{}).Error; err != nil {
			return err
		}
		return tx.Where("role_id in ?", roleIDs).Delete(&model.UserRole{}).Error
	})
}

// ==== 菜单 ====

// menuDB 返回带 sys_menu 条件的查询
func (r *SysRbacRepository) menuDB() *gorm.DB {
	return db.DB.Model(&model.Menu{}).Where("deleted = 0")
}

// GetMenuByID 按 id 查询菜单
func (r *SysRbacRepository) GetMenuByID(menuID int64) (*model.Menu, error) {
	menu := new(model.Menu)
	err := r.menuDB().Where("menu_id = ?", menuID).First(menu).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMenuNotFound
	}
	if err != nil {
		return nil, err
	}
	return menu, nil
}

// PageMenus 分页查询菜单
func (r *SysRbacRepository) PageMenus(param model.MenuParam, page common.PageParam) ([]model.Menu, int64, error) {
	query := r.menuDB()
	if param.Title != "" {
		query = query.Where("title like ?", "%"+param.Title+"%")
	}
	if param.Path != "" {
		query = query.Where("path like ?", "%"+param.Path+"%")
	}
	if param.Authority != "" {
		query = query.Where("authority like ?", "%"+param.Authority+"%")
	}
	if param.ParentID != nil {
		query = query.Where("parent_id = ?", *param.ParentID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.Menu
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "sort_number asc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// ListMenus 查询菜单列表
func (r *SysRbacRepository) ListMenus(param model.MenuParam) ([]model.Menu, error) {
	var list []model.Menu
	query := r.menuDB()
	if param.Title != "" {
		query = query.Where("title like ?", "%"+param.Title+"%")
	}
	if param.Path != "" {
		query = query.Where("path like ?", "%"+param.Path+"%")
	}
	if param.Authority != "" {
		query = query.Where("authority like ?", "%"+param.Authority+"%")
	}
	if param.ParentID != nil {
		query = query.Where("parent_id = ?", *param.ParentID)
	}
	err := query.Order("sort_number asc").Order("menu_id asc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// CreateMenu 新增菜单
func (r *SysRbacRepository) CreateMenu(menu *model.Menu) error {
	return db.DB.Create(menu).Error
}

// UpdateMenu 修改菜单
func (r *SysRbacRepository) UpdateMenu(menu *model.Menu) error {
	return db.DB.Save(menu).Error
}

// RemoveMenu 按 id 删除菜单及其子菜单
func (r *SysRbacRepository) RemoveMenu(menuIDs []int64) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Menu{}).Where("menu_id in ?", menuIDs).
			Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error; err != nil {
			return err
		}
		return tx.Where("menu_id in ?", menuIDs).Delete(&model.RoleMenu{}).Error
	})
}

// CountChildren 统计子菜单数量
func (r *SysRbacRepository) CountChildren(menuID int64) (int64, error) {
	var count int64
	err := r.menuDB().Where("parent_id = ?", menuID).Count(&count).Error
	return count, err
}

// ==== 角色菜单 ====

// ListRoleMenus 查询角色已授权的菜单id
func (r *SysRbacRepository) ListRoleMenus(roleID int64) ([]int64, error) {
	var menuIDs []int64
	err := db.DB.Model(&model.RoleMenu{}).Where("role_id = ?", roleID).
		Pluck("menu_id", &menuIDs).Error
	if err != nil {
		return nil, err
	}
	return menuIDs, nil
}

// ReplaceRoleMenus 覆盖角色的菜单授权
func (r *SysRbacRepository) ReplaceRoleMenus(roleID int64, menuIDs []int64) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RoleMenu{}).Error; err != nil {
			return err
		}
		if len(menuIDs) == 0 {
			return nil
		}
		list := make([]model.RoleMenu, 0, len(menuIDs))
		for _, menuID := range menuIDs {
			list = append(list, model.RoleMenu{RoleID: roleID, MenuID: menuID})
		}
		return tx.Create(&list).Error
	})
}

// ==== 用户角色 ====

// ListRoleIDsByUserID 查询用户拥有的角色id
func (r *SysRbacRepository) ListRoleIDsByUserID(userID int64) ([]int64, error) {
	var roleIDs []int64
	err := db.DB.Model(&model.UserRole{}).Where("user_id = ?", userID).
		Pluck("role_id", &roleIDs).Error
	if err != nil {
		return nil, err
	}
	return roleIDs, nil
}

// ListRolesByUserID 查询用户拥有的角色
func (r *SysRbacRepository) ListRolesByUserID(userID int64) ([]model.Role, error) {
	var list []model.Role
	err := db.DB.Model(&model.Role{}).
		Joins("inner join sys_user_role ur on ur.role_id = sys_role.role_id").
		Where("ur.user_id = ? and sys_role.deleted = 0", userID).
		Order("sys_role.role_id asc").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	for i := range list {
		uid := userID
		list[i].UserID = &uid
	}
	return list, nil
}

// ReplaceUserRoles 覆盖用户的角色
func (r *SysRbacRepository) ReplaceUserRoles(userID int64, roleIDs []int64) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		if len(roleIDs) == 0 {
			return nil
		}
		list := make([]model.UserRole, 0, len(roleIDs))
		for _, roleID := range roleIDs {
			list = append(list, model.UserRole{UserID: userID, RoleID: roleID})
		}
		return tx.Create(&list).Error
	})
}

// ListMenusByUserID 查询用户拥有的菜单，管理员角色返回全部菜单
func (r *SysRbacRepository) ListMenusByUserID(userID int64) ([]model.Menu, error) {
	roles, err := r.ListRolesByUserID(userID)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return []model.Menu{}, nil
	}

	isAdmin := false
	for _, role := range roles {
		if role.RoleCode == model.RoleCodeAdmin {
			isAdmin = true
			break
		}
	}

	var list []model.Menu
	query := r.menuDB()
	if !isAdmin {
		roleIDs := make([]int64, 0, len(roles))
		for _, role := range roles {
			roleIDs = append(roleIDs, role.RoleID)
		}
		query = query.Where("menu_id in (?)",
			db.DB.Model(&model.RoleMenu{}).Select("menu_id").Where("role_id in ?", roleIDs),
		)
	}
	if err := query.Order("sort_number asc").Order("menu_id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
