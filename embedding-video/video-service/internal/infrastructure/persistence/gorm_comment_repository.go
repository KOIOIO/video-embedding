package persistence

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"video-service/internal/application/videoapp"
	"video-service/internal/model"
)

// InsertComment 写入一条评论或二级回复，返回新记录 ID。
func (r *GormVideoRepository) InsertComment(ctx context.Context, comment *videoapp.Comment) (uint64, error) {
	if comment == nil {
		return 0, errors.New("comment is required")
	}
	row := &model.EduVideoComment{
		UserID:         comment.UserID,
		VideoSegmentID: comment.VideoSegmentID,
		RootID:         comment.RootID,
		ParentID:       comment.ParentID,
		ReplyToUserID:  comment.ReplyToUserID,
		Content:        comment.Content,
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

// GetCommentByID 按 ID 查询未删除的评论。
func (r *GormVideoRepository) GetCommentByID(ctx context.Context, id uint64) (videoapp.Comment, bool, error) {
	var row model.EduVideoComment
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted = 0", id).Limit(1).Find(&row).Error; err != nil {
		return videoapp.Comment{}, false, err
	}
	if row.ID == 0 {
		return videoapp.Comment{}, false, nil
	}
	return mapComment(row), true, nil
}

// SegmentExists 判断视频片段是否存在且未删除。
func (r *GormVideoRepository) SegmentExists(ctx context.Context, segmentID uint64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.EduVideoSegment{}).
		Where("id = ? AND deleted = 0", segmentID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListTopComments 分页查询片段下的一级评论（root_id = 0），按创建时间倒序。
func (r *GormVideoRepository) ListTopComments(ctx context.Context, segmentID uint64, page int, pageSize int) ([]videoapp.Comment, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.EduVideoComment{}).
		Where("video_segment_id = ? AND root_id = 0 AND deleted = 0", segmentID)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []model.EduVideoComment
	if err := base.Order("create_time DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return mapComments(rows), total, nil
}

// ListReplies 分页查询一级评论下的二级回复，按创建时间正序。
func (r *GormVideoRepository) ListReplies(ctx context.Context, rootID uint64, page int, pageSize int) ([]videoapp.Comment, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.EduVideoComment{}).
		Where("root_id = ? AND parent_id <> 0 AND deleted = 0", rootID)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []model.EduVideoComment
	if err := base.Order("create_time ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return mapComments(rows), total, nil
}

// CountComments 统计片段下的评论总数（含二级回复）。
func (r *GormVideoRepository) CountComments(ctx context.Context, segmentID uint64) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.EduVideoComment{}).
		Where("video_segment_id = ? AND deleted = 0", segmentID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetCommentReactionCounts 批量查询评论互动计数（like 与 double_like）。
func (r *GormVideoRepository) GetCommentReactionCounts(ctx context.Context, commentIDs []uint64) (map[uint64]videoapp.VideoReactionCounts, error) {
	result := make(map[uint64]videoapp.VideoReactionCounts, len(commentIDs))
	if len(commentIDs) == 0 {
		return result, nil
	}
	rows, err := r.db.WithContext(ctx).Model(&model.EduCommentLike{}).
		Select("comment_id, reaction_type, COUNT(*) AS cnt").
		Where("comment_id IN ? AND deleted = 0", commentIDs).
		Group("comment_id, reaction_type").
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var commentID uint64
		var reactionType string
		var count int64
		if err := rows.Scan(&commentID, &reactionType, &count); err != nil {
			return nil, err
		}
		counts := result[commentID]
		switch videoapp.VideoReactionType(reactionType) {
		case videoapp.VideoReactionLike:
			counts.LikeCount = count
		case videoapp.VideoReactionDoubleLike:
			counts.DoubleLikeCount = count
		}
		result[commentID] = counts
	}
	return result, rows.Err()
}

// GetUserCommentReactionTypes 批量查询用户对评论的当前互动类型。
func (r *GormVideoRepository) GetUserCommentReactionTypes(ctx context.Context, commentIDs []uint64, userID uint64) (map[uint64]videoapp.VideoReactionType, error) {
	result := make(map[uint64]videoapp.VideoReactionType, len(commentIDs))
	if len(commentIDs) == 0 || userID == 0 {
		return result, nil
	}
	rows, err := r.db.WithContext(ctx).Model(&model.EduCommentLike{}).
		Select("comment_id, reaction_type").
		Where("comment_id IN ? AND user_id = ? AND deleted = 0", commentIDs, userID).
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var commentID uint64
		var reactionType string
		if err := rows.Scan(&commentID, &reactionType); err != nil {
			return nil, err
		}
		result[commentID] = videoapp.VideoReactionType(reactionType)
	}
	return result, rows.Err()
}

