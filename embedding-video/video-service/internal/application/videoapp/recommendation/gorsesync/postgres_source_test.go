package gorsesync

import (
	"strings"
	"testing"
)

func TestBuildQueriesUseSysUserPlayableItemsAndFeedbackSources(t *testing.T) {
	userQuery := BuildUsersQuery("id")
	for _, want := range []string{
		"FROM public.sys_user u",
		`u."id" AS user_id`,
		"FROM public.edu_user_knowledge_mastery ukm",
		"FROM public.edu_knowledge_answer_record ar",
		"FROM public.edu_user_question_feedback qf",
		"FROM public.edu_generated_question_feedback gf",
		"FROM public.edu_question_search_record qs",
		"FROM public.edu_special_practice_session ps",
		"FROM public.edu_student_word_record wr",
		"FROM public.edu_student_word_study_detail wsd",
		"FROM public.english_reading_history erh",
		"FROM public.english_listening_session els",
		"FROM public.english_storybook_session ess",
		"FROM public.student_profile_snapshot sps",
		"AS recent_subjects",
		"AS recent_knowledge",
		"AS learning_labels",
	} {
		if !strings.Contains(userQuery, want) {
			t.Fatalf("user query missing %q:\n%s", want, userQuery)
		}
	}

	itemQuery := BuildItemsQuery()
	for _, want := range []string{
		"FROM public.edu_video_segment s",
		"JOIN public.edu_video_resource r ON r.id = s.video_id",
		"r.is_published",
		"r.is_recommend",
		"r.status",
		"s.knowledge_tags",
		"s.embedding::text",
	} {
		if !strings.Contains(itemQuery, want) {
			t.Fatalf("item query missing %q:\n%s", want, itemQuery)
		}
	}

	feedbackQuery := BuildFeedbackQuery("id")
	for _, want := range []string{
		"'segment_reaction' AS source",
		"'video_reaction' AS source",
		"'watch' AS source",
		"'exposure' AS source",
		"'question_search_watch' AS source",
		"FROM public.edu_recommend_exposure e",
		"FROM public.edu_question_search_record qs",
		"jsonb_array_elements",
		"recommend_videos_json",
		"watched",
		"JOIN valid_users vu ON vu.user_id = e.user_id",
	} {
		if !strings.Contains(feedbackQuery, want) {
			t.Fatalf("feedback query missing %q:\n%s", want, feedbackQuery)
		}
	}
	if strings.Contains(feedbackQuery, "exposure_no_click") {
		t.Fatalf("feedback query should not synthesize exposure_no_click negatives:\n%s", feedbackQuery)
	}
}
