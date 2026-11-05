package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	systemservice "admin/server/internal/system/service"
)

// kefuFileService 是访客附件的业务处理
var kefuFileService = service.NewKefuFileService()

// handleFileError 按错误类型输出访客附件相关的响应
func handleFileError(c *gin.Context, err error, fallback string) {
	var sizeErr *service.KefuFileSizeError
	switch {
	case errors.As(err, &sizeErr),
		errors.Is(err, service.ErrFileDisabled),
		errors.Is(err, service.ErrFileExtNotAllowed),
		errors.Is(err, service.ErrFileIPDailyLimited),
		errors.Is(err, systemservice.ErrFileEmpty),
		errors.Is(err, systemservice.ErrFileTooLarge):
		// 限额类错误的中文提示由 service 生成，直接输出
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuFileUpload 访客上传附件，聊天窗免登录调用；
// 校验顺序为开关 → 后缀 → 单文件大小 → 单 IP 当日累计
func KefuFileUpload(c *gin.Context) {
	header, err := c.FormFile("file")
	if err != nil {
		common.BadRequest(c, "请选择要上传的文件")
		return
	}

	file, err := header.Open()
	if err != nil {
		common.ServerError(c, "读取上传文件失败")
		return
	}
	defer file.Close()

	result, err := kefuFileService.Upload(
		c.ClientIP(), header.Filename, file, header.Size,
		header.Header.Get("Content-Type"))
	if err != nil {
		handleFileError(c, err, "上传附件失败")
		return
	}
	common.SuccessWithMessage(c, "上传成功", result)
}

// KefuFilePath 读取访客上传的附件，聊天窗免登录访问；
// 附件路径含随机名且限制后缀白名单，不做逐条鉴权（与公开附件的行为一致）
func KefuFilePath(c *gin.Context) {
	fullPath, err := kefuFileService.RawPath(c.Param("path"))
	if err != nil {
		common.NotFound(c, "文件不存在")
		return
	}
	c.File(fullPath)
}
