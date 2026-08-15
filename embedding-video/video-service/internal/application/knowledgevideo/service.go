package knowledgevideo

import "context"

type Service struct {
	Importer *ImportService
	Playback PlaybackService
	Batches  BatchReader
	Tree     KnowledgeTreeReader
}

func (s *Service) ListTree(ctx context.Context) ([]KnowledgeTreeNode, error) {
	return s.Tree.ListKnowledgeTree(ctx)
}

func (s *Service) Import(ctx context.Context, input ImportInput) (ImportResult, error) {
	return s.Importer.Import(ctx, input)
}

func (s *Service) GetBatch(ctx context.Context, batchID uint64) (Batch, []Video, bool, error) {
	return s.Batches.GetBatch(ctx, batchID)
}

func (s *Service) Resolve(ctx context.Context, userID, knowledgePointID uint64) (PlaybackResolution, error) {
	return s.Playback.Resolve(ctx, userID, knowledgePointID)
}

func (s *Service) ListPlayback(ctx context.Context, knowledgePointID uint64) (PlaybackResolution, error) {
	return s.Playback.List(ctx, knowledgePointID)
}

func (s *Service) RecordPlayback(ctx context.Context, userID, knowledgeVideoID uint64) error {
	return s.Playback.Record(ctx, userID, knowledgeVideoID)
}

func (s *Service) ReportWatchSession(ctx context.Context, input WatchSessionInput) (WatchSessionResult, error) {
	return s.Playback.ReportWatchSession(ctx, input)
}
