package handler

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/system/service"
	"admin/server/model"
	"admin/server/pkg/captcha"
)

// SysUserLoginForm 是登录提交的数据
type SysUserLoginForm struct {
	// 账号
	Username string `json:"username"`
	// 密码
	Password string `json:"password"`
	// 图形验证码
	Code string `json:"code"`
	// 图形验证码 key
	CaptchaKey string `json:"captchaKey"`
	// 是否记住密码
	Remember bool `json:"remember"`
}

// SysUserLogin 账号密码登录，登录前先校验图形验证码
func SysUserLogin(c *gin.Context) {
	var form SysUserLoginForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	form.Username = strings.TrimSpace(form.Username)
	form.Code = strings.TrimSpace(form.Code)
	if form.Username == "" {
		common.BadRequest(c, "账号不能为空")
		return
	}
	if form.Password == "" {
		common.BadRequest(c, "密码不能为空")
		return
	}
	if form.CaptchaKey == "" || form.Code == "" {
		common.BadRequest(c, "请输入图形验证码")
		return
	}

	// 验证码只能用一次，校验失败也要重新获取
	if !captcha.Verify(form.CaptchaKey, form.Code) {
		writeLoginRecord(c, form.Username, 0, "图形验证码错误", model.LoginTypeFailed)
		common.BadRequest(c, "图形验证码错误或已过期")
		return
	}

	result, err := sysUserService.Login(form.Username, form.Password)
	if err != nil {
		writeLoginRecord(c, form.Username, 0, err.Error(), model.LoginTypeFailed)
		if errors.Is(err, service.ErrPasswordIncorrect) {
			common.BadRequest(c, service.ErrPasswordIncorrect.Error())
			return
		}
		if errors.Is(err, service.ErrUserDisabled) {
			common.BadRequest(c, service.ErrUserDisabled.Error())
			return
		}
		common.ServerError(c, "登录失败")
		return
	}
	if result.User != nil {
		writeLoginRecord(c, result.User.Username, result.User.UserID, "登录成功", model.LoginTypeSuccess)
	}
	common.SuccessWithMessage(c, "登录成功", result)
}

// SysCaptcha 获取图形验证码，开发环境会同时返回验证码文本
func SysCaptcha(c *gin.Context) {
	result, err := captcha.Generate()
	if err != nil {
		common.ServerError(c, "获取图形验证码失败")
		return
	}
	common.Success(c, result)
}

// SysAuthUser 获取当前登录用户的个人信息、角色与权限
func SysAuthUser(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "登录状态已失效，请重新登录")
		return
	}

	user, err := sysUserService.GetByID(userID)
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

// SysUserLogout 退出登录，token 本身无状态，前端清除本地 token 即可
func SysUserLogout(c *gin.Context) {
	userID, _ := common.CurrentUserID(c)
	writeLoginRecord(c, common.CurrentUsername(c), userID, "退出登录", model.LoginTypeLogout)
	common.SuccessWithMessage(c, "退出登录成功", nil)
}

// writeLoginRecord 写入登录日志，失败不影响响应
func writeLoginRecord(c *gin.Context, username string, userID int64, comments string, loginType int) {
	userAgent := c.GetHeader("User-Agent")
	record := &model.SysLoginRecord{
		UserID:    userID,
		Username:  username,
		Nickname:  username,
		IP:        c.ClientIP(),
		Device:    common.ParseDevice(userAgent),
		OS:        common.ParseOS(userAgent),
		Browser:   common.ParseBrowser(userAgent),
		LoginType: loginType,
		Comments:  comments,
	}
	if userID > 0 {
		if user, err := sysUserService.GetByID(userID); err == nil {
			record.Nickname = user.Nickname
		}
	}
	_ = sysRecordService.AddLoginRecord(record)
}

// SysUserInfo 修改当前登录用户的个人信息
func SysUserInfo(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "登录状态已失效，请重新登录")
		return
	}

	var form model.SysUserInfoForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	user, err := sysUserService.UpdateInfo(userID, form)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			common.NotFound(c, service.ErrUserNotFound.Error())
			return
		}
		common.ServerError(c, "修改个人信息失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", user)
}

// SysAuthPassword 修改当前登录用户的密码
func SysAuthPassword(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "登录状态已失效，请重新登录")
		return
	}

	var form model.SysUserOwnPasswordForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := sysUserService.UpdateOwnPassword(userID, form.OldPassword, form.Password); err != nil {
		if errors.Is(err, service.ErrPasswordIncorrect) {
			common.BadRequest(c, "原密码错误")
			return
		}
		if errors.Is(err, service.ErrPasswordRequired) {
			common.BadRequest(c, service.ErrPasswordRequired.Error())
			return
		}
		common.ServerError(c, "修改密码失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}
