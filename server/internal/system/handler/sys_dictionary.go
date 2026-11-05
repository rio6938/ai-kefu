package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/system/service"
	"admin/server/model"
)

// sysDictionaryService 是字典与字典数据的业务处理
var sysDictionaryService = service.NewSysDictionaryService()

// handleDictError 按错误类型输出字典相关的响应
func handleDictError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrDictionaryNotFound),
		errors.Is(err, service.ErrDictionaryDataNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrDictCodeRequired),
		errors.Is(err, service.ErrDictNameRequired),
		errors.Is(err, service.ErrDictCodeExists),
		errors.Is(err, service.ErrDictDataCodeRequired),
		errors.Is(err, service.ErrDictDataNameRequired),
		errors.Is(err, service.ErrDictDataCodeExists):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// SysDictionaryPage 分页查询字典
func SysDictionaryPage(c *gin.Context) {
	var param model.DictionaryParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := sysDictionaryService.Page(param, param.PageParam)
	if err != nil {
		common.ServerError(c, "查询字典失败")
		return
	}
	common.Page(c, list, count)
}

// SysDictionaryList 查询字典列表
func SysDictionaryList(c *gin.Context) {
	var param model.DictionaryParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, err := sysDictionaryService.List(param)
	if err != nil {
		common.ServerError(c, "查询字典失败")
		return
	}
	common.Success(c, list)
}

// SysDictionaryAdd 新增字典
func SysDictionaryAdd(c *gin.Context) {
	var form model.DictionaryForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := sysDictionaryService.Add(form); err != nil {
		handleDictError(c, err, "添加字典失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// SysDictionaryUpdate 修改字典
func SysDictionaryUpdate(c *gin.Context) {
	var form model.DictionaryForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.DictID == 0 {
		common.BadRequest(c, "字典id不能为空")
		return
	}

	if err := sysDictionaryService.Update(form); err != nil {
		handleDictError(c, err, "修改字典失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// SysDictionaryRemove 按 id 删除字典
func SysDictionaryRemove(c *gin.Context) {
	dictID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "字典id不正确")
		return
	}

	if err := sysDictionaryService.Remove(dictID); err != nil {
		handleDictError(c, err, "删除字典失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// SysDictionaryDataPage 分页查询字典数据
func SysDictionaryDataPage(c *gin.Context) {
	var param model.DictionaryDataParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := sysDictionaryService.PageData(param, param.PageParam)
	if err != nil {
		common.ServerError(c, "查询字典数据失败")
		return
	}
	common.Page(c, list, count)
}

// SysDictionaryDataList 查询字典数据列表
func SysDictionaryDataList(c *gin.Context) {
	var param model.DictionaryDataParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, err := sysDictionaryService.ListData(param)
	if err != nil {
		common.ServerError(c, "查询字典数据失败")
		return
	}
	common.Success(c, list)
}

// SysDictionaryDataAdd 新增字典数据
func SysDictionaryDataAdd(c *gin.Context) {
	var form model.DictionaryDataForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := sysDictionaryService.AddData(form); err != nil {
		handleDictError(c, err, "添加字典数据失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// SysDictionaryDataUpdate 修改字典数据
func SysDictionaryDataUpdate(c *gin.Context) {
	var form model.DictionaryDataForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.DictDataID == 0 {
		common.BadRequest(c, "字典数据id不能为空")
		return
	}

	if err := sysDictionaryService.UpdateData(form); err != nil {
		handleDictError(c, err, "修改字典数据失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// SysDictionaryDataRemove 按 id 删除字典数据
func SysDictionaryDataRemove(c *gin.Context) {
	dictDataID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "字典数据id不正确")
		return
	}

	if err := sysDictionaryService.RemoveData(dictDataID); err != nil {
		handleDictError(c, err, "删除字典数据失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// SysDictionaryDataRemoveBatch 批量删除字典数据
func SysDictionaryDataRemoveBatch(c *gin.Context) {
	dictDataIDs, ok := parseIDs(c)
	if !ok {
		return
	}

	if err := sysDictionaryService.RemoveDataBatch(dictDataIDs); err != nil {
		common.ServerError(c, "删除字典数据失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}
