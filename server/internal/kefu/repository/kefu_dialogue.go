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
	// ErrDialogueNotFound 是对话库条目不存在
	ErrDialogueNotFound = errors.New("对话不存在")
)

// KefuDialogueRepository 是对话库的数据访问，一个文件管主表与对话内容表
type KefuDialogueRepository struct{}

// NewKefuDialogueRepository 创建对话库的数据访问
func NewKefuDialogueRepository() *KefuDialogueRepository {
	return &KefuDialogueRepository{}
}

// db 返回带 kefu_dialogue 条件的查询
func (r *KefuDialogueRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuDialogue{}).Where("deleted = 0")
}

// GetByID 按 id 查询对话库条目，并回填对话内容
func (r *KefuDialogueRepository) GetByID(dialogueID int64) (*model.KefuDialogue, error) {
	record := new(model.KefuDialogue)
	err := r.db().Where("dialogue_id = ?", dialogueID).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDialogueNotFound
	}
	if err != nil {
		return nil, err
	}
	list := []model.KefuDialogue{*record}
	if err := r.fillMessages(list); err != nil {
		return nil, err
	}
	*record = list[0]
	return record, nil
}

// Page 分页查询对话库，不返回对话内容，只回填轮数
func (r *KefuDialogueRepository) Page(
	param model.KefuDialogueParam, page common.PageParam,
) ([]model.KefuDialogue, int64, error) {
	query := r.db()
	if param.Title != "" {
		query = query.Where("title like ?", "%"+param.Title+"%")
	}
	if param.RouteID != nil {
		query = query.Where("route_id = ?", *param.RouteID)
	}
	if param.Source != "" {
		query = query.Where("source = ?", param.Source)
	}
	if param.Status != nil {
		query = query.Where("status = ?", *param.Status)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.KefuDialogue
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "dialogue_id desc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	if err := r.fillMessageCount(list); err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// ListExamples 查询可喂给 AI 的对话示例：启用中，且绑定该关联链或未绑定关联链；
// 绑定该关联链的排在前面，同组按创建时间倒序，最多取 limit 条
func (r *KefuDialogueRepository) ListExamples(
	routeID int64, limit int,
) ([]model.KefuDialogue, error) {
	if limit <= 0 {
		return nil, nil
	}

	var list []model.KefuDialogue
	if err := r.db().
		Where("status = ? and (route_id = 0 or route_id = ?)",
			model.KefuDialogueStatusNormal, routeID).
		Order("dialogue_id desc").Find(&list).Error; err != nil {
		return nil, err
	}

	// 绑定当前关联链的示例更贴近会话场景，稳定地排到前面后截断
	matched := make([]model.KefuDialogue, 0, len(list))
	generic := make([]model.KefuDialogue, 0, len(list))
	for _, item := range list {
		if item.RouteID > 0 {
			matched = append(matched, item)
			continue
		}
		generic = append(generic, item)
	}
	list = append(matched, generic...)
	if len(list) > limit {
		list = list[:limit]
	}
	if err := r.fillMessages(list); err != nil {
		return nil, err
	}
	return list, nil
}

// ExistsByTitle 判断标题是否已存在，excludeID 为修改时的自身 id
func (r *KefuDialogueRepository) ExistsByTitle(title string, excludeID int64) (bool, error) {
	query := r.db().Where("title = ?", title)
	if excludeID > 0 {
		query = query.Where("dialogue_id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create 新增对话库条目，主表与对话内容在一个事务里完成
func (r *KefuDialogueRepository) Create(
	record *model.KefuDialogue, messages []model.KefuDialogueMessage,
) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		return saveDialogueMessages(tx, record.DialogueID, messages)
	})
}

// Update 修改对话库条目，对话内容整体替换，保证不多不少
func (r *KefuDialogueRepository) Update(
	record *model.KefuDialogue, messages []model.KefuDialogueMessage,
) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(record).Error; err != nil {
			return err
		}
		return saveDialogueMessages(tx, record.DialogueID, messages)
	})
}

