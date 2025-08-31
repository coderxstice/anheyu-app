// internal/infra/persistence/ent/comment_repo.go
package ent

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/anzhiyu-c/anheyu-app/ent"
	entcomment "github.com/anzhiyu-c/anheyu-app/ent/comment"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"

	"entgo.io/ent/dialect/sql"
)

type commentRepo struct {
	db     *ent.Client
	dbType string
}

func NewCommentRepo(db *ent.Client, dbType string) repository.CommentRepository {
	return &commentRepo{
		db:     db,
		dbType: dbType,
	}
}

func toDomain(c *ent.Comment) *model.Comment {
	if c == nil {
		return nil
	}
	var ua, loc string
	if c.UserAgent != nil {
		ua = *c.UserAgent
	}
	if c.IPLocation != nil {
		loc = *c.IPLocation
	}
	domainComment := &model.Comment{
		ID:                c.ID,
		TargetPath:        c.TargetPath,
		TargetTitle:       c.TargetTitle,
		ParentID:          c.ParentID,
		UserID:            c.UserID,
		Author:            model.Author{Nickname: c.Nickname, Email: c.Email, Website: c.Website, IP: c.IPAddress, UserAgent: ua, Location: loc},
		Content:           c.Content,
		ContentHTML:       c.ContentHTML,
		LikeCount:         c.LikeCount,
		Status:            model.Status(c.Status),
		IsAdminAuthor:     c.IsAdminComment,
		AllowNotification: c.AllowNotification,
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
		PinnedAt:          c.PinnedAt,
	}
	return domainComment
}

func (r *commentRepo) Create(ctx context.Context, params *repository.CreateCommentParams) (*model.Comment, error) {
	creator := r.db.Comment.Create().
		SetTargetPath(params.TargetPath).
		SetNickname(params.Nickname).
		SetEmailMd5(params.EmailMD5).
		SetContent(params.Content).
		SetContentHTML(params.ContentHTML).
		SetIPAddress(params.IPAddress).
		SetIPLocation(params.IPLocation).
		SetStatus(params.Status).
		SetIsAdminComment(params.IsAdminComment).
		SetAllowNotification(params.AllowNotification)

	if params.TargetTitle != nil {
		creator.SetTargetTitle(*params.TargetTitle)
	}
	if params.UserID != nil {
		creator.SetUserID(*params.UserID)
	}
	if params.ParentID != nil {
		creator.SetParentID(*params.ParentID)
	}
	if params.Email != nil {
		creator.SetEmail(*params.Email)
	}
	if params.Website != nil {
		creator.SetWebsite(*params.Website)
	}
	if params.UserAgent != nil {
		creator.SetUserAgent(*params.UserAgent)
	}

	newEntComment, err := creator.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, newEntComment.ID)
}

func (r *commentRepo) FindAllPublishedByPath(ctx context.Context, path string) ([]*model.Comment, error) {
	log.Printf("[DEBUG] Repo.FindAllPublishedByPath: 开始查询路径 '%s' 的所有已发布评论", path)

	query := r.db.Comment.Query().
		Where(
			entcomment.TargetPath(path),
			entcomment.StatusEQ(int(model.StatusPublished)),
			entcomment.DeletedAtIsNil(),
		)

	// 按置顶状态和创建时间排序：置顶的在前，然后按创建时间降序
	entComments, err := query.Modify(func(s *sql.Selector) {
		var pinnedOrder string
		if r.dbType == "mysql" {
			pinnedOrder = fmt.Sprintf("`%s` IS NULL ASC, `%s` DESC", entcomment.FieldPinnedAt, entcomment.FieldPinnedAt)
		} else {
			pinnedOrder = fmt.Sprintf(`"%s" DESC NULLS LAST`, entcomment.FieldPinnedAt)
		}

		// 根据数据库类型使用不同的列名引用方式
		var createdAtOrder string
		if r.dbType == "mysql" {
			createdAtOrder = fmt.Sprintf("`%s` DESC", entcomment.FieldCreatedAt)
		} else {
			createdAtOrder = fmt.Sprintf(`"%s" DESC`, entcomment.FieldCreatedAt)
		}

		s.OrderBy(pinnedOrder, createdAtOrder)
	}).All(ctx)
	if err != nil {
		log.Printf("[ERROR] Repo.FindAllPublishedByPath: 查询失败: %v", err)
		return nil, err
	}
	log.Printf("[DEBUG] Repo.FindAllPublishedByPath: 查询成功，共找到 %d 条评论", len(entComments))

	domainComments := make([]*model.Comment, len(entComments))
	for i, c := range entComments {
		domainComments[i] = toDomain(c)
	}
	return domainComments, nil
}

