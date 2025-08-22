package ent

import (
	"github.com/anzhiyu-c/anheyu-app/ent"
	"github.com/anzhiyu-c/anheyu-app/ent/article"
	"github.com/anzhiyu-c/anheyu-app/ent/postcategory"
	"github.com/anzhiyu-c/anheyu-app/ent/posttag"
	"github.com/anzhiyu-c/anheyu-app/ent/predicate"
	"github.com/anzhiyu-c/anheyu-app/internal/constant"
	"github.com/anzhiyu-c/anheyu-app/internal/domain/model"
	"github.com/anzhiyu-c/anheyu-app/internal/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/idgen"
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"entgo.io/ent/dialect/sql"
)

type articleRepo struct {
	db *ent.Client
}

// NewArticleRepo 是 articleRepo 的构造函数。
func NewArticleRepo(db *ent.Client) repository.ArticleRepository {
	return &articleRepo{db: db}
}

// === 私有辅助函数 (Private Helpers) ===

// toModel 负责将 ent.Article 实体转换为 model.Article 领域模型。
func (r *articleRepo) toModel(a *ent.Article) *model.Article {
	if a == nil {
		return nil
	}
	publicID, _ := idgen.GeneratePublicID(a.ID, idgen.EntityTypeArticle)
	var tags []*model.PostTag
	if a.Edges.PostTags != nil {
		tags = make([]*model.PostTag, len(a.Edges.PostTags))
		for i, t := range a.Edges.PostTags {
			tagPublicID, _ := idgen.GeneratePublicID(t.ID, idgen.EntityTypePostTag)
			tags[i] = &model.PostTag{ID: tagPublicID, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, Name: t.Name}
		}
	}
	var categories []*model.PostCategory
	if a.Edges.PostCategories != nil {
		categories = make([]*model.PostCategory, len(a.Edges.PostCategories))
		for i, c := range a.Edges.PostCategories {
			categoryPublicID, _ := idgen.GeneratePublicID(c.ID, idgen.EntityTypePostCategory)
			categories[i] = &model.PostCategory{ID: categoryPublicID, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, Name: c.Name, Description: c.Description}
		}
	}

	var abbrlinkStr string
	if a.Abbrlink != nil {
		abbrlinkStr = *a.Abbrlink
	}

	return &model.Article{
		ID:                   publicID,
		CreatedAt:            a.CreatedAt,
		UpdatedAt:            a.UpdatedAt,
		Title:                a.Title,
		ContentMd:            a.ContentMd,
		ContentHTML:          a.ContentHTML,
		CoverURL:             a.CoverURL,
		Status:               string(a.Status),
		ViewCount:            a.ViewCount,
		WordCount:            a.WordCount,
		ReadingTime:          a.ReadingTime,
		IPLocation:           a.IPLocation,
		PrimaryColor:         a.PrimaryColor,
		IsPrimaryColorManual: a.IsPrimaryColorManual,
		PostTags:             tags,
		PostCategories:       categories,
		HomeSort:             a.HomeSort,
		PinSort:              a.PinSort,
		TopImgURL:            a.TopImgURL,
		Summaries:            a.Summaries,
		Abbrlink:             abbrlinkStr,
		Copyright:            a.Copyright,
		CopyrightAuthor:      a.CopyrightAuthor,
		CopyrightAuthorHref:  a.CopyrightAuthorHref,
		CopyrightURL:         a.CopyrightURL,
	}
}

// toModelSlice 将 ent.Article 切片转换为 model.Article 切片，减少代码重复。
func (r *articleRepo) toModelSlice(entities []*ent.Article) []*model.Article {
	models := make([]*model.Article, len(entities))
	for i, entity := range entities {
		models[i] = r.toModel(entity)
	}
	return models
}

// getAdjacentArticle 是一个通用的辅助函数，用于获取上一篇或下一篇文章。
func (r *articleRepo) getAdjacentArticle(ctx context.Context, currentArticleID uint, createdAt time.Time, isPrev bool) (*model.Article, error) {
	query := r.db.Article.Query().
		Where(
			article.IDNEQ(currentArticleID),
			article.StatusEQ(article.StatusPUBLISHED),
			article.DeletedAtIsNil(),
		)

	if isPrev {
		query = query.Where(article.CreatedAtLT(createdAt)).
			Order(ent.Desc(article.FieldCreatedAt), ent.Desc(article.FieldID))
	} else {
		query = query.Where(article.CreatedAtGT(createdAt)).
			Order(ent.Asc(article.FieldCreatedAt), ent.Asc(article.FieldID))
	}

	entity, err := query.First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil // 未找到是正常情况
		}
		return nil, err
	}
	return r.toModel(entity), nil
}

