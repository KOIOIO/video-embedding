package persistence

import (
	"context"
	"sort"
	"time"

	"gorm.io/gorm"

	"video-service/internal/application/knowledgevideo"
	"video-service/internal/model"
)

type GormKnowledgeVideoRepository struct {
	db        *gorm.DB
	allocator knowledgevideo.IDReservation
}

func NewGormKnowledgeVideoRepository(db *gorm.DB) *GormKnowledgeVideoRepository {
	return NewGormKnowledgeVideoRepositoryWithAllocator(db, postgresKnowledgeVideoIDAllocator{db: db})
}

func NewGormKnowledgeVideoRepositoryWithAllocator(db *gorm.DB, allocator knowledgevideo.IDReservation) *GormKnowledgeVideoRepository {
	return &GormKnowledgeVideoRepository{db: db, allocator: allocator}
}

func (r *GormKnowledgeVideoRepository) LookupKnowledgePoints(ctx context.Context, ids []uint64) ([]knowledgevideo.DictionaryKnowledgePoint, error) {
	ids = uniqueKnowledgeVideoIDs(ids)
	if len(ids) == 0 {
		return []knowledgevideo.DictionaryKnowledgePoint{}, nil
	}
	nameExpr := "''"
	if r.db.Migrator().HasColumn("dict_knowledge_point", "name") {
		nameExpr = "COALESCE(name, '')"
	}
	query := r.db.WithContext(ctx).Table("dict_knowledge_point").Select("id, "+nameExpr+" AS name").Where("id IN ?", ids)
	if r.db.Migrator().HasColumn("dict_knowledge_point", "deleted") {
		query = query.Where("COALESCE(deleted, 0) = 0")
	}
	var rows []knowledgevideo.DictionaryKnowledgePoint
	if err := query.Order("id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *GormKnowledgeVideoRepository) ListKnowledgeTree(ctx context.Context) ([]knowledgevideo.KnowledgeTreeNode, error) {
	hasParentID := r.db.Migrator().HasColumn("dict_knowledge_point", "parent_id")
	parentExpr := "0"
	if hasParentID {
		parentExpr = "COALESCE(parent_id, 0)"
	}
	nameExpr := "''"
	if r.db.Migrator().HasColumn("dict_knowledge_point", "name") {
		nameExpr = "COALESCE(name, '')"
	}
	type dictionaryRow struct {
		ID       uint64
		ParentID uint64
		Name     string
	}
	query := r.db.WithContext(ctx).Table("dict_knowledge_point").Select("id, " + parentExpr + " AS parent_id, " + nameExpr + " AS name")
	if r.db.Migrator().HasColumn("dict_knowledge_point", "deleted") {
		query = query.Where("COALESCE(deleted, 0) = 0")
	}
	var dictionary []dictionaryRow
	if err := query.Order("id ASC").Scan(&dictionary).Error; err != nil {
		return nil, err
	}
	var videos []model.EduKnowledgeVideo
	if err := r.db.WithContext(ctx).Where("deleted = 0").Order("id ASC").Find(&videos).Error; err != nil {
		return nil, err
	}
	videosByPoint := make(map[uint64][]knowledgevideo.KnowledgeTreeVideo, len(videos))
	for _, video := range videos {
		videosByPoint[video.KnowledgePointID] = append(videosByPoint[video.KnowledgePointID], knowledgevideo.KnowledgeTreeVideo{ID: video.ID, BatchID: video.BatchID, VideoName: video.SourceFileName, Duration: video.Duration, Status: knowledgevideo.VideoStatus(video.Status), ErrorMessage: video.ErrorMessage})
	}
	nodes := make(map[uint64]dictionaryRow, len(dictionary))
	children := make(map[uint64][]uint64)
	var roots []uint64
	for _, row := range dictionary {
		nodes[row.ID] = row
	}
	for _, row := range dictionary {
		if row.ParentID == 0 || nodes[row.ParentID].ID == 0 {
			roots = append(roots, row.ID)
		} else {
			children[row.ParentID] = append(children[row.ParentID], row.ID)
		}
	}
	var build func(uint64) knowledgevideo.KnowledgeTreeNode
	build = func(id uint64) knowledgevideo.KnowledgeTreeNode {
		row := nodes[id]
		videos := videosByPoint[id]
		if videos == nil {
			videos = []knowledgevideo.KnowledgeTreeVideo{}
		}
		node := knowledgevideo.KnowledgeTreeNode{ID: row.ID, ParentID: row.ParentID, Name: row.Name, Children: []knowledgevideo.KnowledgeTreeNode{}, Videos: videos}
		if len(videos) > 0 {
			node.Video = &node.Videos[0]
		}
		for _, childID := range children[id] {
			node.Children = append(node.Children, build(childID))
		}
		return node
	}
	result := make([]knowledgevideo.KnowledgeTreeNode, 0, len(roots))
	for _, id := range roots {
		result = append(result, build(id))
	}
	if !hasParentID {
		return []knowledgevideo.KnowledgeTreeNode{{ID: 0, Name: "全部知识点", Children: result}}, nil
	}
	return result, nil
}

func (r *GormKnowledgeVideoRepository) ReserveBatchID(ctx context.Context) (uint64, error) {
	return r.allocator.ReserveBatchID(ctx)
}

func (r *GormKnowledgeVideoRepository) ReserveVideoIDs(ctx context.Context, count int) ([]uint64, error) {
	if count <= 0 {
		return []uint64{}, nil
	}
	return r.allocator.ReserveVideoIDs(ctx, count)
}

func (r *GormKnowledgeVideoRepository) CreateBatch(ctx context.Context, batch knowledgevideo.Batch, videos []knowledgevideo.Video) error {
	batchModel := toKnowledgeVideoBatchModel(batch)
	videoModels := make([]model.EduKnowledgeVideo, 0, len(videos))
	for _, video := range videos {
		videoModels = append(videoModels, toKnowledgeVideoModel(video))
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&batchModel).Error; err != nil {
			return err
		}
		if len(videoModels) == 0 {
			return nil
		}
		return tx.CreateInBatches(&videoModels, 100).Error
	})
}

