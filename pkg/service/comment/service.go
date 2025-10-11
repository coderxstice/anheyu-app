// internal/app/service/comment/service.go
package comment

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/internal/app/task"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/auth"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/pkg/handler/comment/dto"
	"github.com/anzhiyu-c/anheyu-app/pkg/idgen"
	filesvc "github.com/anzhiyu-c/anheyu-app/pkg/service/file"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/parser"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/utility"

	"github.com/google/uuid"
)

// htmlInternalURIRegex 匹配HTML中的 src="anzhiyu://file/ID"
var htmlInternalURIRegex = regexp.MustCompile(`src="anzhiyu://file/([a-zA-Z0-9_-]+)"`)

// Service 评论服务的核心业务逻辑。
type Service struct {
	repo       repository.CommentRepository
	userRepo   repository.UserRepository
	txManager  repository.TransactionManager
	geoService utility.GeoIPService
	settingSvc setting.SettingService
	cacheSvc   utility.CacheService
	broker     *task.Broker
	fileSvc    filesvc.FileService
	parserSvc  *parser.Service
	pushooSvc  utility.PushooService
}

// NewService 创建一个新的评论服务实例。
func NewService(
	repo repository.CommentRepository,
	userRepo repository.UserRepository,
	txManager repository.TransactionManager,
	geoService utility.GeoIPService,
	settingSvc setting.SettingService,
	cacheSvc utility.CacheService,
	broker *task.Broker,
	fileSvc filesvc.FileService,
	parserSvc *parser.Service,
	pushooSvc utility.PushooService,
) *Service {
	return &Service{
		repo:       repo,
		userRepo:   userRepo,
		txManager:  txManager,
		geoService: geoService,
		settingSvc: settingSvc,
		cacheSvc:   cacheSvc,
		broker:     broker,
		fileSvc:    fileSvc,
		parserSvc:  parserSvc,
		pushooSvc:  pushooSvc,
	}
}

// UploadImage 负责处理评论图片的上传业务逻辑。
func (s *Service) UploadImage(ctx context.Context, viewerID uint, originalFilename string, fileReader io.Reader) (*model.FileItem, error) {
	newFileName := uuid.New().String() + filepath.Ext(originalFilename)
	fileItem, err := s.fileSvc.UploadFileByPolicyFlag(ctx, viewerID, fileReader, constant.PolicyFlagCommentImage, newFileName)
	if err != nil {
		return nil, fmt.Errorf("上传评论图片失败: %w", err)
	}
	return fileItem, nil
}

