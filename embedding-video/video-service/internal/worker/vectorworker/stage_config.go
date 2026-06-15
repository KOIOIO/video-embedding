package vectorworker

import "nlp-video-analysis/internal/config"

var vectorStageOrder = []string{
	VectorStagePrepare,
	VectorStageCoarse,
	VectorStageRefine,
	VectorStageFinalize,
}

func vectorStageQueueKeyFromConfig(cfg config.Config, stage string) string {
	switch stage {
	case VectorStagePrepare:
		return config.VectorPrepareQueueKey(cfg)
	case VectorStageCoarse:
		return config.VectorCoarseQueueKey(cfg)
	case VectorStageRefine:
		return config.VectorRefineQueueKey(cfg)
	case VectorStageFinalize:
		return config.VectorFinalizeQueueKey(cfg)
	default:
		return ""
	}
}

func vectorStageWorkerCountFromConfig(cfg config.Config, stage string) int {
	var n int
	switch stage {
	case VectorStagePrepare:
		n = cfg.VectorStageWorkers.Prepare
	case VectorStageCoarse:
		n = cfg.VectorStageWorkers.Coarse
	case VectorStageRefine:
		n = cfg.VectorStageWorkers.Refine
	case VectorStageFinalize:
		n = cfg.VectorStageWorkers.Finalize
	}
	if n > 0 {
		return n
	}
	switch stage {
	case VectorStagePrepare, VectorStageFinalize:
		return 1
	case VectorStageCoarse, VectorStageRefine:
		return 2
	default:
		return 1
	}
}
