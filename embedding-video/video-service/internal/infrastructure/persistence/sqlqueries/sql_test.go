package sqlqueries

import (
	"strings"
	"testing"
)

func TestUpsertWatchRecordQueryPreservesProgress(t *testing.T) {
	if !strings.Contains(UpsertWatchRecordQuery, "is_watched = edu_user_video_recommend.is_watched OR EXCLUDED.is_watched") {
		t.Fatal("UpsertWatchRecordQuery should keep is_watched=true when a later partial report sends false")
	}
	if !strings.Contains(UpsertWatchRecordQuery, "watch_duration = GREATEST(COALESCE(edu_user_video_recommend.watch_duration, 0), EXCLUDED.watch_duration)") {
		t.Fatal("UpsertWatchRecordQuery should keep the greatest watch_duration across out-of-order reports")
	}
}
