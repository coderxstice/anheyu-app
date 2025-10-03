/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-06-20 13:27:06
 * @LastEditTime: 2025-07-12 15:21:28
 * @LastEditors: 安知鱼
 */
package user

import (
	"context"
	"fmt"

	"github.com/anzhiyu-c/anheyu-app/internal/pkg/security"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
)

// UserService 定义了用户相关的业务逻辑接口
type UserService interface {
	GetUserInfoByUsername(ctx context.Context, username string) (*model.User, error)
	GetUserInfoByID(ctx context.Context, userID uint) (*model.User, error)
	UpdateUserPassword(ctx context.Context, username, oldPassword, newPassword string) error
	UpdateUserPasswordByID(ctx context.Context, userID uint, oldPassword, newPassword string) error
	UpdateUserProfile(ctx context.Context, username string, nickname, website *string) error
	UpdateUserProfileByID(ctx context.Context, userID uint, nickname, website *string) error
}

// userService 是 UserService 接口的实现
type userService struct {
	userRepo repository.UserRepository
}

// NewUserService 是 userService 的构造函数
func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{
		userRepo: userRepo,
	}
}

// GetUserInfoByUsername 实现了获取用户信息的业务逻辑
func (s *userService) GetUserInfoByUsername(ctx context.Context, username string) (*model.User, error) {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息时数据库出错: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("用户 '%s' 不存在", username)
	}
	return user, nil
}

// GetUserInfoByID 实现了根据用户ID获取用户信息的业务逻辑
func (s *userService) GetUserInfoByID(ctx context.Context, userID uint) (*model.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息时数据库出错: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("用户不存在")
	}
	return user, nil
}

// UpdateUserPassword 实现了修改用户密码的业务逻辑
func (s *userService) UpdateUserPassword(ctx context.Context, username, oldPassword, newPassword string) error {
	// 1. 获取用户信息
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("获取用户信息失败: %w", err)
	}
	if user == nil {
		return fmt.Errorf("当前登录用户不存在")
	}

	// 2. 校验旧密码
	if !security.CheckPasswordHash(oldPassword, user.PasswordHash) {
		return fmt.Errorf("旧密码不正确")
	}

	// 3. 哈希新密码
	newHashedPassword, err := security.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("生成新密码失败: %w", err)
	}

	// 4. 更新领域模型并保存
	user.PasswordHash = newHashedPassword
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}

	return nil
}

// UpdateUserPasswordByID 实现了根据用户ID修改密码的业务逻辑
func (s *userService) UpdateUserPasswordByID(ctx context.Context, userID uint, oldPassword, newPassword string) error {
	// 1. 获取用户信息
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("获取用户信息失败: %w", err)
	}
	if user == nil {
		return fmt.Errorf("当前登录用户不存在")
	}

	// 2. 校验旧密码
	if !security.CheckPasswordHash(oldPassword, user.PasswordHash) {
		return fmt.Errorf("旧密码不正确")
	}

	// 3. 哈希新密码
	newHashedPassword, err := security.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("生成新密码失败: %w", err)
	}

	// 4. 更新领域模型并保存
	user.PasswordHash = newHashedPassword
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}

	return nil
}

// UpdateUserProfile 实现了更新用户基本信息的业务逻辑
func (s *userService) UpdateUserProfile(ctx context.Context, username string, nickname, website *string) error {
	// 1. 获取用户信息
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("获取用户信息失败: %w", err)
	}
	if user == nil {
		return fmt.Errorf("当前登录用户不存在")
	}

	// 2. 更新字段（仅更新提供的字段）
	if nickname != nil {
		user.Nickname = *nickname
	}
	if website != nil {
		user.Website = *website
	}

	// 3. 保存更新
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("更新用户信息失败: %w", err)
	}

	return nil
}

// UpdateUserProfileByID 实现了根据用户ID更新基本信息的业务逻辑
func (s *userService) UpdateUserProfileByID(ctx context.Context, userID uint, nickname, website *string) error {
	// 1. 获取用户信息
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("获取用户信息失败: %w", err)
	}
	if user == nil {
		return fmt.Errorf("当前登录用户不存在")
	}

	// 2. 更新字段（仅更新提供的字段）
	if nickname != nil {
		user.Nickname = *nickname
	}
	if website != nil {
		user.Website = *website
	}

	// 3. 保存更新
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("更新用户信息失败: %w", err)
	}

	return nil
}
