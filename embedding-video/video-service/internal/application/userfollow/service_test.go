package userfollow

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"video-service/internal/model"
)

// --- mock follow repository ---

type mockFollowRepo struct {
	follows       map[[2]uint64]bool // key: [follower, following]
	createErr     error
	deleteErr     error
	isFollowing   map[[2]uint64]bool
	followers     map[uint64]int64
	following     map[uint64]int64
	followingList []model.FollowWithProfile
	followersList []model.FollowWithProfile
	createCalls   int
	deleteCalls   int
}

func newMockFollowRepo() *mockFollowRepo {
	return &mockFollowRepo{
		follows:     make(map[[2]uint64]bool),
		isFollowing: make(map[[2]uint64]bool),
		followers:   make(map[uint64]int64),
		following:   make(map[uint64]int64),
	}
}

func (m *mockFollowRepo) CreateFollow(_ context.Context, followerID, followingID uint64) error {
	m.createCalls++
	if m.createErr != nil {
		return m.createErr
	}
	key := [2]uint64{followerID, followingID}
	if m.follows[key] {
		return errors.New("duplicate key value violates unique constraint")
	}
	m.follows[key] = true
	m.isFollowing[key] = true
	m.following[followerID]++
	m.followers[followingID]++
	return nil
}

func (m *mockFollowRepo) DeleteFollow(_ context.Context, followerID, followingID uint64) error {
	m.deleteCalls++
	if m.deleteErr != nil {
		return m.deleteErr
	}
	key := [2]uint64{followerID, followingID}
	if !m.follows[key] {
		return errors.New("follow relation not found")
	}
	delete(m.follows, key)
	delete(m.isFollowing, key)
	m.following[followerID]--
	m.followers[followingID]--
	return nil
}

func (m *mockFollowRepo) IsFollowing(_ context.Context, followerID, followingID uint64) (bool, error) {
	return m.isFollowing[[2]uint64{followerID, followingID}], nil
}

func (m *mockFollowRepo) CountFollowers(_ context.Context, userID uint64) (int64, error) {
	return m.followers[userID], nil
}

func (m *mockFollowRepo) CountFollowing(_ context.Context, userID uint64) (int64, error) {
	return m.following[userID], nil
}

func (m *mockFollowRepo) ListFollowing(_ context.Context, _ uint64, _, _ int) ([]model.FollowWithProfile, error) {
	return m.followingList, nil
}

func (m *mockFollowRepo) ListFollowers(_ context.Context, _ uint64, _, _ int) ([]model.FollowWithProfile, error) {
	return m.followersList, nil
}

// --- mock profile counter ---

type mockProfileCounter struct {
	followCounts map[uint64]int
	fansCounts   map[uint64]int
}

func newMockProfileCounter() *mockProfileCounter {
	return &mockProfileCounter{
		followCounts: make(map[uint64]int),
		fansCounts:   make(map[uint64]int),
	}
}

func (m *mockProfileCounter) IncrementFollowCount(userID uint64, delta int) error {
	m.followCounts[userID] += delta
	if m.followCounts[userID] < 0 {
		m.followCounts[userID] = 0
	}
	return nil
}

func (m *mockProfileCounter) IncrementFansCount(userID uint64, delta int) error {
	m.fansCounts[userID] += delta
	if m.fansCounts[userID] < 0 {
		m.fansCounts[userID] = 0
	}
	return nil
}

// --- helpers ---

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.EduUserFollow{}, &model.EduUserProfile{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// --- tests ---

func TestFollowCannotFollowSelf(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db, newMockFollowRepo(), newMockProfileCounter())
	err := svc.Follow(context.Background(), 1001, 1001)
	if !errors.Is(err, ErrCannotFollowSelf) {
		t.Fatalf("Follow(self) error = %v, want ErrCannotFollowSelf", err)
	}
}

func TestUnfollowCannotUnfollowSelf(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db, newMockFollowRepo(), newMockProfileCounter())
	err := svc.Unfollow(context.Background(), 1001, 1001)
	if !errors.Is(err, ErrCannotFollowSelf) {
		t.Fatalf("Unfollow(self) error = %v, want ErrCannotFollowSelf", err)
	}
}

