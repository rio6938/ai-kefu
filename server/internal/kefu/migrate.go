package kefu

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"admin/server/model"
	"admin/server/pkg/db"
	"admin/server/pkg/log"
)

// 客服中心菜单的根路由地址
const menuRootPath = "/kefu"

// 默认渠道标识
const defaultChannelCode = "web-default"

// analysisMenuPath 是分析页菜单的路由地址，由系统模块初始化，
// 分析页的两个视角按钮权限挂在该菜单下
const analysisMenuPath = "/dashboard/analysis"

// 分析页的按钮权限标识
const (
	// authorityAnalysisAdmin 是管理员视角权限
	authorityAnalysisAdmin = "kefu:analysis:admin"
	// authorityAnalysisAgent 是客服视角权限
	authorityAnalysisAgent = "kefu:analysis:agent"
)

// sessionStatusDictCode 是会话状态字典标识
const sessionStatusDictCode = "kefu_session_status"

// 客服模块的角色标识
const (
	// roleCodeAgent 是客服角色
	roleCodeAgent = "kefu"
	// roleCodeKefuAdmin 是客服主管角色
	roleCodeKefuAdmin = "kefu_admin"
)

// Migrate 自动迁移客服模块的表结构，并幂等补齐角色、菜单、字典、配置等基础数据
func Migrate() error {
	if err := db.DB.AutoMigrate(
		&model.KefuChannel{},
		&model.KefuAgent{},
		&model.KefuVisitor{},
		&model.KefuSession{},
		&model.KefuSessionTransfer{},
		&model.KefuMessage{},
		&model.KefuAiReply{},
		&model.KefuPrompt{},
		&model.KefuKnowledgeCategory{},
		&model.KefuKnowledge{},
		&model.KefuKnowledgeChunk{},
		&model.KefuRoute{},
		&model.KefuRouteCategory{},
		&model.KefuRouteChannel{},
		&model.KefuDialogue{},
		&model.KefuDialogueMessage{},
		&model.KefuModel{},
		&model.KefuApiKey{},
		&model.KefuQuickReply{},
		&model.KefuMessageLeave{},
		&model.KefuSessionTag{},
		&model.KefuMailbox{},
		&model.KefuMailThread{},
		&model.KefuMailLog{},
	); err != nil {
		return fmt.Errorf("auto migrate kefu tables: %w", err)
	}

	if err := seedRoles(); err != nil {
		return err
	}
	if err := seedMenus(); err != nil {
		return err
	}
	if err := seedRoleMenus(); err != nil {
		return err
	}
	if err := seedAnalysisMenus(); err != nil {
		return err
	}
	if err := removeReviewMenus(); err != nil {
		return err
	}
	if err := seedDictionaries(); err != nil {
		return err
	}
	if err := removeServiceTypeDictionary(); err != nil {
		return err
	}
	if err := removeReviewDictionary(); err != nil {
		return err
	}
	if err := syncSessionStatus(); err != nil {
		return err
	}
	if err := seedConfigs(); err != nil {
		return err
	}
	if err := removeReviewConfig(); err != nil {
		return err
	}
	if err := seedDefaultChannel(); err != nil {
		return err
	}
	if err := seedDefaultRoute(); err != nil {
		return err
	}
	return seedDefaultModel()
}

// seedRoles 幂等新增客服与客服主管角色，已存在时跳过，不覆盖人工改过的名称与备注
func seedRoles() error {
	roles := []model.Role{
		{RoleCode: roleCodeAgent, RoleName: "客服", Comments: "客服：工作台、自己的会话、知识库查看"},
		{RoleCode: roleCodeKefuAdmin, RoleName: "客服主管", Comments: "客服主管：全部客服模块"},
	}
	now := model.Now()
	for i := range roles {
		var count int64
		if err := db.DB.Model(&model.Role{}).Where("role_code = ?", roles[i].RoleCode).
			Count(&count).Error; err != nil {
			return fmt.Errorf("count sys_role %s: %w", roles[i].RoleCode, err)
		}
		if count > 0 {
			continue
		}
		roles[i].CreateTime = now
		roles[i].UpdateTime = now
		if err := db.DB.Create(&roles[i]).Error; err != nil {
			return fmt.Errorf("create seed role %s: %w", roles[i].RoleCode, err)
		}
	}
	return nil
}

// menuPageSeed 描述要幂等创建的菜单，parentPath 为空表示顶级菜单
type menuPageSeed struct {
	// 上级菜单的路由地址
	parentPath string
	// 菜单内容
	menu model.Menu
}

// menuButtonSeed 描述要幂等创建的按钮权限
type menuButtonSeed struct {
	// 所属菜单的路由地址
	parentPath string
	// 按钮名称
	title string
	// 权限标识
	authority string
	// 排序号
	sortNumber int
}

