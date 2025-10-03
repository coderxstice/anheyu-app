/*
 * @Description: 已登录用户账户相关控制器
 * @Author: 安知鱼
 * @Date: 2025-06-15 13:03:21
 * @LastEditTime: 2025-07-16 10:58:24
 * @LastEditors: 安知鱼
 */
package user_handler

import (
	"net/http"

	"github.com/anzhiyu-c/anheyu-app/internal/pkg/auth"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/idgen"
	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/user"

	"github.com/gin-gonic/gin"
)

// UserHandler 封装已登录用户账户相关的控制器方法
type UserHandler struct {
	userSvc    user.UserService
	settingSvc setting.SettingService
}

// NewUserHandler 是 UserHandler 的构造函数
func NewUserHandler(userSvc user.UserService, settingSvc setting.SettingService) *UserHandler {
	return &UserHandler{
		userSvc:    userSvc,
		settingSvc: settingSvc,
	}
}

// UserGroup 是内部用户组模型的简化版本，用于响应
type UserGroup struct {
	ID          string `json:"id"`          // 用户组的公共ID，改为 string 类型
	Name        string `json:"name"`        // 用户组名称
	Description string `json:"description"` // 用户组描述
	// Permissions 和 Settings 根据需要决定是否包含或简化
}

// GetUserInfoResponse 用于定义获取用户信息时的响应结构体，包含公共ID
type GetUserInfoResponse struct {
	ID          string    `json:"id"`          // 用户的公共ID
	CreatedAt   string    `json:"created_at"`  // 创建时间
	UpdatedAt   string    `json:"updated_at"`  // 更新时间
	Username    string    `json:"username"`    // 用户名
	Nickname    string    `json:"nickname"`    // 昵称
	Avatar      string    `json:"avatar"`      // 头像URL
	Email       string    `json:"email"`       // 邮箱
	Website     string    `json:"website"`     // 个人网站
	LastLoginAt *string   `json:"lastLoginAt"` // 最后登录时间
	UserGroupID uint      `json:"userGroupID"` // 原始用户组ID (数字类型)，根据需求决定是否暴露
	UserGroup   UserGroup `json:"userGroup"`   // 用户的用户组信息 (嵌套 DTO)
	Status      int       `json:"status"`      // 用户状态
}

// GetUserInfo 获取当前登录用户的信息
// @Summary      获取当前用户信息
// @Description  获取当前登录用户的详细信息，包括用户组信息
// @Tags         用户管理
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  response.Response{data=GetUserInfoResponse}  "获取成功"
// @Failure      401  {object}  response.Response  "未授权"
// @Failure      404  {object}  response.Response  "用户未找到"
// @Router       /user/info [get]
func (h *UserHandler) GetUserInfo(c *gin.Context) {
	// 1. 从 Gin 上下文获取 claims (由 JWT 中间件注入)
	claimsValue, exists := c.Get(auth.ClaimsKey)
	if !exists {
		response.Fail(c, http.StatusUnauthorized, "未登录或无法获取当前用户信息")
		return
	}

	claims, ok := claimsValue.(*auth.CustomClaims)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "用户信息格式不正确")
		return
	}

	// 2. 解码公共 UserID 为内部 ID
	internalUserID, entityType, err := idgen.DecodePublicID(claims.UserID)
	if err != nil || entityType != idgen.EntityTypeUser {
		response.Fail(c, http.StatusUnauthorized, "用户ID无效")
		return
	}

	// 3. 调用 Service（需要添加 GetUserInfoByID 方法）
	user, err := h.userSvc.GetUserInfoByID(c.Request.Context(), internalUserID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}

	// 3. 将内部数据库ID转换为公共ID
	publicUserID, err := idgen.GeneratePublicID(user.ID, idgen.EntityTypeUser) // 统一使用 GeneratePublicID
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "生成用户公共ID失败")
		return
	}

	// 4. 生成用户组的公共ID
	publicUserGroupID, err := idgen.GeneratePublicID(user.UserGroup.ID, idgen.EntityTypeUserGroup) // 统一使用 GeneratePublicID
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "生成用户组公共ID失败")
		return
	}

	// 5. 构建响应体，仅暴露必要信息和公共ID
	var lastLoginAtStr *string
	if user.LastLoginAt != nil {
		t := user.LastLoginAt.Format("2006-01-02 15:04:05") // 格式化时间
		lastLoginAtStr = &t
	}

	gravatarBaseURL := h.settingSvc.Get(constant.KeyGravatarURL.String())
	avatar := gravatarBaseURL + user.Avatar

	resp := GetUserInfoResponse{
		ID:          publicUserID,
		CreatedAt:   user.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   user.UpdatedAt.Format("2006-01-02 15:04:05"),
		Username:    user.Username,
		Nickname:    user.Nickname,
		Avatar:      avatar,
		Email:       user.Email,
		Website:     user.Website,
		LastLoginAt: lastLoginAtStr,
		UserGroupID: user.UserGroupID, // 保留原始 UserGroupID (数字类型)
		UserGroup: UserGroup{
			ID:          publicUserGroupID, // 用户组的公共ID
			Name:        user.UserGroup.Name,
			Description: user.UserGroup.Description,
		},
		Status: user.Status,
	}

	response.Success(c, resp, "获取用户信息成功")
}