func TestFollowSuccessUpdatesCounts(t *testing.T) {
	db := newTestDB(t)
	followRepo := newMockFollowRepo()
	profileRepo := newMockProfileCounter()
	svc := NewService(db, followRepo, profileRepo)

	err := svc.Follow(context.Background(), 1001, 2002)
	if err != nil {
		t.Fatalf("Follow error: %v", err)
	}
	if followRepo.createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", followRepo.createCalls)
	}
	if profileRepo.followCounts[1001] != 1 {
		t.Fatalf("follower follow_count = %d, want 1", profileRepo.followCounts[1001])
	}
	if profileRepo.fansCounts[2002] != 1 {
		t.Fatalf("following fans_count = %d, want 1", profileRepo.fansCounts[2002])
	}
}

func TestFollowDuplicateReturnsAlreadyFollowing(t *testing.T) {
	db := newTestDB(t)
	followRepo := newMockFollowRepo()
	svc := NewService(db, followRepo, newMockProfileCounter())

	// first follow
	if err := svc.Follow(context.Background(), 1001, 2002); err != nil {
		t.Fatalf("first Follow error: %v", err)
	}
	// duplicate follow
	err := svc.Follow(context.Background(), 1001, 2002)
	if !errors.Is(err, ErrAlreadyFollowing) {
		t.Fatalf("duplicate Follow error = %v, want ErrAlreadyFollowing", err)
	}
}

func TestUnfollowSuccessUpdatesCounts(t *testing.T) {
	db := newTestDB(t)
	followRepo := newMockFollowRepo()
	profileRepo := newMockProfileCounter()
	svc := NewService(db, followRepo, profileRepo)

	// setup: create a follow relation
	followRepo.follows[[2]uint64{1001, 2002}] = true
	followRepo.isFollowing[[2]uint64{1001, 2002}] = true
	followRepo.following[1001] = 1
	followRepo.followers[2002] = 1
	profileRepo.followCounts[1001] = 1
	profileRepo.fansCounts[2002] = 1

	err := svc.Unfollow(context.Background(), 1001, 2002)
	if err != nil {
		t.Fatalf("Unfollow error: %v", err)
	}
	if followRepo.deleteCalls != 1 {
		t.Fatalf("deleteCalls = %d, want 1", followRepo.deleteCalls)
	}
	if profileRepo.followCounts[1001] != 0 {
		t.Fatalf("follower follow_count = %d, want 0", profileRepo.followCounts[1001])
	}
	if profileRepo.fansCounts[2002] != 0 {
		t.Fatalf("following fans_count = %d, want 0", profileRepo.fansCounts[2002])
	}
}

func TestGetRelationNone(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db, newMockFollowRepo(), newMockProfileCounter())
	rel, err := svc.GetRelation(context.Background(), 1001, 2002)
	if err != nil {
		t.Fatalf("GetRelation error: %v", err)
	}
	if rel != "none" {
		t.Fatalf("relation = %q, want %q", rel, "none")
	}
}

func TestGetRelationFollowing(t *testing.T) {
	db := newTestDB(t)
	followRepo := newMockFollowRepo()
	followRepo.isFollowing[[2]uint64{1001, 2002}] = true
	svc := NewService(db, followRepo, newMockProfileCounter())
	rel, err := svc.GetRelation(context.Background(), 1001, 2002)
	if err != nil {
		t.Fatalf("GetRelation error: %v", err)
	}
	if rel != "following" {
		t.Fatalf("relation = %q, want %q", rel, "following")
	}
}

func TestGetRelationMutual(t *testing.T) {
	db := newTestDB(t)
	followRepo := newMockFollowRepo()
	followRepo.isFollowing[[2]uint64{1001, 2002}] = true
	followRepo.isFollowing[[2]uint64{2002, 1001}] = true
	svc := NewService(db, followRepo, newMockProfileCounter())
	rel, err := svc.GetRelation(context.Background(), 1001, 2002)
	if err != nil {
		t.Fatalf("GetRelation error: %v", err)
	}
	if rel != "mutual" {
		t.Fatalf("relation = %q, want %q", rel, "mutual")
	}
}

func TestGetRelationSameUserReturnsNone(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db, newMockFollowRepo(), newMockProfileCounter())
	rel, err := svc.GetRelation(context.Background(), 1001, 1001)
	if err != nil {
		t.Fatalf("GetRelation error: %v", err)
	}
	if rel != "none" {
		t.Fatalf("relation = %q, want %q", rel, "none")
	}
}

func TestGetRelationZeroUserReturnsNone(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db, newMockFollowRepo(), newMockProfileCounter())
	rel, err := svc.GetRelation(context.Background(), 0, 2002)
	if err != nil {
		t.Fatalf("GetRelation error: %v", err)
	}
	if rel != "none" {
		t.Fatalf("relation = %q, want %q", rel, "none")
	}
}
