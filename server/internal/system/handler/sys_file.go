package handler

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/system/service"
	"admin/server/model"
)

// sysFileService 是文件上传记录与用户文件的业务处理
var sysFileService = service.NewSysFileService()

// handleFileError 按错误类型输出文件相关的响应
func handleFileError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrFileNotFound),
		errors.Is(err, service.ErrUserFileNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrFileEmpty),
		errors.Is(err, service.ErrFileTooLarge),
		errors.Is(err, service.ErrFileNameRequired):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// SysFileUpload 上传文件
func SysFileUpload(c *gin.Context) {
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

	userID, _ := common.CurrentUserID(c)
	record, err := sysFileService.Save(header.Filename, file, header.Size, header.Header.Get("Content-Type"), userID)
	if err != nil {
		handleFileError(c, err, "上传文件失败")
		return
	}
	common.SuccessWithMessage(c, "上传成功", record)
}

// SysFileUploadBase64 上传 base64 文件
func SysFileUploadBase64(c *gin.Context) {
	var form struct {
		// base64 数据，可带 data:image/png;base64, 前缀
		Base64 string `json:"base64"`
		// 文件名称
		FileName string `json:"fileName"`
	}
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if strings.TrimSpace(form.Base64) == "" {
		common.BadRequest(c, "文件内容不能为空")
		return
	}

	userID, _ := common.CurrentUserID(c)
	record, err := sysFileService.SaveBase64(form.Base64, form.FileName, userID)
	if err != nil {
		handleFileError(c, err, "上传文件失败")
		return
	}
	common.SuccessWithMessage(c, "上传成功", record)
}

// SysFilePage 分页查询文件记录
func SysFilePage(c *gin.Context) {
	var param model.FileRecordParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := sysFileService.Page(param, param.PageParam)
	if err != nil {
		common.ServerError(c, "查询文件失败")
		return
	}
	common.Page(c, list, count)
}

// SysFileList 查询文件记录列表
func SysFileList(c *gin.Context) {
	var param model.FileRecordParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, err := sysFileService.List(param)
	if err != nil {
		common.ServerError(c, "查询文件失败")
		return
	}
	common.Success(c, list)
}

// SysFileRemove 按 id 删除文件
func SysFileRemove(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "文件id不正确")
		return
	}

	if err := sysFileService.Remove([]int64{id}); err != nil {
		handleFileError(c, err, "删除文件失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// SysFileRemoveBatch 批量删除文件
func SysFileRemoveBatch(c *gin.Context) {
	ids, ok := parseIDs(c)
	if !ok {
		return
	}

	if err := sysFileService.Remove(ids); err != nil {
		handleFileError(c, err, "删除文件失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// SysFilePath 读取磁盘上的文件，支持 download 直接下载
func SysFilePath(c *gin.Context) {
	relative := strings.TrimPrefix(c.Param("path"), "/")
	if relative == "" {
		common.NotFound(c, "文件不存在")
		return
	}

	clean := filepath.Clean(filepath.FromSlash(relative))
	fullPath := filepath.Join(service.UploadDir, clean)
	if _, err := os.Stat(fullPath); err != nil {
		common.NotFound(c, "文件不存在")
		return
	}

	// 下载地址直接指定文件名，其余情况由浏览器按类型展示
	if c.GetBool("download") {
		fileName := filepath.Base(fullPath)
		c.Header("Content-Disposition", "attachment; filename=\""+url.PathEscape(fileName)+"\"")
	}
	c.File(fullPath)
}

// SysUserFilePage 分页查询用户文件
func SysUserFilePage(c *gin.Context) {
	var param model.UserFileParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := sysFileService.PageUserFiles(param, param.PageParam)
	if err != nil {
		common.ServerError(c, "查询用户文件失败")
		return
	}
	common.Page(c, list, count)
}

// SysUserFileList 查询用户文件列表
func SysUserFileList(c *gin.Context) {
	var param model.UserFileParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, err := sysFileService.ListUserFiles(param)
	if err != nil {
		common.ServerError(c, "查询用户文件失败")
		return
	}
	common.Success(c, list)
}

// SysUserFileAdd 新增用户文件
func SysUserFileAdd(c *gin.Context) {
	var form model.UserFileForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	if err := sysFileService.AddUserFile(userID, form); err != nil {
		handleFileError(c, err, "添加用户文件失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// SysUserFileUpdate 修改用户文件
func SysUserFileUpdate(c *gin.Context) {
	var form model.UserFileForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.ID == 0 {
		common.BadRequest(c, "文件id不能为空")
		return
	}

	if err := sysFileService.UpdateUserFile(form); err != nil {
		handleFileError(c, err, "修改用户文件失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// SysUserFileRemove 按 id 删除用户文件
func SysUserFileRemove(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "文件id不正确")
		return
	}

	if err := sysFileService.RemoveUserFiles([]int64{id}); err != nil {
		handleFileError(c, err, "删除用户文件失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// SysUserFileRemoveBatch 批量删除用户文件
func SysUserFileRemoveBatch(c *gin.Context) {
	ids, ok := parseIDs(c)
	if !ok {
		return
	}

	if err := sysFileService.RemoveUserFiles(ids); err != nil {
		handleFileError(c, err, "删除用户文件失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}