// UpdateUserPasswordRequest 修改当前用户密码的请求体
type UpdateUserPasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

// UpdateUserPassword 用于已登录用户修改自身密码
// @Summary      修改用户密码
// @Description  当前登录用户修改自己的密码
// @Tags         用户管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      UpdateUserPasswordRequest  true  "密码修改信息"
// @Success      200   {object}  response.Response  "修改成功"
// @Failure      400   {object}  response.Response  "参数错误"
// @Failure      401   {object}  response.Response  "旧密码错误或未授权"
// @Router       /user/update-password [post]
func (h *UserHandler) UpdateUserPassword(c *gin.Context) {
	// 1. 解析参数
	var req UpdateUserPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误：旧密码和新密码都不能为空，且新密码至少6位")
		return
	}

	// 2. 从上下文获取 claims
	claimsValue, exists := c.Get(auth.ClaimsKey)
	if !exists {
		response.Fail(c, http.StatusUnauthorized, "未登录或无法获取当前用户信息")
		return
	}

	claims, ok := claimsValue.(*auth.CustomClaims)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "用户信息格式不正确")
		return
	}

	// 3. 解码公共 UserID 为内部 ID
	internalUserID, entityType, err := idgen.DecodePublicID(claims.UserID)
	if err != nil || entityType != idgen.EntityTypeUser {
		response.Fail(c, http.StatusUnauthorized, "用户ID无效")
		return
	}

	// 4. 调用 Service
	err = h.userSvc.UpdateUserPasswordByID(c.Request.Context(), internalUserID, req.OldPassword, req.NewPassword)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}

	// 5. 返回成功响应
	response.Success(c, nil, "密码修改成功")
}

// UpdateUserProfileRequest 更新用户基本信息的请求体
type UpdateUserProfileRequest struct {
	Nickname *string `json:"nickname" binding:"omitempty,min=2,max=50"`
	Website  *string `json:"website" binding:"omitempty,url"`
}

// UpdateUserProfile 用于已登录用户修改自己的基本信息
// @Summary      更新用户信息
// @Description  当前登录用户更新自己的基本信息（昵称、网站等）
// @Tags         用户管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      UpdateUserProfileRequest  true  "用户信息"
// @Success      200   {object}  response.Response  "更新成功"
// @Failure      400   {object}  response.Response  "参数错误"
// @Failure      401   {object}  response.Response  "未授权"
// @Router       /user/profile [put]
func (h *UserHandler) UpdateUserProfile(c *gin.Context) {
	// 1. 解析参数
	var req UpdateUserProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误：昵称长度需在2-50个字符，网站需为有效URL")
		return
	}

	// 2. 从上下文获取 claims
	claimsValue, exists := c.Get(auth.ClaimsKey)
	if !exists {
		response.Fail(c, http.StatusUnauthorized, "未登录或无法获取当前用户信息")
		return
	}

	claims, ok := claimsValue.(*auth.CustomClaims)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "用户信息格式不正确")
		return
	}

	// 3. 解码公共 UserID 为内部 ID
	internalUserID, entityType, err := idgen.DecodePublicID(claims.UserID)
	if err != nil || entityType != idgen.EntityTypeUser {
		response.Fail(c, http.StatusUnauthorized, "用户ID无效")
		return
	}

	// 4. 调用 Service
	err = h.userSvc.UpdateUserProfileByID(c.Request.Context(), internalUserID, req.Nickname, req.Website)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	// 5. 返回成功响应
	response.Success(c, nil, "用户信息更新成功")
}

// ========== 管理员用户管理接口 ==========

