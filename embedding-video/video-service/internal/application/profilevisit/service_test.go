package profilevisit

import (
	"context"
	"errors"
	"testing"
	"time"

	"video-service/internal/infrastructure/persistence"
)

type mockVisitRepo struct {
	upsertCalls int
	upsertErr   error
	statsUnique int64
	statsTotal  int64
	statsErr    error
	lastVisitor uint64
	lastOwner   uint64
	lastDate    time.Time
}

func (m *mockVisitRepo) UpsertVisit(_ context.Context, visitorID, ownerID uint64, visitDate time.Time) error {
	m.upsertCalls++
	m.lastVisitor = visitorID
	m.lastOwner = ownerID
	m.lastDate = visitDate
	return m.upsertErr
}

func (m *mockVisitRepo) GetDailyStats(_ context.Context, _ uint64, _ time.Time) (int64, int64, error) {
	return m.statsUnique, m.statsTotal, m.statsErr
}

func (m *mockVisitRepo) GetRecentStats(_ context.Context, _ uint64, _ int) ([]persistence.DailyVisitStat, error) {
	return nil, nil
}

func TestRecordVisit_SelfVisitNotRecorded(t *testing.T) {
	repo := &mockVisitRepo{}
	svc := NewService(repo)
	err := svc.RecordVisit(context.Background(), 1001, 1001)
	if err != nil {
		t.Fatalf("self visit should not error: %v", err)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("self visit should not call UpsertVisit, got %d calls", repo.upsertCalls)
	}
}

func TestRecordVisit_NormalVisit(t *testing.T) {
	repo := &mockVisitRepo{}
	svc := NewService(repo)
	err := svc.RecordVisit(context.Background(), 1001, 2002)
	if err != nil {
		t.Fatalf("normal visit error: %v", err)
	}
	if repo.upsertCalls != 1 {
		t.Fatalf("expected 1 UpsertVisit call, got %d", repo.upsertCalls)
	}
	if repo.lastVisitor != 1001 || repo.lastOwner != 2002 {
		t.Fatalf("unexpected visitor/owner: %d/%d", repo.lastVisitor, repo.lastOwner)
	}
}

func TestRecordVisit_AnonymousVisit(t *testing.T) {
	repo := &mockVisitRepo{}
	svc := NewService(repo)
	err := svc.RecordVisit(context.Background(), 0, 2002)
	if err != nil {
		t.Fatalf("anonymous visit error: %v", err)
	}
	if repo.upsertCalls != 1 {
		t.Fatalf("expected 1 UpsertVisit call, got %d", repo.upsertCalls)
	}
	if repo.lastVisitor != 0 {
		t.Fatalf("expected visitor 0, got %d", repo.lastVisitor)
	}
}

func TestRecordVisit_InvalidOwner(t *testing.T) {
	repo := &mockVisitRepo{}
	svc := NewService(repo)
	err := svc.RecordVisit(context.Background(), 1001, 0)
	if !errors.Is(err, ErrInvalidOwnerID) {
		t.Fatalf("expected ErrInvalidOwnerID, got %v", err)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("should not call UpsertVisit for invalid owner")
	}
}

func TestGetDailyStats_DefaultToday(t *testing.T) {
	repo := &mockVisitRepo{statsUnique: 5, statsTotal: 12}
	svc := NewService(repo)
	unique, total, err := svc.GetDailyStats(context.Background(), 1001, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if unique != 5 || total != 12 {
		t.Fatalf("unexpected stats: unique=%d total=%d", unique, total)
	}
}

func TestGetDailyStats_WithDate(t *testing.T) {
	repo := &mockVisitRepo{statsUnique: 3, statsTotal: 7}
	svc := NewService(repo)
	unique, total, err := svc.GetDailyStats(context.Background(), 1001, "2026-09-01")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if unique != 3 || total != 7 {
		t.Fatalf("unexpected stats: unique=%d total=%d", unique, total)
	}
}

func TestGetDailyStats_InvalidDate(t *testing.T) {
	repo := &mockVisitRepo{}
	svc := NewService(repo)
	_, _, err := svc.GetDailyStats(context.Background(), 1001, "not-a-date")
	if !errors.Is(err, ErrInvalidDate) {
		t.Fatalf("expected ErrInvalidDate, got %v", err)
	}
}

func TestGetDailyStats_InvalidOwner(t *testing.T) {
	repo := &mockVisitRepo{}
	svc := NewService(repo)
	_, _, err := svc.GetDailyStats(context.Background(), 0, "")
	if !errors.Is(err, ErrInvalidOwnerID) {
		t.Fatalf("expected ErrInvalidOwnerID, got %v", err)
	}
}
