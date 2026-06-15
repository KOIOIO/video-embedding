package vectorworker

import (
	"encoding/json"
	"testing"
)

func TestVectorStageTaskJSONRoundTrip(t *testing.T) {
	in := VectorStageTask{
		TaskID:       "42",
		VideoID:      42,
		RawKey:       "raw/video.mp4",
		Stage:        VectorStageCoarseClip,
		SegmentIndex: 3,
		SegmentID:    99,
		StartSec:     120,
		EndSec:       160,
		NextStartSec: 161,
		ObjectKey:    "segments/coarse/video_42/42/seg_003_120_160.mp4",
		RetryCount:   2,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out VectorStageTask
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Fatalf("round trip = %+v, want %+v", out, in)
	}
}

func TestVectorStageQueueKey(t *testing.T) {
	cases := map[string]string{
		VectorStagePrepare:  "video:vector:prepare",
		VectorStageCoarse:   "video:vector:coarse",
		VectorStageRefine:   "video:vector:refine",
		VectorStageFinalize: "video:vector:finalize",
	}
	for stage, want := range cases {
		if got := VectorStageQueueKey(stage); got != want {
			t.Fatalf("VectorStageQueueKey(%q) = %q, want %q", stage, got, want)
		}
	}
}

func TestVectorStageQueueKeyForRecorderOnlyStages(t *testing.T) {
	for _, stage := range []string{
		VectorStageCoarseClip,
		VectorStageCoarseASR,
		VectorStageSegmentLLM,
		VectorStageRefineASR,
		VectorStageEmbedding,
	} {
		if got := VectorStageQueueKey(stage); got != "" {
			t.Fatalf("VectorStageQueueKey(%q) = %q, want empty", stage, got)
		}
	}
}

func TestVectorStageQueueKeyUnknown(t *testing.T) {
	if got := VectorStageQueueKey("unknown"); got != "" {
		t.Fatalf("VectorStageQueueKey(unknown) = %q, want empty", got)
	}
}