// AdminListUsersRequest 管理员查询用户列表的请求参数
type AdminListUsersRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"pageSize" binding:"omitempty,min=1,max=100"`
	Keyword  string `form:"keyword"`
	GroupID  *uint  `form:"groupID"`
	Status   *int   `form:"status" binding:"omitempty,min=1,max=3"`
}

// AdminListUsersResponse 管理员查询用户列表的响应
type AdminListUsersResponse struct {
	Users []AdminUserDTO `json:"users"`
	Total int64          `json:"total"`
	Page  int            `json:"page"`
	Size  int            `json:"size"`
}

// AdminUserDTO 管理员用户列表的用户DTO
type AdminUserDTO struct {
	ID          string    `json:"id"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
	Username    string    `json:"username"`
	Nickname    string    `json:"nickname"`
	Avatar      string    `json:"avatar"`
	Email       string    `json:"email"`
	Website     string    `json:"website"`
	LastLoginAt *string   `json:"lastLoginAt"`
	UserGroupID string    `json:"userGroupID"`
	UserGroup   UserGroup `json:"userGroup"`
	Status      int       `json:"status"`
}

// AdminListUsers 管理员获取用户列表
// @Summary      管理员获取用户列表
// @Description  管理员分页查询用户列表，支持搜索和筛选
// @Tags         管理员-用户管理
// @Security     BearerAuth
// @Produce      json
// @Param        page      query     int     false  "页码，默认1"
// @Param        pageSize  query     int     false  "每页数量，默认10"
// @Param        keyword   query     string  false  "搜索关键词（用户名、昵称、邮箱）"
// @Param        groupID   query     int     false  "用户组ID筛选"
// @Param        status    query     int     false  "用户状态筛选（1:正常 2:未激活 3:已封禁）"
// @Success      200  {object}  response.Response{data=AdminListUsersResponse}  "查询成功"
// @Failure      400  {object}  response.Response  "参数错误"
// @Failure      401  {object}  response.Response  "未授权"
// @Router       /admin/users [get]
func (h *UserHandler) AdminListUsers(c *gin.Context) {
	// 1. 解析参数
	var req AdminListUsersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 设置默认值
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 10
	}

	// 2. 调用服务层
	users, total, err := h.userSvc.AdminListUsers(
		c.Request.Context(),
		req.Page,
		req.PageSize,
		req.Keyword,
		req.GroupID,
		req.Status,
	)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 3. 转换为 DTO
	gravatarBaseURL := h.settingSvc.Get(constant.KeyGravatarURL.String())
	userDTOs := make([]AdminUserDTO, len(users))
	for i, user := range users {
		publicUserID, _ := idgen.GeneratePublicID(user.ID, idgen.EntityTypeUser)
		publicGroupID, _ := idgen.GeneratePublicID(user.UserGroup.ID, idgen.EntityTypeUserGroup)

		var lastLoginAtStr *string
		if user.LastLoginAt != nil {
			t := user.LastLoginAt.Format("2006-01-02 15:04:05")
			lastLoginAtStr = &t
		}

		userDTOs[i] = AdminUserDTO{
			ID:          publicUserID,
			CreatedAt:   user.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:   user.UpdatedAt.Format("2006-01-02 15:04:05"),
			Username:    user.Username,
			Nickname:    user.Nickname,
			Avatar:      gravatarBaseURL + user.Avatar,
			Email:       user.Email,
			Website:     user.Website,
			LastLoginAt: lastLoginAtStr,
			UserGroupID: publicGroupID,
			UserGroup: UserGroup{
				ID:          publicGroupID,
				Name:        user.UserGroup.Name,
				Description: user.UserGroup.Description,
			},
			Status: user.Status,
		}
	}

	// 4. 返回响应
	response.Success(c, AdminListUsersResponse{
		Users: userDTOs,
		Total: total,
		Page:  req.Page,
		Size:  req.PageSize,
	}, "查询成功")
}

// AdminCreateUserRequest 管理员创建用户的请求体
type AdminCreateUserRequest struct {
	Username    string `json:"username" binding:"required,min=3,max=50"`
	Password    string `json:"password" binding:"required,min=6"`
	Email       string `json:"email" binding:"required,email"`
	Nickname    string `json:"nickname" binding:"omitempty,max=50"`
	UserGroupID string `json:"userGroupID" binding:"required"`
}