func (r *GormKnowledgeVideoRepository) GetVideo(ctx context.Context, id uint64) (knowledgevideo.Video, bool, error) {
	var row model.EduKnowledgeVideo
	err := r.db.WithContext(ctx).Where("id = ? AND deleted = 0", id).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return knowledgevideo.Video{}, false, nil
	}
	if err != nil {
		return knowledgevideo.Video{}, false, err
	}
	return toKnowledgeVideo(row), true, nil
}

func (r *GormKnowledgeVideoRepository) ListReadyByKnowledgePoint(ctx context.Context, knowledgePointID uint64) ([]knowledgevideo.Video, error) {
	var rows []model.EduKnowledgeVideo
	if err := r.db.WithContext(ctx).
		Where("knowledge_point_id = ? AND status = ? AND deleted = 0", knowledgePointID, int16(knowledgevideo.VideoReady)).
		Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	videos := make([]knowledgevideo.Video, 0, len(rows))
	for _, row := range rows {
		videos = append(videos, toKnowledgeVideo(row))
	}
	return videos, nil
}

func (r *GormKnowledgeVideoRepository) RecordPlayback(ctx context.Context, record knowledgevideo.PlayRecord) error {
	row := model.EduKnowledgeVideoPlayRecord{
		UserID:           record.UserID,
		KnowledgePointID: record.KnowledgePointID,
		KnowledgeVideoID: record.KnowledgeVideoID,
		CreateTime:       record.CreateTime,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *GormKnowledgeVideoRepository) MarkTranscoding(ctx context.Context, id uint64) (bool, error) {
	return r.updateActiveVideo(ctx, id, map[string]any{"status": int16(knowledgevideo.VideoTranscoding), "error_message": ""})
}

func (r *GormKnowledgeVideoRepository) MarkReady(ctx context.Context, id uint64, masterObjectKey string, duration int) (bool, error) {
	return r.updateActiveVideo(ctx, id, map[string]any{
		"status":                int16(knowledgevideo.VideoReady),
		"hls_master_object_key": masterObjectKey,
		"duration":              duration,
		"error_message":         "",
	})
}

func (r *GormKnowledgeVideoRepository) MarkFailed(ctx context.Context, id uint64, errorMessage string) (bool, error) {
	return r.updateActiveVideo(ctx, id, map[string]any{"status": int16(knowledgevideo.VideoFailed), "error_message": errorMessage})
}

func (r *GormKnowledgeVideoRepository) SetEnqueueTime(ctx context.Context, id uint64, enqueueTime time.Time) (bool, error) {
	return r.updateActiveVideo(ctx, id, map[string]any{"enqueue_time": enqueueTime})
}

func (r *GormKnowledgeVideoRepository) RefreshBatchStatus(ctx context.Context, batchID uint64) (knowledgevideo.Batch, bool, error) {
	var result knowledgevideo.Batch
	var found bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var batch model.EduKnowledgeVideoBatch
		if err := tx.Where("id = ?", batchID).First(&batch).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}
		found = true

		var stats struct {
			Total  int
			Ready  int
			Failed int
		}
		if err := tx.Model(&model.EduKnowledgeVideo{}).
			Select(`COUNT(*) AS total,
				COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS ready,
				COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS failed`, int16(knowledgevideo.VideoReady), int16(knowledgevideo.VideoFailed)).
			Where("batch_id = ? AND deleted = 0", batchID).Scan(&stats).Error; err != nil {
			return err
		}
		status := knowledgevideo.BatchProcessing
		switch {
		case stats.Total > 0 && stats.Ready == stats.Total:
			status = knowledgevideo.BatchCompleted
		case stats.Total > 0 && stats.Failed == stats.Total:
			status = knowledgevideo.BatchFailed
		case stats.Total > 0 && stats.Ready+stats.Failed == stats.Total && stats.Ready > 0 && stats.Failed > 0:
			status = knowledgevideo.BatchPartialFailed
		}
		updates := map[string]any{
			"total_count":  stats.Total,
			"ready_count":  stats.Ready,
			"failed_count": stats.Failed,
			"status":       int16(status),
		}
		if err := tx.Model(&model.EduKnowledgeVideoBatch{}).Where("id = ?", batchID).Updates(updates).Error; err != nil {
			return err
		}
		batch.TotalCount = stats.Total
		batch.ReadyCount = stats.Ready
		batch.FailedCount = stats.Failed
		batch.Status = int16(status)
		result = toKnowledgeVideoBatch(batch)
		return nil
	})
	if err != nil || !found {
		return knowledgevideo.Batch{}, found, err
	}
	return result, true, nil
}

