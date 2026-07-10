package redis

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"

	"nlp-video-analysis/internal/application/videoapp"
	domainvideo "nlp-video-analysis/internal/domain/video"
)

func TestRandomPlayBucketStoreFillsDedupesTrimsPopsAndExpiresByUser(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	store := NewRandomPlayBucketStore(rdb, "test:bucket:")
	ctx := context.Background()

	if err := store.Fill(ctx, 6, []videoapp.RecommendResultItem{
		randomPlayBucketTestItem(101, 11, "/raw/101.mp4"),
		randomPlayBucketTestItem(102, 12, "/raw/102.mp4"),
		randomPlayBucketTestItem(101, 11, "/raw/101-duplicate.mp4"),
		{},
		randomPlayBucketTestItem(103, 13, "/raw/103.mp4"),
	}, 3, 30*time.Minute); err != nil {
		t.Fatalf("Fill returned error: %v", err)
	}
	rawValues, err := rdb.LRange(ctx, store.key(6), 0, -1).Result()
	if err != nil {
		t.Fatalf("read raw bucket json: %v", err)
	}
	if len(rawValues) == 0 || !strings.Contains(rawValues[0], `"/raw/101.mp4"`) {
		t.Fatalf("raw bucket values = %#v, want full playback json with video_url", rawValues)
	}
	if err := store.Fill(ctx, 6, []videoapp.RecommendResultItem{
		randomPlayBucketTestItem(102, 12, "/raw/102.mp4"),
		randomPlayBucketTestItem(104, 14, "/raw/104.mp4"),
		randomPlayBucketTestItem(105, 15, "/raw/105.mp4"),
	}, 3, 30*time.Minute); err != nil {
		t.Fatalf("Fill second returned error: %v", err)
	}

	listed, err := store.List(ctx, 6)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	wantListed := []videoapp.RecommendResultItem{
		randomPlayBucketTestItem(101, 11, "/raw/101.mp4"),
		randomPlayBucketTestItem(102, 12, "/raw/102.mp4"),
		randomPlayBucketTestItem(103, 13, "/raw/103.mp4"),
	}
	if !reflect.DeepEqual(listed, wantListed) {
		t.Fatalf("listed = %#v, want first three unique playable items", listed)
	}
	length, err := store.Len(ctx, 6)
	if err != nil {
		t.Fatalf("Len returned error: %v", err)
	}
	if length != 3 {
		t.Fatalf("Len = %d, want 3", length)
	}

	item, found, err := store.Pop(ctx, 6)
	if err != nil {
		t.Fatalf("Pop returned error: %v", err)
	}
	if !found || !reflect.DeepEqual(item, randomPlayBucketTestItem(101, 11, "/raw/101.mp4")) {
		t.Fatalf("Pop returned item=%+v found=%v, want segment 101 true", item, found)
	}
	otherUserItem, otherFound, err := store.Pop(ctx, 7)
	if err != nil {
		t.Fatalf("Pop other user returned error: %v", err)
	}
	if otherFound || otherUserItem.VideoSegmentID != 0 {
		t.Fatalf("other user pop item=%+v found=%v, want empty", otherUserItem, otherFound)
	}

	mr.FastForward(31 * time.Minute)
	length, err = store.Len(ctx, 6)
	if err != nil {
		t.Fatalf("Len after expiry returned error: %v", err)
	}
	if length != 0 {
		t.Fatalf("Len after expiry = %d, want 0", length)
	}
}

func randomPlayBucketTestItem(segmentID uint64, videoID uint64, videoURL string) videoapp.RecommendResultItem {
	return videoapp.RecommendResultItem{
		VideoID:        videoID,
		VideoSegmentID: segmentID,
		RecommendScore: 1,
		StartTimeSec:   10,
		EndTimeSec:     40,
		TitleOverride:  "segment title",
		Video: domainvideo.Video{
			ID:          videoID,
			Title:       "video title",
			VideoURL:    videoURL,
			CoverURL:    "/covers/test.jpg",
			Status:      domainvideo.StatusDone,
			IsPublished: true,
		},
	}
}
