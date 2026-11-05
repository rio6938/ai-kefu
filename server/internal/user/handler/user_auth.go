package handler

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/user/service"
	"admin/server/model"
)

// UserRegisterForm 是注册提交的数据
type UserRegisterForm struct {
	// 账号
	Username string `json:"username"`
	// 密码
	Password string `json:"password"`
	// 昵称
	Nickname string `json:"nickname"`
}

// UserLoginForm 是登录提交的数据
type UserLoginForm struct {
	// 账号
	Username string `json:"username"`
	// 密码
	Password string `json:"password"`
}

// UserRegister 注册前台用户，不校验邮箱和手机号
func UserRegister(c *gin.Context) {
	var form UserRegisterForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	form.Username = strings.TrimSpace(form.Username)
	form.Nickname = strings.TrimSpace(form.Nickname)
	if form.Username == "" {
		common.BadRequest(c, "账号不能为空")
		return
	}
	if !usernamePattern.MatchString(form.Username) {
		common.BadRequest(c, "账号为4-20位字母、数字或下划线")
		return
	}
	if len(strings.TrimSpace(form.Password)) < 5 {
		common.BadRequest(c, "密码长度不能少于5位")
		return
	}
	if form.Nickname == "" {
		common.BadRequest(c, "昵称不能为空")
		return
	}

	user, err := userService.Register(form.Username, form.Password, form.Nickname)
	if err != nil {
		if errors.Is(err, service.ErrUsernameExists) {
			common.BadRequest(c, service.ErrUsernameExists.Error())
			return
		}
		if errors.Is(err, service.ErrPasswordRequired) || errors.Is(err, service.ErrNicknameRequired) {
			common.BadRequest(c, err.Error())
			return
		}
		common.ServerError(c, "注册失败")
		return
	}
	writeUserLoginRecord(c, user.Username, user.ID, user.Nickname, "注册成功", model.LoginTypeRegister)
	common.SuccessWithMessage(c, "注册成功", nil)
}

// UserLogin 账号密码登录，签发前台用户 token
func UserLogin(c *gin.Context) {
	var form UserLoginForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	form.Username = strings.TrimSpace(form.Username)
	if form.Username == "" {
		common.BadRequest(c, "账号不能为空")
		return
	}
	if form.Password == "" {
		common.BadRequest(c, "密码不能为空")
		return
	}

	result, err := userService.Login(form.Username, form.Password)
	if err != nil {
		writeUserLoginRecord(c, form.Username, 0, "", err.Error(), model.LoginTypeFailed)
		if errors.Is(err, service.ErrPasswordIncorrect) || errors.Is(err, service.ErrUserDisabled) {
			common.BadRequest(c, err.Error())
			return
		}
		common.ServerError(c, "登录失败")
		return
	}
	if result.User != nil {
		writeUserLoginRecord(c, result.User.Username, result.User.ID, result.User.Nickname, "登录成功", model.LoginTypeSuccess)
	}
	common.SuccessWithMessage(c, "登录成功", result)
}

// UserAuthUser 获取当前登录的前台用户信息
func UserAuthUser(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "登录状态已失效，请重新登录")
		return
	}

	user, err := userService.GetByID(userID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			common.Unauthorized(c, "登录状态已失效，请重新登录")
			return
		}
		common.ServerError(c, "获取用户信息失败")
		return
	}
	common.Success(c, user)
}

// UserLogout 退出登录，token 本身无状态，前端清除本地 token 即可
func UserLogout(c *gin.Context) {
	userID, _ := common.CurrentUserID(c)
	writeUserLoginRecord(c, common.CurrentUsername(c), userID, "", "退出登录", model.LoginTypeLogout)
	common.SuccessWithMessage(c, "退出登录成功", nil)
}

// writeUserLoginRecord 写入前台用户登录日志，失败不影响响应
func writeUserLoginRecord(c *gin.Context, username string, userID int64, nickname, comments string, loginType int) {
	userAgent := c.GetHeader("User-Agent")
	record := &model.UserLoginRecord{
		UserID:    userID,
		Username:  username,
		Nickname:  nickname,
		IP:        c.ClientIP(),
		Device:    common.ParseDevice(userAgent),
		OS:        common.ParseOS(userAgent),
		Browser:   common.ParseBrowser(userAgent),
		LoginType: loginType,
		Comments:  comments,
	}
	if record.Nickname == "" {
		record.Nickname = username
	}
	_ = userRecordService.AddLoginRecord(record)
}