func (r *GormKnowledgeVideoRepository) ListStalePending(ctx context.Context, cutoff time.Time, limit int) ([]knowledgevideo.Video, error) {
	if limit <= 0 {
		return []knowledgevideo.Video{}, nil
	}
	var rows []model.EduKnowledgeVideo
	if err := r.db.WithContext(ctx).
		Where("status = ? AND deleted = 0 AND (enqueue_time IS NULL OR enqueue_time < ?)", int16(knowledgevideo.VideoPending), cutoff).
		Order("id ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]knowledgevideo.Video, 0, len(rows))
	for _, row := range rows {
		out = append(out, toKnowledgeVideo(row))
	}
	return out, nil
}

func (r *GormKnowledgeVideoRepository) GetBatch(ctx context.Context, batchID uint64) (knowledgevideo.Batch, []knowledgevideo.Video, bool, error) {
	var batch model.EduKnowledgeVideoBatch
	if err := r.db.WithContext(ctx).Where("id = ?", batchID).First(&batch).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return knowledgevideo.Batch{}, nil, false, nil
		}
		return knowledgevideo.Batch{}, nil, false, err
	}
	var rows []model.EduKnowledgeVideo
	if err := r.db.WithContext(ctx).Where("batch_id = ? AND deleted = 0", batchID).Order("id ASC").Find(&rows).Error; err != nil {
		return knowledgevideo.Batch{}, nil, false, err
	}
	videos := make([]knowledgevideo.Video, 0, len(rows))
	for _, row := range rows {
		videos = append(videos, toKnowledgeVideo(row))
	}
	return toKnowledgeVideoBatch(batch), videos, true, nil
}

func (r *GormKnowledgeVideoRepository) updateActiveVideo(ctx context.Context, id uint64, updates map[string]any) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.EduKnowledgeVideo{}).Where("id = ? AND deleted = 0", id).Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