// ListLatest 获取全站最新的已发布评论列表（分页）。
func (s *Service) ListLatest(ctx context.Context, page, pageSize int) (*dto.ListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	comments, total, err := s.repo.FindAllPublishedPaginated(ctx, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("获取最新评论列表失败: %w", err)
	}

	// 收集所有需要查询的父评论ID
	parentIDs := make(map[uint]struct{})
	for _, comment := range comments {
		if comment.ParentID != nil {
			parentIDs[*comment.ParentID] = struct{}{}
		}
	}

	parentMap := make(map[uint]*model.Comment)
	if len(parentIDs) > 0 {
		// 将 map 的 key 转换为 slice
		ids := make([]uint, 0, len(parentIDs))
		for id := range parentIDs {
			ids = append(ids, id)
		}

		// 使用 FindManyByIDs 一次性批量查询所有父评论
		parents, err := s.repo.FindManyByIDs(ctx, ids)
		if err != nil {
			// 即便查询失败，也不应中断整个请求，仅记录日志。
			// 这样即使父评论信息丢失，主评论列表依然可以展示。
			log.Printf("警告：批量获取父评论失败: %v", err)
		} else {
			// 将查询结果转换为 map 以便后续快速查找
			for _, parent := range parents {
				parentMap[parent.ID] = parent
			}
		}
	}

	responses := make([]*dto.Response, len(comments))
	for i, comment := range comments {
		var parent *model.Comment
		if comment.ParentID != nil {
			// 从 map 中安全地获取父评论
			parent = parentMap[*comment.ParentID]
		}
		responses[i] = s.toResponseDTO(ctx, comment, parent, false)
	}

	return &dto.ListResponse{
		List:     responses,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *Service) Create(ctx context.Context, req *dto.CreateRequest, ip, ua string, claims *auth.CustomClaims) (*dto.Response, error) {
	limitStr := s.settingSvc.Get(constant.KeyCommentLimitPerMinute.String())
	limit, err := strconv.Atoi(limitStr)
	if err == nil && limit > 0 {
		redisKey := fmt.Sprintf("comment:rate_limit:%s:%s", ip, time.Now().Format("200601021504"))
		count, err := s.cacheSvc.Increment(ctx, redisKey)
		if err != nil {
			log.Printf("警告：Redis速率限制检查失败: %v", err)
		} else {
			if count == 1 {
				s.cacheSvc.Expire(ctx, redisKey, 70*time.Second)
			}
			if count > int64(limit) {
				return nil, errors.New("您的评论太频繁了，请稍后再试")
			}
		}
	}

	var parentDBID *uint
	var parentComment *model.Comment
	if req.ParentID != nil && *req.ParentID != "" {
		pID, _, err := idgen.DecodePublicID(*req.ParentID)
		if err != nil {
			return nil, errors.New("无效的父评论ID")
		}
		parentComment, err = s.repo.FindByID(ctx, pID)
		if err != nil {
			return nil, errors.New("回复的父评论不存在")
		}
		if parentComment.TargetPath != req.TargetPath {
			return nil, errors.New("回复的评论与当前页面不匹配")
		}
		parentDBID = &pID
	}

	// 从 Markdown 内容生成 HTML
	safeHTML, err := s.parserSvc.ToHTML(ctx, req.Content)
	if err != nil {
		return nil, fmt.Errorf("Markdown内容解析失败: %w", err)
	}
	var emailMD5 string
	if req.Email != nil {
		emailMD5 = fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(*req.Email))))
	}
	ipLocation := "未知"
	if ip != "" && s.geoService != nil {
		location, err := s.geoService.Lookup(ip)
		if err == nil {
			ipLocation = location
		}
	}
	status := model.StatusPublished
	forbiddenWords := s.settingSvc.Get(constant.KeyCommentForbiddenWords.String())
	if forbiddenWords != "" {
		for _, word := range strings.Split(forbiddenWords, ",") {
			trimmedWord := strings.TrimSpace(word)
			if trimmedWord != "" && strings.Contains(req.Content, trimmedWord) {
				status = model.StatusPending
				break
			}
		}
	}
	var isAdmin bool
	var userID *uint
	if claims != nil {
		userDBID, _, _ := idgen.DecodePublicID(claims.UserID)
		user, err := s.userRepo.FindByID(ctx, userDBID)
		if err == nil && user != nil {
			uid := user.ID
			userID = &uid
			if user.UserGroup.ID == 1 && req.Email != nil && user.Email == *req.Email {
				isAdmin = true
			}
		}
	} else {
		if req.Email != nil && *req.Email != "" {
			admins, err := s.userRepo.FindByGroupID(ctx, 1)
			if err != nil {
				log.Printf("警告：查询管理员列表失败: %v", err)
			} else {
				for _, admin := range admins {
					if admin.Email == *req.Email {
						return nil, constant.ErrAdminEmailUsedByGuest
					}
				}
			}
		}
	}

	// 使用前端传递的匿名标识，并在后端进行双重验证
	isAnonymous := req.IsAnonymous

	// 如果前端标记为匿名评论，且配置了匿名邮箱，则验证邮箱是否匹配
	if isAnonymous {
		anonymousEmail := s.settingSvc.Get(constant.KeyCommentAnonymousEmail.String())
		if anonymousEmail != "" {
			// 如果配置了匿名邮箱，但用户邮箱不匹配，拒绝请求
			if req.Email == nil || *req.Email != anonymousEmail {
				log.Printf("警告：前端标记为匿名评论，但邮箱不匹配。前端邮箱: %v, 配置的匿名邮箱: %s", req.Email, anonymousEmail)
				return nil, fmt.Errorf("匿名评论邮箱验证失败")
			}
		}
	}

	params := &repository.CreateCommentParams{
		TargetPath:        req.TargetPath,
		TargetTitle:       req.TargetTitle,
		UserID:            userID,
		ParentID:          parentDBID,
		Nickname:          req.Nickname,
		Email:             req.Email,
		EmailMD5:          emailMD5,
		Website:           req.Website,
		Content:           req.Content,
		ContentHTML:       safeHTML,
		UserAgent:         &ua,
		IPAddress:         ip,
		IPLocation:        ipLocation,
		Status:            int(status),
		IsAdminComment:    isAdmin,
		IsAnonymous:       isAnonymous,
		AllowNotification: req.AllowNotification,
	}

	newComment, err := s.repo.Create(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("保存评论失败: %w", err)
	}

	if newComment.IsPublished() {
		log.Printf("[DEBUG] 评论已发布，开始处理通知逻辑，评论ID: %d", newComment.ID)

		// 发送邮件通知
		if s.broker != nil {
			log.Printf("[DEBUG] 邮件通知任务已分发，评论ID: %d", newComment.ID)
			go s.broker.DispatchCommentNotification(newComment.ID)
		} else {
			log.Printf("[DEBUG] broker 为 nil，跳过邮件通知")
		}

		// 发送即时通知
		log.Printf("[DEBUG] 检查即时通知服务，pushooSvc 是否为 nil: %t", s.pushooSvc == nil)
		if s.pushooSvc != nil {
			go func() {
				log.Printf("[DEBUG] 开始处理即时通知逻辑")
				pushChannel := s.settingSvc.Get(constant.KeyPushooChannel.String())
				notifyAdmin := s.settingSvc.GetBool(constant.KeyCommentNotifyAdmin.String())
				scMailNotify := s.settingSvc.GetBool(constant.KeyScMailNotify.String())
				notifyReply := s.settingSvc.GetBool(constant.KeyCommentNotifyReply.String())
				adminEmail := s.settingSvc.Get(constant.KeyFrontDeskSiteOwnerEmail.String())

				log.Printf("[DEBUG] 即时通知配置检查:")
				log.Printf("[DEBUG]   - pushChannel: '%s'", pushChannel)
				log.Printf("[DEBUG]   - notifyAdmin: %t", notifyAdmin)
				log.Printf("[DEBUG]   - scMailNotify: %t", scMailNotify)
				log.Printf("[DEBUG]   - notifyReply: %t", notifyReply)

				if pushChannel != "" {
					log.Printf("[DEBUG] pushChannel 不为空，继续检查通知条件")

					// 检查新评论者是否是管理员本人
					var newCommenterEmail string
					if newComment.Author.Email != nil {
						newCommenterEmail = *newComment.Author.Email
					}
					isAdminComment := newCommenterEmail != "" && newCommenterEmail == adminEmail

					// 判断是否有回复父评论
					hasParentComment := parentComment != nil && parentComment.AllowNotification
					var parentEmail string
					if hasParentComment && parentComment.Author.Email != nil {
						parentEmail = *parentComment.Author.Email
					}

					// 判断被回复者是否是管理员
					parentIsAdmin := parentEmail != "" && parentEmail == adminEmail

					// 场景一：通知博主有新评论（顶级评论）
					// 条件：开启了博主通知、不是管理员自己评论、且没有父评论（或父评论作者不是管理员）
					if (notifyAdmin || scMailNotify) && !isAdminComment {
						// 如果有父评论且父评论作者是管理员，跳过博主通知（会在场景二中通知）
						if !parentIsAdmin {
							log.Printf("[DEBUG] 满足博主通知条件，开始发送即时通知")
							if err := s.pushooSvc.SendCommentNotification(ctx, newComment, nil); err != nil {
								log.Printf("[ERROR] 发送博主即时通知失败: %v", err)
							} else {
								log.Printf("[DEBUG] 博主即时通知发送成功")
							}
						} else {
							log.Printf("[DEBUG] 被回复者是管理员，将在场景二统一通知，跳过场景一")
						}
					}

					// 场景二：通知被回复者有新回复
					// 条件：开启了回复通知、有父评论、父评论允许通知、且不是自己回复自己
					if notifyReply && hasParentComment {
						// 如果新评论者不是父评论作者本人（避免自己回复自己）
						if parentEmail != "" && newCommenterEmail != parentEmail {
							log.Printf("[DEBUG] 满足回复通知条件，开始发送即时通知（被回复者：%s）", parentEmail)
							if err := s.pushooSvc.SendCommentNotification(ctx, newComment, parentComment); err != nil {
								log.Printf("[ERROR] 发送回复即时通知失败: %v", err)
							} else {
								log.Printf("[DEBUG] 回复即时通知发送成功")
							}
						} else {
							log.Printf("[DEBUG] 不满足回复通知条件（自己回复自己或无邮箱），跳过")
						}
					}
				} else {
					log.Printf("[DEBUG] pushChannel 为空，跳过即时通知")
				}
			}()
		} else {
			log.Printf("[DEBUG] pushooSvc 为 nil，跳过即时通知")
		}
	} else {
		log.Printf("[DEBUG] 评论未发布，跳过所有通知逻辑")
	}

	return s.toResponseDTO(ctx, newComment, parentComment, false), nil
}