// AdminCreateUser 管理员创建新用户
// @Summary      管理员创建用户
// @Description  管理员创建新用户
// @Tags         管理员-用户管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      AdminCreateUserRequest  true  "用户信息"
// @Success      200   {object}  response.Response{data=AdminUserDTO}  "创建成功"
// @Failure      400   {object}  response.Response  "参数错误"
// @Failure      401   {object}  response.Response  "未授权"
// @Router       /admin/users [post]
func (h *UserHandler) AdminCreateUser(c *gin.Context) {
	// 1. 解析参数
	var req AdminCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 2. 解码用户组ID
	userGroupID, entityType, err := idgen.DecodePublicID(req.UserGroupID)
	if err != nil || entityType != idgen.EntityTypeUserGroup {
		response.Fail(c, http.StatusBadRequest, "用户组ID无效")
		return
	}

	// 3. 调用服务层创建用户
	user, err := h.userSvc.AdminCreateUser(
		c.Request.Context(),
		req.Username,
		req.Password,
		req.Email,
		req.Nickname,
		userGroupID,
	)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	// 4. 转换为 DTO
	publicUserID, _ := idgen.GeneratePublicID(user.ID, idgen.EntityTypeUser)
	publicGroupID, _ := idgen.GeneratePublicID(user.UserGroup.ID, idgen.EntityTypeUserGroup)
	gravatarBaseURL := h.settingSvc.Get(constant.KeyGravatarURL.String())

	var lastLoginAtStr *string
	if user.LastLoginAt != nil {
		t := user.LastLoginAt.Format("2006-01-02 15:04:05")
		lastLoginAtStr = &t
	}

	userDTO := AdminUserDTO{
		ID:          publicUserID,
		CreatedAt:   user.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   user.UpdatedAt.Format("2006-01-02 15:04:05"),
		Username:    user.Username,
		Nickname:    user.Nickname,
		Avatar:      gravatarBaseURL + user.Avatar,
		Email:       user.Email,
		Website:     user.Website,
		LastLoginAt: lastLoginAtStr,
		UserGroupID: publicGroupID,
		UserGroup: UserGroup{
			ID:          publicGroupID,
			Name:        user.UserGroup.Name,
			Description: user.UserGroup.Description,
		},
		Status: user.Status,
	}

	// 5. 返回响应
	response.Success(c, userDTO, "用户创建成功")
}

// AdminUpdateUserRequest 管理员更新用户的请求体
type AdminUpdateUserRequest struct {
	Username    *string `json:"username" binding:"omitempty,min=3,max=50"`
	Email       *string `json:"email" binding:"omitempty,email"`
	Nickname    *string `json:"nickname" binding:"omitempty,max=50"`
	UserGroupID *string `json:"userGroupID"`
	Status      *int    `json:"status" binding:"omitempty,min=1,max=3"`
}

// AdminUpdateUser 管理员更新用户信息
// @Summary      管理员更新用户
// @Description  管理员更新用户信息
// @Tags         管理员-用户管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                  true  "用户ID"
// @Param        body  body      AdminUpdateUserRequest  true  "用户信息"
// @Success      200   {object}  response.Response  "更新成功"
// @Failure      400   {object}  response.Response  "参数错误"
// @Failure      401   {object}  response.Response  "未授权"
// @Router       /admin/users/:id [put]
func (h *UserHandler) AdminUpdateUser(c *gin.Context) {
	// 1. 获取用户ID
	publicUserID := c.Param("id")
	userID, entityType, err := idgen.DecodePublicID(publicUserID)
	if err != nil || entityType != idgen.EntityTypeUser {
		response.Fail(c, http.StatusBadRequest, "用户ID无效")
		return
	}

	// 2. 解析参数
	var req AdminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 3. 解码用户组ID（如果提供）
	var userGroupID *uint
	if req.UserGroupID != nil {
		gid, entityType, err := idgen.DecodePublicID(*req.UserGroupID)
		if err != nil || entityType != idgen.EntityTypeUserGroup {
			response.Fail(c, http.StatusBadRequest, "用户组ID无效")
			return
		}
		userGroupID = &gid
	}

	// 4. 调用服务层更新用户
	err = h.userSvc.AdminUpdateUser(
		c.Request.Context(),
		userID,
		req.Username,
		req.Email,
		req.Nickname,
		userGroupID,
		req.Status,
	)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	// 5. 返回响应
	response.Success(c, nil, "用户信息更新成功")
}