type postgresKnowledgeVideoIDAllocator struct {
	db *gorm.DB
}

func (a postgresKnowledgeVideoIDAllocator) ReserveBatchID(ctx context.Context) (uint64, error) {
	var id uint64
	err := a.db.WithContext(ctx).Raw("SELECT nextval('edu_knowledge_video_batch_id_seq')").Scan(&id).Error
	return id, err
}

func (a postgresKnowledgeVideoIDAllocator) ReserveVideoIDs(ctx context.Context, count int) ([]uint64, error) {
	rows := make([]struct{ ID uint64 }, 0, count)
	err := a.db.WithContext(ctx).Raw("SELECT nextval('edu_knowledge_video_id_seq') AS id FROM generate_series(1, ?)", count).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, err
}

func uniqueKnowledgeVideoIDs(ids []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(ids))
	out := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func toKnowledgeVideoBatchModel(batch knowledgevideo.Batch) model.EduKnowledgeVideoBatch {
	return model.EduKnowledgeVideoBatch{
		ID:            batch.ID,
		UploadUserID:  batch.UploadUserID,
		ZipFileName:   batch.ZipFileName,
		XLSXFileName:  batch.XLSXFileName,
		XLSXObjectKey: batch.XLSXObjectKey,
		TotalCount:    batch.TotalCount,
		ReadyCount:    batch.ReadyCount,
		FailedCount:   batch.FailedCount,
		Status:        int16(batch.Status),
		ErrorMessage:  batch.ErrorMessage,
		CreateTime:    batch.CreateTime,
		UpdateTime:    batch.UpdateTime,
	}
}

func toKnowledgeVideoBatch(batch model.EduKnowledgeVideoBatch) knowledgevideo.Batch {
	return knowledgevideo.Batch{
		ID:            batch.ID,
		UploadUserID:  batch.UploadUserID,
		ZipFileName:   batch.ZipFileName,
		XLSXFileName:  batch.XLSXFileName,
		XLSXObjectKey: batch.XLSXObjectKey,
		TotalCount:    batch.TotalCount,
		ReadyCount:    batch.ReadyCount,
		FailedCount:   batch.FailedCount,
		Status:        knowledgevideo.BatchStatus(batch.Status),
		ErrorMessage:  batch.ErrorMessage,
		CreateTime:    batch.CreateTime,
		UpdateTime:    batch.UpdateTime,
	}
}

func toKnowledgeVideoModel(video knowledgevideo.Video) model.EduKnowledgeVideo {
	return model.EduKnowledgeVideo{
		ID:                 video.ID,
		BatchID:            video.BatchID,
		KnowledgePointID:   video.KnowledgePointID,
		KnowledgePointName: video.KnowledgePointName,
		SourceFileName:     video.SourceFileName,
		SourceObjectKey:    video.SourceObjectKey,
		HLSObjectPrefix:    video.HLSObjectPrefix,
		HLSMasterObjectKey: video.HLSMasterObjectKey,
		Duration:           video.Duration,
		Status:             int16(video.Status),
		ErrorMessage:       video.ErrorMessage,
		EnqueueTime:        video.EnqueueTime,
		CreateTime:         video.CreateTime,
		UpdateTime:         video.UpdateTime,
	}
}

func toKnowledgeVideo(video model.EduKnowledgeVideo) knowledgevideo.Video {
	return knowledgevideo.Video{
		ID:                 video.ID,
		BatchID:            video.BatchID,
		KnowledgePointID:   video.KnowledgePointID,
		KnowledgePointName: video.KnowledgePointName,
		SourceFileName:     video.SourceFileName,
		SourceObjectKey:    video.SourceObjectKey,
		HLSObjectPrefix:    video.HLSObjectPrefix,
		HLSMasterObjectKey: video.HLSMasterObjectKey,
		Duration:           video.Duration,
		Status:             knowledgevideo.VideoStatus(video.Status),
		ErrorMessage:       video.ErrorMessage,
		EnqueueTime:        video.EnqueueTime,
		CreateTime:         video.CreateTime,
		UpdateTime:         video.UpdateTime,
	}
}

var _ knowledgevideo.KnowledgeVideoRepository = (*GormKnowledgeVideoRepository)(nil)
