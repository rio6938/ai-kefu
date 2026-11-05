package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/system/service"
	"admin/server/model"
)

// sysRbacService 是角色、菜单与权限的业务处理
var sysRbacService = service.NewSysRbacService()

// parseID 把路径参数解析为数字 id
func parseID(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

// parseIDs 解析请求体中的 id 数组
func parseIDs(c *gin.Context) ([]int64, bool) {
	var ids []int64
	if err := c.ShouldBindJSON(&ids); err != nil || len(ids) == 0 {
		common.BadRequest(c, "请选择要操作的数据")
		return nil, false
	}
	return ids, true
}

// handleRoleError 按错误类型输出角色相关的响应
func handleRoleError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrRoleNotFound):
		common.NotFound(c, service.ErrRoleNotFound.Error())
	case errors.Is(err, service.ErrRoleCodeExists),
		errors.Is(err, service.ErrRoleCodeRequired),
		errors.Is(err, service.ErrRoleNameRequired),
		errors.Is(err, service.ErrAdminRoleCannotRemove):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// handleMenuError 按错误类型输出菜单相关的响应
func handleMenuError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrMenuNotFound):
		common.NotFound(c, service.ErrMenuNotFound.Error())
	case errors.Is(err, service.ErrMenuTitleRequired),
		errors.Is(err, service.ErrMenuHasChildren),
		errors.Is(err, service.ErrMenuParentInvalid):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// SysRolePage 分页查询角色
func SysRolePage(c *gin.Context) {
	var param model.RoleParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := sysRbacService.PageRoles(param, param.PageParam)
	if err != nil {
		common.ServerError(c, "查询角色失败")
		return
	}
	common.Page(c, list, count)
}

// SysRoleList 查询角色列表
func SysRoleList(c *gin.Context) {
	var param model.RoleParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, err := sysRbacService.ListRoles(param)
	if err != nil {
		common.ServerError(c, "查询角色失败")
		return
	}
	common.Success(c, list)
}

// SysRoleAdd 新增角色
func SysRoleAdd(c *gin.Context) {
	var form model.RoleForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := sysRbacService.AddRole(form); err != nil {
		handleRoleError(c, err, "添加角色失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// SysRoleUpdate 修改角色
func SysRoleUpdate(c *gin.Context) {
	var form model.RoleForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.RoleID == 0 {
		common.BadRequest(c, "角色id不能为空")
		return
	}

	if err := sysRbacService.UpdateRole(form); err != nil {
		handleRoleError(c, err, "修改角色失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// SysRoleRemove 按 id 删除角色
func SysRoleRemove(c *gin.Context) {
	roleID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "角色id不正确")
		return
	}

	if err := sysRbacService.RemoveRoles([]int64{roleID}); err != nil {
		handleRoleError(c, err, "删除角色失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// SysRoleRemoveBatch 批量删除角色
func SysRoleRemoveBatch(c *gin.Context) {
	roleIDs, ok := parseIDs(c)
	if !ok {
		return
	}

	if err := sysRbacService.RemoveRoles(roleIDs); err != nil {
		handleRoleError(c, err, "删除角色失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// SysRoleMenuList 查询角色的菜单授权树
func SysRoleMenuList(c *gin.Context) {
	roleID, err := parseID(c.Param("roleId"))
	if err != nil {
		common.BadRequest(c, "角色id不正确")
		return
	}

	list, err := sysRbacService.ListRoleMenus(roleID)
	if err != nil {
		handleRoleError(c, err, "查询角色权限失败")
		return
	}
	common.Success(c, list)
}

// SysRoleMenuUpdate 修改角色的菜单授权
func SysRoleMenuUpdate(c *gin.Context) {
	roleID, err := parseID(c.Param("roleId"))
	if err != nil {
		common.BadRequest(c, "角色id不正确")
		return
	}

	var menuIDs []int64
	if err := c.ShouldBindJSON(&menuIDs); err != nil {
		menuIDs = []int64{}
	}

	if err := sysRbacService.UpdateRoleMenus(roleID, menuIDs); err != nil {
		handleRoleError(c, err, "保存角色权限失败")
		return
	}
	common.SuccessWithMessage(c, "保存成功", nil)
}

// SysMenuPage 分页查询菜单
func SysMenuPage(c *gin.Context) {
	var param model.MenuParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := sysRbacService.PageMenus(param, param.PageParam)
	if err != nil {
		common.ServerError(c, "查询菜单失败")
		return
	}
	common.Page(c, list, count)
}

// SysMenuList 查询菜单树
func SysMenuList(c *gin.Context) {
	var param model.MenuParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, err := sysRbacService.ListMenus(param)
	if err != nil {
		common.ServerError(c, "查询菜单失败")
		return
	}
	common.Success(c, list)
}

// SysMenuAdd 新增菜单
func SysMenuAdd(c *gin.Context) {
	var form model.MenuForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := sysRbacService.AddMenu(form); err != nil {
		handleMenuError(c, err, "添加菜单失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// SysMenuUpdate 修改菜单
func SysMenuUpdate(c *gin.Context) {
	var form model.MenuForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.MenuID == 0 {
		common.BadRequest(c, "菜单id不能为空")
		return
	}

	if err := sysRbacService.UpdateMenu(form); err != nil {
		handleMenuError(c, err, "修改菜单失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// SysMenuRemove 按 id 删除菜单
func SysMenuRemove(c *gin.Context) {
	menuID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "菜单id不正确")
		return
	}

	if err := sysRbacService.RemoveMenu(menuID); err != nil {
		handleMenuError(c, err, "删除菜单失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}