// ListByPath
func (s *Service) ListByPath(ctx context.Context, path string, page, pageSize int) (*dto.ListResponse, error) {
	// 1. 一次性获取该路径下的所有已发布评论
	allComments, err := s.repo.FindAllPublishedByPath(ctx, path)
	if err != nil {
		return nil, err
	}

	// 2. 在内存中构建评论树和关系图
	commentMap := make(map[uint]*model.Comment, len(allComments))
	var rootComments []*model.Comment
	descendantsMap := make(map[uint][]*model.Comment)

	for _, c := range allComments {
		comment := c
		commentMap[comment.ID] = comment
		if comment.IsTopLevel() {
			rootComments = append(rootComments, comment)
		}
	}

	for _, c := range allComments {
		if !c.IsTopLevel() {
			ancestor := c
			visited := make(map[uint]bool)
			for ancestor.ParentID != nil {
				if visited[ancestor.ID] {
					break
				}
				visited[ancestor.ID] = true

				parent, ok := commentMap[*ancestor.ParentID]
				if !ok {
					ancestor = nil
					break
				}
				ancestor = parent
			}

			if ancestor != nil && ancestor.IsTopLevel() {
				rootID := ancestor.ID
				descendantsMap[rootID] = append(descendantsMap[rootID], c)
			}
		}
	}

	// 3. 对根评论进行排序
	sort.Slice(rootComments, func(i, j int) bool {
		iPinned := rootComments[i].PinnedAt != nil
		jPinned := rootComments[j].PinnedAt != nil
		if iPinned != jPinned {
			return iPinned
		}
		if iPinned && jPinned {
			return rootComments[i].PinnedAt.After(*rootComments[j].PinnedAt)
		}
		return rootComments[i].CreatedAt.After(rootComments[j].CreatedAt)
	})

	// 4. 对根评论进行分页
	total := int64(len(rootComments))
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= len(rootComments) {
		return &dto.ListResponse{List: []*dto.Response{}, Total: total, Page: page, PageSize: pageSize}, nil
	}
	if end > len(rootComments) {
		end = len(rootComments)
	}
	paginatedRootComments := rootComments[start:end]

	// 5. 组装最终响应
	const previewLimit = 3
	rootResponses := make([]*dto.Response, len(paginatedRootComments))
	for i, root := range paginatedRootComments {
		rootResp := s.toResponseDTO(ctx, root, nil, false)
		descendants := descendantsMap[root.ID]

		rootResp.TotalChildren = int64(len(descendants))

		var previewChildren []*model.Comment
		if len(descendants) > previewLimit {
			previewChildren = descendants[len(descendants)-previewLimit:]
		} else {
			previewChildren = descendants
		}

		// 反转切片，确保预览评论按时间倒序（从新到旧）显示
		for i, j := 0, len(previewChildren)-1; i < j; i, j = i+1, j-1 {
			previewChildren[i], previewChildren[j] = previewChildren[j], previewChildren[i]
		}

		childResponses := make([]*dto.Response, len(previewChildren))
		for j, child := range previewChildren {
			parent, _ := commentMap[*child.ParentID]
			childResponses[j] = s.toResponseDTO(ctx, child, parent, false)
		}
		rootResp.Children = childResponses
		rootResponses[i] = rootResp
	}

	return &dto.ListResponse{
		List:     rootResponses,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ListChildren - 最终正确版本
func (s *Service) ListChildren(ctx context.Context, parentPublicID string, page, pageSize int) (*dto.ListResponse, error) {
	parentDBID, _, err := idgen.DecodePublicID(parentPublicID)
	if err != nil {
		return nil, errors.New("无效的父评论ID")
	}

	// 1. 查找父评论，并获取其所属的页面路径
	parentComment, err := s.repo.FindByID(ctx, parentDBID)
	if err != nil {
		return nil, fmt.Errorf("查找父评论失败: %w", err)
	}

	// 2. 获取该路径下的所有评论，以便构建完整的关系树
	allComments, err := s.repo.FindAllPublishedByPath(ctx, parentComment.TargetPath)
	if err != nil {
		return nil, err
	}

	commentMap := make(map[uint]*model.Comment, len(allComments))
	for _, c := range allComments {
		commentMap[c.ID] = c
	}

	// 3. 递归查找指定父评论的所有后代
	var allDescendants []*model.Comment
	var findChildren func(pID uint)

	findChildren = func(pID uint) {
		for _, comment := range allComments {
			if comment.ParentID != nil && *comment.ParentID == pID {
				allDescendants = append(allDescendants, comment)
				findChildren(comment.ID)
			}
		}
	}
	findChildren(parentDBID)

	// 4. 对所有后代按时间倒序重新排序 (从新到旧)
	sort.Slice(allDescendants, func(i, j int) bool {
		return allDescendants[i].CreatedAt.After(allDescendants[j].CreatedAt)
	})

	// 5. 对所有后代进行分页
	total := int64(len(allDescendants))
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= len(allDescendants) {
		return &dto.ListResponse{List: []*dto.Response{}, Total: total, Page: page, PageSize: pageSize}, nil
	}
	if end > len(allDescendants) {
		end = len(allDescendants)
	}
	paginatedDescendants := allDescendants[start:end]

	// 6. 组装响应
	childResponses := make([]*dto.Response, len(paginatedDescendants))
	for i, child := range paginatedDescendants {
		parent, _ := commentMap[*child.ParentID]
		childResponses[i] = s.toResponseDTO(ctx, child, parent, false)
	}

	return &dto.ListResponse{
		List:     childResponses,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// toResponseDTO 将领域模型 comment 转换为API响应的DTO。
func (s *Service) toResponseDTO(ctx context.Context, c *model.Comment, parent *model.Comment, isAdminView bool) *dto.Response {
	if c == nil {
		return nil
	}
	publicID, _ := idgen.GeneratePublicID(c.ID, idgen.EntityTypeComment)

	// 统一使用解析后的HTML，确保表情包正确显示
	parsedHTML, err := s.parserSvc.ToHTML(ctx, c.Content)
	var renderedContentHTML string
	if err != nil {
		log.Printf("【WARN】解析评论 %s 的表情包失败: %v", publicID, err)
		renderedContentHTML = c.ContentHTML
	} else {
		renderedContentHTML = parsedHTML
	}

	// 渲染图片URL
	renderedContentHTML, err = s.renderHTMLURLs(ctx, renderedContentHTML)
	if err != nil {
		log.Printf("【WARN】渲染评论 %s 的HTML链接失败: %v", publicID, err)
		renderedContentHTML = c.ContentHTML
	}

	var emailMD5 string
	if c.Author.Email != nil {
		emailMD5 = fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(*c.Author.Email))))
	}
	var parentPublicID *string
	var replyToNick *string
	if parent != nil {
		pID, _ := idgen.GeneratePublicID(parent.ID, idgen.EntityTypeComment)
		parentPublicID = &pID
		replyToNick = &parent.Author.Nickname
	}
	showUA := s.settingSvc.GetBool(constant.KeyCommentShowUA.String())
	showRegion := s.settingSvc.GetBool(constant.KeyCommentShowRegion.String())

	resp := &dto.Response{
		ID:             publicID,
		CreatedAt:      c.CreatedAt,
		PinnedAt:       c.PinnedAt,
		Nickname:       c.Author.Nickname,
		EmailMD5:       emailMD5,
		Website:        c.Author.Website,
		ContentHTML:    renderedContentHTML,
		IsAdminComment: c.IsAdminAuthor,
		TargetPath:     c.TargetPath,
		TargetTitle:    c.TargetTitle,
		ParentID:       parentPublicID,
		ReplyToNick:    replyToNick,
		LikeCount:      c.LikeCount,
		Children:       []*dto.Response{},
	}

	if showUA {
		ua := c.Author.UserAgent
		resp.UserAgent = &ua
	}
	if showRegion {
		loc := c.Author.Location
		resp.IPLocation = loc
	}

	if isAdminView {
		resp.Email = c.Author.Email
		resp.IPAddress = &c.Author.IP
		resp.Content = &c.Content
		status := int(c.Status)
		resp.Status = &status
	}

	return resp
}

// renderHTMLURLs 将HTML内容中的内部URI（anzhiyu://file/...）替换为可访问的临时URL。
func (s *Service) renderHTMLURLs(ctx context.Context, htmlContent string) (string, error) {
	var firstError error
	replacer := func(match string) string {
		parts := htmlInternalURIRegex.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		publicID := parts[1]
		fileModel, err := s.fileSvc.FindFileByPublicID(ctx, publicID)
		if err != nil {
			log.Printf("【ERROR】渲染图片失败：找不到文件, PublicID=%s, 错误: %v", publicID, err)
			return `src=""`
		}
		url, err := s.fileSvc.GetDownloadURLForFile(ctx, fileModel, publicID)
		if err != nil {
			if firstError == nil {
				firstError = err
			}
			log.Printf("【ERROR】渲染图片失败：为文件 %s 生成URL时出错: %v", publicID, err)
			return `src=""`
		}
		return `src="` + url + `"`
	}
	result := htmlInternalURIRegex.ReplaceAllStringFunc(htmlContent, replacer)
	return result, firstError
}

// LikeComment 为评论增加点赞数。
func (s *Service) LikeComment(ctx context.Context, publicID string) (int, error) {
	dbID, _, err := idgen.DecodePublicID(publicID)
	if err != nil {
		return 0, errors.New("无效的评论ID")
	}
	updatedComment, err := s.repo.IncrementLikeCount(ctx, dbID)
	if err != nil {
		return 0, fmt.Errorf("点赞失败: %w", err)
	}
	return updatedComment.LikeCount, nil
}

// UnlikeComment 为评论减少点赞数。
func (s *Service) UnlikeComment(ctx context.Context, publicID string) (int, error) {
	dbID, _, err := idgen.DecodePublicID(publicID)
	if err != nil {
		return 0, errors.New("无效的评论ID")
	}
	updatedComment, err := s.repo.DecrementLikeCount(ctx, dbID)
	if err != nil {
		return 0, fmt.Errorf("取消点赞失败: %w", err)
	}
	return updatedComment.LikeCount, nil
}

// AdminList 管理员根据条件查询评论列表。
func (s *Service) AdminList(ctx context.Context, req *dto.AdminListRequest) (*dto.ListResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 10
	}

	params := repository.AdminListParams{
		Page:       req.Page,
		PageSize:   req.PageSize,
		Nickname:   req.Nickname,
		Email:      req.Email,
		IPAddress:  req.IPAddress,
		Content:    req.Content,
		TargetPath: req.TargetPath,
		Status:     req.Status,
	}
	comments, total, err := s.repo.FindWithConditions(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("获取评论列表失败: %w", err)
	}

	responses := make([]*dto.Response, len(comments))
	for i, comment := range comments {
		responses[i] = s.toResponseDTO(ctx, comment, nil, true)
	}

	return &dto.ListResponse{
		List:     responses,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Delete 批量删除评论。
func (s *Service) Delete(ctx context.Context, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, errors.New("必须提供至少一个评论ID")
	}
	dbIDs := make([]uint, 0, len(ids))
	for _, publicID := range ids {
		dbID, entityType, err := idgen.DecodePublicID(publicID)
		if err != nil || entityType != idgen.EntityTypeComment {
			log.Printf("警告：跳过无效的评论ID '%s' 进行删除", publicID)
			continue
		}
		dbIDs = append(dbIDs, dbID)
	}
	if len(dbIDs) == 0 {
		return 0, errors.New("未提供任何有效的评论ID")
	}
	return s.repo.DeleteByIDs(ctx, dbIDs)
}

// UpdateStatus 更新评论的状态。
func (s *Service) UpdateStatus(ctx context.Context, publicID string, status int) (*dto.Response, error) {
	s_ := model.Status(status)
	if s_ != model.StatusPublished && s_ != model.StatusPending {
		return nil, errors.New("无效的状态值，必须是 1 (已发布) 或 2 (待审核)")
	}
	dbID, entityType, err := idgen.DecodePublicID(publicID)
	if err != nil || entityType != idgen.EntityTypeComment {
		return nil, errors.New("无效的评论ID")
	}
	updatedComment, err := s.repo.UpdateStatus(ctx, dbID, s_)
	if err != nil {
		return nil, fmt.Errorf("更新评论状态失败: %w", err)
	}
	return s.toResponseDTO(ctx, updatedComment, nil, true), nil
}

// SetPin 设置或取消评论的置顶状态。
func (s *Service) SetPin(ctx context.Context, publicID string, isPinned bool) (*dto.Response, error) {
	dbID, _, err := idgen.DecodePublicID(publicID)
	if err != nil {
		return nil, errors.New("无效的评论ID")
	}
	var pinTime *time.Time
	if isPinned {
		now := time.Now()
		pinTime = &now
	}
	updatedComment, err := s.repo.SetPin(ctx, dbID, pinTime)
	if err != nil {
		return nil, fmt.Errorf("设置评论置顶状态失败: %w", err)
	}
	return s.toResponseDTO(ctx, updatedComment, nil, true), nil
}

// UpdatePath 是一项内部服务，用于在文章或页面的路径（slug）变更时，同步更新所有相关评论的路径。
// 这个方法通常由其他服务（如ArticleService）通过事件或直接调用的方式触发。
func (s *Service) UpdatePath(ctx context.Context, oldPath, newPath string) (int, error) {
	if oldPath == "" || newPath == "" || oldPath == newPath {
		return 0, errors.New("无效的旧路径或新路径")
	}
	return s.repo.UpdatePath(ctx, oldPath, newPath)
}