// seedMenus 幂等创建客服中心的菜单与按钮权限，兼容已初始化过的库
func seedMenus() error {
	pages := []menuPageSeed{
		{
			menu: model.Menu{
				Title: "客服中心", Path: menuRootPath, MenuType: 0,
				SortNumber: 6, Icon: "IconProMessageOutlined",
			},
		},
		{
			parentPath: menuRootPath,
			menu: model.Menu{
				Title: "客服工作台", Path: "/kefu/workbench", Component: "/kefu/workbench",
				MenuType: 0, SortNumber: 1, Icon: "IconProDesktopOutlined",
			},
		},
		{
			parentPath: menuRootPath,
			menu: model.Menu{
				Title: "会话记录", Path: "/kefu/session", Component: "/kefu/session",
				MenuType: 0, SortNumber: 2, Icon: "IconProMessageAddOutlined",
			},
		},
		{
			parentPath: "/kefu/session",
			menu: model.Menu{
				Title: "会话详情", Path: "/kefu/session/details", Component: "/kefu/session/details",
				MenuType: 0, SortNumber: 1, Icon: "IconProMessageOutlined", Hide: 1,
			},
		},
		{
			parentPath: menuRootPath,
			menu: model.Menu{
				Title: "知识库", Path: "/kefu/knowledge", Component: "/kefu/knowledge",
				MenuType: 0, SortNumber: 4, Icon: "IconProBookOutlined",
			},
		},
		{
			parentPath: menuRootPath,
			menu: model.Menu{
				Title: "提示词", Path: "/kefu/prompt", Component: "/kefu/prompt",
				MenuType: 0, SortNumber: 5, Icon: "IconProEditOutlined",
			},
		},
		{
			parentPath: menuRootPath,
			menu: model.Menu{
				Title: "客服管理", Path: "/kefu/agent", Component: "/kefu/agent",
				MenuType: 0, SortNumber: 6, Icon: "IconProUserOutlined",
			},
		},
		{
			parentPath: menuRootPath,
			menu: model.Menu{
				Title: "模型配置", Path: "/kefu/model", Component: "/kefu/model",
				MenuType: 0, SortNumber: 7, Icon: "IconProModelOutlined",
			},
		},
		{
			parentPath: menuRootPath,
			menu: model.Menu{
				Title: "渠道管理", Path: "/kefu/channel", Component: "/kefu/channel",
				MenuType: 0, SortNumber: 8, Icon: "IconProLinkOutlined",
			},
		},
		{
			parentPath: menuRootPath,
			menu: model.Menu{
				Title: "快捷回复", Path: "/kefu/quickreply", Component: "/kefu/quickreply",
				MenuType: 0, SortNumber: 9, Icon: "IconProTagOutlined",
			},
		},
		{
			parentPath: menuRootPath,
			menu: model.Menu{
				Title: "关联链", Path: "/kefu/route", Component: "/kefu/route",
				MenuType: 0, SortNumber: 10, Icon: "IconProControlOutlined",
			},
		},
		{
			parentPath: menuRootPath,
			menu: model.Menu{
				Title: "对话库", Path: "/kefu/dialogue", Component: "/kefu/dialogue",
				MenuType: 0, SortNumber: 11, Icon: "IconProFormOutlined",
			},
		},
		{
			parentPath: menuRootPath,
			menu: model.Menu{
				Title: "发信记录", Path: "/kefu/maillog", Component: "/kefu/maillog",
				MenuType: 0, SortNumber: 12, Icon: "IconProFormOutlined",
			},
		},
	}
	for i := range pages {
		if _, err := ensurePageMenu(&pages[i]); err != nil {
			return err
		}
	}

	buttons := []menuButtonSeed{
		{parentPath: "/kefu/session", title: "查询会话", authority: "kefu:session:list", sortNumber: 10},
		{parentPath: "/kefu/session", title: "发送消息", authority: "kefu:session:send", sortNumber: 11},
		{parentPath: "/kefu/session", title: "分配会话", authority: "kefu:session:assign", sortNumber: 12},
		{parentPath: "/kefu/session", title: "转接会话", authority: "kefu:session:transfer", sortNumber: 13},
		{parentPath: "/kefu/session", title: "结束会话", authority: "kefu:session:close", sortNumber: 14},
		{parentPath: "/kefu/session", title: "开关 AI", authority: "kefu:session:ai-mode", sortNumber: 15},
		{parentPath: "/kefu/session", title: "生成 AI 草稿", authority: "kefu:session:ai-draft", sortNumber: 16},
		// 留言没有独立菜单，权限挂在会话记录下，处理时在「留言」页签操作
		{parentPath: "/kefu/session", title: "查询留言", authority: "kefu:leave:list", sortNumber: 17},
		{parentPath: "/kefu/session", title: "处理留言", authority: "kefu:leave:handle", sortNumber: 18},
		{parentPath: "/kefu/session", title: "设置会话标签与备注", authority: "kefu:session:tag", sortNumber: 19},
		{parentPath: "/kefu/workbench", title: "翻译", authority: "kefu:session:translate", sortNumber: 1},
		{parentPath: "/kefu/knowledge", title: "查询知识库", authority: "kefu:knowledge:list", sortNumber: 1},
		{parentPath: "/kefu/knowledge", title: "上传知识库", authority: "kefu:knowledge:save", sortNumber: 2},
		{parentPath: "/kefu/knowledge", title: "删除知识库", authority: "kefu:knowledge:remove", sortNumber: 3},
		{parentPath: "/kefu/prompt", title: "查询提示词", authority: "kefu:prompt:list", sortNumber: 1},
		{parentPath: "/kefu/prompt", title: "保存提示词", authority: "kefu:prompt:save", sortNumber: 2},
		{parentPath: "/kefu/prompt", title: "删除提示词", authority: "kefu:prompt:remove", sortNumber: 3},
		{parentPath: "/kefu/agent", title: "查询客服", authority: "kefu:agent:list", sortNumber: 1},
		{parentPath: "/kefu/agent", title: "添加客服", authority: "kefu:agent:save", sortNumber: 2},
		{parentPath: "/kefu/agent", title: "修改客服", authority: "kefu:agent:update", sortNumber: 3},
		{parentPath: "/kefu/agent", title: "删除客服", authority: "kefu:agent:remove", sortNumber: 4},
		{parentPath: "/kefu/model", title: "查询模型", authority: "kefu:model:list", sortNumber: 1},
		{parentPath: "/kefu/model", title: "保存模型", authority: "kefu:model:save", sortNumber: 2},
		{parentPath: "/kefu/model", title: "删除模型", authority: "kefu:model:remove", sortNumber: 3},
		// API 密钥暂时没有独立页面，按钮权限挂在模型配置下
		{parentPath: "/kefu/model", title: "查询 API 密钥", authority: "kefu:apikey:list", sortNumber: 4},
		{parentPath: "/kefu/model", title: "签发 API 密钥", authority: "kefu:apikey:save", sortNumber: 5},
		{parentPath: "/kefu/channel", title: "查询渠道", authority: "kefu:channel:list", sortNumber: 1},
		{parentPath: "/kefu/channel", title: "保存渠道", authority: "kefu:channel:save", sortNumber: 2},
		{parentPath: "/kefu/quickreply", title: "查询快捷回复", authority: "kefu:quickreply:list", sortNumber: 1},
		{parentPath: "/kefu/quickreply", title: "保存快捷回复", authority: "kefu:quickreply:save", sortNumber: 2},
		{parentPath: "/kefu/quickreply", title: "删除快捷回复", authority: "kefu:quickreply:remove", sortNumber: 3},
		{parentPath: "/kefu/route", title: "查询关联链", authority: "kefu:route:list", sortNumber: 1},
		{parentPath: "/kefu/route", title: "保存关联链", authority: "kefu:route:save", sortNumber: 2},
		{parentPath: "/kefu/route", title: "删除关联链", authority: "kefu:route:remove", sortNumber: 3},
		{parentPath: "/kefu/dialogue", title: "查询对话库", authority: "kefu:dialogue:list", sortNumber: 1},
		{parentPath: "/kefu/dialogue", title: "保存对话", authority: "kefu:dialogue:save", sortNumber: 2},
		{parentPath: "/kefu/dialogue", title: "删除对话", authority: "kefu:dialogue:remove", sortNumber: 3},
		{parentPath: "/kefu/maillog", title: "查询发信记录", authority: "kefu:mail:log:list", sortNumber: 1},
		{parentPath: "/kefu/maillog", title: "重发邮件", authority: "kefu:mail:log:resend", sortNumber: 2},
	}
	for _, item := range buttons {
		if err := ensureButtonMenu(item); err != nil {
			return err
		}
	}
	return nil
}

