package api

import (
	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/resp"
	"interastral-peace.com/alnitak/internal/service"
	"interastral-peace.com/alnitak/utils"
)

// ========== 认证类型 API ==========

// AddAuthType 添加认证类型
func AddAuthType(c *gin.Context) {
	var req dto.AddAuthTypeReq
	if err := c.ShouldBind(&req); err != nil {
		resp.FailWithMessage(c, "参数有误")
		return
	}

	if err := service.AddAuthType(c, req); err != nil {
		resp.FailWithMessage(c, err.Error())
		return
	}

	resp.Ok(c)
}

// EditAuthType 编辑认证类型
func EditAuthType(c *gin.Context) {
	var req dto.EditAuthTypeReq
	if err := c.ShouldBind(&req); err != nil {
		resp.FailWithMessage(c, "参数有误")
		return
	}

	if err := service.EditAuthType(c, req); err != nil {
		resp.FailWithMessage(c, err.Error())
		return
	}

	resp.Ok(c)
}

// DeleteAuthType 删除认证类型
func DeleteAuthType(c *gin.Context) {
	id := utils.StringToUint(c.Param("id"))
	if id == 0 {
		resp.FailWithMessage(c, "参数有误")
		return
	}

	if err := service.DeleteAuthType(c, id); err != nil {
		resp.FailWithMessage(c, err.Error())
		return
	}

	resp.Ok(c)
}

// GetAuthTypeList 获取认证类型列表（公开）
func GetAuthTypeList(c *gin.Context) {
	category := c.Query("category")
	list := service.GetAuthTypeList(category)
	resp.OkWithData(c, gin.H{"list": list})
}

// GetAllAuthTypeList 获取所有认证类型列表（管理用）
func GetAllAuthTypeList(c *gin.Context) {
	page := utils.StringToInt(c.Query("page"))
	pageSize := utils.StringToInt(c.Query("pageSize"))
	if pageSize > 30 {
		pageSize = 30
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if page <= 0 {
		page = 1
	}

	total, list := service.GetAllAuthTypeList(page, pageSize)
	resp.OkWithData(c, gin.H{
		"total": total,
		"list":  list,
	})
}

// GetAuthTypeByID 获取认证类型详情
func GetAuthTypeByID(c *gin.Context) {
	id := utils.StringToUint(c.Param("id"))
	if id == 0 {
		resp.FailWithMessage(c, "参数有误")
		return
	}

	authType, err := service.GetAuthTypeByID(id)
	if err != nil {
		resp.FailWithMessage(c, err.Error())
		return
	}

	resp.OkWithData(c, authType)
}

// ========== 用户认证 API ==========

// AddUserAuth 添加用户认证
func AddUserAuth(c *gin.Context) {
	var req dto.AddUserAuthReq
	if err := c.ShouldBind(&req); err != nil {
		resp.FailWithMessage(c, "参数有误")
		return
	}

	if err := service.AddUserAuth(c, req); err != nil {
		resp.FailWithMessage(c, err.Error())
		return
	}

	resp.Ok(c)
}

// EditUserAuth 编辑用户认证
func EditUserAuth(c *gin.Context) {
	var req dto.EditUserAuthReq
	if err := c.ShouldBind(&req); err != nil {
		resp.FailWithMessage(c, "参数有误")
		return
	}

	if err := service.EditUserAuth(c, req); err != nil {
		resp.FailWithMessage(c, err.Error())
		return
	}

	resp.Ok(c)
}

// DeleteUserAuth 删除用户认证
func DeleteUserAuth(c *gin.Context) {
	var req dto.DeleteUserAuthReq
	if err := c.ShouldBind(&req); err != nil {
		resp.FailWithMessage(c, "参数有误")
		return
	}

	if err := service.DeleteUserAuth(c, req); err != nil {
		resp.FailWithMessage(c, err.Error())
		return
	}

	resp.Ok(c)
}

// GetUserAuthList 获取用户认证列表（公开，用户自己查看）
func GetUserAuthList(c *gin.Context) {
	uid := utils.StringToUint(c.Query("uid"))
	if uid == 0 {
		resp.FailWithMessage(c, "参数有误")
		return
	}

	list := service.GetUserAuthList(uid)
	resp.OkWithData(c, gin.H{"list": list})
}

// GetUserAuthListWithUser 获取用户认证列表（管理用，带用户信息）
func GetUserAuthListWithUser(c *gin.Context) {
	page := utils.StringToInt(c.Query("page"))
	pageSize := utils.StringToInt(c.Query("pageSize"))
	authTypeID := utils.StringToUint(c.Query("authTypeId"))

	if pageSize > 30 {
		pageSize = 30
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if page <= 0 {
		page = 1
	}

	total, list := service.GetUserAuthListWithUser(page, pageSize, authTypeID)
	resp.OkWithData(c, gin.H{
		"total": total,
		"list":  list,
	})
}

// GetUserAuthByID 获取用户认证详情
func GetUserAuthByID(c *gin.Context) {
	id := utils.StringToUint(c.Param("id"))
	if id == 0 {
		resp.FailWithMessage(c, "参数有误")
		return
	}

	userAuth, err := service.GetUserAuthByID(id)
	if err != nil {
		resp.FailWithMessage(c, err.Error())
		return
	}

	resp.OkWithData(c, userAuth)
}

// GetUserAuthByUid 获取指定用户的认证信息
func GetUserAuthByUid(c *gin.Context) {
	uid := utils.StringToUint(c.Param("uid"))
	if uid == 0 {
		resp.FailWithMessage(c, "参数有误")
		return
	}

	list, err := service.GetUserAuthByUid(uid)
	if err != nil {
		resp.FailWithMessage(c, "获取失败")
		return
	}

	resp.OkWithData(c, gin.H{"list": list})
}

// GetUserPrimaryAuth 获取用户主要认证（用于前端展示）
func GetUserPrimaryAuth(c *gin.Context) {
	uid := utils.StringToUint(c.Query("uid"))
	if uid == 0 {
		resp.FailWithMessage(c, "参数有误")
		return
	}

	auth := service.GetUserPrimaryAuth(uid)
	if auth == nil {
		resp.OkWithData(c, gin.H{"auth": nil})
		return
	}
	resp.OkWithData(c, gin.H{"auth": auth})
}