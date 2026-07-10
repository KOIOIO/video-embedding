package gorsesync

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	recommendationapp "nlp-video-analysis/internal/application/videoapp/recommendation"
)

type PostgresSource struct {
	DB           *sql.DB
	UserIDColumn string
	Limit        int
}

func (s PostgresSource) LoadUsers(ctx context.Context) ([]recommendationapp.GorseUser, error) {
	rows, err := s.DB.QueryContext(ctx, BuildUsersQuery(s.userIDColumn()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]recommendationapp.GorseUser, 0)
	for rows.Next() {
		var row struct {
			UserID          uint64
			GradeID         uint64
			ClassID         uint64
			UserType        string
			RecentSubjects  string
			RecentKnowledge string
			LearningLabels  string
		}
		if err := rows.Scan(&row.UserID, &row.GradeID, &row.ClassID, &row.UserType, &row.RecentSubjects, &row.RecentKnowledge, &row.LearningLabels); err != nil {
			return nil, err
		}
		users = append(users, recommendationapp.MapGorseUser(recommendationapp.GorseUserSource{
			UserID:          row.UserID,
			GradeID:         row.GradeID,
			ClassID:         row.ClassID,
			UserType:        row.UserType,
			RecentSubjects:  splitPiped(row.RecentSubjects),
			RecentKnowledge: splitPiped(row.RecentKnowledge),
			LearningLabels:  splitPiped(row.LearningLabels),
		}))
	}
	return users, rows.Err()
}

func (s PostgresSource) LoadItems(ctx context.Context) ([]recommendationapp.GorseItem, error) {
	rows, err := s.DB.QueryContext(ctx, BuildItemsQuery())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]recommendationapp.GorseItem, 0)
	for rows.Next() {
		var row struct {
			VideoSegmentID  uint64
			VideoID         uint64
			Title           string
			Summary         string
			KnowledgeTags   string
			DurationSec     int
			ViewCount       int
			LikeCount       int
			DoubleLikeCount int
			DislikeCount    int
			EmbeddingText   string
			IsDeleted       bool
			IsPublished     bool
			IsPlayable      bool
			IsRecommend     bool
			PublishedAt     time.Time
		}
		if err := rows.Scan(
			&row.VideoSegmentID,
			&row.VideoID,
			&row.Title,
			&row.Summary,
			&row.KnowledgeTags,
			&row.DurationSec,
			&row.ViewCount,
			&row.LikeCount,
			&row.DoubleLikeCount,
			&row.DislikeCount,
			&row.EmbeddingText,
			&row.IsDeleted,
			&row.IsPublished,
			&row.IsPlayable,
			&row.IsRecommend,
			&row.PublishedAt,
		); err != nil {
			return nil, err
		}
		embedding, _ := recommendationapp.ParseVectorText(row.EmbeddingText)
		items = append(items, recommendationapp.MapGorseItem(recommendationapp.GorseItemSource{
			VideoSegmentID:  row.VideoSegmentID,
			VideoID:         row.VideoID,
			Title:           row.Title,
			Summary:         row.Summary,
			KnowledgeTags:   splitPiped(row.KnowledgeTags),
			DurationSec:     row.DurationSec,
			ViewCount:       row.ViewCount,
			LikeCount:       row.LikeCount,
			DoubleLikeCount: row.DoubleLikeCount,
			DislikeCount:    row.DislikeCount,
			Embedding:       embedding,
			IsDeleted:       row.IsDeleted,
			IsPublished:     row.IsPublished,
			IsPlayable:      row.IsPlayable,
			IsRecommend:     row.IsRecommend,
			PublishedAt:     row.PublishedAt,
		}))
	}
	return items, rows.Err()
}

