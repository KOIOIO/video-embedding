package recboletrainer

import (
	"os"
	"testing"
	"time"
)

func TestTrainerEnabledFromEnvDefaultsDisabled(t *testing.T) {
	previous, hadPrevious := os.LookupEnv(recBoleTrainerEnabledEnv)
	t.Cleanup(func() {
		if hadPrevious {
			_ = os.Setenv(recBoleTrainerEnabledEnv, previous)
			return
		}
		_ = os.Unsetenv(recBoleTrainerEnabledEnv)
	})
	_ = os.Unsetenv(recBoleTrainerEnabledEnv)

	if EnabledFromEnv() {
		t.Fatalf("EnabledFromEnv() = true, want false when %s is unset", recBoleTrainerEnabledEnv)
	}
}

func TestTrainerEnabledFromEnvAcceptsTrueValues(t *testing.T) {
	tests := []string{"1", "true", "TRUE", "yes", "on"}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			t.Setenv(recBoleTrainerEnabledEnv, value)
			if !EnabledFromEnv() {
				t.Fatalf("EnabledFromEnv() = false, want true for %s=%q", recBoleTrainerEnabledEnv, value)
			}
		})
	}
}

func TestTrainerEnabledFromEnvRejectsFalseValues(t *testing.T) {
	tests := []string{"0", "false", "FALSE", "no", "off", "unexpected"}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			t.Setenv(recBoleTrainerEnabledEnv, value)
			if EnabledFromEnv() {
				t.Fatalf("EnabledFromEnv() = true, want false for %s=%q", recBoleTrainerEnabledEnv, value)
			}
		})
	}
}

func TestNextRunAtUsesConfiguredDailyTimes(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	schedule := []DailyTime{
		{Hour: 0, Minute: 0},
		{Hour: 4, Minute: 0},
		{Hour: 8, Minute: 0},
		{Hour: 12, Minute: 0},
		{Hour: 16, Minute: 0},
		{Hour: 20, Minute: 0},
	}

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "before 04 slot",
			now:  time.Date(2026, 6, 23, 0, 10, 0, 0, loc),
			want: time.Date(2026, 6, 23, 4, 0, 0, 0, loc),
		},
		{
			name: "between 04 and 08",
			now:  time.Date(2026, 6, 23, 4, 1, 0, 0, loc),
			want: time.Date(2026, 6, 23, 8, 0, 0, 0, loc),
		},
		{
			name: "after last run rolls to next day",
			now:  time.Date(2026, 6, 23, 20, 1, 0, 0, loc),
			want: time.Date(2026, 6, 24, 0, 0, 0, 0, loc),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := nextRunAt(tc.now, schedule)
			if !got.Equal(tc.want) {
				t.Fatalf("nextRunAt() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestDefaultScheduleRunsEverHourStartingAtQuarterPast(t *testing.T) {
	got := defaultSchedule()
	if len(got) != 24 {
		t.Fatalf("defaultSchedule len = %d, want 24: %+v", len(got), got)
	}
	for i := 0; i < 24; i++ {
		want := DailyTime{Hour: i, Minute: 15}
		if got[i] != want {
			t.Fatalf("defaultSchedule[%d] = %+v, want %+v", i, got[i], want)
		}
	}
}

func TestNextRunAtMovesPastExactCurrentSlot(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	schedule := []DailyTime{{Hour: 0, Minute: 0}, {Hour: 4, Minute: 0}}
	now := time.Date(2026, 6, 23, 4, 0, 0, 0, loc)

	got := nextRunAt(now, schedule)
	want := time.Date(2026, 6, 24, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("nextRunAt() = %s, want %s", got, want)
	}
}
