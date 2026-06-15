package vectorworker

import "testing"

func TestCalcCoarseWorkersCapsAtTwenty(t *testing.T) {
	got := calcCoarseWorkers(60, 4, 40)
	if got != 20 {
		t.Fatalf("calcCoarseWorkers(60, 4, 40) = %d, want 20", got)
	}
}

func TestCalcCoarseWorkersFallsBackWhenUnset(t *testing.T) {
	got := calcCoarseWorkers(0, 4, 40)
	if got != 4 {
		t.Fatalf("calcCoarseWorkers(0, 4, 40) = %d, want 4", got)
	}
}

func TestCalcCoarseWorkersDoesNotExceedSegments(t *testing.T) {
	got := calcCoarseWorkers(60, 4, 6)
	if got != 6 {
		t.Fatalf("calcCoarseWorkers(60, 4, 6) = %d, want 6", got)
	}
}

func TestCalcCoarsePipelineStageWorkersUsesSameCap(t *testing.T) {
	clipWorkers, uploadWorkers, asrWorkers := calcCoarsePipelineStageWorkers(60, 8, 40)
	if clipWorkers != 20 || uploadWorkers != 20 || asrWorkers != 20 {
		t.Fatalf("calcCoarsePipelineStageWorkers(60, 8, 40) = %d/%d/%d, want 20/20/20", clipWorkers, uploadWorkers, asrWorkers)
	}
}
