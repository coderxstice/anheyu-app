package ent

import (
	"anheyu-app/internal/domain/model"
	"anheyu-app/internal/domain/repository"
	"context"
	"errors"
	"fmt"

	"anheyu-app/ent"
	"anheyu-app/ent/user"
	"anheyu-app/ent/usergroup"
)

// entUserRepository 是 UserRepository 的 Ent 实现
type entUserRepository struct {
	client *ent.Client
}

// NewEntUserRepository 是 entUserRepository 的构造函数
func NewEntUserRepository(client *ent.Client) repository.UserRepository {
	return &entUserRepository{client: client}
}

// Transaction 实现了事务操作
func (r *entUserRepository) Transaction(ctx context.Context, fn func(repo repository.UserRepository) error) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	// 使用事务客户端 tx 创建一个新的 repo
	txRepo := NewEntUserRepository(tx.Client())

	defer func() {
		if v := recover(); v != nil {
			tx.Rollback()
			panic(v)
		}
	}()

	if err := fn(txRepo); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("执行事务失败: %v, 回滚事务也失败: %v", err, rbErr)
		}
		return err
	}

	return tx.Commit()
}

// FindByUsername 按用户名查找用户，并预加载用户组信息
func (r *entUserRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	entUser, err := r.client.User.
		Query().
		Where(
			user.Username(username),
			user.DeletedAtIsNil(),
		).
		WithUserGroup().
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainUser(entUser), nil
}

// FindByEmail 按邮箱查找用户
func (r *entUserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	entUser, err := r.client.User.
		Query().
		Where(
			user.Email(email),
			user.DeletedAtIsNil(),
		).
		WithUserGroup().
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainUser(entUser), nil
}

// Count 计算用户总数
func (r *entUserRepository) Count(ctx context.Context) (int64, error) {
	count, err := r.client.User.
		Query().
		Where(user.DeletedAtIsNil()).
		Count(ctx)
	return int64(count), err
}

// FindByGroupID 根据用户组ID查找用户列表
func (r *entUserRepository) FindByGroupID(ctx context.Context, groupID uint) ([]*model.User, error) {
	entUsers, err := r.client.User.
		Query().
		Where(
			// 使用 HasUserGroupWith 来通过关联的用户组进行过滤
			user.HasUserGroupWith(usergroup.ID(groupID)),
			user.DeletedAtIsNil(),
		).
		// 预加载用户组信息，虽然我们已经用它作为查询条件，
		// 但加载出来可以在 toDomainUser 中使用，避免 N+1 查询
		WithUserGroup().
		All(ctx)

	if err != nil {
		// Ent 在找不到记录时，All() 方法会返回一个空的 slice 和 nil error，
		// 所以不需要像 Only() 那样特殊处理 IsNotFound 错误。
		return nil, err
	}

	// 将查询到的 ent.User 列表转换为 domain.User 列表
	domainUsers := make([]*model.User, len(entUsers))
	for i, u := range entUsers {
		domainUsers[i] = toDomainUser(u)
	}

	return domainUsers, nil
}

// FindByID 根据 ID 查找用户
func (r *entUserRepository) FindByID(ctx context.Context, id uint) (*model.User, error) {
	entUser, err := r.client.User.
		Query().
		Where(
			user.ID(id),
			user.DeletedAtIsNil(),
		).
		WithUserGroup().
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainUser(entUser), nil
}

// Create 创建一个新用户，以满足 UserRepository 接口
func (r *entUserRepository) Create(ctx context.Context, user *model.User) error {
	// 创建用户时，必须指定用户组ID
	if user.UserGroupID == 0 {
		return errors.New("创建用户时必须提供用户组ID")
	}
	createBuilder := r.client.User.
		Create().
		SetUsername(user.Username).
		SetPasswordHash(user.PasswordHash).
		SetNickname(user.Nickname).
		SetAvatar(user.Avatar).
		SetEmail(user.Email).
		SetStatus(user.Status).
		SetUserGroupID(user.UserGroupID)

	// LastLoginAt 是可选的指针类型
	if user.LastLoginAt != nil {
		createBuilder.SetLastLoginAt(*user.LastLoginAt)
	}

	created, err := createBuilder.Save(ctx)
	if err != nil {
		return err
	}
	// 同步数据库生成的值
	user.ID = created.ID
	user.CreatedAt = created.CreatedAt
	user.UpdatedAt = created.UpdatedAt
	return nil
}

// Update 更新一个现有用户，以满足 UserRepository 接口
func (r *entUserRepository) Update(ctx context.Context, user *model.User) error {
	if user.ID == 0 {
		return errors.New("无法更新ID为0的用户")
	}

	updateBuilder := r.client.User.
		UpdateOneID(user.ID).
		SetUsername(user.Username).
		SetPasswordHash(user.PasswordHash).
		SetNickname(user.Nickname).
		SetAvatar(user.Avatar).
		SetEmail(user.Email).
		SetStatus(user.Status).
		SetUserGroupID(user.UserGroupID)

	// LastLoginAt 是可选的指针类型
	if user.LastLoginAt != nil {
		updateBuilder.SetLastLoginAt(*user.LastLoginAt)
	} else {
		updateBuilder.ClearLastLoginAt() // 如果传入 nil，则清除该字段
	}

	updated, err := updateBuilder.Save(ctx)
	if err != nil {
		return err
	}
	// 同步更新时间
	user.UpdatedAt = updated.UpdatedAt
	return nil
}

// Save 创建或更新用户
func (r *entUserRepository) Save(ctx context.Context, user *model.User) error {
	// 如果 ID 为 0，执行创建操作
	if user.ID == 0 {
		return r.Create(ctx, user)
	}
	// 如果 ID 不为 0，执行更新操作
	return r.Update(ctx, user)
}

// Delete 软删除用户
func (r *entUserRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.client.User.Delete().Where(user.ID(id)).Exec(ctx)
	return err
}

// --- 数据转换辅助函数 ---

func toDomainUser(u *ent.User) *model.User {
	if u == nil {
		return nil
	}
	domainUser := &model.User{
		ID:           u.ID,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
		Username:     u.Username,
		PasswordHash: u.PasswordHash,
		Nickname:     u.Nickname,
		Avatar:       u.Avatar,
		Email:        u.Email,
		LastLoginAt:  u.LastLoginAt,
		Status:       u.Status,
	}
	// Edges 是 Ent 用于存储关联模型的地方
	if u.Edges.UserGroup != nil {
		domainUser.UserGroupID = u.Edges.UserGroup.ID
		domainUser.UserGroup = *toDomainUserGroup(u.Edges.UserGroup)
	}
	return domainUser
}
