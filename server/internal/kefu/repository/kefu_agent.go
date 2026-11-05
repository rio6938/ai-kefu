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
	// ErrAgentNotFound 是客服档案不存在
	ErrAgentNotFound = errors.New("客服档案不存在")
	// ErrAgentUserNotFound 是关联的管理员账号不存在
	ErrAgentUserNotFound = errors.New("关联的管理员账号不存在")
)

// KefuAgentRepository 是客服档案的数据访问
type KefuAgentRepository struct{}

// NewKefuAgentRepository 创建客服档案的数据访问
func NewKefuAgentRepository() *KefuAgentRepository {
	return &KefuAgentRepository{}
}

// db 返回带 kefu_agent 条件的查询
func (r *KefuAgentRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuAgent{}).Where("deleted = 0")
}

// GetByID 按 id 查询客服档案
func (r *KefuAgentRepository) GetByID(agentID int64) (*model.KefuAgent, error) {
	agent := new(model.KefuAgent)
	err := r.db().Where("agent_id = ?", agentID).First(agent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAgentNotFound
	}
	if err != nil {
		return nil, err
	}
	return agent, nil
}

// GetByUserID 按管理员用户id查询客服档案
func (r *KefuAgentRepository) GetByUserID(userID int64) (*model.KefuAgent, error) {
	agent := new(model.KefuAgent)
	err := r.db().Where("user_id = ?", userID).First(agent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAgentNotFound
	}
	if err != nil {
		return nil, err
	}
	return agent, nil
}

// ListByIDs 按 id 批量查询客服档案，会话列表回填客服信息时使用
func (r *KefuAgentRepository) ListByIDs(agentIDs []int64) ([]model.KefuAgent, error) {
	if len(agentIDs) == 0 {
		return nil, nil
	}
	var list []model.KefuAgent
	if err := r.db().Where("agent_id in ?", agentIDs).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// HasRoleCode 判断管理员用户是否拥有指定的角色，
// 客服模块据此区分普通客服与客服主管（主管可以查看全部会话）
func (r *KefuAgentRepository) HasRoleCode(userID int64, roleCodes []string) (bool, error) {
	if len(roleCodes) == 0 {
		return false, nil
	}
	var count int64
	err := db.DB.Table("sys_user_role AS ur").
		Joins("join sys_role AS r on r.role_id = ur.role_id").
		Where("ur.user_id = ? and r.deleted = 0 and r.role_code in ?", userID, roleCodes).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListUsersByIDs 查询客服关联的管理员账号，展示客服名称时使用
func (r *KefuAgentRepository) ListUsersByIDs(userIDs []int64) ([]model.SysUser, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var list []model.SysUser
	if err := db.DB.Model(&model.SysUser{}).Where("deleted = 0 and user_id in ?", userIDs).
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListUserIDsByRoleCode 查询拥有指定角色的管理员用户id，
// 差样本提醒需要通知客服主管，这里只取未删除的账号
func (r *KefuAgentRepository) ListUserIDsByRoleCode(roleCode string) ([]int64, error) {
	var userIDs []int64
	err := db.DB.Table("sys_user_role AS ur").
		Select("ur.user_id").
		Joins("join sys_role AS r on r.role_id = ur.role_id").
		Joins("join sys_user AS u on u.user_id = ur.user_id").
		Where("r.deleted = 0 and r.role_code = ? and u.deleted = 0", roleCode).
		Pluck("ur.user_id", &userIDs).Error
	if err != nil {
		return nil, err
	}
	return userIDs, nil
}

// Page 分页查询客服档案
func (r *KefuAgentRepository) Page(
	param model.KefuAgentParam, page common.PageParam,
) ([]model.KefuAgent, int64, error) {
	query := r.db()
	if param.UserID != nil {
		query = query.Where("user_id = ?", *param.UserID)
	}
	if param.AgentNo != "" {
		query = query.Where("agent_no like ?", "%"+param.AgentNo+"%")
	}
	if param.AcceptStatus != nil {
		query = query.Where("accept_status = ?", *param.AcceptStatus)
	}
	if param.Status != nil {
		query = query.Where("status = ?", *param.Status)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.KefuAgent
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "agent_id asc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// ListCandidates 查询可参与分配的客服档案，在线与接待负载在业务层判断；
// 工作状态只允许在线（含小休、离线以外的值）的客服进入候选池
func (r *KefuAgentRepository) ListCandidates() ([]model.KefuAgent, error) {
	var list []model.KefuAgent
	err := r.db().
		Where("status = ? and accept_status = ? and work_status = ?",
			model.KefuAgentStatusNormal, model.KefuAgentAcceptOn,
			model.KefuAgentWorkOnline).
		Order("agent_id asc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// List 查询启用中的客服档案
func (r *KefuAgentRepository) List() ([]model.KefuAgent, error) {
	query := r.db().Where("status = ?", model.KefuAgentStatusNormal)
	var list []model.KefuAgent
	if err := query.Order("agent_id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Create 新增客服档案
func (r *KefuAgentRepository) Create(agent *model.KefuAgent) error {
	return db.DB.Create(agent).Error
}

// Update 修改客服档案
func (r *KefuAgentRepository) Update(agent *model.KefuAgent) error {
	return db.DB.Save(agent).Error
}

// UpdateFields 按字段更新客服档案，只更新传入的字段
func (r *KefuAgentRepository) UpdateFields(agentID int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["update_time"] = model.Now()
	return db.DB.Model(&model.KefuAgent{}).Where("agent_id = ?", agentID).
		Updates(fields).Error
}

// Remove 软删除客服档案
func (r *KefuAgentRepository) Remove(agentID int64) error {
	return db.DB.Model(&model.KefuAgent{}).Where("agent_id = ?", agentID).
		Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error
}

// ExistsByUserID 判断管理员用户是否已经建立了客服档案，
// excludeAgentID 用于修改时排除自己
func (r *KefuAgentRepository) ExistsByUserID(userID int64, excludeAgentID int64) (bool, error) {
	query := r.db().Where("user_id = ?", userID)
	if excludeAgentID > 0 {
		query = query.Where("agent_id <> ?", excludeAgentID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetUserByID 查询管理员账号，新建客服档案时校验用户存在
func (r *KefuAgentRepository) GetUserByID(userID int64) (*model.SysUser, error) {
	user := new(model.SysUser)
	err := db.DB.Model(&model.SysUser{}).Where("deleted = 0 and user_id = ?", userID).
		First(user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAgentUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// AddRole 给管理员用户挂上客服角色，已挂过时跳过
func (r *KefuAgentRepository) AddRole(userID int64, roleCode string) error {
	var role model.Role
	err := db.DB.Where("role_code = ? and deleted = 0", roleCode).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	var count int64
	if err := db.DB.Model(&model.UserRole{}).
		Where("user_id = ? and role_id = ?", userID, role.RoleID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return db.DB.Create(&model.UserRole{UserID: userID, RoleID: role.RoleID}).Error
}
