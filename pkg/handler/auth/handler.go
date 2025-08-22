package auth_handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/idgen"
	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/auth"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"

	"github.com/gin-gonic/gin"
)

// AuthHandler 封装了所有认证相关的控制器方法
type AuthHandler struct {
	authSvc    auth.AuthService
	tokenSvc   auth.TokenService
	settingSvc setting.SettingService
}

// NewAuthHandler 是 AuthHandler 的构造函数，用于依赖注入
func NewAuthHandler(authSvc auth.AuthService, tokenSvc auth.TokenService, settingSvc setting.SettingService) *AuthHandler {
	return &AuthHandler{
		authSvc:    authSvc,
		tokenSvc:   tokenSvc,
		settingSvc: settingSvc,
	}
}

// LoginRequest 定义了登录请求的结构
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest 定义了注册请求的结构
type RegisterRequest struct {
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required,min=6"`
	RepeatPassword string `json:"repeat_password" binding:"required"`
}

// RefreshTokenRequest 定义了刷新令牌请求的结构
type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// ActivateUserRequest 定义了激活用户请求的结构
type ActivateUserRequest struct {
	PublicUserID string `json:"id" binding:"required"` // 公共用户ID
	Sign         string `json:"sign" binding:"required"`
}

// ForgotPasswordRequest 定义了忘记密码请求的结构
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest 定义了重置密码请求的结构
type ResetPasswordRequest struct {
	PublicUserID   string `json:"id" binding:"required"` // 公共用户ID
	Sign           string `json:"sign" binding:"required"`
	Password       string `json:"password" binding:"required,min=6"`
	RepeatPassword string `json:"repeat_password" binding:"required"`
}

// UserGroupResponse 定义了用户组的响应结构，用于嵌套在用户信息中
type UserGroupResponse struct {
	ID          string `json:"id"`          // 用户组的公共ID，改为 string 类型
	Name        string `json:"name"`        // 用户组名称
	Description string `json:"description"` // 用户组描述
	// 根据需要，可以添加 Permissions 或其他用户组相关的公开信息
}