func (s PostgresSource) LoadFeedback(ctx context.Context) ([]recommendationapp.GorseFeedback, error) {
	limit := s.Limit
	if limit <= 0 {
		limit = 50000
	}
	rows, err := s.DB.QueryContext(ctx, BuildFeedbackQuery(s.userIDColumn()), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	feedback := make([]recommendationapp.GorseFeedback, 0)
	for rows.Next() {
		var row struct {
			UserID          uint64
			VideoSegmentID  uint64
			Source          string
			ReactionType    string
			WatchDuration   int
			SegmentDuration int
			EventTime       time.Time
		}
		if err := rows.Scan(&row.UserID, &row.VideoSegmentID, &row.Source, &row.ReactionType, &row.WatchDuration, &row.SegmentDuration, &row.EventTime); err != nil {
			return nil, err
		}
		src, ok := mapFeedbackSource(row.Source, row.ReactionType)
		if !ok {
			continue
		}
		watchRatio := 0.0
		if row.SegmentDuration > 0 {
			watchRatio = float64(row.WatchDuration) / float64(row.SegmentDuration)
		}
		if mapped, ok := recommendationapp.MapGorseFeedback(recommendationapp.GorseFeedbackSource{
			UserID:         row.UserID,
			VideoSegmentID: row.VideoSegmentID,
			Kind:           src,
			WatchRatio:     watchRatio,
			EventTime:      row.EventTime,
		}); ok {
			feedback = append(feedback, mapped)
		}
	}
	return feedback, rows.Err()
}

func (s PostgresSource) userIDColumn() string {
	if strings.TrimSpace(s.UserIDColumn) != "" {
		return s.UserIDColumn
	}
	return "id"
}

func BuildUsersQuery(userIDColumn string) string {
	userIDColumn = quoteIdent(defaultString(userIDColumn, "id"))
	return fmt.Sprintf(`
WITH valid_users AS (
  SELECT u.%s AS user_id,
         COALESCE(u.grade_id, 0) AS grade_id,
         COALESCE(u.class_id, 0) AS class_id,
         COALESCE(u.user_type::text, '') AS user_type
  FROM public.sys_user u
  WHERE u.%s IS NOT NULL
    AND u.%s > 0
    AND (NOT EXISTS (
          SELECT 1 FROM information_schema.columns
          WHERE table_schema = 'public' AND table_name = 'sys_user' AND column_name = 'deleted'
        ) OR COALESCE(u.deleted, 0) = 0)
),
mastery AS (
  SELECT ukm.user_id,
         COALESCE(string_agg(DISTINCT NULLIF(TRIM(dkp.name), ''), '|')
           FILTER (WHERE COALESCE(ukm.mastery, 0) < 0.6), '') AS weak_knowledge,
         BOOL_OR(COALESCE(ukm.mastery, 0) < 0.6) AS has_weak_mastery,
         BOOL_OR(COALESCE(ukm.mastery, 0) >= 0.8) AS has_strong_mastery
  FROM public.edu_user_knowledge_mastery ukm
  LEFT JOIN public.dict_knowledge_point dkp ON dkp.id = ukm.knowledge_point_id AND COALESCE(dkp.deleted, 0) = 0
  WHERE ukm.user_id > 0
  GROUP BY ukm.user_id
),
answer AS (
  SELECT ar.user_id,
         COUNT(*) AS answer_count,
         AVG(CASE WHEN ar.is_correct = 1 THEN 1.0 ELSE 0.0 END) AS answer_accuracy,
         COALESCE(string_agg(DISTINCT NULLIF(TRIM(dkp.name), ''), '|'), '') AS answer_knowledge
  FROM public.edu_knowledge_answer_record ar
  LEFT JOIN public.dict_knowledge_point dkp ON dkp.id = ar.knowledge_point_id AND COALESCE(dkp.deleted, 0) = 0
  WHERE ar.user_id > 0
  GROUP BY ar.user_id
),
question_feedback AS (
  SELECT qf.user_id,
         COALESCE(SUM(qf.feedback_count), 0) AS question_feedback_count
  FROM public.edu_user_question_feedback qf
  WHERE qf.user_id > 0
  GROUP BY qf.user_id
),
generated_feedback AS (
  SELECT gf.user_id,
         COUNT(*) AS generated_feedback_count,
         AVG(CASE WHEN gf.is_correct = 1 THEN 1.0 ELSE 0.0 END) AS generated_accuracy
  FROM public.edu_generated_question_feedback gf
  WHERE gf.user_id > 0
  GROUP BY gf.user_id
),
question_search AS (
  SELECT qs.user_id,
         COUNT(*) AS question_search_count,
         COALESCE(string_agg(DISTINCT NULLIF(TRIM(BOTH '|' FROM regexp_replace(COALESCE(qs.first_question_knowledge, ''), '[#/]+', '|', 'g')), ''), '|'), '') AS search_knowledge
  FROM public.edu_question_search_record qs
  WHERE COALESCE(qs.deleted, 0) = 0
    AND qs.user_id > 0
  GROUP BY qs.user_id
),
special_practice AS (
  SELECT ps.user_id,
         COUNT(*) AS practice_count,
         COALESCE(string_agg(DISTINCT NULLIF(TRIM(ps.subject_code), ''), '|'), '') AS practice_subjects,
         COALESCE(string_agg(DISTINCT NULLIF(TRIM(pn.name), ''), '|'), '') AS practice_knowledge
  FROM public.edu_special_practice_session ps
  LEFT JOIN public.edu_special_practice_node pn ON pn.id = ps.node_id AND COALESCE(pn.deleted, 0) = 0
  WHERE ps.user_id > 0
  GROUP BY ps.user_id
),
word_record AS (
  SELECT wr.user_id,
         COUNT(*) AS word_count,
         COUNT(*) FILTER (WHERE COALESCE(wr.familiarity, 0) <= 2 OR COALESCE(wr.error_count, 0) > 0) AS weak_word_count
  FROM public.edu_student_word_record wr
  WHERE wr.user_id > 0
  GROUP BY wr.user_id
),
word_study AS (
  SELECT wsd.user_id,
         COUNT(*) AS word_study_count
  FROM public.edu_student_word_study_detail wsd
  WHERE wsd.user_id > 0
  GROUP BY wsd.user_id
),
english_reading AS (
  SELECT erh.user_id,
         COUNT(*) AS reading_count
  FROM public.english_reading_history erh
  WHERE COALESCE(erh.deleted, 0) = 0
    AND erh.user_id > 0
  GROUP BY erh.user_id
),
english_listening AS (
  SELECT els.user_id,
         COUNT(*) AS listening_count,
         AVG(CASE WHEN COALESCE(els.total_score, 0) > 0 THEN els.score::numeric / els.total_score ELSE NULL END) AS listening_score_rate
  FROM public.english_listening_session els
  WHERE COALESCE(els.deleted, 0) = 0
    AND els.user_id > 0
  GROUP BY els.user_id
),
english_storybook AS (
  SELECT ess.user_id,
         COUNT(*) AS storybook_count
  FROM public.english_storybook_session ess
  WHERE COALESCE(ess.deleted, 0) = 0
    AND ess.user_id > 0
  GROUP BY ess.user_id
),
student_profile AS (
  SELECT sps.student_id AS user_id,
         COUNT(*) AS profile_count
  FROM public.student_profile_snapshot sps
  WHERE COALESCE(sps.deleted, 0) = 0
    AND sps.student_id > 0
  GROUP BY sps.student_id
)
SELECT vu.user_id,
       vu.grade_id,
       vu.class_id,
       vu.user_type,
       CONCAT_WS('|',
         NULLIF(sp.practice_subjects, ''),
         CASE WHEN COALESCE(wr.word_count, 0) > 0 OR COALESCE(wsd.word_study_count, 0) > 0 THEN 'word' END,
         CASE WHEN COALESCE(er.reading_count, 0) > 0 OR COALESCE(el.listening_count, 0) > 0 OR COALESCE(es.storybook_count, 0) > 0 THEN 'english' END
       ) AS recent_subjects,
       CONCAT_WS('|',
         NULLIF(m.weak_knowledge, ''),
         NULLIF(a.answer_knowledge, ''),
         NULLIF(qs.search_knowledge, ''),
         NULLIF(sp.practice_knowledge, '')
       ) AS recent_knowledge,
       CONCAT_WS('|',
         CASE WHEN COALESCE(m.has_weak_mastery, FALSE) THEN 'mastery:weak' END,
         CASE WHEN COALESCE(m.has_strong_mastery, FALSE) THEN 'mastery:strong' END,
         CASE WHEN COALESCE(a.answer_count, 0) > 0 THEN 'answer:active' END,
         CASE WHEN COALESCE(a.answer_count, 0) > 0 AND COALESCE(a.answer_accuracy, 0) < 0.6 THEN 'answer:low_accuracy' END,
         CASE WHEN COALESCE(a.answer_count, 0) > 0 AND COALESCE(a.answer_accuracy, 0) >= 0.85 THEN 'answer:high_accuracy' END,
         CASE WHEN COALESCE(qf.question_feedback_count, 0) > 0 THEN 'question_feedback:active' END,
         CASE WHEN COALESCE(gf.generated_feedback_count, 0) > 0 THEN 'generated_question:active' END,
         CASE WHEN COALESCE(gf.generated_feedback_count, 0) > 0 AND COALESCE(gf.generated_accuracy, 0) < 0.6 THEN 'generated_question:low_accuracy' END,
         CASE WHEN COALESCE(qs.question_search_count, 0) > 0 THEN 'question_search:active' END,
         CASE WHEN COALESCE(sp.practice_count, 0) > 0 THEN 'practice:active' END,
         CASE WHEN COALESCE(wr.word_count, 0) > 0 OR COALESCE(wsd.word_study_count, 0) > 0 THEN 'word:active' END,
         CASE WHEN COALESCE(wr.weak_word_count, 0) > 0 THEN 'word:weak' END,
         CASE WHEN COALESCE(er.reading_count, 0) > 0 THEN 'english:reading' END,
         CASE WHEN COALESCE(el.listening_count, 0) > 0 THEN 'english:listening' END,
         CASE WHEN COALESCE(el.listening_count, 0) > 0 AND COALESCE(el.listening_score_rate, 0) < 0.6 THEN 'english:listening_low_score' END,
         CASE WHEN COALESCE(es.storybook_count, 0) > 0 THEN 'english:storybook' END,
         CASE WHEN COALESCE(sps.profile_count, 0) > 0 THEN 'profile:available' END
       ) AS learning_labels
FROM valid_users vu
LEFT JOIN mastery m ON m.user_id = vu.user_id
LEFT JOIN answer a ON a.user_id = vu.user_id
LEFT JOIN question_feedback qf ON qf.user_id = vu.user_id
LEFT JOIN generated_feedback gf ON gf.user_id = vu.user_id
LEFT JOIN question_search qs ON qs.user_id = vu.user_id
LEFT JOIN special_practice sp ON sp.user_id = vu.user_id
LEFT JOIN word_record wr ON wr.user_id = vu.user_id
LEFT JOIN word_study wsd ON wsd.user_id = vu.user_id
LEFT JOIN english_reading er ON er.user_id = vu.user_id
LEFT JOIN english_listening el ON el.user_id = vu.user_id
LEFT JOIN english_storybook es ON es.user_id = vu.user_id
LEFT JOIN student_profile sps ON sps.user_id = vu.user_id
ORDER BY vu.user_id`, userIDColumn, userIDColumn, userIDColumn)
}

func BuildItemsQuery() string {
	return `
SELECT s.id AS video_segment_id,
       s.video_id AS video_id,
       COALESCE(r.title, '') AS title,
       COALESCE(s.content_summary, '') AS summary,
       COALESCE(array_to_string(s.knowledge_tags, '|'), '') AS knowledge_tags,
       GREATEST(COALESCE(s.end_time, 0) - COALESCE(s.start_time, 0), 1) AS duration_sec,
       COALESCE(r.view_count, 0) AS view_count,
       COALESCE(s.like_count, 0) AS like_count,
       COALESCE(s.double_like_count, 0) AS double_like_count,
       COALESCE(s.dislike_count, 0) AS dislike_count,
       COALESCE(s.embedding::text, '') AS embedding,
       (s.deleted <> 0 OR r.deleted <> 0) AS is_deleted,
       COALESCE(r.is_published, FALSE) AS is_published,
       (s.status = 1 AND r.status = 3 AND TRIM(COALESCE(r.video_url, '')) <> '') AS is_playable,
       COALESCE(r.is_recommend, FALSE) AS is_recommend,
       COALESCE(r.update_time, r.create_time, s.create_time, NOW()) AS published_at
FROM public.edu_video_segment s
JOIN public.edu_video_resource r ON r.id = s.video_id
WHERE s.id > 0 AND s.video_id > 0
ORDER BY s.id`
}

func BuildFeedbackQuery(userIDColumn string) string {
	userIDColumn = quoteIdent(defaultString(userIDColumn, "id"))
	return fmt.Sprintf(`
WITH valid_users AS (
  SELECT DISTINCT u.%s AS user_id
  FROM public.sys_user u
  WHERE u.%s IS NOT NULL AND u.%s > 0
),
valid_segments AS (
  SELECT s.id AS video_segment_id,
         s.video_id AS video_id,
         GREATEST(COALESCE(s.end_time, 0) - COALESCE(s.start_time, 0), 1) AS segment_duration
  FROM public.edu_video_segment s
  JOIN public.edu_video_resource r ON r.id = s.video_id
  WHERE s.deleted = 0
    AND s.status = 1
    AND s.video_id > 0
    AND r.deleted = 0
),
events AS (
  SELECT ur.user_id,
         ur.video_segment_id,
         'segment_reaction' AS source,
         ur.reaction_type,
         0 AS watch_duration,
         vs.segment_duration,
         ur.update_time AS event_time
  FROM public.edu_user_reaction ur
  JOIN valid_segments vs ON vs.video_segment_id = ur.video_segment_id AND vs.video_id = ur.video_id
  WHERE ur.deleted = 0 AND ur.reaction_type IN ('like', 'double_like', 'dislike')

  UNION ALL

  SELECT vur.user_id,
         vs.video_segment_id,
         'video_reaction' AS source,
         vur.reaction_type,
         0 AS watch_duration,
         vs.segment_duration,
         vur.update_time AS event_time
  FROM public.edu_video_user_reaction vur
  JOIN valid_segments vs ON vs.video_id = vur.video_id
  WHERE vur.deleted = 0 AND vur.reaction_type IN ('like', 'double_like', 'dislike')

  UNION ALL

  SELECT uvr.user_id,
         uvr.video_segment_id,
         'watch' AS source,
         '' AS reaction_type,
         COALESCE(uvr.watch_duration, 0) AS watch_duration,
         vs.segment_duration,
         uvr.update_time AS event_time
  FROM public.edu_user_video_recommend uvr
  JOIN valid_segments vs ON vs.video_segment_id = uvr.video_segment_id AND vs.video_id = uvr.video_id
  WHERE uvr.deleted = 0 AND COALESCE(uvr.is_watched, FALSE) = TRUE

  UNION ALL

  SELECT e.user_id,
         e.video_segment_id,
         'exposure' AS source,
         '' AS reaction_type,
         0 AS watch_duration,
         vs.segment_duration,
         e.create_time AS event_time
  FROM public.edu_recommend_exposure e
  JOIN valid_segments vs ON vs.video_segment_id = e.video_segment_id AND vs.video_id = e.video_id
  JOIN valid_users vu ON vu.user_id = e.user_id
  WHERE e.deleted = 0

  UNION ALL

  SELECT qs.user_id,
         vs.video_segment_id,
         'question_search_watch' AS source,
         '' AS reaction_type,
         extracted.watch_duration,
         vs.segment_duration,
         qs.create_time AS event_time
  FROM public.edu_question_search_record qs
  CROSS JOIN LATERAL jsonb_array_elements(
    CASE
      WHEN NULLIF(TRIM(COALESCE(qs.recommend_videos_json, '')), '') IS NULL THEN '[]'::jsonb
      WHEN LEFT(TRIM(qs.recommend_videos_json), 1) = '[' THEN qs.recommend_videos_json::jsonb
      ELSE '[]'::jsonb
    END
  ) rec(video)
  CROSS JOIN LATERAL (
    SELECT CASE
             WHEN COALESCE(NULLIF(rec.video->>'videoSegmentId', ''), NULLIF(rec.video->>'id', '')) ~ '^[0-9]+$'
             THEN COALESCE(NULLIF(rec.video->>'videoSegmentId', ''), NULLIF(rec.video->>'id', ''))::bigint
             ELSE 0
           END AS video_segment_id,
           CASE
             WHEN COALESCE(rec.video->>'watchDuration', '') ~ '^[0-9]+$'
             THEN (rec.video->>'watchDuration')::int
             ELSE 0
           END AS watch_duration
  ) extracted
  JOIN valid_segments vs ON vs.video_segment_id = extracted.video_segment_id
  JOIN valid_users vu ON vu.user_id = qs.user_id
  WHERE COALESCE(qs.deleted, 0) = 0
    AND extracted.video_segment_id > 0
    AND COALESCE((rec.video->>'watched')::boolean, FALSE) = TRUE
)
SELECT e.user_id,
       e.video_segment_id,
       e.source,
       e.reaction_type,
       e.watch_duration,
       e.segment_duration,
       e.event_time
FROM events e
JOIN valid_users vu ON vu.user_id = e.user_id
ORDER BY e.event_time DESC
LIMIT $1`, userIDColumn, userIDColumn, userIDColumn)
}

func mapFeedbackSource(source string, reactionType string) (recommendationapp.GorseFeedbackKind, bool) {
	switch source {
	case "segment_reaction", "video_reaction":
		switch recommendationapp.GorseFeedbackKind(strings.TrimSpace(reactionType)) {
		case recommendationapp.GorseFeedbackLike:
			return recommendationapp.GorseFeedbackLike, true
		case recommendationapp.GorseFeedbackDoubleLike:
			return recommendationapp.GorseFeedbackDoubleLike, true
		case recommendationapp.GorseFeedbackDislike:
			return recommendationapp.GorseFeedbackDislike, true
		default:
			return "", false
		}
	case "watch":
		return recommendationapp.GorseFeedbackWatch, true
	case "question_search_watch":
		return recommendationapp.GorseFeedbackWatch, true
	case "exposure":
		return recommendationapp.GorseFeedbackExposure, true
	default:
		return "", false
	}
}

func quoteIdent(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "id"
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func splitPiped(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, "|")
}