// ensurePageMenu 按路由地址幂等创建菜单，父菜单不存在时跳过，避免迁移整体失败
func ensurePageMenu(seed *menuPageSeed) (*model.Menu, error) {
	parentID := int64(0)
	if seed.parentPath != "" {
		var parent model.Menu
		err := db.DB.Where("path = ? and deleted = 0", seed.parentPath).First(&parent).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Logger.Warn("skip kefu menu, parent not found",
				zap.String("path", seed.menu.Path), zap.String("parentPath", seed.parentPath))
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("find menu %s: %w", seed.parentPath, err)
		}
		parentID = parent.MenuID
	}

	var existing model.Menu
	err := db.DB.Where("path = ? and deleted = 0", seed.menu.Path).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find menu %s: %w", seed.menu.Path, err)
	}

	now := model.Now()
	menu := seed.menu
	menu.ParentID = parentID
	menu.CreateTime = now
	menu.UpdateTime = now
	if err := db.DB.Create(&menu).Error; err != nil {
		return nil, fmt.Errorf("create menu %s: %w", menu.Path, err)
	}
	return &menu, nil
}

// ensureButtonMenu 按权限标识幂等创建按钮菜单，按钮没有路由地址不能按 path 判断
func ensureButtonMenu(seed menuButtonSeed) error {
	var parent model.Menu
	err := db.DB.Where("path = ? and deleted = 0", seed.parentPath).First(&parent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Logger.Warn("skip kefu menu button, parent not found",
			zap.String("authority", seed.authority), zap.String("parentPath", seed.parentPath))
		return nil
	}
	if err != nil {
		return fmt.Errorf("find menu %s: %w", seed.parentPath, err)
	}

	var count int64
	if err := db.DB.Model(&model.Menu{}).
		Where("authority = ? and menu_type = 1 and deleted = 0", seed.authority).
		Count(&count).Error; err != nil {
		return fmt.Errorf("count menu %s: %w", seed.authority, err)
	}
	if count > 0 {
		return nil
	}

	now := model.Now()
	menu := model.Menu{
		ParentID:   parent.MenuID,
		Title:      seed.title,
		MenuType:   1,
		SortNumber: seed.sortNumber,
		Authority:  seed.authority,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := db.DB.Create(&menu).Error; err != nil {
		return fmt.Errorf("create menu %s: %w", seed.authority, err)
	}
	return nil
}

// agentMenuKeys 是客服角色可用的菜单与按钮，主管与管理员拥有全部
var agentMenuKeys = []string{
	menuRootPath,
	"/kefu/workbench",
	"/kefu/session",
	"/kefu/session/details",
	"/kefu/knowledge",
	"/kefu/quickreply",
	"kefu:session:list",
	"kefu:session:send",
	"kefu:session:close",
	"kefu:session:ai-mode",
	"kefu:session:ai-draft",
	"kefu:session:translate",
	"kefu:leave:list",
	"kefu:leave:handle",
	"kefu:session:tag",
	"kefu:knowledge:list",
	"kefu:quickreply:list",
}

// seedRoleMenus 按角色幂等授权客服中心的菜单：客服只授工作台与会话，主管与管理员授全部
func seedRoleMenus() error {
	menus, err := listKefuMenus()
	if err != nil {
		return err
	}
	if len(menus) == 0 {
		return nil
	}

	if err := grantMenus(roleCodeKefuAdmin, menus); err != nil {
		return err
	}
	// 超级管理员角色直接读全部菜单，这里一并授权，避免菜单管理页看不到授权状态
	if err := grantMenus(model.RoleCodeAdmin, menus); err != nil {
		return err
	}

	allowed := make(map[string]bool, len(agentMenuKeys))
	for _, key := range agentMenuKeys {
		allowed[key] = true
	}
	agentMenus := make([]model.Menu, 0, len(allowed))
	for _, menu := range menus {
		if allowed[menuKey(menu)] {
			agentMenus = append(agentMenus, menu)
		}
	}
	return grantMenus(roleCodeAgent, agentMenus)
}

// seedAnalysisMenus 幂等创建分析页的按钮权限，并把分析页授权给各角色：
// 管理员与客服主管通用两个视角，客服只用客服视角
// （分析页菜单由系统模块初始化，不在客服中心菜单树下，因此单独授权）
func seedAnalysisMenus() error {
	buttons := []menuButtonSeed{
		{parentPath: analysisMenuPath, title: "分析页管理员视角", authority: authorityAnalysisAdmin, sortNumber: 1},
		{parentPath: analysisMenuPath, title: "分析页客服视角", authority: authorityAnalysisAgent, sortNumber: 2},
	}
	for _, item := range buttons {
		if err := ensureButtonMenu(item); err != nil {
			return err
		}
	}

	allKeys := []string{analysisMenuPath, authorityAnalysisAdmin, authorityAnalysisAgent}
	all, err := listMenusByKeys(allKeys)
	if err != nil {
		return err
	}
	if err := grantMenus(roleCodeKefuAdmin, all); err != nil {
		return err
	}
	if err := grantMenus(model.RoleCodeAdmin, all); err != nil {
		return err
	}

	agentMenus, err := listMenusByKeys([]string{analysisMenuPath, authorityAnalysisAgent})
	if err != nil {
		return err
	}
	return grantMenus(roleCodeAgent, agentMenus)
}

// listMenusByKeys 按菜单路由地址或按钮权限标识查询菜单
func listMenusByKeys(keys []string) ([]model.Menu, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	var list []model.Menu
	if err := db.DB.Where("deleted = 0 and (path in ? or authority in ?)", keys, keys).
		Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list menus by keys: %w", err)
	}
	return list, nil
}

// listKefuMenus 查询客服中心下的全部菜单与按钮，按层级展开
func listKefuMenus() ([]model.Menu, error) {
	var menus []model.Menu
	if err := db.DB.Where("deleted = 0").Find(&menus).Error; err != nil {
		return nil, fmt.Errorf("list menus: %w", err)
	}

	children := make(map[int64][]model.Menu, len(menus))
	var rootID int64
	for _, menu := range menus {
		if menu.Path == menuRootPath {
			rootID = menu.MenuID
			continue
		}
		children[menu.ParentID] = append(children[menu.ParentID], menu)
	}
	if rootID == 0 {
		return nil, nil
	}

	var list []model.Menu
	var walk func(parentID int64)
	walk = func(parentID int64) {
		for _, menu := range children[parentID] {
			list = append(list, menu)
			walk(menu.MenuID)
		}
	}
	for _, menu := range menus {
		if menu.MenuID == rootID {
			list = append(list, menu)
		}
	}
	walk(rootID)
	return list, nil
}

// menuKey 返回菜单的授权判断键：菜单用路由地址，按钮用权限标识
func menuKey(menu model.Menu) string {
	if menu.Path != "" {
		return menu.Path
	}
	return menu.Authority
}