// UpdateStatus 修改对话库条目的状态
func (r *KefuDialogueRepository) UpdateStatus(dialogueID int64, status int) error {
	return db.DB.Model(&model.KefuDialogue{}).Where("dialogue_id = ?", dialogueID).
		Updates(map[string]any{"status": status, "update_time": model.Now()}).Error
}

// Remove 删除对话库条目：软删主表，对话内容直接物理删除
func (r *KefuDialogueRepository) Remove(dialogueID int64) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.KefuDialogue{}).Where("dialogue_id = ?", dialogueID).
			Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error; err != nil {
			return err
		}
		return tx.Where("dialogue_id = ?", dialogueID).
			Delete(&model.KefuDialogueMessage{}).Error
	})
}

// fillMessages 批量回填对话内容与轮数
func (r *KefuDialogueRepository) fillMessages(list []model.KefuDialogue) error {
	if len(list) == 0 {
		return nil
	}

	dialogueIDs := make([]int64, 0, len(list))
	for _, item := range list {
		dialogueIDs = append(dialogueIDs, item.DialogueID)
	}

	var messages []model.KefuDialogueMessage
	if err := db.DB.Model(&model.KefuDialogueMessage{}).
		Where("dialogue_id in ?", dialogueIDs).
		Order("dialogue_id asc, sort_number asc, dialogue_message_id asc").
		Find(&messages).Error; err != nil {
		return err
	}

	messageMap := make(map[int64][]model.KefuDialogueMessage, len(dialogueIDs))
	for _, item := range messages {
		messageMap[item.DialogueID] = append(messageMap[item.DialogueID], item)
	}
	for i := range list {
		items := messageMap[list[i].DialogueID]
		if items == nil {
			items = []model.KefuDialogueMessage{}
		}
		list[i].Messages = items
		list[i].MessageCount = len(items)
	}
	return nil
}

// fillMessageCount 只统计对话内容的条数，列表页不需要完整内容
func (r *KefuDialogueRepository) fillMessageCount(list []model.KefuDialogue) error {
	if len(list) == 0 {
		return nil
	}

	dialogueIDs := make([]int64, 0, len(list))
	for _, item := range list {
		dialogueIDs = append(dialogueIDs, item.DialogueID)
	}

	type messageCount struct {
		// 对话id
		DialogueID int64
		// 对话内容条数
		Count int64
	}
	var counts []messageCount
	if err := db.DB.Model(&model.KefuDialogueMessage{}).
		Select("dialogue_id, count(*) as count").
		Where("dialogue_id in ?", dialogueIDs).
		Group("dialogue_id").Scan(&counts).Error; err != nil {
		return err
	}

	countMap := make(map[int64]int, len(counts))
	for _, item := range counts {
		countMap[item.DialogueID] = int(item.Count)
	}
	for i := range list {
		list[i].Messages = []model.KefuDialogueMessage{}
		list[i].MessageCount = countMap[list[i].DialogueID]
	}
	return nil
}

// saveDialogueMessages 全量替换对话内容，按数组顺序写入排序号
func saveDialogueMessages(
	tx *gorm.DB, dialogueID int64, messages []model.KefuDialogueMessage,
) error {
	if err := tx.Where("dialogue_id = ?", dialogueID).
		Delete(&model.KefuDialogueMessage{}).Error; err != nil {
		return err
	}
	if len(messages) == 0 {
		return nil
	}

	list := make([]model.KefuDialogueMessage, 0, len(messages))
	now := model.Now()
	for i, item := range messages {
		list = append(list, model.KefuDialogueMessage{
			DialogueID: dialogueID,
			SortNumber: i + 1,
			Role:       item.Role,
			Content:    item.Content,
			CreateTime: now,
			UpdateTime: now,
		})
	}
	return tx.Create(&list).Error
}
