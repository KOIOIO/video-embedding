package sqlqueries

import (
	"strings"
	"testing"
)

func TestUpsertUserVideoRecommendQueryStartsAsUnwatched(t *testing.T) {
	if !strings.Contains(UpsertUserVideoRecommendQuery, "is_watched, watch_duration") {
		t.Fatal("UpsertUserVideoRecommendQuery should write watch columns")
	}
	if !strings.Contains(UpsertUserVideoRecommendQuery, "?, ?, ?, ?, ?, FALSE, 0, 0, ?, ?") {
		t.Fatal("UpsertUserVideoRecommendQuery should initialize recommendation rows as unwatched with zero duration")
	}
	if strings.Contains(UpsertUserVideoRecommendQuery, "is_watched = EXCLUDED.is_watched") {
		t.Fatal("recommendation refresh should not overwrite watch state")
	}
}

func TestUpsertWatchRecordQueryPreservesProgress(t *testing.T) {
	if !strings.Contains(UpsertWatchRecordQuery, "is_watched = edu_user_video_recommend.is_watched OR EXCLUDED.is_watched") {
		t.Fatal("UpsertWatchRecordQuery should keep is_watched=true when a later partial report sends false")
	}
	if !strings.Contains(UpsertWatchRecordQuery, "watch_duration = GREATEST(COALESCE(edu_user_video_recommend.watch_duration, 0), EXCLUDED.watch_duration)") {
		t.Fatal("UpsertWatchRecordQuery should keep the greatest watch_duration across out-of-order reports")
	}
}

func TestProfileRerankQueryIncludesProfileDistanceAndUserSignals(t *testing.T) {
	required := []string{
		"(s.embedding <=> ?) AS profile_distance",
		"LEFT JOIN edu_user_reaction ur",
		"LEFT JOIN edu_video_user_reaction vur",
		"LEFT JOIN edu_user_video_recommend w",
		"AS user_disliked",
		"AS user_video_disliked",
		"AS user_watched",
		"ORDER BY s.embedding <=> ?",
	}
	for _, fragment := range required {
		if !strings.Contains(RecommendByQuestionWithProfileQuery, fragment) {
			t.Fatalf("RecommendByQuestionWithProfileQuery missing %q", fragment)
		}
	}
}

func TestWeakKnowledgeVectorQueryUsesEmbeddingsAndPlayableUserFilters(t *testing.T) {
	required := []string{
		"(s.embedding <=> ?) AS distance",
		"COALESCE(CAST(s.knowledge_tags AS TEXT), '') AS knowledge_tags",
		"r.title AS video_title",
		"r.description AS description",
		"s.embedding IS NOT NULL",
		"LEFT JOIN edu_user_reaction ur",
		"LEFT JOIN edu_video_user_reaction vur",
		"LEFT JOIN edu_user_video_recommend watched",
		"watched.is_watched = true",
		"r.is_published = true",
		"(? = false OR r.is_recommend = true)",
		"r.status = 3",
		"TRIM(COALESCE(r.video_url, '')) <> ''",
		"ur.id IS NULL",
		"vur.id IS NULL",
		"watched.id IS NULL",
		"ORDER BY s.embedding <=> ?",
	}
	for _, fragment := range required {
		if !strings.Contains(RecommendByWeakKnowledgeVectorQuery, fragment) {
			t.Fatalf("RecommendByWeakKnowledgeVectorQuery missing %q", fragment)
		}
	}
}

func TestGetUserVideoProfileQueryRequiresActiveVersionedProfile(t *testing.T) {
	required := []string{
		"profile_vector::text AS profile_vector",
		"FROM edu_user_video_profile",
		"model_version = ?",
		"status = 1",
		"deleted = 0",
	}
	for _, fragment := range required {
		if !strings.Contains(GetUserVideoProfileQuery, fragment) {
			t.Fatalf("GetUserVideoProfileQuery missing %q", fragment)
		}
	}
}

func TestRecBoleQueriesUseVersionedActiveEmbeddings(t *testing.T) {
	for _, fragment := range []string{
		"embedding::text AS embedding",
		"FROM recsys.recommend_user_embedding",
		"model_name = ?",
		"model_version = ?",
		"status = 1",
		"deleted = 0",
	} {
		if !strings.Contains(GetUserRecBoleEmbeddingQuery, fragment) {
			t.Fatalf("GetUserRecBoleEmbeddingQuery missing %q", fragment)
		}
	}
	for _, fragment := range []string{
		"FROM recsys.recommend_item_embedding ie",
		"JOIN edu_video_segment s ON s.id = ie.video_segment_id",
		"(ie.embedding <=> ?) AS distance",
		"ie.model_name = ?",
		"ie.model_version = ?",
		"ie.status = 1",
		"ORDER BY ie.embedding <=> ?",
	} {
		if !strings.Contains(RecommendByRecBoleQuery, fragment) {
			t.Fatalf("RecommendByRecBoleQuery missing %q", fragment)
		}
	}
}
