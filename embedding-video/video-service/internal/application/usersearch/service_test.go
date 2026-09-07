package usersearch

import (
	"context"
	"testing"

	"video-service/internal/infrastructure/persistence"
)

type mockRepo struct {
	lastKeyword  string
	lastPage     int
	lastPageSize int
	results      []persistence.UserSearchResult
	total        int64
	err          error
}

func (m *mockRepo) SearchUsers(_ context.Context, _ uint64, keyword string, page, pageSize int) ([]persistence.UserSearchResult, int64, error) {
	m.lastKeyword = keyword
	m.lastPage = page
	m.lastPageSize = pageSize
	return m.results, m.total, m.err
}

func TestSearchRejectsEmptyKeyword(t *testing.T) {
	svc := NewService(&mockRepo{})
	_, _, err := svc.Search(context.Background(), 1, "  ", 1, 20)
	if err == nil {
		t.Fatal("expected error for empty keyword")
	}
}

func TestSearchDefaultsPageAndPageSize(t *testing.T) {
	repo := &mockRepo{}
	svc := NewService(repo)
	_, _, err := svc.Search(context.Background(), 1, "alice", 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastPage != 1 {
		t.Fatalf("page = %d, want 1", repo.lastPage)
	}
	if repo.lastPageSize != 20 {
		t.Fatalf("pageSize = %d, want 20", repo.lastPageSize)
	}
}

func TestSearchCapsPageSizeAt50(t *testing.T) {
	repo := &mockRepo{}
	svc := NewService(repo)
	_, _, err := svc.Search(context.Background(), 1, "alice", 1, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastPageSize != 50 {
		t.Fatalf("pageSize = %d, want 50 (capped)", repo.lastPageSize)
	}
}

func TestSearchTrimsKeyword(t *testing.T) {
	repo := &mockRepo{}
	svc := NewService(repo)
	_, _, err := svc.Search(context.Background(), 1, "  bob  ", 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastKeyword != "bob" {
		t.Fatalf("keyword = %q, want %q", repo.lastKeyword, "bob")
	}
}

func TestSearchConvertsResultsToDTO(t *testing.T) {
	repo := &mockRepo{
		results: []persistence.UserSearchResult{
			{ID: 2, Username: "alice", Nickname: "爱丽丝", AvatarURL: "/a.png", IsFollowing: true},
			{ID: 3, Username: "bob", Nickname: "鲍勃", AvatarURL: "", IsFollowing: false},
		},
		total: 2,
	}
	svc := NewService(repo)
	list, total, err := svc.Search(context.Background(), 1, "a", 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}
	if list[0].ID != 2 || list[0].Nickname != "爱丽丝" || !list[0].IsFollowing {
		t.Fatalf("list[0] = %+v, unexpected", list[0])
	}
	if list[1].ID != 3 || list[1].AvatarURL != "" || list[1].IsFollowing {
		t.Fatalf("list[1] = %+v, unexpected", list[1])
	}
}