// grantMenus 幂等给角色授权菜单，已存在的授权跳过
func grantMenus(roleCode string, menus []model.Menu) error {
	var role model.Role
	err := db.DB.Where("role_code = ? and deleted = 0", roleCode).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Logger.Warn("skip kefu role menus, role not found", zap.String("roleCode", roleCode))
		return nil
	}
	if err != nil {
		return fmt.Errorf("find role %s: %w", roleCode, err)
	}

	var existing []int64
	if err := db.DB.Model(&model.RoleMenu{}).Where("role_id = ?", role.RoleID).
		Pluck("menu_id", &existing).Error; err != nil {
		return fmt.Errorf("list role menus: %w", err)
	}
	exists := make(map[int64]bool, len(existing))
	for _, menuID := range existing {
		exists[menuID] = true
	}

	list := make([]model.RoleMenu, 0, len(menus))
	for _, menu := range menus {
		if exists[menu.MenuID] {
			continue
		}
		list = append(list, model.RoleMenu{RoleID: role.RoleID, MenuID: menu.MenuID})
	}
	if len(list) == 0 {
		return nil
	}
	if err := db.DB.Create(&list).Error; err != nil {
		return fmt.Errorf("grant menus to %s: %w", roleCode, err)
	}
	return nil
}

// dictionarySeed 描述一个要幂等创建的字典及其字典数据
type dictionarySeed struct {
	// 字典内容
	dictionary model.Dictionary
	// 字典数据，编码与名称相同的关键词类字典直接按值填写
	data []model.DictionaryData
}

