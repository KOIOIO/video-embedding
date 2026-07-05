package persistence

import (
	"errors"
	"reflect"
	"testing"
)

func TestRunWithMigrationAdvisoryLockOrdersLockMigrateUnlock(t *testing.T) {
	var calls []string

	err := runWithMigrationAdvisoryLock(
		func() error {
			calls = append(calls, "lock")
			return nil
		},
		func() error {
			calls = append(calls, "unlock")
			return nil
		},
		func() error {
			calls = append(calls, "migrate")
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runWithMigrationAdvisoryLock() error = %v", err)
	}

	want := []string{"lock", "migrate", "unlock"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestRunWithMigrationAdvisoryLockUnlocksAfterMigrationFailure(t *testing.T) {
	migrateErr := errors.New("migrate failed")
	var calls []string

	err := runWithMigrationAdvisoryLock(
		func() error {
			calls = append(calls, "lock")
			return nil
		},
		func() error {
			calls = append(calls, "unlock")
			return nil
		},
		func() error {
			calls = append(calls, "migrate")
			return migrateErr
		},
	)
	if !errors.Is(err, migrateErr) {
		t.Fatalf("runWithMigrationAdvisoryLock() error = %v, want migrate error", err)
	}

	want := []string{"lock", "migrate", "unlock"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestRunWithMigrationAdvisoryLockSkipsMigrationWhenLockFails(t *testing.T) {
	lockErr := errors.New("lock failed")
	var calls []string

	err := runWithMigrationAdvisoryLock(
		func() error {
			calls = append(calls, "lock")
			return lockErr
		},
		func() error {
			calls = append(calls, "unlock")
			return nil
		},
		func() error {
			calls = append(calls, "migrate")
			return nil
		},
	)
	if !errors.Is(err, lockErr) {
		t.Fatalf("runWithMigrationAdvisoryLock() error = %v, want lock error", err)
	}

	want := []string{"lock"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}
