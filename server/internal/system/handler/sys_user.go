package handler

import (
	"errors"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/system/service"
	"admin/server/model"
)

// usernamePattern 是账号格式校验用的正则
var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{4,20}$`)

// sysUserService 是系统用户的业务处理
var sysUserService = service.NewSysUserService()

// SysUserPage 分页查询用户
func SysUserPage(c *gin.Context) {
	var param model.SysUserParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := sysUserService.Page(param, param.PageParam)
	if err != nil {
		common.ServerError(c, "查询用户失败")
		return
	}
	common.Page(c, list, count)
}

// SysUserList 查询用户列表
func SysUserList(c *gin.Context) {
	var param model.SysUserParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, err := sysUserService.List(param)
	if err != nil {
		common.ServerError(c, "查询用户失败")
		return
	}
	common.Success(c, list)
}

// SysUserDetail 按 id 查询用户
func SysUserDetail(c *gin.Context) {
	userID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "用户id不正确")
		return
	}

	user, err := sysUserService.GetByID(userID)
	if err != nil {
		handleUserError(c, err, "查询用户失败")
		return
	}
	common.Success(c, user)
}

// SysUserAdd 新增用户
func SysUserAdd(c *gin.Context) {
	var form model.SysUserForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if msg := checkUserForm(form, true); msg != "" {
		common.BadRequest(c, msg)
		return
	}

	if err := sysUserService.Add(form); err != nil {
		handleUserError(c, err, "添加用户失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// SysUserUpdate 修改用户
func SysUserUpdate(c *gin.Context) {
	var form model.SysUserForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.UserID == 0 {
		common.BadRequest(c, "用户id不能为空")
		return
	}
	if msg := checkUserForm(form, false); msg != "" {
		common.BadRequest(c, msg)
		return
	}

	if err := sysUserService.Update(form); err != nil {
		handleUserError(c, err, "修改用户失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// SysUserRemove 按 id 删除用户
func SysUserRemove(c *gin.Context) {
	userID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "用户id不正确")
		return
	}

	if err := sysUserService.Remove([]int64{userID}); err != nil {
		handleUserError(c, err, "删除用户失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// SysUserRemoveBatch 批量删除用户
func SysUserRemoveBatch(c *gin.Context) {
	var userIDs []int64
	if err := c.ShouldBindJSON(&userIDs); err != nil || len(userIDs) == 0 {
		common.BadRequest(c, "请选择要删除的用户")
		return
	}

	if err := sysUserService.Remove(userIDs); err != nil {
		handleUserError(c, err, "删除用户失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// SysUserUpdateStatus 修改用户状态
func SysUserUpdateStatus(c *gin.Context) {
	var form model.SysUserStatusForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.UserID == 0 {
		common.BadRequest(c, "用户id不能为空")
		return
	}
	if form.Status != 0 && form.Status != 1 {
		common.BadRequest(c, "用户状态不正确")
		return
	}

	if err := sysUserService.UpdateStatus(form.UserID, form.Status); err != nil {
		handleUserError(c, err, "修改用户状态失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// SysUserUpdatePassword 重置用户密码
func SysUserUpdatePassword(c *gin.Context) {
	var form model.SysUserPasswordForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.UserID == 0 {
		common.BadRequest(c, "用户id不能为空")
		return
	}
	if len(strings.TrimSpace(form.Password)) < 5 {
		common.BadRequest(c, "密码长度不能少于5位")
		return
	}

	if err := sysUserService.UpdatePassword(form.UserID, form.Password); err != nil {
		handleUserError(c, err, "重置密码失败")
		return
	}
	common.SuccessWithMessage(c, "重置成功", nil)
}

// SysUserExistence 检查用户字段是否已存在
func SysUserExistence(c *gin.Context) {
	field := c.Query("field")
	if field != "username" {
		common.BadRequest(c, "不支持的字段")
		return
	}

	value := c.Query("value")
	if value == "" {
		common.BadRequest(c, "字段值不能为空")
		return
	}

	var excludeID int64
	if id := c.Query("id"); id != "" {
		parsed, err := parseID(id)
		if err != nil {
			common.BadRequest(c, "用户id不正确")
			return
		}
		excludeID = parsed
	}

	exists, err := sysUserService.ExistsByUsername(value, excludeID)
	if err != nil {
		common.ServerError(c, "检查用户失败")
		return
	}
	if exists {
		common.Error(c, common.CodeBadRequest, "账号已经存在")
		return
	}
	common.SuccessWithMessage(c, "账号可用", nil)
}

// checkUserForm 校验用户提交数据，返回空字符串表示校验通过
func checkUserForm(form model.SysUserForm, isAdd bool) string {
	if strings.TrimSpace(form.Username) == "" {
		return "账号不能为空"
	}
	if !usernamePattern.MatchString(form.Username) {
		return "账号为4-20位字母、数字或下划线"
	}
	if strings.TrimSpace(form.Nickname) == "" {
		return "昵称不能为空"
	}
	if isAdd && len(strings.TrimSpace(form.Password)) < 5 {
		return "密码长度不能少于5位"
	}
	return ""
}

// handleUserError 按错误类型输出响应
func handleUserError(c *gin.Context, err error, message string) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		common.NotFound(c, service.ErrUserNotFound.Error())
	case errors.Is(err, service.ErrUsernameExists):
		common.BadRequest(c, service.ErrUsernameExists.Error())
	case errors.Is(err, service.ErrPasswordRequired):
		common.BadRequest(c, service.ErrPasswordRequired.Error())
	case errors.Is(err, service.ErrUserProtectedRemove),
		errors.Is(err, service.ErrUserProtectedDisable),
		errors.Is(err, service.ErrUserProtectedPassword),
		errors.Is(err, service.ErrUserProtectedUpdate):
		common.Forbidden(c, err.Error())
	default:
		common.ServerError(c, message)
	}
}
