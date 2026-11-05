package repository

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"admin/server/internal/common"
	"admin/server/model"
	"admin/server/pkg/db"
)

// ErrUserNotFound 是用户不存在时返回的错误
var ErrUserNotFound = errors.New("用户不存在")

// SysUserRepository 是系统用户的数据访问
type SysUserRepository struct{}

// NewSysUserRepository 创建系统用户的数据访问
func NewSysUserRepository() *SysUserRepository {
	return &SysUserRepository{}
}

// db 返回带 sys_user 条件的查询
func (r *SysUserRepository) db() *gorm.DB {
	return db.DB.Model(&model.SysUser{}).Where("deleted = 0")
}

// GetByID 按 id 查询用户
func (r *SysUserRepository) GetByID(userID int64) (*model.SysUser, error) {
	user := new(model.SysUser)
	err := r.db().Where("user_id = ?", userID).First(user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetByUsername 按账号查询用户
func (r *SysUserRepository) GetByUsername(username string) (*model.SysUser, error) {
	user := new(model.SysUser)
	err := r.db().Where("username = ?", username).First(user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// Page 分页查询用户
func (r *SysUserRepository) Page(param model.SysUserParam, page common.PageParam) ([]model.SysUser, int64, error) {
	query := r.db()
	if param.Username != "" {
		query = query.Where("username like ?", "%"+param.Username+"%")
	}
	if param.Nickname != "" {
		query = query.Where("nickname like ?", "%"+param.Nickname+"%")
	}
	if param.Sex != "" {
		query = query.Where("sex = ?", param.Sex)
	}
	if param.Phone != "" {
		query = query.Where("phone like ?", "%"+param.Phone+"%")
	}
	if param.Status != nil {
		query = query.Where("status = ?", *param.Status)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.SysUser
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClauses(page) {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// List 查询用户列表
func (r *SysUserRepository) List(param model.SysUserParam) ([]model.SysUser, error) {
	var list []model.SysUser
	query := r.db()
	if err := query.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListByIDs 按 id 批量查询用户
func (r *SysUserRepository) ListByIDs(userIDs []int64) ([]model.SysUser, error) {
	if len(userIDs) == 0 {
		return []model.SysUser{}, nil
	}
	var list []model.SysUser
	err := r.db().Where("user_id in ?", userIDs).Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// Create 新增用户
func (r *SysUserRepository) Create(user *model.SysUser) error {
	return db.DB.Create(user).Error
}

// Update 修改用户
func (r *SysUserRepository) Update(user *model.SysUser) error {
	return db.DB.Save(user).Error
}

// UpdateFields 按 id 修改指定字段
func (r *SysUserRepository) UpdateFields(userID int64, fields map[string]any) error {
	return db.DB.Model(&model.SysUser{}).Where("user_id = ?", userID).Updates(fields).Error
}

// Remove 按 id 批量软删除用户
func (r *SysUserRepository) Remove(userIDs []int64) error {
	return db.DB.Model(&model.SysUser{}).
		Where("user_id in ?", userIDs).
		Updates(map[string]any{"deleted": 1}).Error
}

// ExistsByUsername 判断账号是否已存在，id 不为 0 时排除自身
func (r *SysUserRepository) ExistsByUsername(username string, excludeID int64) (bool, error) {
	query := r.db().Where("username = ?", username)
	if excludeID != 0 {
		query = query.Where("user_id != ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// orderClauses 把前端传来的排序参数转换为 gorm 的排序语句，
// 多字段排序时前端会传 sort=a,b 与 order=asc,desc 这样的组合
func orderClauses(page common.PageParam) []string {
	return orderClausesWith(page, "user_id asc")
}

// orderClausesWith 与 orderClauses 相同，但可以指定默认排序
func orderClausesWith(page common.PageParam, defaultOrder string) []string {
	if page.Sort == "" {
		return []string{defaultOrder}
	}

	fields := strings.Split(page.Sort, ",")
	orders := strings.Split(page.Order, ",")
	clauses := make([]string, 0, len(fields))
	for i, field := range fields {
		column := columnName(strings.TrimSpace(field))
		if column == "" {
			continue
		}

		// order 数量不足时沿用一个 order，保持与单字段排序一致的表现
		order := orders[len(orders)-1]
		if i < len(orders) {
			order = orders[i]
		}
		if strings.EqualFold(strings.TrimSpace(order), "desc") {
			order = "desc"
		} else {
			order = "asc"
		}
		clauses = append(clauses, column+" "+order)
	}
	if len(clauses) == 0 {
		return []string{defaultOrder}
	}
	return clauses
}

// columnName 把前端字段名转换为数据库列名，非白名单字段返回空
func columnName(field string) string {
	switch field {
	case "userId":
		return "user_id"
	case "roleId":
		return "role_id"
	case "menuId":
		return "menu_id"
	case "dictId":
		return "dict_id"
	case "dictDataId":
		return "dict_data_id"
	case "sortNumber":
		return "sort_number"
	case "loginType":
		return "login_type"
	case "spendTime":
		return "spend_time"
	case "createTime":
		return "create_time"
	case "updateTime":
		return "update_time"
	case "username", "nickname", "sex", "status", "ip", "name", "path",
		"title", "module", "description", "url", "requestMethod", "length":
		return field
	default:
		return ""
	}
}
