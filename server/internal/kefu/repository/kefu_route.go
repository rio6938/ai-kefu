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
	// ErrRouteNotFound 是关联链不存在
	ErrRouteNotFound = errors.New("关联链不存在")
)

// KefuRouteRepository 是关联链的数据访问，一个文件管主表与两张关联表
type KefuRouteRepository struct{}

// NewKefuRouteRepository 创建关联链的数据访问
func NewKefuRouteRepository() *KefuRouteRepository {
	return &KefuRouteRepository{}
}

// db 返回带 kefu_route 条件的查询
func (r *KefuRouteRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuRoute{}).Where("deleted = 0")
}

// GetByID 按 id 查询关联链，并回填分类id与渠道id
func (r *KefuRouteRepository) GetByID(routeID int64) (*model.KefuRoute, error) {
	record := new(model.KefuRoute)
	err := r.db().Where("route_id = ?", routeID).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRouteNotFound
	}
	if err != nil {
		return nil, err
	}
	list := []model.KefuRoute{*record}
	if err := r.fillRelations(list); err != nil {
		return nil, err
	}
	*record = list[0]
	return record, nil
}

// Page 分页查询关联链，channelID 大于 0 时按绑定的渠道筛选
func (r *KefuRouteRepository) Page(
	param model.KefuRouteParam, page common.PageParam,
) ([]model.KefuRoute, int64, error) {
	query := r.db()
	if param.Name != "" {
		query = query.Where("name like ?", "%"+param.Name+"%")
	}
	if param.Status != nil {
		query = query.Where("status = ?", *param.Status)
	}
	if param.ChannelID != nil {
		query = query.Where("route_id in (select route_id from kefu_route_channel where channel_id = ?)",
			*param.ChannelID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.KefuRoute
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "route_id desc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	if err := r.fillRelations(list); err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// List 查询启用中的关联链，下拉选择时使用
func (r *KefuRouteRepository) List() ([]model.KefuRoute, error) {
	var list []model.KefuRoute
	if err := r.db().Where("status = ?", model.KefuRouteStatusNormal).
		Order("route_id desc").Find(&list).Error; err != nil {
		return nil, err
	}
	if err := r.fillRelations(list); err != nil {
		return nil, err
	}
	return list, nil
}

// GetByChannelID 按渠道查询绑定的关联链，会话建立时快照使用
func (r *KefuRouteRepository) GetByChannelID(channelID int64) (*model.KefuRoute, error) {
	var routeID int64
	err := db.DB.Model(&model.KefuRouteChannel{}).
		Where("channel_id = ?", channelID).Pluck("route_id", &routeID).Error
	if err != nil {
		return nil, err
	}
	if routeID == 0 {
		return nil, ErrRouteNotFound
	}
	return r.GetByID(routeID)
}

// ExistsByName 判断关联链名称是否已存在，excludeID 为修改时的自身 id
func (r *KefuRouteRepository) ExistsByName(name string, excludeID int64) (bool, error) {
	query := r.db().Where("name = ?", name)
	if excludeID > 0 {
		query = query.Where("route_id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ChannelBoundRouteID 返回渠道已绑定的关联链id，0 表示未绑定
func (r *KefuRouteRepository) ChannelBoundRouteID(channelID int64) (int64, error) {
	var routeID int64
	err := db.DB.Model(&model.KefuRouteChannel{}).
		Where("channel_id = ?", channelID).Pluck("route_id", &routeID).Error
	if err != nil {
		return 0, err
	}
	return routeID, nil
}

// RouteMapByChannelIDs 批量查询渠道已绑定的关联链，返回 渠道id -> 关联链id
func (r *KefuRouteRepository) RouteMapByChannelIDs(
	channelIDs []int64,
) (map[int64]int64, error) {
	result := make(map[int64]int64, len(channelIDs))
	if len(channelIDs) == 0 {
		return result, nil
	}

	var list []model.KefuRouteChannel
	if err := db.DB.Model(&model.KefuRouteChannel{}).
		Where("channel_id in ?", channelIDs).Find(&list).Error; err != nil {
		return nil, err
	}
	for _, item := range list {
		result[item.ChannelID] = item.RouteID
	}
	return result, nil
}

// ListNamesByIDs 按 id 批量查询关联链名称，会话列表回填 routeName 时使用
func (r *KefuRouteRepository) ListNamesByIDs(routeIDs []int64) (map[int64]string, error) {
	result := make(map[int64]string, len(routeIDs))
	if len(routeIDs) == 0 {
		return result, nil
	}

	var list []model.KefuRoute
	if err := r.db().Where("route_id in ?", routeIDs).Find(&list).Error; err != nil {
		return nil, err
	}
	for _, item := range list {
		result[item.RouteID] = item.Name
	}
	return result, nil
}

// Create 新增关联链，主表与两张关联表在一个事务里完成
func (r *KefuRouteRepository) Create(
	record *model.KefuRoute, categoryIDs, channelIDs []int64,
) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		return saveRouteRelations(tx, record.RouteID, categoryIDs, channelIDs)
	})
}

// Update 修改关联链，关联表整体替换，保证不多不少
func (r *KefuRouteRepository) Update(
	record *model.KefuRoute, categoryIDs, channelIDs []int64,
) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(record).Error; err != nil {
			return err
		}
		return saveRouteRelations(tx, record.RouteID, categoryIDs, channelIDs)
	})
}

