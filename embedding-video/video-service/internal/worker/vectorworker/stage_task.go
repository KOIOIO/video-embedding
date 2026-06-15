package vectorworker

const (
	VectorStagePrepare    = "vector.prepare"
	VectorStageCoarse     = "vector.coarse"
	VectorStageCoarseClip = "vector.coarse.clip"
	VectorStageCoarseASR  = "vector.coarse.asr"
	VectorStageSegmentLLM = "vector.segment.llm"
	VectorStageRefine     = "vector.refine"
	VectorStageRefineASR  = "vector.refine.asr"
	VectorStageEmbedding  = "vector.embedding"
	VectorStageFinalize   = "vector.finalize"
)

type VectorStageTask struct {
	TaskID       string `json:"task_id"`
	VideoID      uint64 `json:"video_id"`
	RawKey       string `json:"raw_key,omitempty"`
	Stage        string `json:"stage"`
	SegmentIndex int    `json:"segment_index,omitempty"`
	SegmentID    uint64 `json:"segment_id,omitempty"`
	StartSec     int    `json:"start_sec,omitempty"`
	EndSec       int    `json:"end_sec,omitempty"`
	NextStartSec int    `json:"next_start_sec,omitempty"`
	ObjectKey    string `json:"object_key,omitempty"`
	RetryCount   int    `json:"retry_count,omitempty"`
}

func VectorStageQueueKey(stage string) string {
	switch stage {
	case VectorStagePrepare:
		return "video:vector:prepare"
	case VectorStageCoarse:
		return "video:vector:coarse"
	case VectorStageRefine:
		return "video:vector:refine"
	case VectorStageFinalize:
		return "video:vector:finalize"
	default:
		return ""
	}
}