// === 公开方法实现 (Public Methods Implementation) ===

// FindByID 实现了通过数据库 uint ID 查找文章的方法。
func (r *articleRepo) FindByID(ctx context.Context, id uint) (*model.Article, error) {
	entArticle, err := r.db.Article.Query().
		Where(article.ID(id)).
		WithPostTags().
		WithPostCategories().
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModel(entArticle), nil
}

// GetArchiveSummary 获取文章归档摘要
func (r *articleRepo) GetArchiveSummary(ctx context.Context) ([]*model.ArchiveItem, error) {
	var items []*model.ArchiveItem
	err := r.db.Article.Query().
		Where(
			article.StatusEQ(article.StatusPUBLISHED),
			article.DeletedAtIsNil(),
		).
		Modify(func(s *sql.Selector) {
			yearExprStr := fmt.Sprintf("EXTRACT(YEAR FROM %s)", s.C(article.FieldCreatedAt))
			monthExprStr := fmt.Sprintf("EXTRACT(MONTH FROM %s)", s.C(article.FieldCreatedAt))
			s.Select(
				sql.As(yearExprStr, "year"),
				sql.As(monthExprStr, "month"),
				sql.As(sql.Count(s.C(article.FieldID)), "count"),
			)
			s.GroupBy(yearExprStr, monthExprStr)
			s.OrderBy(sql.Desc("year"), sql.Desc("month"))
		}).
		Scan(ctx, &items)

	if err != nil {
		return nil, fmt.Errorf("查询归档摘要失败: %w", err)
	}
	return items, nil
}

// GetPrevArticle 获取上一篇文章
func (r *articleRepo) GetPrevArticle(ctx context.Context, currentArticleID uint, createdAt time.Time) (*model.Article, error) {
	return r.getAdjacentArticle(ctx, currentArticleID, createdAt, true)
}

// GetNextArticle 获取下一篇文章
func (r *articleRepo) GetNextArticle(ctx context.Context, currentArticleID uint, createdAt time.Time) (*model.Article, error) {
	return r.getAdjacentArticle(ctx, currentArticleID, createdAt, false)
}

