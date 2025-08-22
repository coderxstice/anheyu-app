package ent

import (
	"anheyu-app/internal/domain/model"
	"anheyu-app/internal/domain/repository"
	"anheyu-app/internal/pkg/types"
	"context"
	"fmt"

	"anheyu-app/ent"
	"anheyu-app/ent/fileentity"
)

type entFileEntityRepository struct {
	client *ent.Client
}

func NewEntFileEntityRepository(client *ent.Client) repository.FileEntityRepository {
	return &entFileEntityRepository{client: client}
}

func (r *entFileEntityRepository) Create(ctx context.Context, version *model.FileStorageVersion) error {
	createBuilder := r.client.FileEntity.Create().
		SetFileID(version.FileID).
		SetEntityID(version.EntityID).
		SetIsCurrent(version.IsCurrent)

	if version.Version.Valid {
		createBuilder.SetVersion(version.Version.String)
	}
	if version.UploadedByUserID.Valid {
		createBuilder.SetUploadedByUserID(version.UploadedByUserID.Uint64)
	}

	created, err := createBuilder.Save(ctx)
	if err != nil {
		return err
	}
	version.ID = created.ID
	version.CreatedAt = created.CreatedAt
	version.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *entFileEntityRepository) FindByID(ctx context.Context, id uint) (*model.FileStorageVersion, error) {
	entVersion, err := r.client.FileEntity.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainFileStorageVersion(entVersion), nil
}

func (r *entFileEntityRepository) Update(ctx context.Context, version *model.FileStorageVersion) error {
	updateBuilder := r.client.FileEntity.UpdateOneID(version.ID).
		SetFileID(version.FileID).
		SetEntityID(version.EntityID).
		SetIsCurrent(version.IsCurrent)

	if version.Version.Valid {
		updateBuilder.SetVersion(version.Version.String)
	}
	if version.UploadedByUserID.Valid {
		updateBuilder.SetUploadedByUserID(version.UploadedByUserID.Uint64)
	}

	_, err := updateBuilder.Save(ctx)
	return err
}

func (r *entFileEntityRepository) Delete(ctx context.Context, id uint) error {
	// This model has soft-delete enabled via mixin
	return r.client.FileEntity.DeleteOneID(id).Exec(ctx)
}

func (r *entFileEntityRepository) FindCurrentByFileID(ctx context.Context, fileID uint) (*model.FileStorageVersion, error) {
	entVersion, err := r.client.FileEntity.Query().
		Where(
			fileentity.FileID(fileID),
			fileentity.IsCurrent(true),
			fileentity.DeletedAtIsNil(), // Respect soft-delete
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainFileStorageVersion(entVersion), nil
}

func (r *entFileEntityRepository) MarkOldVersionsAsNotCurrent(ctx context.Context, fileID uint, excludeVersionID uint) error {
	query := r.client.FileEntity.Update().
		Where(
			fileentity.FileID(fileID),
			fileentity.IsCurrent(true),
			fileentity.DeletedAtIsNil(),
		)
	if excludeVersionID > 0 {
		query = query.Where(fileentity.IDNEQ(excludeVersionID))
	}
	_, err := query.SetIsCurrent(false).Save(ctx)
	return err
}

func (r *entFileEntityRepository) FindByFileAndEntityID(ctx context.Context, fileID, entityID uint) (*model.FileStorageVersion, error) {
	entVersion, err := r.client.FileEntity.Query().
		Where(
			fileentity.FileID(fileID),
			fileentity.EntityID(entityID),
			fileentity.DeletedAtIsNil(),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainFileStorageVersion(entVersion), nil
}

func (r *entFileEntityRepository) Transaction(ctx context.Context, fn func(repo repository.FileEntityRepository) error) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	txRepo := NewEntFileEntityRepository(tx.Client())
	defer func() {
		if v := recover(); v != nil {
			tx.Rollback()
			panic(v)
		}
	}()
	if err := fn(txRepo); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			err = fmt.Errorf("事务执行失败: %w, 回滚事务也失败: %v", err, rerr)
		}
		return err
	}
	return tx.Commit()
}

func (r *entFileEntityRepository) DeleteByFileID(ctx context.Context, fileID uint) error {
	// This should be a hard delete of all versions for a file
	_, err := r.client.FileEntity.Delete().Where(fileentity.FileID(fileID)).Exec(ctx)
	return err
}

func (r *entFileEntityRepository) HardDelete(ctx context.Context, id uint) error {
	return r.client.FileEntity.DeleteOneID(id).Exec(ctx)
}

// --- 数据转换辅助函数 ---

func toDomainFileStorageVersion(v *ent.FileEntity) *model.FileStorageVersion {
	if v == nil {
		return nil
	}
	domain := &model.FileStorageVersion{
		ID:        v.ID,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
		FileID:    v.FileID,
		EntityID:  v.EntityID,
		IsCurrent: v.IsCurrent,
	}

	if v.Version != nil {
		domain.Version.String = *v.Version
		domain.Version.Valid = true
	}
	if v.UploadedByUserID != nil {
		domain.UploadedByUserID = types.NullUint64{Uint64: *v.UploadedByUserID, Valid: true}
	}

	return domain
}