// FindByID 根据数据库ID查找单条评论。
func (r *commentRepo) FindByID(ctx context.Context, id uint) (*model.Comment, error) {
	entComment, err := r.db.Comment.Query().
		Where(entcomment.ID(id)).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return toDomain(entComment), nil
}

func (r *commentRepo) IncrementLikeCount(ctx context.Context, id uint) (*model.Comment, error) {
	_, err := r.db.Comment.UpdateOneID(id).AddLikeCount(1).Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}
func (r *commentRepo) DecrementLikeCount(ctx context.Context, id uint) (*model.Comment, error) {
	_, err := r.db.Comment.UpdateOneID(id).AddLikeCount(-1).Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}
func (r *commentRepo) FindWithConditions(ctx context.Context, params repository.AdminListParams) ([]*model.Comment, int64, error) {
	query := r.db.Comment.Query().Where(entcomment.DeletedAtIsNil())

	if params.Nickname != nil && *params.Nickname != "" {
		query = query.Where(entcomment.NicknameContains(*params.Nickname))
	}
	if params.Email != nil && *params.Email != "" {
		query = query.Where(entcomment.EmailContains(*params.Email))
	}
	if params.Website != nil && *params.Website != "" {
		query = query.Where(entcomment.WebsiteContains(*params.Website))
	}
	if params.IPAddress != nil && *params.IPAddress != "" {
		query = query.Where(entcomment.IPAddressContains(*params.IPAddress))
	}
	if params.Content != nil && *params.Content != "" {
		query = query.Where(entcomment.ContentContains(*params.Content))
	}
	if params.TargetPath != nil && *params.TargetPath != "" {
		query = query.Where(entcomment.TargetPathContains(*params.TargetPath))
	}
	if params.Status != nil {
		query = query.Where(entcomment.StatusEQ(*params.Status))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	query = query.Modify(func(s *sql.Selector) {
		var pinnedOrder string
		if r.dbType == "mysql" {
			pinnedOrder = fmt.Sprintf("`%s` IS NULL ASC, `%s` DESC", entcomment.FieldPinnedAt, entcomment.FieldPinnedAt)
		} else {
			pinnedOrder = fmt.Sprintf(`"%s" DESC NULLS LAST`, entcomment.FieldPinnedAt)
		}

		// 根据数据库类型使用不同的列名引用方式
		var createdAtOrder string
		if r.dbType == "mysql" {
			createdAtOrder = fmt.Sprintf("`%s` DESC", entcomment.FieldCreatedAt)
		} else {
			createdAtOrder = fmt.Sprintf(`"%s" DESC`, entcomment.FieldCreatedAt)
		}

		s.OrderBy(pinnedOrder, createdAtOrder)
	}).
		Limit(params.PageSize).
		Offset((params.Page - 1) * params.PageSize)

	entComments, err := query.All(ctx)
	if err != nil {
		return nil, 0, err
	}

	domainComments := make([]*model.Comment, len(entComments))
	for i, c := range entComments {
		domainComments[i] = toDomain(c)
	}

	return domainComments, int64(total), nil
}
func (r *commentRepo) DeleteByIDs(ctx context.Context, ids []uint) (int, error) {
	info, err := r.db.Comment.Update().
		Where(entcomment.IDIn(ids...)).
		SetDeletedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return info, nil
}
func (r *commentRepo) UpdateStatus(ctx context.Context, id uint, status model.Status) (*model.Comment, error) {
	_, err := r.db.Comment.UpdateOneID(id).SetStatus(int(status)).Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}
func (r *commentRepo) SetPin(ctx context.Context, id uint, pinTime *time.Time) (*model.Comment, error) {
	updater := r.db.Comment.UpdateOneID(id)
	if pinTime != nil {
		updater.SetPinnedAt(*pinTime)
	} else {
		updater.ClearPinnedAt()
	}
	_, err := updater.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}
func (r *commentRepo) UpdatePath(ctx context.Context, oldPath, newPath string) (int, error) {
	info, err := r.db.Comment.Update().
		Where(entcomment.TargetPath(oldPath)).
		SetTargetPath(newPath).
		Save(ctx)
	return info, err
}
func (r *commentRepo) FindPublishedChildrenByParentID(ctx context.Context, parentID uint, page, pageSize int) ([]*model.Comment, int64, error) {
	query := r.db.Comment.Query().
		Where(
			entcomment.ParentID(parentID),
			entcomment.StatusEQ(int(model.StatusPublished)),
			entcomment.DeletedAtIsNil(),
		)

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	entComments, err := query.
		Order(ent.Asc(entcomment.FieldCreatedAt)).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	domainComments := make([]*model.Comment, len(entComments))
	for i, c := range entComments {
		domainComments[i] = toDomain(c)
	}

	return domainComments, int64(total), nil
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