// FindRelatedArticles 查找与当前文章相关的文章
func (r *articleRepo) FindRelatedArticles(ctx context.Context, articleModel *model.Article, limit int) ([]*model.Article, error) {
	if len(articleModel.PostTags) == 0 && len(articleModel.PostCategories) == 0 {
		return nil, nil
	}

	currentArticleDbID, _, _ := idgen.DecodePublicID(articleModel.ID)
	tagIDs := make([]uint, len(articleModel.PostTags))
	for i, t := range articleModel.PostTags {
		tagIDs[i], _, _ = idgen.DecodePublicID(t.ID)
	}
	categoryIDs := make([]uint, len(articleModel.PostCategories))
	for i, c := range articleModel.PostCategories {
		categoryIDs[i], _, _ = idgen.DecodePublicID(c.ID)
	}

	var relationPredicate predicate.Article
	if len(tagIDs) > 0 {
		relationPredicate = article.HasPostTagsWith(posttag.IDIn(tagIDs...))
	}
	if len(categoryIDs) > 0 {
		catPredicate := article.HasPostCategoriesWith(postcategory.IDIn(categoryIDs...))
		if relationPredicate != nil {
			relationPredicate = article.Or(relationPredicate, catPredicate)
		} else {
			relationPredicate = catPredicate
		}
	}

	entities, err := r.db.Article.Query().
		Where(
			article.IDNEQ(currentArticleDbID),
			article.StatusEQ(article.StatusPUBLISHED),
			article.DeletedAtIsNil(),
			relationPredicate,
		).
		Order(ent.Desc(article.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModelSlice(entities), nil
}

// GetBySlugOrID 通过 abbrlink 或公共 ID 获取一篇文章
func (r *articleRepo) GetBySlugOrID(ctx context.Context, slugOrID string) (*model.Article, error) {
	// 尝试将 slugOrID 解码为数据库 ID
	dbID, _, err := idgen.DecodePublicID(slugOrID)

	var wherePredicate predicate.Article
	if err == nil {
		// 如果解码成功，则查询条件为 ID 或 abbrlink 匹配
		wherePredicate = article.Or(article.ID(dbID), article.AbbrlinkEQ(slugOrID))
	} else {
		// 如果解码失败，则查询条件仅为 abbrlink 匹配
		wherePredicate = article.AbbrlinkEQ(slugOrID)
	}

	entity, err := r.db.Article.Query().
		Where(
			wherePredicate,
			article.DeletedAtIsNil(),
			article.StatusEQ(article.StatusPUBLISHED),
		).
		WithPostTags().
		WithPostCategories().
		Only(ctx)

	if err != nil {
		return nil, err
	}
	return r.toModel(entity), nil
}

// GetSiteStats 高效地获取站点范围内的统计数据
func (r *articleRepo) GetSiteStats(ctx context.Context) (*model.SiteStats, error) {
	publishedAndNotDeleted := []predicate.Article{
		article.StatusEQ(article.StatusPUBLISHED),
		article.DeletedAtIsNil(),
	}

	totalPosts, err := r.db.Article.Query().Where(publishedAndNotDeleted...).Count(ctx)
	if err != nil {
		return nil, err
	}

	// 使用 Scan 来更健壮地处理 SUM，即使没有文章也不会报错
	var v []struct {
		Sum int `json:"sum"`
	}
	err = r.db.Article.Query().
		Where(publishedAndNotDeleted...).
		Aggregate(ent.Sum(article.FieldWordCount)).
		Scan(ctx, &v)

	totalWords := 0
	if err == nil && len(v) > 0 {
		totalWords = v[0].Sum
	} else if err != nil {
		return nil, err // 如果 Scan 真的出错了，还是需要返回错误
	}

	return &model.SiteStats{TotalPosts: totalPosts, TotalWords: totalWords}, nil
}

// UpdateViewCounts 批量更新文章的浏览量
func (r *articleRepo) UpdateViewCounts(ctx context.Context, updates map[uint]int) error {
	if len(updates) == 0 {
		return nil
	}
	tx, err := r.db.Tx(ctx)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() {
		if v := recover(); v != nil {
			tx.Rollback()
			panic(v)
		}
	}()
	for id, increment := range updates {
		if err := tx.Article.UpdateOneID(id).AddViewCount(increment).Exec(ctx); err != nil {
			if rberr := tx.Rollback(); rberr != nil {
				return fmt.Errorf("更新文章ID %d 失败后，回滚事务也失败: update_err=%v, rollback_err=%v", id, err, rberr)
			}
			return fmt.Errorf("更新文章ID %d 的浏览量失败: %w", id, err)
		}
	}
	return tx.Commit()
}

// IncrementViewCount 原子地为给定文章的浏览次数加一
func (r *articleRepo) IncrementViewCount(ctx context.Context, publicID string) error {
	dbID, _, err := idgen.DecodePublicID(publicID)
	if err != nil {
		return err
	}
	_, err = r.db.Article.UpdateOneID(dbID).AddViewCount(1).Save(ctx)
	if err != nil {
		log.Printf("[ERROR] IncrementViewCount: 未能更新文章 (DB ID %d) 的浏览次数: %v", dbID, err)
	}
	return err
}

// Create 创建新文章
func (r *articleRepo) Create(ctx context.Context, params *model.CreateArticleParams) (*model.Article, error) {
	topImgURL := params.TopImgURL
	if topImgURL == "" {
		topImgURL = params.CoverURL
	}

	creator := r.db.Article.Create().
		SetTitle(params.Title).
		SetContentMd(params.ContentMd).
		SetContentHTML(params.ContentHTML).
		SetCoverURL(params.CoverURL).
		AddPostTagIDs(params.PostTagIDs...).
		AddPostCategoryIDs(params.PostCategoryIDs...).
		SetWordCount(params.WordCount).
		SetReadingTime(params.ReadingTime).
		SetIPLocation(params.IPLocation).
		SetPrimaryColor(params.PrimaryColor).
		SetIsPrimaryColorManual(params.IsPrimaryColorManual).
		SetHomeSort(params.HomeSort).
		SetPinSort(params.PinSort).
		SetTopImgURL(topImgURL).
		SetSummaries(params.Summaries).
		SetCopyright(params.Copyright).
		SetCopyrightAuthor(params.CopyrightAuthor).
		SetCopyrightAuthorHref(params.CopyrightAuthorHref).
		SetCopyrightURL(params.CopyrightURL)

	if params.Abbrlink != "" {
		creator.SetAbbrlink(params.Abbrlink)
	}

	if params.Status != "" {
		creator.SetStatus(article.Status(params.Status))
	} else {
		creator.SetStatus(article.StatusDRAFT)
	}

	newEntity, err := creator.Save(ctx)
	if err != nil {
		return nil, err
	}

	publicID, _ := idgen.GeneratePublicID(newEntity.ID, idgen.EntityTypeArticle)
	return r.GetByID(ctx, publicID)
}

// Update 更新文章
func (r *articleRepo) Update(ctx context.Context, publicID string, req *model.UpdateArticleRequest, computed *model.UpdateArticleComputedParams) (*model.Article, error) {
	dbID, _, err := idgen.DecodePublicID(publicID)
	if err != nil {
		return nil, err
	}
	updater := r.db.Article.UpdateOneID(dbID)
	if req.Title != nil {
		updater.SetTitle(*req.Title)
	}
	if req.ContentMd != nil {
		updater.SetContentMd(*req.ContentMd)
	}
	if req.CoverURL != nil {
		updater.SetCoverURL(*req.CoverURL)
	}
	if req.Status != nil {
		updater.SetStatus(article.Status(*req.Status))
	}
	if req.PostTagIDs != nil {
		tagDBIDs, err := idgen.DecodePublicIDBatch(req.PostTagIDs)
		if err != nil {
			return nil, err
		}
		updater.ClearPostTags().AddPostTagIDs(tagDBIDs...)
	}
	if req.PostCategoryIDs != nil {
		categoryDBIDs, err := idgen.DecodePublicIDBatch(req.PostCategoryIDs)
		if err != nil {
			return nil, err
		}
		updater.ClearPostCategories().AddPostCategoryIDs(categoryDBIDs...)
	}
	if req.IPLocation != nil {
		updater.SetIPLocation(*req.IPLocation)
	}
	if req.HomeSort != nil {
		updater.SetHomeSort(*req.HomeSort)
	}
	if req.PinSort != nil {
		updater.SetPinSort(*req.PinSort)
	}
	if req.Summaries != nil {
		updater.SetSummaries(req.Summaries)
	}
	if req.TopImgURL != nil {
		updater.SetTopImgURL(*req.TopImgURL)
	} else if req.CoverURL != nil {
		// fallback to cover_url if top_img_url is explicitly set to nil but cover_url is provided
		updater.SetTopImgURL(*req.CoverURL)
	}
	if req.Abbrlink != nil {
		if *req.Abbrlink == "" {
			updater.ClearAbbrlink()
		} else {
			updater.SetAbbrlink(*req.Abbrlink)
		}
	}
	if req.Copyright != nil {
		updater.SetCopyright(*req.Copyright)
	}
	if req.CopyrightAuthor != nil {
		updater.SetCopyrightAuthor(*req.CopyrightAuthor)
	}
	if req.CopyrightAuthorHref != nil {
		updater.SetCopyrightAuthorHref(*req.CopyrightAuthorHref)
	}
	if req.CopyrightURL != nil {
		updater.SetCopyrightURL(*req.CopyrightURL)
	}
	if computed != nil {
		if computed.WordCount > 0 || (req.ContentMd != nil && *req.ContentMd == "") {
			updater.SetWordCount(computed.WordCount)
			updater.SetReadingTime(computed.ReadingTime)
			updater.SetContentHTML(computed.ContentHTML)
		}
		if computed.PrimaryColor != nil {
			updater.SetPrimaryColor(*computed.PrimaryColor)
		}
		if computed.IsPrimaryColorManual != nil {
			updater.SetIsPrimaryColorManual(*computed.IsPrimaryColorManual)
		}
	}
	_, err = updater.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, publicID)
}

// ListPublic 获取公开的文章列表
func (r *articleRepo) ListPublic(ctx context.Context, options *model.ListPublicArticlesOptions) ([]*model.Article, int, error) {
	baseQuery := r.db.Article.Query().Where(
		article.StatusEQ(article.StatusPUBLISHED),
		article.DeletedAtIsNil(),
	)

	if options.CategoryName != "" {
		baseQuery = baseQuery.Where(article.HasPostCategoriesWith(postcategory.NameEQ(options.CategoryName)))
	}
	if options.TagName != "" {
		baseQuery = baseQuery.Where(article.HasPostTagsWith(posttag.NameEQ(options.TagName)))
	}

	applyDateFilter := func(s *sql.Selector) {
		if options.Year > 0 {
			s.Where(sql.ExprP(fmt.Sprintf("EXTRACT(YEAR FROM %s) = %d", s.C(article.FieldCreatedAt), options.Year)))
		}
		if options.Month > 0 {
			s.Where(sql.ExprP(fmt.Sprintf("EXTRACT(MONTH FROM %s) = %d", s.C(article.FieldCreatedAt), options.Month)))
		}
	}

	countQuery := baseQuery.Clone().Modify(applyDateFilter)
	total, err := countQuery.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	mainQuery := baseQuery.Clone().Modify(applyDateFilter)

	q := mainQuery.
		Order(
			ent.Desc(article.FieldPinSort),
			ent.Desc(article.FieldCreatedAt),
		).
		WithPostTags().
		WithPostCategories()

	if options.Page > 0 && options.PageSize > 0 {
		q = q.Offset((options.Page - 1) * options.PageSize).Limit(options.PageSize)
	}

	entities, err := q.Select(
		article.FieldID, article.FieldCreatedAt, article.FieldUpdatedAt,
		article.FieldTitle, article.FieldCoverURL, article.FieldStatus,
		article.FieldViewCount, article.FieldWordCount, article.FieldReadingTime,
		article.FieldIPLocation, article.FieldPrimaryColor, article.FieldIsPrimaryColorManual,
		article.FieldHomeSort, article.FieldPinSort, article.FieldTopImgURL,
		article.FieldSummaries, article.FieldAbbrlink, article.FieldCopyright,
		article.FieldCopyrightAuthor, article.FieldCopyrightAuthorHref, article.FieldCopyrightURL,
	).All(ctx)

	if err != nil {
		return nil, 0, err
	}
	return r.toModelSlice(entities), total, nil
}

// List 获取后台管理文章列表
func (r *articleRepo) List(ctx context.Context, options *model.ListArticlesOptions) ([]*model.Article, int, error) {
	query := r.db.Article.Query().Where(article.DeletedAtIsNil())
	if options.Query != "" {
		query = query.Where(article.TitleContains(options.Query))
	}
	if options.Status != "" {
		query = query.Where(article.StatusEQ(article.Status(options.Status)))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	q := query.Order(ent.Desc(article.FieldCreatedAt)).
		WithPostTags().
		WithPostCategories()

	if options.Page > 0 && options.PageSize > 0 {
		q = q.Offset((options.Page - 1) * options.PageSize).Limit(options.PageSize)
	}

	var entities []*ent.Article
	if !options.WithContent {
		entities, err = q.Select(
			article.FieldID, article.FieldCreatedAt, article.FieldUpdatedAt,
			article.FieldTitle, article.FieldCoverURL, article.FieldStatus,
			article.FieldViewCount, article.FieldWordCount, article.FieldReadingTime,
			article.FieldIPLocation, article.FieldPrimaryColor, article.FieldIsPrimaryColorManual,
			article.FieldHomeSort, article.FieldPinSort, article.FieldTopImgURL,
			article.FieldSummaries, article.FieldAbbrlink, article.FieldCopyright,
			article.FieldCopyrightAuthor, article.FieldCopyrightAuthorHref, article.FieldCopyrightURL,
		).All(ctx)
	} else {
		entities, err = q.All(ctx)
	}

	if err != nil {
		return nil, 0, err
	}
	return r.toModelSlice(entities), total, nil
}

// ListHome 获取首页推荐文章
func (r *articleRepo) ListHome(ctx context.Context) ([]*model.Article, error) {
	entities, err := r.db.Article.Query().
		Where(
			article.HomeSortGT(0),
			article.StatusEQ(article.StatusPUBLISHED),
			article.DeletedAtIsNil(),
		).
		Order(ent.Asc(article.FieldHomeSort)).
		Limit(6).
		WithPostTags().
		WithPostCategories().
		All(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModelSlice(entities), nil
}

// GetByID 根据公共ID获取单个文章
func (r *articleRepo) GetByID(ctx context.Context, publicID string) (*model.Article, error) {
	dbID, _, err := idgen.DecodePublicID(publicID)
	if err != nil {
		return nil, err
	}
	entity, err := r.db.Article.Query().
		Where(article.ID(dbID), article.DeletedAtIsNil()).
		WithPostTags().
		WithPostCategories().
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModel(entity), nil
}

// GetRandom 获取一篇随机文章
func (r *articleRepo) GetRandom(ctx context.Context) (*model.Article, error) {
	ids, err := r.db.Article.Query().
		Where(
			article.StatusEQ(article.StatusPUBLISHED),
			article.DeletedAtIsNil(),
		).
		IDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, constant.ErrNotFound
	}
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)
	randomID := ids[random.Intn(len(ids))]

	fullArticle, err := r.db.Article.Query().
		Where(article.ID(randomID)).
		WithPostTags().
		WithPostCategories().
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModel(fullArticle), nil
}

// Delete 软删除文章
func (r *articleRepo) Delete(ctx context.Context, publicID string) error {
	dbID, _, err := idgen.DecodePublicID(publicID)
	if err != nil {
		return err
	}
	return r.db.Article.DeleteOneID(dbID).Exec(ctx)
}