// ApplyCommentReactionState 按最终状态幂等写入用户对评论的互动，并同步评论互动计数。
func (r *GormVideoRepository) ApplyCommentReactionState(ctx context.Context, commentID uint64, userID uint64, reactionType videoapp.VideoReactionType, active bool) (bool, error) {
	var found bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var comment model.EduVideoComment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("id = ? AND deleted = ?", commentID, 0).
			First(&comment).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				found = false
				return nil
			}
			return err
		}
		found = true

		newColumn := reactionCounterColumn(reactionType)
		if newColumn == "" {
			return nil
		}

		var like model.EduCommentLike
		res := tx.Where("comment_id = ? AND user_id = ?", commentID, userID).Limit(1).Find(&like)
		if res.Error != nil {
			return res.Error
		}
		rowExists := res.RowsAffected > 0

		if !rowExists {
			if !active {
				return nil
			}
			if err := tx.Create(&model.EduCommentLike{
				CommentID:    commentID,
				UserID:       userID,
				ReactionType: string(reactionType),
				Deleted:      0,
			}).Error; err != nil {
				return err
			}
			return tx.Model(&model.EduVideoComment{}).
				Where("id = ?", commentID).
				UpdateColumn(newColumn, gorm.Expr(newColumn+" + ?", 1)).Error
		}

		oldType := videoapp.VideoReactionType(like.ReactionType)
		oldColumn := reactionCounterColumn(oldType)
		isActive := like.Deleted == 0

		if !active {
			if !isActive || oldType != reactionType {
				return nil
			}
			if err := tx.Model(&model.EduCommentLike{}).
				Where("id = ?", like.ID).
				Update("deleted", 1).Error; err != nil {
				return err
			}
			if oldColumn == "" {
				return nil
			}
			return tx.Model(&model.EduVideoComment{}).
				Where("id = ?", commentID).
				UpdateColumn(oldColumn, gorm.Expr(oldColumn+" - ?", 1)).Error
		}

		if isActive && oldType == reactionType {
			return nil
		}
		if err := tx.Model(&model.EduCommentLike{}).
			Where("id = ?", like.ID).
			Updates(map[string]any{
				"reaction_type": string(reactionType),
				"deleted":       0,
			}).Error; err != nil {
			return err
		}
		if isActive && oldColumn != "" {
			if err := tx.Model(&model.EduVideoComment{}).
				Where("id = ?", commentID).
				UpdateColumn(oldColumn, gorm.Expr(oldColumn+" - ?", 1)).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.EduVideoComment{}).
			Where("id = ?", commentID).
			UpdateColumn(newColumn, gorm.Expr(newColumn+" + ?", 1)).Error
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

// GetUserNamesByIDs 批量查询用户展示名，优先 real_name，回退 username。
func (r *GormVideoRepository) GetUserNamesByIDs(ctx context.Context, userIDs []uint64) (map[uint64]string, error) {
	result := make(map[uint64]string, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	type nameRow struct {
		ID          uint64
		Username    string
		RealName    string
	}
	var rows []nameRow
	if err := r.db.WithContext(ctx).Table("sys_user").
		Select("id, username, real_name").
		Where("id IN ? AND deleted = 0", userIDs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		name := row.RealName
		if name == "" {
			name = row.Username
		}
		if name == "" {
			name = fmt.Sprintf("用户%d", row.ID)
		}
		result[row.ID] = name
	}
	return result, nil
}

func mapComment(row model.EduVideoComment) videoapp.Comment {
	return videoapp.Comment{
		ID:              row.ID,
		UserID:          row.UserID,
		VideoSegmentID:  row.VideoSegmentID,
		RootID:          row.RootID,
		ParentID:        row.ParentID,
		ReplyToUserID:   row.ReplyToUserID,
		Content:         row.Content,
		LikeCount:       row.LikeCount,
		DoubleLikeCount: row.DoubleLikeCount,
		CreatedAt:       row.CreateTime,
	}
}

func mapComments(rows []model.EduVideoComment) []videoapp.Comment {
	comments := make([]videoapp.Comment, 0, len(rows))
	for _, row := range rows {
		comments = append(comments, mapComment(row))
	}
	return comments
}

// GetUserDisplayInfoByIDs 批量查询用户展示信息，LEFT JOIN edu_user_profile，回退 sys_user.username。
func (r *GormVideoRepository) GetUserDisplayInfoByIDs(ctx context.Context, userIDs []uint64) (map[uint64]videoapp.UserDisplayInfo, error) {
	result := make(map[uint64]videoapp.UserDisplayInfo, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	type row struct {
		ID        uint64 `gorm:"column:id"`
		Username  string `gorm:"column:username"`
		Nickname  string `gorm:"column:nickname"`
		AvatarURL string `gorm:"column:avatar_url"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("sys_user AS u").
		Select(`u.id, u.username,
			COALESCE(p.nickname, '') AS nickname,
			COALESCE(p.avatar_url, '') AS avatar_url`).
		Joins("LEFT JOIN edu_user_profile AS p ON p.user_id = u.id AND p.deleted = 0").
		Where("u.id IN ? AND u.deleted = 0", userIDs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.ID] = videoapp.UserDisplayInfo{
			UserID:    r.ID,
			Username:  r.Username,
			Nickname:  r.Nickname,
			AvatarURL: r.AvatarURL,
		}
	}
	return result, nil
}

// FindUserIDsByNicknames 按昵称批量查找用户 ID，用于评论 @提及 解析。
func (r *GormVideoRepository) FindUserIDsByNicknames(ctx context.Context, nicknames []string) (map[string]uint64, error) {
	result := make(map[string]uint64, len(nicknames))
	if len(nicknames) == 0 {
		return result, nil
	}
	type row struct {
		UserID   uint64 `gorm:"column:user_id"`
		Nickname string `gorm:"column:nickname"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("edu_user_profile").
		Select("user_id, nickname").
		Where("nickname IN ? AND deleted = 0", nicknames).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.Nickname] = r.UserID
	}
	return result, nil
}