// UpdateStatus 修改关联链状态
func (r *KefuRouteRepository) UpdateStatus(routeID int64, status int) error {
	return db.DB.Model(&model.KefuRoute{}).Where("route_id = ?", routeID).
		Updates(map[string]any{"status": status, "update_time": model.Now()}).Error
}

// Remove 删除关联链：软删主表，关联行直接物理删除
func (r *KefuRouteRepository) Remove(routeID int64) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.KefuRoute{}).Where("route_id = ?", routeID).
			Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error; err != nil {
			return err
		}
		if err := tx.Where("route_id = ?", routeID).
			Delete(&model.KefuRouteCategory{}).Error; err != nil {
			return err
		}
		return tx.Where("route_id = ?", routeID).
			Delete(&model.KefuRouteChannel{}).Error
	})
}

// fillRelations 批量回填关联链绑定的分类id与渠道id
func (r *KefuRouteRepository) fillRelations(list []model.KefuRoute) error {
	if len(list) == 0 {
		return nil
	}

	routeIDs := make([]int64, 0, len(list))
	for _, item := range list {
		routeIDs = append(routeIDs, item.RouteID)
	}

	var categories []model.KefuRouteCategory
	if err := db.DB.Model(&model.KefuRouteCategory{}).
		Where("route_id in ?", routeIDs).Find(&categories).Error; err != nil {
		return err
	}
	var channels []model.KefuRouteChannel
	if err := db.DB.Model(&model.KefuRouteChannel{}).
		Where("route_id in ?", routeIDs).Find(&channels).Error; err != nil {
		return err
	}

	categoryMap := make(map[int64][]int64, len(routeIDs))
	for _, item := range categories {
		categoryMap[item.RouteID] = append(categoryMap[item.RouteID], item.CategoryID)
	}
	channelMap := make(map[int64][]int64, len(routeIDs))
	for _, item := range channels {
		channelMap[item.RouteID] = append(channelMap[item.RouteID], item.ChannelID)
	}

	for i := range list {
		list[i].CategoryIDs = categoryMap[list[i].RouteID]
		list[i].ChannelIDs = channelMap[list[i].RouteID]
	}
	return nil
}

// saveRouteRelations 全量替换关联链的分类与渠道关联行
func saveRouteRelations(
	tx *gorm.DB, routeID int64, categoryIDs, channelIDs []int64,
) error {
	if err := tx.Where("route_id = ?", routeID).
		Delete(&model.KefuRouteCategory{}).Error; err != nil {
		return err
	}
	if err := tx.Where("route_id = ?", routeID).
		Delete(&model.KefuRouteChannel{}).Error; err != nil {
		return err
	}

	list := make([]model.KefuRouteCategory, 0, len(categoryIDs))
	for _, categoryID := range categoryIDs {
		list = append(list, model.KefuRouteCategory{RouteID: routeID, CategoryID: categoryID})
	}
	if len(list) > 0 {
		if err := tx.Create(&list).Error; err != nil {
			return err
		}
	}

	channels := make([]model.KefuRouteChannel, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		channels = append(channels, model.KefuRouteChannel{RouteID: routeID, ChannelID: channelID})
	}
	if len(channels) > 0 {
		if err := tx.Create(&channels).Error; err != nil {
			return err
		}
	}
	return nil
}
