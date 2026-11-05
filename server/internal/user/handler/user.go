package handler

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/user/service"
	"admin/server/model"
)

// usernamePattern 是账号格式校验用的正则
var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{4,20}$`)

// userService 是前台用户的业务处理
var userService = service.NewUserService()

// UserPage 分页查询用户
func UserPage(c *gin.Context) {
	var param model.UserParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := userService.Page(param, param.PageParam)
	if err != nil {
		common.ServerError(c, "查询用户失败")
		return
	}
	common.Page(c, list, count)
}

// UserDetail 按 id 查询用户
func UserDetail(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "用户id不正确")
		return
	}

	user, err := userService.GetByID(id)
	if err != nil {
		handleUserError(c, err, "查询用户失败")
		return
	}
	common.Success(c, user)
}

// UserAdd 新增用户
func UserAdd(c *gin.Context) {
	var form model.UserForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if msg := checkUserForm(form, true); msg != "" {
		common.BadRequest(c, msg)
		return
	}

	if err := userService.Add(form); err != nil {
		handleUserError(c, err, "添加用户失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// UserUpdate 修改用户
func UserUpdate(c *gin.Context) {
	var form model.UserForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.ID == 0 {
		common.BadRequest(c, "用户id不能为空")
		return
	}
	if msg := checkUserForm(form, false); msg != "" {
		common.BadRequest(c, msg)
		return
	}

	if err := userService.Update(form); err != nil {
		handleUserError(c, err, "修改用户失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// UserRemove 按 id 删除用户
func UserRemove(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "用户id不正确")
		return
	}

	if err := userService.Remove([]int64{id}); err != nil {
		handleUserError(c, err, "删除用户失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// UserRemoveBatch 批量删除用户
func UserRemoveBatch(c *gin.Context) {
	var ids []int64
	if err := c.ShouldBindJSON(&ids); err != nil || len(ids) == 0 {
		common.BadRequest(c, "请选择要删除的用户")
		return
	}

	if err := userService.Remove(ids); err != nil {
		handleUserError(c, err, "删除用户失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// UserUpdateStatus 修改用户状态
func UserUpdateStatus(c *gin.Context) {
	var form model.UserStatusForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.ID == 0 {
		common.BadRequest(c, "用户id不能为空")
		return
	}
	if form.Status != 0 && form.Status != 1 {
		common.BadRequest(c, "用户状态不正确")
		return
	}

	if err := userService.UpdateStatus(form.ID, form.Status); err != nil {
		handleUserError(c, err, "修改用户状态失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// UserUpdatePassword 重置用户密码
func UserUpdatePassword(c *gin.Context) {
	var form model.UserPasswordForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.ID == 0 {
		common.BadRequest(c, "用户id不能为空")
		return
	}
	if len(strings.TrimSpace(form.Password)) < 5 {
		common.BadRequest(c, "密码长度不能少于5位")
		return
	}

	if err := userService.UpdatePassword(form.ID, form.Password); err != nil {
		handleUserError(c, err, "重置密码失败")
		return
	}
	common.SuccessWithMessage(c, "重置成功", nil)
}

// UserExistence 检查账号是否已存在
func UserExistence(c *gin.Context) {
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

	exists, err := userService.ExistsByUsername(value, excludeID)
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

// parseID 把路径参数解析为数字 id
func parseID(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

// checkUserForm 校验用户提交数据，返回空字符串表示校验通过
func checkUserForm(form model.UserForm, isAdd bool) string {
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
	case errors.Is(err, service.ErrNicknameRequired):
		common.BadRequest(c, service.ErrNicknameRequired.Error())
	default:
		common.ServerError(c, message)
	}
}