// seedDictionaries 幂等创建客服模块的字典与字典数据，不覆盖人工改过的字典数据
func seedDictionaries() error {
	seeds := []dictionarySeed{
		{
			dictionary: model.Dictionary{
				DictCode: sessionStatusDictCode, DictName: "会话状态", SortNumber: 2,
			},
			data: []model.DictionaryData{
				{DictDataCode: "0", DictDataName: "待接待", SortNumber: 1},
				{DictDataCode: "1", DictDataName: "AI 进行中", SortNumber: 2},
				{DictDataCode: "2", DictDataName: "已结束", SortNumber: 3},
				{DictDataCode: "3", DictDataName: "留言", SortNumber: 4},
				{DictDataCode: "4", DictDataName: "人工进行中", SortNumber: 5},
			},
		},
		{
			dictionary: model.Dictionary{
				DictCode: "kefu_transfer_keyword", DictName: "转接关键词", SortNumber: 4,
				Comments: "客户消息命中这些关键词时提示建议转接",
			},
			data: []model.DictionaryData{
				{DictDataCode: "退款", DictDataName: "退款", SortNumber: 1},
				{DictDataCode: "发票", DictDataName: "发票", SortNumber: 2},
				{DictDataCode: "报错", DictDataName: "报错", SortNumber: 3},
			},
		},
		{
			dictionary: model.Dictionary{
				DictCode: "kefu_sensitive_word", DictName: "AI 敏感词", SortNumber: 5,
				Comments: "AI 输出前按前缀匹配拦截，命中后改发安全话术",
			},
		},
		{
			dictionary: model.Dictionary{
				DictCode: "kefu_handoff_keyword", DictName: "转人工关键词", SortNumber: 6,
				Comments: "配置中心 kefu.handoff.keywords 未维护时回退到这里的字典数据",
			},
			data: []model.DictionaryData{
				{DictDataCode: "人工客服", DictDataName: "人工客服", SortNumber: 1},
				{DictDataCode: "转人工", DictDataName: "转人工", SortNumber: 2},
				{DictDataCode: "找人工", DictDataName: "找人工", SortNumber: 3},
				{DictDataCode: "真人", DictDataName: "真人", SortNumber: 4},
			},
		},
		{
			dictionary: model.Dictionary{
				DictCode: model.KefuChannelTypeDictCode, DictName: "渠道类型", SortNumber: 7,
				Comments: "渠道的类型，网页渠道与国内电商店铺渠道",
			},
			data: []model.DictionaryData{
				{DictDataCode: "web", DictDataName: "网页", SortNumber: 1},
				{DictDataCode: "websdk", DictDataName: "网页 SDK", SortNumber: 2},
				{DictDataCode: "pdd", DictDataName: "拼多多", SortNumber: 3},
				{DictDataCode: "jd", DictDataName: "京东", SortNumber: 4},
				{DictDataCode: "taobao", DictDataName: "淘宝", SortNumber: 5},
				{DictDataCode: "hook", DictDataName: "Hook", SortNumber: 6},
				{DictDataCode: "email", DictDataName: "邮箱", SortNumber: 7},
			},
		},
		{
			dictionary: model.Dictionary{
				DictCode: model.KefuAgentWorkStatusDictCode, DictName: "坐席工作状态", SortNumber: 8,
				Comments: "坐席工作台底部状态条的工作状态，与接单开关相互独立",
			},
			data: []model.DictionaryData{
				{DictDataCode: "0", DictDataName: "离线", SortNumber: 1},
				{DictDataCode: "1", DictDataName: "在线", SortNumber: 2},
				{DictDataCode: "2", DictDataName: "小休", SortNumber: 3},
			},
		},
		{
			dictionary: model.Dictionary{
				DictCode: model.KefuRateTagDictCode, DictName: "会话评价标签", SortNumber: 9,
				Comments: "访客评价会话时可多选的标签，运营可自行维护",
			},
			data: []model.DictionaryData{
				{DictDataCode: "响应速度", DictDataName: "响应速度", SortNumber: 1},
				{DictDataCode: "专业程度", DictDataName: "专业程度", SortNumber: 2},
				{DictDataCode: "服务态度", DictDataName: "服务态度", SortNumber: 3},
				{DictDataCode: "解决问题", DictDataName: "解决问题", SortNumber: 4},
			},
		},
		{
			dictionary: model.Dictionary{
				DictCode: model.KefuSessionTagDictCode, DictName: "会话标签", SortNumber: 10,
				Comments: "坐席给会话打的标签，运营自行维护字典数据",
			},
		},
		{
			dictionary: model.Dictionary{
				DictCode: model.KefuMailProviderDictCode, DictName: "邮箱服务商", SortNumber: 11,
				Comments: "渠道绑定邮箱时选择的服务商预设",
			},
			data: []model.DictionaryData{
				{DictDataCode: "gmail", DictDataName: "Gmail 个人", SortNumber: 1},
				{DictDataCode: "qq", DictDataName: "QQ 邮箱", SortNumber: 2},
				{DictDataCode: "163", DictDataName: "163 邮箱", SortNumber: 3},
				{DictDataCode: "outlook", DictDataName: "Outlook 个人", SortNumber: 4},
				{DictDataCode: "custom", DictDataName: "自定义", SortNumber: 5},
			},
		},
		{
			dictionary: model.Dictionary{
				DictCode: model.KefuMailReplyModeDictCode, DictName: "邮箱自动回复模式", SortNumber: 12,
				Comments: "邮箱渠道自动回复的策略，默认仅草稿",
			},
			data: []model.DictionaryData{
				{DictDataCode: "off", DictDataName: "关闭", SortNumber: 1},
				{DictDataCode: "draft", DictDataName: "仅草稿", SortNumber: 2},
				{DictDataCode: "delay", DictDataName: "延时自动", SortNumber: 3},
				{DictDataCode: "auto", DictDataName: "立即自动", SortNumber: 4},
			},
		},
		{
			dictionary: model.Dictionary{
				DictCode: model.KefuMailAuthTypeDictCode, DictName: "邮箱认证方式", SortNumber: 13,
				Comments: "密码或授权码、OAuth2",
			},
			data: []model.DictionaryData{
				{DictDataCode: "password", DictDataName: "密码或授权码", SortNumber: 1},
				{DictDataCode: "oauth2", DictDataName: "OAuth2", SortNumber: 2},
			},
		},
		{
			dictionary: model.Dictionary{
				DictCode: model.KefuMailSkipReasonDictCode, DictName: "邮件跳过原因", SortNumber: 14,
				Comments: "入站邮件被跳过时的原因，收信记录里展示",
			},
			data: []model.DictionaryData{
				{DictDataCode: "auto_submitted", DictDataName: "自动回复头", SortNumber: 1},
				{DictDataCode: "bulk", DictDataName: "群发邮件", SortNumber: 2},
				{DictDataCode: "loop", DictDataName: "循环邮件", SortNumber: 3},
				{DictDataCode: "duplicate", DictDataName: "重复投递", SortNumber: 4},
				{DictDataCode: "bounce", DictDataName: "退信", SortNumber: 5},
				{DictDataCode: "sender_missing", DictDataName: "发件人为空", SortNumber: 6},
				{DictDataCode: "spam", DictDataName: "疑似垃圾邮件", SortNumber: 7},
				{DictDataCode: "other", DictDataName: "其它", SortNumber: 8},
			},
		},
	}

	for _, seed := range seeds {
		dictionary, err := ensureDictionary(seed.dictionary)
		if err != nil {
			return err
		}
		for i := range seed.data {
			seed.data[i].DictID = dictionary.DictID
			if err := ensureDictionaryData(seed.data[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

// syncSessionStatus 兼容旧数据：原「进行中」拆成 AI 进行中与人工进行中，
// 已绑定客服的会话补成人工进行中，同时更新会话状态字典的名称，可重复执行
func syncSessionStatus() error {
	if err := db.DB.Model(&model.KefuSession{}).
		Where("status = ? and agent_id > 0", model.KefuSessionStatusAi).
		Update("status", model.KefuSessionStatusHuman).Error; err != nil {
		return fmt.Errorf("migrate kefu session status: %w", err)
	}

	var dictionary model.Dictionary
	err := db.DB.Where("dict_code = ? and deleted = 0", sessionStatusDictCode).
		First(&dictionary).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find dictionary %s: %w", sessionStatusDictCode, err)
	}

	// 只改还是旧名称的数据，避免覆盖运营自己调整过的名称
	seeds := []struct {
		// 字典数据标识
		code string
		// 旧名称
		oldNames []string
		// 新名称
		name string
	}{
		{code: "0", oldNames: []string{"排队中"}, name: "待接待"},
		{code: "1", oldNames: []string{"进行中"}, name: "AI 进行中"},
	}
	now := model.Now()
	for _, seed := range seeds {
		if err := db.DB.Model(&model.DictionaryData{}).
			Where("dict_id = ? and dict_data_code = ? and dict_data_name in ?",
				dictionary.DictID, seed.code, seed.oldNames).
			Updates(map[string]any{
				"dict_data_name": seed.name,
				"update_time":    now,
			}).Error; err != nil {
			return fmt.Errorf("migrate kefu session status dictionary: %w", err)
		}
	}
	return nil
}

// serviceTypeDictCode 是已下线的客服类型字典标识，仅用于清理历史数据
const serviceTypeDictCode = "kefu_service_type"

// removeServiceTypeDictionary 下线客服类型：把历史字典与字典数据软删除，
// 系统已经不再使用客服类型，保留它们只会在字典管理里造成误解，可重复执行
func removeServiceTypeDictionary() error {
	var dictionary model.Dictionary
	err := db.DB.Where("dict_code = ? and deleted = 0", serviceTypeDictCode).
		First(&dictionary).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find dictionary %s: %w", serviceTypeDictCode, err)
	}

	now := model.Now()
	if err := db.DB.Model(&model.DictionaryData{}).
		Where("dict_id = ? and deleted = 0", dictionary.DictID).
		Updates(map[string]any{"deleted": 1, "update_time": now}).Error; err != nil {
		return fmt.Errorf("remove dictionary data %s: %w", serviceTypeDictCode, err)
	}
	if err := db.DB.Model(&model.Dictionary{}).
		Where("dict_id = ?", dictionary.DictID).
		Updates(map[string]any{"deleted": 1, "update_time": now}).Error; err != nil {
		return fmt.Errorf("remove dictionary %s: %w", serviceTypeDictCode, err)
	}

	log.Logger.Debug("removed kefu service type dictionary",
		zap.String("dictCode", serviceTypeDictCode))
	return nil
}

// removeReviewMenus 下线 AI 审批台：软删除菜单、按钮与角色授权，
// 审批功能已移除，保留菜单会进入一个不存在的页面，可重复执行
func removeReviewMenus() error {
	keys := []string{"/kefu/review", "kefu:review:list", "kefu:review:score"}
	var menus []model.Menu
	if err := db.DB.Where("deleted = 0 and (path in ? or authority in ?)", keys, keys).
		Find(&menus).Error; err != nil {
		return fmt.Errorf("find review menus: %w", err)
	}
	if len(menus) == 0 {
		return nil
	}

	menuIDs := make([]int64, 0, len(menus))
	for _, menu := range menus {
		menuIDs = append(menuIDs, menu.MenuID)
	}
	if err := db.DB.Where("menu_id in ?", menuIDs).
		Delete(&model.RoleMenu{}).Error; err != nil {
		return fmt.Errorf("remove review role menus: %w", err)
	}

	now := model.Now()
	if err := db.DB.Model(&model.Menu{}).Where("menu_id in ?", menuIDs).
		Updates(map[string]any{"deleted": 1, "update_time": now}).Error; err != nil {
		return fmt.Errorf("remove review menus: %w", err)
	}
	log.Logger.Debug("removed kefu review menus", zap.Int("count", len(menuIDs)))
	return nil
}

// reviewTagDictCode 是已下线的 AI 审批标签字典标识，仅用于清理历史数据
const reviewTagDictCode = "kefu_review_tag"

// removeReviewDictionary 下线 AI 审批标签字典，可重复执行
func removeReviewDictionary() error {
	var dictionary model.Dictionary
	err := db.DB.Where("dict_code = ? and deleted = 0", reviewTagDictCode).
		First(&dictionary).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find dictionary %s: %w", reviewTagDictCode, err)
	}

	now := model.Now()
	if err := db.DB.Model(&model.DictionaryData{}).
		Where("dict_id = ? and deleted = 0", dictionary.DictID).
		Updates(map[string]any{"deleted": 1, "update_time": now}).Error; err != nil {
		return fmt.Errorf("remove dictionary data %s: %w", reviewTagDictCode, err)
	}
	if err := db.DB.Model(&model.Dictionary{}).
		Where("dict_id = ?", dictionary.DictID).
		Updates(map[string]any{"deleted": 1, "update_time": now}).Error; err != nil {
		return fmt.Errorf("remove dictionary %s: %w", reviewTagDictCode, err)
	}
	return nil
}

// removeReviewConfig 下线入审规则配置，可重复执行
func removeReviewConfig() error {
	now := model.Now()
	if err := db.DB.Model(&model.Config{}).
		Where("config_key = ? and deleted = 0", "kefu.review.mode").
		Updates(map[string]any{"deleted": 1, "update_time": now}).Error; err != nil {
		return fmt.Errorf("remove kefu review config: %w", err)
	}
	return nil
}

// ensureDictionary 按字典标识幂等创建字典，已存在时返回已有的字典
func ensureDictionary(seed model.Dictionary) (*model.Dictionary, error) {
	var existing model.Dictionary
	err := db.DB.Where("dict_code = ? and deleted = 0", seed.DictCode).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find dictionary %s: %w", seed.DictCode, err)
	}

	now := model.Now()
	seed.CreateTime = now
	seed.UpdateTime = now
	if err := db.DB.Create(&seed).Error; err != nil {
		return nil, fmt.Errorf("create dictionary %s: %w", seed.DictCode, err)
	}
	return &seed, nil
}

// ensureDictionaryData 按字典与数据标识幂等创建字典数据
func ensureDictionaryData(seed model.DictionaryData) error {
	var count int64
	if err := db.DB.Model(&model.DictionaryData{}).
		Where("dict_id = ? and dict_data_code = ? and deleted = 0", seed.DictID, seed.DictDataCode).
		Count(&count).Error; err != nil {
		return fmt.Errorf("count dictionary data %s: %w", seed.DictDataCode, err)
	}
	if count > 0 {
		return nil
	}

	now := model.Now()
	seed.CreateTime = now
	seed.UpdateTime = now
	if err := db.DB.Create(&seed).Error; err != nil {
		return fmt.Errorf("create dictionary data %s: %w", seed.DictDataCode, err)
	}
	return nil
}

// seedConfigs 幂等创建客服模块的配置项，配置项由本模块维护默认值
func seedConfigs() error {
	configs := []model.Config{
		{ConfigKey: "kefu.ai.enabled", ConfigValue: "true", ValueType: model.ConfigTypeBool, ConfigName: "启用 AI 自动回复", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 1, Comments: "总开关，关闭后所有会话都不再触发 AI"},
		{ConfigKey: "kefu.ai.context_rounds", ConfigValue: "10", ValueType: model.ConfigTypeInt, ConfigName: "上下文轮数", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 2, Comments: "组装提示词时带上的最近对话轮数"},
		{ConfigKey: "kefu.ai.timeout_ms", ConfigValue: "30000", ValueType: model.ConfigTypeInt, ConfigName: "AI 总超时", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 3, Comments: "单位毫秒，超时后走兜底话术"},
		{ConfigKey: "kefu.ai.first_token_timeout_ms", ConfigValue: "8000", ValueType: model.ConfigTypeInt, ConfigName: "AI 首字超时", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 4, Comments: "单位毫秒，超过则视为失败"},
		{ConfigKey: "kefu.ai.escalation_enabled", ConfigValue: "true", ValueType: model.ConfigTypeBool, ConfigName: "答不上来转人工", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 25, Comments: "开启后 AI 找不到知识依据时回复固定话术并转人工"},
		{ConfigKey: "kefu.assign.max_transfer_per_day", ConfigValue: "3", ValueType: model.ConfigTypeInt, ConfigName: "24 小时转接上限", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 6, Comments: "同一会话 24 小时内的最大转接次数"},
		{ConfigKey: "kefu.assign.queue_timeout_minutes", ConfigValue: "10", ValueType: model.ConfigTypeInt, ConfigName: "排队超时", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 7, Comments: "单位分钟，超时后转留言"},
		{ConfigKey: "kefu.assign.avg_wait_seconds", ConfigValue: "60", ValueType: model.ConfigTypeInt, ConfigName: "排队人均等待", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 27, Comments: "单位秒，访客端预计等待按排队名次乘该值估算"},
		{ConfigKey: "kefu.assign.queue_warn_minutes", ConfigValue: "3", ValueType: model.ConfigTypeInt, ConfigName: "待接待超时提醒", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 28, Comments: "单位分钟，待接待超过该时长在工作台标红提醒"},
		{ConfigKey: "kefu.handoff.visitor_enabled", ConfigValue: "true", ValueType: model.ConfigTypeBool, ConfigName: "访客自助转人工", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 29, Comments: "关闭后聊天窗不展示「转人工」按钮"},
		{ConfigKey: "kefu.rate.enabled", ConfigValue: "true", ValueType: model.ConfigTypeBool, ConfigName: "会话评价开关", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 30, Comments: "关闭后会话结束后不展示评价卡片"},
		{ConfigKey: "kefu.workbench.enter_send", ConfigValue: "true", ValueType: model.ConfigTypeBool, ConfigName: "坐席回车发送", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 31, Comments: "开启后按回车发送，Ctrl+Enter 始终可发送"},
		{ConfigKey: "kefu.browse.enabled", ConfigValue: "false", ValueType: model.ConfigTypeBool, ConfigName: "访客浏览轨迹", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 32, Comments: "默认关闭，只采集页面标题与地址，不采集表单内容"},
		{ConfigKey: "kefu.rag.top_k", ConfigValue: "5", ValueType: model.ConfigTypeInt, ConfigName: "召回条数", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 8, Comments: "知识库检索返回的分块数量"},
		{ConfigKey: "kefu.rag.score_threshold", ConfigValue: "0.35", ValueType: model.ConfigTypeFloat, ConfigName: "召回相似度阈值", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 9},
		{ConfigKey: "kefu.rag.chunk_size", ConfigValue: "600", ValueType: model.ConfigTypeInt, ConfigName: "分块大小", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 10, Comments: "单位字符"},
		{ConfigKey: "kefu.rag.chunk_overlap", ConfigValue: "80", ValueType: model.ConfigTypeInt, ConfigName: "分块重叠", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 11, Comments: "单位字符"},
		{ConfigKey: "kefu.rag.embed_batch_size", ConfigValue: "10", ValueType: model.ConfigTypeInt, ConfigName: "向量化批大小", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 13, Comments: "单次请求向量化的分块数量，不能超过 embedding 接口的批量上限"},
		{ConfigKey: "kefu.web.chat_url", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "聊天窗地址", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 12, IsPublic: 1, Comments: "chat 项目的部署地址，用于生成嵌入代码"},
		{ConfigKey: "kefu.handoff.work_time", ConfigValue: "上午 9:00-12:00，下午 14:00-19:00", ValueType: model.ConfigTypeString, ConfigName: "人工客服在线时间", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 14, Comments: "没有人工客服在线时，转人工提示里展示的在线时间"},
		{ConfigKey: "kefu.web.widget_url", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "悬浮脚本地址", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 15, IsPublic: 1, Comments: "widget.js 的部署地址，用于生成悬浮客服嵌入代码"},
		{ConfigKey: "kefu.file.enabled", ConfigValue: "true", ValueType: model.ConfigTypeBool, ConfigName: "访客附件开关", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 16, IsPublic: 1, Comments: "关闭后访客不能上传附件，客服侧上传不受影响"},
		{ConfigKey: "kefu.file.max_size_mb", ConfigValue: "10", ValueType: model.ConfigTypeInt, ConfigName: "附件单文件上限", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 17, IsPublic: 1, Comments: "单位 MB，访客单个附件的最大大小"},
		{ConfigKey: "kefu.file.allowed_ext", ConfigValue: "jpg,jpeg,png,gif,webp,pdf,doc,docx,xls,xlsx,txt,zip", ValueType: model.ConfigTypeString, ConfigName: "附件允许后缀", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 18, IsPublic: 1, Comments: "英文逗号分隔，小写比对"},
		{ConfigKey: "kefu.file.ip_daily_limit_mb", ConfigValue: "50", ValueType: model.ConfigTypeInt, ConfigName: "单 IP 每日附件上限", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 19, IsPublic: 1, Comments: "单位 MB，同一 IP 当日累计可上传的附件总大小"},
		{ConfigKey: "kefu.file.keep_days", ConfigValue: "30", ValueType: model.ConfigTypeInt, ConfigName: "访客附件保留天数", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 20, Comments: "0 表示不自动清理，只清访客上传的附件"},
		{ConfigKey: "kefu.translate.enabled", ConfigValue: "true", ValueType: model.ConfigTypeBool, ConfigName: "翻译开关", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 21, Comments: "关闭后客服控制台的翻译接口不可用"},
		{ConfigKey: "kefu.translate.auto", ConfigValue: "true", ValueType: model.ConfigTypeBool, ConfigName: "客户消息自动翻译", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 22, Comments: "工作台「自动翻译」开关的默认值，客服可在工作台自行关闭"},
		{ConfigKey: "kefu.translate.target_language", ConfigValue: "zh-CN", ValueType: model.ConfigTypeString, ConfigName: "翻译目标语言", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 23, Comments: "客服未指定目标语言时的默认值，如 zh-CN / en"},
		{ConfigKey: "kefu.translate.timeout_ms", ConfigValue: "15000", ValueType: model.ConfigTypeInt, ConfigName: "翻译超时", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 24, Comments: "单位毫秒，超时后返回中文提示"},
		{ConfigKey: "kefu.dialogue.limit", ConfigValue: "2", ValueType: model.ConfigTypeInt, ConfigName: "对话示例条数", GroupCode: "kefu", GroupName: "客服设置", SortNumber: 26, Comments: "每次回答时喂给 AI 的对话库示例条数，0 表示不注入"},
		// 邮箱客服设置，与系统邮件 mail.* 分开，互不覆盖
		{ConfigKey: "kefu.mail.enabled", ConfigValue: "true", ValueType: model.ConfigTypeBool, ConfigName: "启用邮箱渠道", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 1, Comments: "关闭后不拉信也不发信"},
		{ConfigKey: "kefu.mail.max_size_mb", ConfigValue: "5", ValueType: model.ConfigTypeInt, ConfigName: "单封邮件大小上限", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 2, Comments: "单位 MB，正文与全部附件合计，进出站都校验"},
		{ConfigKey: "kefu.mail.sync_interval_sec", ConfigValue: "60", ValueType: model.ConfigTypeInt, ConfigName: "收信轮询间隔", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 3, Comments: "单位秒，邮箱未单独配置时使用"},
		{ConfigKey: "kefu.mail.first_sync_days", ConfigValue: "7", ValueType: model.ConfigTypeInt, ConfigName: "首次同步天数", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 4, Comments: "首次同步只拉最近 N 天，最多 200 封"},
		{ConfigKey: "kefu.mail.thread_window_days", ConfigValue: "30", ValueType: model.ConfigTypeInt, ConfigName: "线程匹配窗口", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 5, Comments: "无线程头时按主题与发件人匹配线程的天数"},
		{ConfigKey: "kefu.mail.reopen_days", ConfigValue: "7", ValueType: model.ConfigTypeInt, ConfigName: "会话重开窗口", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 6, Comments: "已结束线程再次来信，N 天内重开原会话"},
		{ConfigKey: "kefu.mail.close_days", ConfigValue: "14", ValueType: model.ConfigTypeInt, ConfigName: "邮件会话自动结束", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 7, Comments: "N 天无往来自动结束邮件会话"},
		{ConfigKey: "kefu.mail.default_reply_mode", ConfigValue: "draft", ValueType: model.ConfigTypeString, ConfigName: "默认回复模式", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 8, Comments: "新建邮箱的默认值，只影响新建邮箱"},
		{ConfigKey: "kefu.mail.delay_minutes", ConfigValue: "15", ValueType: model.ConfigTypeInt, ConfigName: "延时发送分钟数", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 9, Comments: "延时自动发送模式的默认分钟数"},
		{ConfigKey: "kefu.mail.auto_ack_enabled", ConfigValue: "true", ValueType: model.ConfigTypeBool, ConfigName: "新线程自动确认", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 10, Comments: "新线程第一封来信是否立即回固定模板"},
		{ConfigKey: "kefu.mail.max_auto_reply_per_thread", ConfigValue: "3", ValueType: model.ConfigTypeInt, ConfigName: "同线程自动回复上限", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 11, Comments: "超过后转草稿或人工"},
		{ConfigKey: "kefu.mail.max_auto_reply_per_sender_day", ConfigValue: "10", ValueType: model.ConfigTypeInt, ConfigName: "同发件人每日上限", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 12, Comments: "同一发件人每日自动回复次数上限"},
		{ConfigKey: "kefu.mail.min_auto_reply_interval_min", ConfigValue: "10", ValueType: model.ConfigTypeInt, ConfigName: "自动回复最小间隔", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 13, Comments: "同一线程两次自动回复的最小间隔，单位分钟"},
		{ConfigKey: "kefu.mail.reply_max_chars", ConfigValue: "2000", ValueType: model.ConfigTypeInt, ConfigName: "AI 邮件最大字符数", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 14, Comments: "超出截断"},
		{ConfigKey: "kefu.mail.context_emails", ConfigValue: "5", ValueType: model.ConfigTypeInt, ConfigName: "邮件上下文封数", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 15, Comments: "组装 AI 上下文时带上的最近邮件数"},
		{ConfigKey: "kefu.mail.raw_keep_days", ConfigValue: "30", ValueType: model.ConfigTypeInt, ConfigName: "原始邮件保留天数", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 16, Comments: "0 表示不清理"},
		{ConfigKey: "kefu.mail.attachment_keep_days", ConfigValue: "30", ValueType: model.ConfigTypeInt, ConfigName: "邮件附件保留天数", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 17, Comments: "0 表示不清理"},
		{ConfigKey: "kefu.mail.oauth.google_client_id", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "Google OAuth2 客户端 id", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 18, Comments: "Gmail 个人需要 OAuth2 时填写"},
		{ConfigKey: "kefu.mail.oauth.google_client_secret", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "Google OAuth2 密钥", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 19, IsSecret: 1},
		{ConfigKey: "kefu.mail.oauth.microsoft_client_id", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "Microsoft OAuth2 客户端 id", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 20, Comments: "Outlook 个人邮箱必填"},
		{ConfigKey: "kefu.mail.oauth.microsoft_client_secret", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "Microsoft OAuth2 密钥", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 21, IsSecret: 1},
		{ConfigKey: "kefu.mail.oauth.redirect_url", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "OAuth2 回调地址", GroupCode: "kefu_mail", GroupName: "邮箱客服设置", SortNumber: 22, Comments: "必须与云平台登记一致"},
	}
	for _, item := range configs {
		if err := seedConfig(item); err != nil {
			return err
		}
	}
	return nil
}

// seedConfig 在配置键不存在时写入配置项，保证新增的内置配置项能补进已有的库
func seedConfig(item model.Config) error {
	var count int64
	if err := db.DB.Model(&model.Config{}).
		Where("config_key = ?", item.ConfigKey).Count(&count).Error; err != nil {
		return fmt.Errorf("count sys_config %s: %w", item.ConfigKey, err)
	}
	if count > 0 {
		return nil
	}

	now := model.Now()
	item.CreateTime = now
	item.UpdateTime = now
	if err := db.DB.Create(&item).Error; err != nil {
		return fmt.Errorf("create sys_config %s: %w", item.ConfigKey, err)
	}
	return nil
}

// seedDefaultChannel 在渠道表为空时写入默认的网页渠道
func seedDefaultChannel() error {
	var count int64
	if err := db.DB.Model(&model.KefuChannel{}).
		Where("channel_code = ?", defaultChannelCode).Count(&count).Error; err != nil {
		return fmt.Errorf("count kefu_channel: %w", err)
	}
	if count > 0 {
		return nil
	}

	now := model.Now()
	channel := &model.KefuChannel{
		ChannelCode: defaultChannelCode,
		ChannelType: model.KefuChannelTypeWeb,
		Name:        "网页默认渠道",
		WelcomeText: "您好，很高兴为您服务",
		ThemeColor:  "#409eff",
		NeedLogin:   0,
		Config:      "{}",
		Status:      model.KefuChannelStatusNormal,
		CreateTime:  now,
		UpdateTime:  now,
	}
	if err := db.DB.Create(channel).Error; err != nil {
		return fmt.Errorf("create default channel: %w", err)
	}

	log.Logger.Debug("seed kefu default channel", zap.String("channelCode", defaultChannelCode))
	return nil
}

// seedDefaultRoute 在关联链表为空时写入默认关联链，并把它绑定到默认渠道；
// 默认关联链不绑分类，即不检索文档库，保证开箱行为可预期
func seedDefaultRoute() error {
	var count int64
	if err := db.DB.Model(&model.KefuRoute{}).Where("deleted = 0").Count(&count).Error; err != nil {
		return fmt.Errorf("count kefu_route: %w", err)
	}
	if count > 0 {
		return nil
	}

	now := model.Now()
	route := &model.KefuRoute{
		Name:       "默认关联链",
		PromptID:   0,
		Status:     model.KefuRouteStatusNormal,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := db.DB.Create(route).Error; err != nil {
		return fmt.Errorf("create default route: %w", err)
	}

	var channel model.KefuChannel
	err := db.DB.Where("channel_code = ? and deleted = 0", defaultChannelCode).
		First(&channel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Logger.Warn("skip kefu default route channel, channel not found",
			zap.String("channelCode", defaultChannelCode))
		return nil
	}
	if err != nil {
		return fmt.Errorf("find default channel: %w", err)
	}

	if err := db.DB.Create(&model.KefuRouteChannel{
		RouteID:   route.RouteID,
		ChannelID: channel.ChannelID,
	}).Error; err != nil {
		return fmt.Errorf("create default route channel: %w", err)
	}

	log.Logger.Debug("seed kefu default route",
		zap.Int64("routeId", route.RouteID), zap.String("channelCode", defaultChannelCode))
	return nil
}

// seedDefaultModel 在模型表为空时写入一条空的 OpenAI 兼容模型配置，等待后台补齐
func seedDefaultModel() error {
	var count int64
	if err := db.DB.Model(&model.KefuModel{}).Where("deleted = 0").Count(&count).Error; err != nil {
		return fmt.Errorf("count kefu_model: %w", err)
	}
	if count > 0 {
		return nil
	}

	now := model.Now()
	record := &model.KefuModel{
		Name:        "默认模型",
		Provider:    model.KefuModelProviderOpenAICompatible,
		BaseURL:     "",
		Model:       "",
		Temperature: 0.7,
		TopP:        1,
		MaxTokens:   2048,
		IsDefault:   model.KefuModelIsDefault,
		Status:      model.KefuModelStatusNormal,
		CreateTime:  now,
		UpdateTime:  now,
	}
	if err := db.DB.Create(record).Error; err != nil {
		return fmt.Errorf("create default model: %w", err)
	}

	log.Logger.Debug("seed kefu default model")
	return nil
}
