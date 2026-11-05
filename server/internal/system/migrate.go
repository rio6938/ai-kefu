package system

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"admin/server/model"
	"admin/server/pkg/db"
	"admin/server/pkg/log"
)

// 初始化的管理员账号
const (
	seedUsername = "admin"
	seedPassword = "admin"
	seedNickname = "管理员"
)

// Migrate 自动迁移 system 模块的表结构，并初始化基础数据
func Migrate() error {
	if err := db.DB.AutoMigrate(
		&model.SysUser{},
		&model.Role{},
		&model.Menu{},
		&model.RoleMenu{},
		&model.UserRole{},
		&model.Dictionary{},
		&model.DictionaryData{},
		&model.Config{},
		&model.SysFile{},
		&model.SysUserFile{},
		&model.SysLoginRecord{},
		&model.SysOperationRecord{},
		&model.SysMessage{},
	); err != nil {
		return fmt.Errorf("auto migrate system tables: %w", err)
	}

	if err := seedAdmin(); err != nil {
		return err
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
	if err := renameUserMenu(); err != nil {
		return err
	}
	if err := fixMenuIcons(); err != nil {
		return err
	}
	if err := ensureUserCenterMenu(); err != nil {
		return err
	}
	if err := ensureUserMenu(); err != nil {
		return err
	}
	if err := ensureMessageMenu(); err != nil {
		return err
	}
	// 幂等补齐日志删除等按钮权限，只有本次新建过按钮时才回填历史角色的授权，
	// 避免每次重启把管理员手工取消的按钮授权又补回来
	buttonCreated, err := ensureButtonMenus()
	if err != nil {
		return err
	}
	if buttonCreated > 0 {
		if err := backfillRoleButtonMenus(); err != nil {
			return err
		}
	}
	if err := ensureConfigMenu(); err != nil {
		return err
	}
	if err := seedConfigs(); err != nil {
		return err
	}
	return seedDictionaries()
}

// seedAdmin 在管理员账号不存在时写入默认账号
func seedAdmin() error {
	var count int64
	if err := db.DB.Model(&model.SysUser{}).Where("username = ?", seedUsername).Count(&count).Error; err != nil {
		return fmt.Errorf("count sys_user: %w", err)
	}
	if count > 0 {
		return nil
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(seedPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash seed password: %w", err)
	}

	now := model.Now()
	user := &model.SysUser{
		Username:   seedUsername,
		Password:   string(hashed),
		Nickname:   seedNickname,
		Sex:        "1",
		Status:     0,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := db.DB.Create(user).Error; err != nil {
		return fmt.Errorf("create seed admin: %w", err)
	}

	log.Logger.Debug("seed admin created", zap.String("username", seedUsername))
	return nil
}

// seedRoles 初始化角色，并把管理员账号挂到管理员角色上
func seedRoles() error {
	now := model.Now()
	roles := []model.Role{
		{RoleCode: model.RoleCodeAdmin, RoleName: "管理员", Comments: "超级管理员，拥有全部权限"},
	}
	for i := range roles {
		var count int64
		if err := db.DB.Model(&model.Role{}).Where("role_code = ?", roles[i].RoleCode).
			Count(&count).Error; err != nil {
			return fmt.Errorf("count sys_role: %w", err)
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

	// 管理员账号绑定管理员角色
	var adminUser model.SysUser
	if err := db.DB.Where("username = ?", seedUsername).First(&adminUser).Error; err != nil {
		return fmt.Errorf("find seed admin: %w", err)
	}
	var adminRole model.Role
	if err := db.DB.Where("role_code = ?", model.RoleCodeAdmin).First(&adminRole).Error; err != nil {
		return fmt.Errorf("find admin role: %w", err)
	}

	var count int64
	if err := db.DB.Model(&model.UserRole{}).
		Where("user_id = ? and role_id = ?", adminUser.UserID, adminRole.RoleID).
		Count(&count).Error; err != nil {
		return fmt.Errorf("count sys_user_role: %w", err)
	}
	if count > 0 {
		return nil
	}
	return db.DB.Create(&model.UserRole{UserID: adminUser.UserID, RoleID: adminRole.RoleID}).Error
}

// seedMenus 初始化菜单与按钮权限
func seedMenus() error {
	var count int64
	if err := db.DB.Model(&model.Menu{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count sys_menu: %w", err)
	}
	if count > 0 {
		return nil
	}

	now := model.Now()
	// 以 tree 的形式描述菜单，写入时按层级展开
	tree := seedMenuTree()
	var walk func(items []model.Menu, parentID int64) error
	walk = func(items []model.Menu, parentID int64) error {
		for i := range items {
			menu := items[i]
			menu.ParentID = parentID
			menu.CreateTime = now
			menu.UpdateTime = now
			children := menu.Children
			menu.Children = nil
			if err := db.DB.Create(&menu).Error; err != nil {
				return fmt.Errorf("create seed menu %s: %w", menu.Title, err)
			}
			if err := walk(children, menu.MenuID); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(tree, 0); err != nil {
		return err
	}

	log.Logger.Debug("seed menus created")
	return nil
}

// seedRoleMenus 给管理员角色授权全部菜单
func seedRoleMenus() error {
	var adminRole model.Role
	if err := db.DB.Where("role_code = ?", model.RoleCodeAdmin).First(&adminRole).Error; err != nil {
		return fmt.Errorf("find admin role: %w", err)
	}

	var menus []model.Menu
	if err := db.DB.Find(&menus).Error; err != nil {
		return fmt.Errorf("list menus: %w", err)
	}
	for _, menu := range menus {
		var count int64
		if err := db.DB.Model(&model.RoleMenu{}).
			Where("role_id = ? and menu_id = ?", adminRole.RoleID, menu.MenuID).
			Count(&count).Error; err != nil {
			return fmt.Errorf("count sys_role_menu: %w", err)
		}
		if count > 0 {
			continue
		}
		if err := db.DB.Create(&model.RoleMenu{RoleID: adminRole.RoleID, MenuID: menu.MenuID}).Error; err != nil {
			return fmt.Errorf("create sys_role_menu: %w", err)
		}
	}
	return nil
}

// renameUserMenu 兼容历史数据，把系统用户管理菜单的标题改为管理员管理
func renameUserMenu() error {
	return db.DB.Model(&model.Menu{}).
		Where("path = ? and title = ?", "/system/user", "系统用户管理").
		Update("title", "管理员管理").Error
}

// fixMenuIcons 兼容历史数据，把旧版 element-ui 的图标类名改为前端全局安装的图标组件名
func fixMenuIcons() error {
	icons := []struct {
		oldIcon string
		newIcon string
	}{
		{"el-icon-house", "IconProHomeOutlined"},
		{"el-icon-data-analysis", "IconProAnalysisOutlined"},
		{"el-icon-setting", "IconProSettingOutlined"},
		{"el-icon-_user-group", "IconProUserOutlined"},
		{"el-icon-user", "IconProUserOutlined"},
		{"el-icon-postcard", "IconProIdcardOutlined"},
		{"el-icon-s-operation", "IconProControlOutlined"},
		{"el-icon-notebook-2", "IconProBookOutlined"},
		{"el-icon-folder", "IconProFolderOutlined"},
		{"el-icon-date", "IconProCalendarOutlined"},
		{"el-icon-_retrieve", "IconProLogOutlined"},
	}
	for _, item := range icons {
		if err := db.DB.Model(&model.Menu{}).
			Where("icon = ?", item.oldIcon).
			Update("icon", item.newIcon).Error; err != nil {
			return fmt.Errorf("update menu icon %s: %w", item.oldIcon, err)
		}
	}
	return nil
}

// seedDictionaries 初始化字典与字典数据
func seedDictionaries() error {
	var count int64
	if err := db.DB.Model(&model.Dictionary{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count sys_dictionary: %w", err)
	}
	if count > 0 {
		return nil
	}

	now := model.Now()
	dicts := []model.Dictionary{
		{DictCode: "sex", DictName: "用户性别", SortNumber: 1},
	}
	for i := range dicts {
		dicts[i].CreateTime = now
		dicts[i].UpdateTime = now
		if err := db.DB.Create(&dicts[i]).Error; err != nil {
			return fmt.Errorf("create seed dictionary %s: %w", dicts[i].DictCode, err)
		}
	}

	data := []struct {
		dictCode string
		code     string
		name     string
		sort     int
	}{
		{"sex", "1", "男", 1},
		{"sex", "2", "女", 2},
	}
	for _, item := range data {
		var dictionary model.Dictionary
		if err := db.DB.Where("dict_code = ?", item.dictCode).First(&dictionary).Error; err != nil {
			return fmt.Errorf("find seed dictionary %s: %w", item.dictCode, err)
		}
		record := &model.DictionaryData{
			DictID:       dictionary.DictID,
			DictDataCode: item.code,
			DictDataName: item.name,
			SortNumber:   item.sort,
			CreateTime:   now,
			UpdateTime:   now,
		}
		if err := db.DB.Create(record).Error; err != nil {
			return fmt.Errorf("create seed dictionary data %s: %w", item.name, err)
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

// seedConfigs 初始化配置中心的配置项，配置项由使用它的模块维护默认值
func seedConfigs() error {
	configs := []model.Config{
		// 短信设置，由 pkg/sms 读取
		{ConfigKey: "sms.enabled", ConfigValue: "false", ValueType: model.ConfigTypeBool, ConfigName: "启用短信服务", GroupCode: "sms", GroupName: "短信设置", SortNumber: 1, Comments: "关闭时发送验证码直接返回未启用"},
		{ConfigKey: "sms.provider", ConfigValue: "aliyun", ValueType: model.ConfigTypeString, ConfigName: "短信厂商", GroupCode: "sms", GroupName: "短信设置", SortNumber: 2, Comments: "支持 aliyun / tencent"},
		{ConfigKey: "sms.access_key_id", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "AccessKey ID", GroupCode: "sms", GroupName: "短信设置", SortNumber: 3, Comments: "腾讯云为 SecretId"},
		{ConfigKey: "sms.access_key_secret", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "AccessKey Secret", GroupCode: "sms", GroupName: "短信设置", SortNumber: 4, IsSecret: 1, Comments: "腾讯云为 SecretKey"},
		{ConfigKey: "sms.sign_name", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "短信签名", GroupCode: "sms", GroupName: "短信设置", SortNumber: 5},
		{ConfigKey: "sms.sdk_app_id", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "短信应用 ID", GroupCode: "sms", GroupName: "短信设置", SortNumber: 6, Comments: "腾讯云必填"},
		{ConfigKey: "sms.region", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "接口地域", GroupCode: "sms", GroupName: "短信设置", SortNumber: 7, Comments: "为空时用厂商默认值"},
		{ConfigKey: "sms.endpoint", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "接口地址", GroupCode: "sms", GroupName: "短信设置", SortNumber: 8, Comments: "为空时用厂商默认值"},
		{ConfigKey: "sms.templates", ConfigValue: `{"code":"","reset_password":""}`, ValueType: model.ConfigTypeJSON, ConfigName: "场景模板 ID", GroupCode: "sms", GroupName: "短信设置", SortNumber: 9, Comments: "键为场景名，值为模板 ID"},
		{ConfigKey: "sms.code_length", ConfigValue: "6", ValueType: model.ConfigTypeInt, ConfigName: "验证码位数", GroupCode: "sms", GroupName: "短信设置", SortNumber: 10, Comments: "4~8 位"},
		{ConfigKey: "sms.expire", ConfigValue: "300", ValueType: model.ConfigTypeInt, ConfigName: "验证码有效期", GroupCode: "sms", GroupName: "短信设置", SortNumber: 11, Comments: "单位秒"},
		{ConfigKey: "sms.interval", ConfigValue: "60", ValueType: model.ConfigTypeInt, ConfigName: "重发间隔", GroupCode: "sms", GroupName: "短信设置", SortNumber: 12, Comments: "同手机号同场景，单位秒"},
		{ConfigKey: "sms.hourly_limit", ConfigValue: "10", ValueType: model.ConfigTypeInt, ConfigName: "每小时发送上限", GroupCode: "sms", GroupName: "短信设置", SortNumber: 13},
		{ConfigKey: "sms.daily_limit", ConfigValue: "20", ValueType: model.ConfigTypeInt, ConfigName: "每日发送上限", GroupCode: "sms", GroupName: "短信设置", SortNumber: 14},
		{ConfigKey: "sms.max_attempts", ConfigValue: "5", ValueType: model.ConfigTypeInt, ConfigName: "最大校验失败次数", GroupCode: "sms", GroupName: "短信设置", SortNumber: 15},
		{ConfigKey: "sms.timeout", ConfigValue: "5", ValueType: model.ConfigTypeInt, ConfigName: "请求超时", GroupCode: "sms", GroupName: "短信设置", SortNumber: 16, Comments: "单位秒"},

		// 邮件设置，由 pkg/mail 在使用时读取
		{ConfigKey: "mail.enabled", ConfigValue: "false", ValueType: model.ConfigTypeBool, ConfigName: "启用邮件服务", GroupCode: "mail", GroupName: "邮件设置", SortNumber: 1},
		{ConfigKey: "mail.host", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "SMTP 服务器", GroupCode: "mail", GroupName: "邮件设置", SortNumber: 2},
		{ConfigKey: "mail.port", ConfigValue: "465", ValueType: model.ConfigTypeInt, ConfigName: "SMTP 端口", GroupCode: "mail", GroupName: "邮件设置", SortNumber: 3, Comments: "SSL 一般为 465，STARTTLS 一般为 587"},
		{ConfigKey: "mail.username", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "SMTP 账号", GroupCode: "mail", GroupName: "邮件设置", SortNumber: 4},
		{ConfigKey: "mail.password", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "SMTP 密码或授权码", GroupCode: "mail", GroupName: "邮件设置", SortNumber: 5, IsSecret: 1},
		{ConfigKey: "mail.from", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "发件人地址", GroupCode: "mail", GroupName: "邮件设置", SortNumber: 6, Comments: "为空时用 SMTP 账号"},
		{ConfigKey: "mail.from_name", ConfigValue: "", ValueType: model.ConfigTypeString, ConfigName: "发件人显示名", GroupCode: "mail", GroupName: "邮件设置", SortNumber: 7},
		{ConfigKey: "mail.ssl", ConfigValue: "true", ValueType: model.ConfigTypeBool, ConfigName: "直连 SSL", GroupCode: "mail", GroupName: "邮件设置", SortNumber: 8, Comments: "关闭时使用 STARTTLS"},
	}
	for _, item := range configs {
		if err := seedConfig(item); err != nil {
			return err
		}
	}
	return nil
}

// seedMenuTree 返回初始化菜单，Children 为子菜单
func seedMenuTree() []model.Menu {
	return []model.Menu{
		{
			Title:      "Dashboard",
			Path:       "/dashboard",
			MenuType:   0,
			SortNumber: 1,
			Icon:       "IconProHomeOutlined",
			Children: []model.Menu{
				{
					Title:      "分析页",
					Path:       "/dashboard/analysis",
					Component:  "/dashboard/analysis",
					MenuType:   0,
					SortNumber: 1,
					Icon:       "IconProAnalysisOutlined",
				},
			},
		},
		{
			Title:      "系统管理",
			Path:       "/system",
			MenuType:   0,
			SortNumber: 2,
			Icon:       "IconProSettingOutlined",
			Children: []model.Menu{
				{
					Title:      "管理员管理",
					Path:       "/system/user",
					Component:  "/system/user",
					MenuType:   0,
					SortNumber: 1,
					Icon:       "IconProUserOutlined",
					Children: []model.Menu{
						{Title: "查询用户", MenuType: 1, SortNumber: 1, Authority: "sys:user:list"},
						{Title: "添加用户", MenuType: 1, SortNumber: 2, Authority: "sys:user:save"},
						{Title: "修改用户", MenuType: 1, SortNumber: 3, Authority: "sys:user:update"},
						{Title: "删除用户", MenuType: 1, SortNumber: 4, Authority: "sys:user:remove"},
						{
							Title:      "用户详情",
							Path:       "/system/user/details",
							Component:  "/system/user/details",
							MenuType:   0,
							SortNumber: 5,
							Icon:       "IconProUserOutlined",
							Hide:       1,
						},
					},
				},
				{
					Title:      "用户管理",
					Path:       "/system/user-center",
					Component:  "/system/user-center",
					MenuType:   0,
					SortNumber: 2,
					Icon:       "IconProUserOutlined",
					Children: []model.Menu{
						{Title: "查询用户", MenuType: 1, SortNumber: 1, Authority: "sys:user-center:list"},
						{Title: "添加用户", MenuType: 1, SortNumber: 2, Authority: "sys:user-center:save"},
						{Title: "修改用户", MenuType: 1, SortNumber: 3, Authority: "sys:user-center:update"},
						{Title: "删除用户", MenuType: 1, SortNumber: 4, Authority: "sys:user-center:remove"},
					},
				},
				{
					Title:      "角色管理",
					Path:       "/system/role",
					Component:  "/system/role",
					MenuType:   0,
					SortNumber: 3,
					Icon:       "IconProIdcardOutlined",
					Children: []model.Menu{
						{Title: "查询角色", MenuType: 1, SortNumber: 1, Authority: "sys:role:list"},
						{Title: "添加角色", MenuType: 1, SortNumber: 2, Authority: "sys:role:save"},
						{Title: "修改角色", MenuType: 1, SortNumber: 3, Authority: "sys:role:update"},
						{Title: "删除角色", MenuType: 1, SortNumber: 4, Authority: "sys:role:remove"},
					},
				},
				{
					Title:      "菜单管理",
					Path:       "/system/menu",
					Component:  "/system/menu",
					MenuType:   0,
					SortNumber: 4,
					Icon:       "IconProControlOutlined",
					Children: []model.Menu{
						{Title: "查询菜单", MenuType: 1, SortNumber: 1, Authority: "sys:menu:list"},
						{Title: "添加菜单", MenuType: 1, SortNumber: 2, Authority: "sys:menu:save"},
						{Title: "修改菜单", MenuType: 1, SortNumber: 3, Authority: "sys:menu:update"},
						{Title: "删除菜单", MenuType: 1, SortNumber: 4, Authority: "sys:menu:remove"},
					},
				},
				{
					Title:      "字典管理",
					Path:       "/system/dictionary",
					Component:  "/system/dictionary",
					MenuType:   0,
					SortNumber: 5,
					Icon:       "IconProBookOutlined",
					Children: []model.Menu{
						{Title: "查询字典", MenuType: 1, SortNumber: 1, Authority: "sys:dict:list"},
						{Title: "添加字典", MenuType: 1, SortNumber: 2, Authority: "sys:dict:save"},
						{Title: "修改字典", MenuType: 1, SortNumber: 3, Authority: "sys:dict:update"},
						{Title: "删除字典", MenuType: 1, SortNumber: 4, Authority: "sys:dict:remove"},
					},
				},
				{
					Title:      "配置管理",
					Path:       "/system/config",
					Component:  "/system/config",
					MenuType:   0,
					SortNumber: 6,
					Icon:       "IconProSettingOutlined",
					Children: []model.Menu{
						{Title: "查询配置", MenuType: 1, SortNumber: 1, Authority: "sys:config:list"},
						{Title: "修改配置", MenuType: 1, SortNumber: 2, Authority: "sys:config:update"},
						{Title: "修改密钥配置", MenuType: 1, SortNumber: 3, Authority: "sys:config:secret"},
					},
				},
				{
					Title:      "文件管理",
					Path:       "/system/file",
					Component:  "/system/file",
					MenuType:   0,
					SortNumber: 7,
					Icon:       "IconProFolderOutlined",
					Children: []model.Menu{
						{Title: "上传文件", MenuType: 1, SortNumber: 1, Authority: "sys:file:upload"},
						{Title: "删除文件", MenuType: 1, SortNumber: 2, Authority: "sys:file:remove"},
						{Title: "查看记录", MenuType: 1, SortNumber: 3, Authority: "sys:file:list"},
					},
				},
				{
					Title:      "登录日志",
					Path:       "/system/login-record",
					Component:  "/system/login-record",
					MenuType:   0,
					SortNumber: 8,
					Icon:       "IconProCalendarOutlined",
					Authority:  "sys:login-record:list",
				},
				{
					Title:      "操作日志",
					Path:       "/system/operation-record",
					Component:  "/system/operation-record",
					MenuType:   0,
					SortNumber: 9,
					Icon:       "IconProLogOutlined",
					Authority:  "sys:operation-record:list",
				},
			},
		},
		{
			Title:      "个人中心",
			Path:       "/user/profile",
			Component:  "/user/profile",
			MenuType:   0,
			SortNumber: 3,
			Icon:       "IconProUserOutlined",
			Hide:       1,
			Children: []model.Menu{
				{Title: "修改个人密码", MenuType: 1, SortNumber: 1, Authority: "sys:auth:password"},
				{Title: "修改个人资料", MenuType: 1, SortNumber: 2, Authority: "sys:auth:user"},
			},
		},
		{
			Title:      "我的消息",
			Path:       "/user/message",
			Component:  "/user/message",
			MenuType:   0,
			SortNumber: 4,
			Icon:       "IconProBellOutlined",
			Hide:       1,
		},
	}
}

// ensureUserCenterMenu 幂等创建用户管理菜单并授权给管理员角色，兼容已初始化过的库；
// 菜单可能已经被 ensureUserMenu 移到用户管理下，两个路径都算已存在
func ensureUserCenterMenu() error {
	var count int64
	if err := db.DB.Model(&model.Menu{}).
		Where("path in ?", []string{"/system/user-center", "/user/user-center"}).
		Count(&count).Error; err != nil {
		return fmt.Errorf("count user center menu: %w", err)
	}
	if count > 0 {
		return nil
	}

	var parent model.Menu
	if err := db.DB.Where("path = ?", "/system").First(&parent).Error; err != nil {
		return fmt.Errorf("find system menu: %w", err)
	}

	// 插到管理员管理之后，原有同级菜单的排序号依次后移
	if err := db.DB.Model(&model.Menu{}).
		Where("parent_id = ? and sort_number >= ?", parent.MenuID, 2).
		UpdateColumn("sort_number", gorm.Expr("sort_number + 1")).Error; err != nil {
		return fmt.Errorf("shift system menu sort: %w", err)
	}

	now := model.Now()
	menu := model.Menu{
		ParentID:   parent.MenuID,
		Title:      "用户管理",
		Path:       "/system/user-center",
		Component:  "/system/user-center",
		MenuType:   0,
		SortNumber: 2,
		Icon:       "IconProUserOutlined",
		CreateTime: now,
		UpdateTime: now,
	}
	if err := db.DB.Create(&menu).Error; err != nil {
		return fmt.Errorf("create user center menu: %w", err)
	}

	buttons := []model.Menu{
		{Title: "查询用户", MenuType: 1, SortNumber: 1, Authority: "sys:user-center:list"},
		{Title: "添加用户", MenuType: 1, SortNumber: 2, Authority: "sys:user-center:save"},
		{Title: "修改用户", MenuType: 1, SortNumber: 3, Authority: "sys:user-center:update"},
		{Title: "删除用户", MenuType: 1, SortNumber: 4, Authority: "sys:user-center:remove"},
	}
	for i := range buttons {
		buttons[i].ParentID = menu.MenuID
		buttons[i].CreateTime = now
		buttons[i].UpdateTime = now
		if err := db.DB.Create(&buttons[i]).Error; err != nil {
			return fmt.Errorf("create user center menu button %s: %w", buttons[i].Title, err)
		}
	}

	var adminRole model.Role
	if err := db.DB.Where("role_code = ?", model.RoleCodeAdmin).First(&adminRole).Error; err != nil {
		return fmt.Errorf("find admin role: %w", err)
	}

	menus := append([]model.Menu{menu}, buttons...)
	for _, item := range menus {
		var linkCount int64
		if err := db.DB.Model(&model.RoleMenu{}).
			Where("role_id = ? and menu_id = ?", adminRole.RoleID, item.MenuID).
			Count(&linkCount).Error; err != nil {
			return fmt.Errorf("count sys_role_menu: %w", err)
		}
		if linkCount > 0 {
			continue
		}
		if err := db.DB.Create(&model.RoleMenu{RoleID: adminRole.RoleID, MenuID: item.MenuID}).Error; err != nil {
			return fmt.Errorf("create sys_role_menu: %w", err)
		}
	}

	log.Logger.Debug("ensure user center menu created")
	return nil
}

// ensureUserMenu 幂等创建用户管理一级菜单，并把前台用户相关页面挂到其下
func ensureUserMenu() error {
	parent, err := ensureUserRootMenu()
	if err != nil {
		return err
	}
	if err := moveUserCenterMenu(parent.MenuID); err != nil {
		return err
	}

	menus := []model.Menu{
		{
			ParentID:   parent.MenuID,
			Title:      "登录日志",
			Path:       "/user/login-record",
			Component:  "/user/login-record",
			MenuType:   0,
			SortNumber: 2,
			Icon:       "IconProCalendarOutlined",
			Authority:  "user:login-record:list",
		},
		{
			ParentID:   parent.MenuID,
			Title:      "操作日志",
			Path:       "/user/operation-record",
			Component:  "/user/operation-record",
			MenuType:   0,
			SortNumber: 3,
			Icon:       "IconProLogOutlined",
			Authority:  "user:operation-record:list",
		},
	}
	for i := range menus {
		menu, err := ensureMenuByPath(&menus[i])
		if err != nil {
			return err
		}
		if err := grantMenuToAdminRole(menu); err != nil {
			return err
		}
	}
	return nil
}

// ensureMessageMenu 幂等创建我的消息菜单并授权给管理员角色，兼容已初始化过的库；
// 该页面和我的资料一样是隐藏菜单，只能从顶栏的消息通知进入
func ensureMessageMenu() error {
	var count int64
	if err := db.DB.Model(&model.Menu{}).
		Where("path = ?", "/user/message").Count(&count).Error; err != nil {
		return fmt.Errorf("count message menu: %w", err)
	}
	if count > 0 {
		return nil
	}

	now := model.Now()
	menu := model.Menu{
		Title:      "我的消息",
		Path:       "/user/message",
		Component:  "/user/message",
		MenuType:   0,
		SortNumber: 4,
		Icon:       "IconProBellOutlined",
		Hide:       1,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := db.DB.Create(&menu).Error; err != nil {
		return fmt.Errorf("create message menu: %w", err)
	}
	if err := grantMenuToAdminRole(&menu); err != nil {
		return err
	}

	log.Logger.Debug("ensure message menu created")
	return nil
}

// ensureConfigMenu 幂等创建配置管理菜单并授权给管理员角色，兼容已初始化过的库
func ensureConfigMenu() error {
	var count int64
	if err := db.DB.Model(&model.Menu{}).
		Where("path = ?", "/system/config").Count(&count).Error; err != nil {
		return fmt.Errorf("count config menu: %w", err)
	}
	if count > 0 {
		return nil
	}

	var parent model.Menu
	if err := db.DB.Where("path = ?", "/system").First(&parent).Error; err != nil {
		return fmt.Errorf("find system menu: %w", err)
	}

	// 插到字典管理之后，原有同级菜单的排序号依次后移
	if err := db.DB.Model(&model.Menu{}).
		Where("parent_id = ? and sort_number >= ?", parent.MenuID, 6).
		UpdateColumn("sort_number", gorm.Expr("sort_number + 1")).Error; err != nil {
		return fmt.Errorf("shift system menu sort: %w", err)
	}

	now := model.Now()
	menu := model.Menu{
		ParentID:   parent.MenuID,
		Title:      "配置管理",
		Path:       "/system/config",
		Component:  "/system/config",
		MenuType:   0,
		SortNumber: 6,
		Icon:       "IconProSettingOutlined",
		CreateTime: now,
		UpdateTime: now,
	}
	if err := db.DB.Create(&menu).Error; err != nil {
		return fmt.Errorf("create config menu: %w", err)
	}

	buttons := []model.Menu{
		{Title: "查询配置", MenuType: 1, SortNumber: 1, Authority: "sys:config:list"},
		{Title: "修改配置", MenuType: 1, SortNumber: 2, Authority: "sys:config:update"},
		{Title: "修改密钥配置", MenuType: 1, SortNumber: 3, Authority: "sys:config:secret"},
	}
	for i := range buttons {
		buttons[i].ParentID = menu.MenuID
		buttons[i].CreateTime = now
		buttons[i].UpdateTime = now
		if err := db.DB.Create(&buttons[i]).Error; err != nil {
			return fmt.Errorf("create config menu button %s: %w", buttons[i].Title, err)
		}
	}

	menus := append([]model.Menu{menu}, buttons...)
	for i := range menus {
		if err := grantMenuToAdminRole(&menus[i]); err != nil {
			return err
		}
	}

	log.Logger.Debug("ensure config menu created")
	return nil
}

// ensureUserRootMenu 幂等创建用户管理一级菜单，创建时挤到 Dashboard 之后
func ensureUserRootMenu() (*model.Menu, error) {
	var menu model.Menu
	err := db.DB.Where("path = ?", "/user").First(&menu).Error
	if err == nil {
		return &menu, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find user menu: %w", err)
	}

	// 原有的一级菜单排序号依次后移，给用户管理让出第 2 位
	if err := db.DB.Model(&model.Menu{}).
		Where("parent_id = 0 and sort_number >= ?", 2).
		UpdateColumn("sort_number", gorm.Expr("sort_number + 1")).Error; err != nil {
		return nil, fmt.Errorf("shift top menu sort: %w", err)
	}

	now := model.Now()
	menu = model.Menu{
		Title:      "用户管理",
		Path:       "/user",
		MenuType:   0,
		SortNumber: 2,
		Icon:       "IconProUserOutlined",
		CreateTime: now,
		UpdateTime: now,
	}
	if err := db.DB.Create(&menu).Error; err != nil {
		return nil, fmt.Errorf("create user menu: %w", err)
	}
	if err := grantMenuToAdminRole(&menu); err != nil {
		return nil, err
	}

	log.Logger.Debug("ensure user menu created")
	return &menu, nil
}

// moveUserCenterMenu 把前台用户列表菜单从系统管理移动到用户管理下
func moveUserCenterMenu(parentID int64) error {
	var menu model.Menu
	err := db.DB.Where("path in ?", []string{"/system/user-center", "/user/user-center"}).
		First(&menu).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := model.Now()
		menu = model.Menu{
			ParentID:   parentID,
			Title:      "用户列表",
			Path:       "/user/user-center",
			Component:  "/user/user-center",
			MenuType:   0,
			SortNumber: 1,
			Icon:       "IconProUserOutlined",
			CreateTime: now,
			UpdateTime: now,
		}
		if err := db.DB.Create(&menu).Error; err != nil {
			return fmt.Errorf("create user list menu: %w", err)
		}
		return grantMenuToAdminRole(&menu)
	}
	if err != nil {
		return fmt.Errorf("find user list menu: %w", err)
	}
	if menu.ParentID == parentID && menu.Path == "/user/user-center" &&
		menu.Title == "用户列表" {
		return nil
	}

	return db.DB.Model(&model.Menu{}).
		Where("menu_id = ?", menu.MenuID).
		Updates(map[string]any{
			"parent_id":   parentID,
			"title":       "用户列表",
			"path":        "/user/user-center",
			"component":   "/user/user-center",
			"sort_number": 1,
			"update_time": model.Now(),
		}).Error
}

// ensureMenuByPath 按路由地址幂等创建菜单，已存在时返回已有的菜单
func ensureMenuByPath(menu *model.Menu) (*model.Menu, error) {
	var existing model.Menu
	err := db.DB.Where("path = ?", menu.Path).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find menu %s: %w", menu.Path, err)
	}

	now := model.Now()
	menu.CreateTime = now
	menu.UpdateTime = now
	if err := db.DB.Create(menu).Error; err != nil {
		return nil, fmt.Errorf("create menu %s: %w", menu.Path, err)
	}
	return menu, nil
}

// grantMenuToAdminRole 给管理员角色授权菜单，已授权时跳过
func grantMenuToAdminRole(menu *model.Menu) error {
	var adminRole model.Role
	if err := db.DB.Where("role_code = ?", model.RoleCodeAdmin).First(&adminRole).Error; err != nil {
		return fmt.Errorf("find admin role: %w", err)
	}

	var count int64
	if err := db.DB.Model(&model.RoleMenu{}).
		Where("role_id = ? and menu_id = ?", adminRole.RoleID, menu.MenuID).
		Count(&count).Error; err != nil {
		return fmt.Errorf("count sys_role_menu: %w", err)
	}
	if count > 0 {
		return nil
	}
	if err := db.DB.Create(&model.RoleMenu{RoleID: adminRole.RoleID, MenuID: menu.MenuID}).Error; err != nil {
		return fmt.Errorf("create sys_role_menu: %w", err)
	}
	return nil
}

// buttonMenuSeed 描述要幂等补齐的按钮权限
type buttonMenuSeed struct {
	// 父菜单的路由地址
	parentPath string
	// 按钮名称
	title string
	// 权限标识
	authority string
	// 排序号
	sortNumber int
}

// ensureMenuByAuthority 按权限标识幂等创建按钮菜单；
// 按钮没有路由地址，不能按 path 判断是否已存在
func ensureMenuByAuthority(menu *model.Menu) (*model.Menu, bool, error) {
	var existing model.Menu
	err := db.DB.Where("authority = ? and menu_type = 1 and deleted = 0", menu.Authority).
		First(&existing).Error
	if err == nil {
		return &existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, fmt.Errorf("find menu %s: %w", menu.Authority, err)
	}

	now := model.Now()
	menu.CreateTime = now
	menu.UpdateTime = now
	if err := db.DB.Create(menu).Error; err != nil {
		return nil, false, fmt.Errorf("create menu %s: %w", menu.Authority, err)
	}
	return menu, true, nil
}

// ensureButtonMenus 幂等补齐日志删除等按钮权限，兼容已初始化过的库；
// 返回本次新建的按钮数量，只有新建过按钮时才需要回填历史角色的授权
func ensureButtonMenus() (int, error) {
	seeds := []buttonMenuSeed{
		{parentPath: "/system/login-record", title: "删除登录日志", authority: "sys:login-record:remove", sortNumber: 1},
		{parentPath: "/system/operation-record", title: "删除操作日志", authority: "sys:operation-record:remove", sortNumber: 1},
		{parentPath: "/user/login-record", title: "删除登录日志", authority: "user:login-record:remove", sortNumber: 1},
		{parentPath: "/user/operation-record", title: "删除操作日志", authority: "user:operation-record:remove", sortNumber: 1},
	}

	created := 0
	for _, seed := range seeds {
		var parent model.Menu
		err := db.DB.Where("path = ? and deleted = 0", seed.parentPath).First(&parent).Error
		if err != nil {
			// 父菜单被删除过时跳过，避免迁移整体失败
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Logger.Debug("skip button menu, parent not found",
					zap.String("path", seed.parentPath))
				continue
			}
			return created, fmt.Errorf("find menu %s: %w", seed.parentPath, err)
		}

		menu, isNew, err := ensureMenuByAuthority(&model.Menu{
			ParentID:   parent.MenuID,
			Title:      seed.title,
			MenuType:   1,
			SortNumber: seed.sortNumber,
			Authority:  seed.authority,
		})
		if err != nil {
			return created, err
		}
		if !isNew {
			continue
		}
		created++
		if err := grantMenuToAdminRole(menu); err != nil {
			return created, err
		}
	}
	return created, nil
}

// backfillRoleButtonMenus 一次性回填历史角色的按钮授权：
// 先补齐已授权菜单的祖先菜单，再把已授权菜单下的按钮补授，
// 使按钮权限启用前"所有角色都能操作"的实际情况延续到授权数据中
func backfillRoleButtonMenus() error {
	var menus []model.Menu
	if err := db.DB.Where("deleted = 0").Find(&menus).Error; err != nil {
		return fmt.Errorf("list menus: %w", err)
	}
	menuMap := make(map[int64]model.Menu, len(menus))
	for _, menu := range menus {
		menuMap[menu.MenuID] = menu
	}

	// 管理员角色返回全部菜单，不需要授权数据，跳过
	var roles []model.Role
	if err := db.DB.Where("deleted = 0 and role_code <> ?", model.RoleCodeAdmin).
		Find(&roles).Error; err != nil {
		return fmt.Errorf("list roles: %w", err)
	}
	for _, role := range roles {
		if err := backfillRoleMenu(role.RoleID, menus, menuMap); err != nil {
			return fmt.Errorf("backfill role %s: %w", role.RoleCode, err)
		}
	}
	if len(roles) > 0 {
		log.Logger.Debug("backfill role button menus", zap.Int("roles", len(roles)))
	}
	return nil
}

// backfillRoleMenu 回填单个角色的菜单授权
func backfillRoleMenu(roleID int64, menus []model.Menu, menuMap map[int64]model.Menu) error {
	var menuIDs []int64
	if err := db.DB.Model(&model.RoleMenu{}).Where("role_id = ?", roleID).
		Pluck("menu_id", &menuIDs).Error; err != nil {
		return err
	}
	if len(menuIDs) == 0 {
		return nil
	}

	granted := make(map[int64]bool, len(menuIDs))
	for _, menuID := range menuIDs {
		granted[menuID] = true
	}
	// 补齐祖先菜单，避免出现有按钮却看不到菜单的授权
	for _, menuID := range menuIDs {
		parentID := menuMap[menuID].ParentID
		for parentID != 0 {
			granted[parentID] = true
			parent, ok := menuMap[parentID]
			if !ok {
				break
			}
			parentID = parent.ParentID
		}
	}
	// 已授权的菜单，其下的按钮一并授权
	for _, menu := range menus {
		if menu.MenuType == 1 && menu.ParentID != 0 && granted[menu.ParentID] {
			granted[menu.MenuID] = true
		}
	}
	return insertRoleMenus(roleID, granted)
}

// insertRoleMenus 幂等写入角色的菜单授权，已存在的授权跳过
func insertRoleMenus(roleID int64, granted map[int64]bool) error {
	var existing []int64
	if err := db.DB.Model(&model.RoleMenu{}).Where("role_id = ?", roleID).
		Pluck("menu_id", &existing).Error; err != nil {
		return err
	}
	exists := make(map[int64]bool, len(existing))
	for _, menuID := range existing {
		exists[menuID] = true
	}

	list := make([]model.RoleMenu, 0, len(granted))
	for menuID := range granted {
		if exists[menuID] {
			continue
		}
		list = append(list, model.RoleMenu{RoleID: roleID, MenuID: menuID})
	}
	if len(list) == 0 {
		return nil
	}
	return db.DB.Create(&list).Error
}