// LoginUserInfoResponse 定义了登录成功时返回给客户端的用户信息结构
type LoginUserInfoResponse struct {
	ID          string            `json:"id"`          // 用户的公共ID
	CreatedAt   time.Time         `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time         `json:"updated_at"`  // 更新时间
	Username    string            `json:"username"`    // 用户名
	Nickname    string            `json:"nickname"`    // 昵称
	Avatar      string            `json:"avatar"`      // 头像URL
	Email       string            `json:"email"`       // 邮箱
	LastLoginAt *time.Time        `json:"lastLoginAt"` // 最后登录时间
	UserGroupID uint              `json:"userGroupID"` // 用户组ID (原始的数据库ID，根据需求决定是否暴露)
	UserGroup   UserGroupResponse `json:"userGroup"`   // 用户的用户组信息 (嵌套 DTO)
	Status      int               `json:"status"`      // 用户状态
}

// Login 处理用户登录请求
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "邮箱或密码格式不正确")
		return
	}

	// 1. 调用认证服务进行登录逻辑处理
	user, err := h.authSvc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}

	// 2. 调用令牌服务生成会话令牌
	// 注意：这里的 GenerateSessionTokens 内部也需要更新为使用 GeneratePublicID
	accessToken, refreshToken, expires, err := h.tokenSvc.GenerateSessionTokens(c.Request.Context(), user)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "生成令牌失败: "+err.Error())
		return
	}

	// 3. 构建 roles 数组
	roles := []string{fmt.Sprintf("%d", user.UserGroupID)}

	// 4. 生成用户的公共 ID
	publicUserID, err := idgen.GeneratePublicID(user.ID, idgen.EntityTypeUser) // 统一使用 GeneratePublicID
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "生成用户公共ID失败")
		return
	}

	// 5. 生成用户组的公共 ID
	publicUserGroupID, err := idgen.GeneratePublicID(user.UserGroup.ID, idgen.EntityTypeUserGroup) // 统一使用 GeneratePublicID
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "生成用户组公共ID失败")
		return
	}

	gravatarBaseURL := h.settingSvc.Get(constant.KeyGravatarURL.String())
	avatar := gravatarBaseURL + user.Avatar

	// 6. 构建 LoginUserInfoResponse DTO，只包含需要暴露给客户端的字段
	userInfoResp := LoginUserInfoResponse{
		ID:          publicUserID, // 返回公共ID
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Username:    user.Username,
		Nickname:    user.Nickname,
		Avatar:      avatar,
		Email:       user.Email,
		LastLoginAt: user.LastLoginAt,
		UserGroupID: user.UserGroupID,
		UserGroup: UserGroupResponse{
			ID:          publicUserGroupID, // 返回用户组的公共ID
			Name:        user.UserGroup.Name,
			Description: user.UserGroup.Description,
		},
		Status: user.Status,
	}

	// 7. 返回成功响应
	response.Success(c, gin.H{
		"userInfo":     userInfoResp, // 返回包含公共ID和用户组信息的 DTO
		"roles":        roles,
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"expires":      expires,
	}, "登录成功")
}

// Register 处理用户注册请求
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误：邮箱、密码或重复密码格式不正确")
		return
	}
	if req.Password != req.RepeatPassword {
		response.Fail(c, http.StatusBadRequest, "两次输入的密码不一致")
		return
	}

	activationRequired, err := h.authSvc.Register(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.Fail(c, http.StatusConflict, err.Error())
		return
	}

	message := "注册成功"
	if activationRequired {
		message = "注册成功，请查收激活邮件以完成注册"
	}
	response.Success(c, gin.H{"activation_required": activationRequired}, message)
}

// RefreshToken 刷新访问 Token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	// 优先从 Header 获取
	refreshToken := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")

	// 如果 Header 中没有，再尝试从 Body 获取
	if refreshToken == "" {
		var req RefreshTokenRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			refreshToken = req.RefreshToken
		}
	}

	if refreshToken == "" {
		response.Fail(c, http.StatusUnauthorized, "未提供RefreshToken")
		return
	}

	// 注意：这里的 RefreshAccessToken 内部也需要更新为使用 DecodePublicID
	accessToken, expires, err := h.tokenSvc.RefreshAccessToken(c.Request.Context(), refreshToken)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}

	response.Success(c, gin.H{
		"accessToken": accessToken,
		"expires":     expires,
	}, "刷新Token成功")
}

// ActivateUser 处理用户激活请求
func (h *AuthHandler) ActivateUser(c *gin.Context) {
	var req ActivateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 将公共ID解码为数据库ID，并验证实体类型
	userID, entityType, err := idgen.DecodePublicID(req.PublicUserID) // 统一使用 DecodePublicID
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的用户激活链接或ID")
		return
	}
	if entityType != idgen.EntityTypeUser {
		response.Fail(c, http.StatusBadRequest, "无效的用户激活链接：ID类型不匹配")
		return
	}

	if err := h.authSvc.ActivateUser(c.Request.Context(), userID, req.Sign); err != nil { // 传递数据库ID
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}

	response.Success(c, nil, "您的账户已成功激活！")
}

// ForgotPasswordRequest 处理发送密码重置邮件的请求
func (h *AuthHandler) ForgotPasswordRequest(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "邮箱格式不正确")
		return
	}
	// 调用 service，无论用户是否存在，都返回成功，防止邮箱枚举攻击
	h.authSvc.RequestPasswordReset(c.Request.Context(), req.Email)
	response.Success(c, nil, "如果该邮箱已注册，您将会收到一封密码重置邮件。")
}

// ResetPassword 执行密码重置
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Password != req.RepeatPassword {
		response.Fail(c, http.StatusBadRequest, "两次输入的密码不一致")
		return
	}

	// 将公共ID解码为数据库ID，并验证实体类型
	userID, entityType, err := idgen.DecodePublicID(req.PublicUserID) // 统一使用 DecodePublicID
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的密码重置链接或ID")
		return
	}
	if entityType != idgen.EntityTypeUser {
		response.Fail(c, http.StatusBadRequest, "无效的密码重置链接：ID类型不匹配")
		return
	}

	if err := h.authSvc.PerformPasswordReset(c.Request.Context(), userID, req.Sign, req.Password); err != nil { // 传递数据库ID
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}

	response.Success(c, nil, "密码重置成功，请使用新密码登录。")
}

// CheckEmail 检查邮箱是否已被注册
func (h *AuthHandler) CheckEmail(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		response.Fail(c, http.StatusBadRequest, "参数错误：缺少 email")
		return
	}

	exists, err := h.authSvc.CheckEmailExists(c.Request.Context(), email)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "查询邮箱时出错: "+err.Error())
		return
	}

	response.Success(c, gin.H{"exists": exists}, "查询成功")

}