// AdminDeleteUser 管理员删除用户
// @Summary      管理员删除用户
// @Description  管理员删除用户（软删除）
// @Tags         管理员-用户管理
// @Security     BearerAuth
// @Produce      json
// @Param        id    path      string  true  "用户ID"
// @Success      200   {object}  response.Response  "删除成功"
// @Failure      400   {object}  response.Response  "参数错误"
// @Failure      401   {object}  response.Response  "未授权"
// @Router       /admin/users/:id [delete]
func (h *UserHandler) AdminDeleteUser(c *gin.Context) {
	// 1. 获取用户ID
	publicUserID := c.Param("id")
	userID, entityType, err := idgen.DecodePublicID(publicUserID)
	if err != nil || entityType != idgen.EntityTypeUser {
		response.Fail(c, http.StatusBadRequest, "用户ID无效")
		return
	}

	// 2. 调用服务层删除用户
	err = h.userSvc.AdminDeleteUser(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	// 3. 返回响应
	response.Success(c, nil, "用户删除成功")
}

// AdminResetPasswordRequest 管理员重置用户密码的请求体
type AdminResetPasswordRequest struct {
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

// AdminResetPassword 管理员重置用户密码
// @Summary      管理员重置用户密码
// @Description  管理员重置指定用户的密码
// @Tags         管理员-用户管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                     true  "用户ID"
// @Param        body  body      AdminResetPasswordRequest  true  "新密码"
// @Success      200   {object}  response.Response  "重置成功"
// @Failure      400   {object}  response.Response  "参数错误"
// @Failure      401   {object}  response.Response  "未授权"
// @Router       /admin/users/:id/reset-password [post]
func (h *UserHandler) AdminResetPassword(c *gin.Context) {
	// 1. 获取用户ID
	publicUserID := c.Param("id")
	userID, entityType, err := idgen.DecodePublicID(publicUserID)
	if err != nil || entityType != idgen.EntityTypeUser {
		response.Fail(c, http.StatusBadRequest, "用户ID无效")
		return
	}

	// 2. 解析参数
	var req AdminResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误：新密码不能为空且至少6位")
		return
	}

	// 3. 调用服务层重置密码
	err = h.userSvc.AdminResetPassword(c.Request.Context(), userID, req.NewPassword)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	// 4. 返回响应
	response.Success(c, nil, "密码重置成功")
}

// AdminUpdateUserStatusRequest 管理员更新用户状态的请求体
type AdminUpdateUserStatusRequest struct {
	Status int `json:"status" binding:"required,min=1,max=3"`
}

// AdminUpdateUserStatus 管理员更新用户状态
// @Summary      管理员更新用户状态
// @Description  管理员更新指定用户的状态（1:正常 2:未激活 3:已封禁）
// @Tags         管理员-用户管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                        true  "用户ID"
// @Param        body  body      AdminUpdateUserStatusRequest  true  "用户状态"
// @Success      200   {object}  response.Response  "更新成功"
// @Failure      400   {object}  response.Response  "参数错误"
// @Failure      401   {object}  response.Response  "未授权"
// @Router       /admin/users/:id/status [put]
func (h *UserHandler) AdminUpdateUserStatus(c *gin.Context) {
	// 1. 获取用户ID
	publicUserID := c.Param("id")
	userID, entityType, err := idgen.DecodePublicID(publicUserID)
	if err != nil || entityType != idgen.EntityTypeUser {
		response.Fail(c, http.StatusBadRequest, "用户ID无效")
		return
	}

	// 2. 解析参数
	var req AdminUpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误：状态值必须为1-3之间")
		return
	}

	// 3. 调用服务层更新状态
	err = h.userSvc.AdminUpdateUserStatus(c.Request.Context(), userID, req.Status)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	// 4. 返回响应
	response.Success(c, nil, "用户状态更新成功")
}

// UserGroupDTO 用户组数据传输对象
type UserGroupDTO struct {
	ID          string `json:"id"`          // 用户组公共ID
	Name        string `json:"name"`        // 用户组名称
	Description string `json:"description"` // 用户组描述
}

// GetUserGroups 获取所有用户组列表
// @Summary      获取用户组列表
// @Description  获取系统中所有用户组
// @Tags         管理员-用户管理
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  response.Response{data=[]UserGroupDTO}  "获取成功"
// @Failure      401  {object}  response.Response  "未授权"
// @Router       /admin/user-groups [get]
func (h *UserHandler) GetUserGroups(c *gin.Context) {
	// 1. 调用服务层获取用户组列表
	groups, err := h.userSvc.ListUserGroups(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 2. 转换为 DTO（包含公共ID）
	groupDTOs := make([]UserGroupDTO, len(groups))
	for i, group := range groups {
		publicGroupID, _ := idgen.GeneratePublicID(group.ID, idgen.EntityTypeUserGroup)
		groupDTOs[i] = UserGroupDTO{
			ID:          publicGroupID,
			Name:        group.Name,
			Description: group.Description,
		}
	}

	// 3. 返回响应
	response.Success(c, groupDTOs, "获取用户组列表成功")
}
