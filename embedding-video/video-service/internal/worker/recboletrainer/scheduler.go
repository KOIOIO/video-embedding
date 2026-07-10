package recboletrainer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"nlp-video-analysis/internal/config"
	"nlp-video-analysis/internal/lifecycle"

	"go.uber.org/zap"
)

const recBoleTrainerEnabledEnv = "RECBOLE_TRAINER_ENABLED"

type DailyTime struct {
	Hour   int
	Minute int
}

type Scheduler struct {
	ScriptPath string
	WorkDir    string
	ConfigFile string
	Location   *time.Location
	Schedule   []DailyTime
	RunTimeout time.Duration
	Now        func() time.Time

	mu      sync.Mutex
	running bool
}

func Register(app *lifecycle.App, cfg config.Config) {
	if !EnabledFromEnv() {
		zap.L().Info("recbole_trainer_disabled", zap.String("env", recBoleTrainerEnabledEnv))
		return
	}
	RegisterScheduler(app, cfg)
}

func RegisterScheduler(app *lifecycle.App, cfg config.Config) {
	serviceRoot, err := os.Getwd()
	if err != nil {
		zap.L().Error("recbole_trainer_workdir_failed", zap.Error(err))
		return
	}
	repoRoot := filepath.Dir(serviceRoot)
	trainingDir := filepath.Join(repoRoot, "recbole-training")
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	scheduler := &Scheduler{
		ScriptPath: filepath.Join(trainingDir, "scripts", "run_recbole_pipeline.sh"),
		WorkDir:    trainingDir,
		ConfigFile: selectedConfigFile(),
		Location:   loc,
		Schedule:   defaultSchedule(),
		RunTimeout: 6 * time.Hour,
		Now:        time.Now,
	}
	app.Go(scheduler.Run)
}

func EnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(recBoleTrainerEnabledEnv))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func selectedConfigFile() string {
	if value := os.Getenv("CONFIG_FILE"); value != "" {
		return value
	}
	if value := os.Getenv("VIDEO_CONFIG_FILE"); value != "" {
		return value
	}
	return "configs/video.yml"
}

func defaultSchedule() []DailyTime {
	var schedule []DailyTime
	for h := 0; h < 24; h++ {
		schedule = append(schedule, DailyTime{Hour: h, Minute: 15})
	}
	return schedule
}

func (s *Scheduler) Run(ctx context.Context) error {
	if s == nil {
		return errors.New("RecBole trainer scheduler is nil")
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.Location == nil {
		s.Location = time.Local
	}
	if len(s.Schedule) == 0 {
		return errors.New("RecBole trainer schedule is empty")
	}
	for {
		now := s.Now().In(s.Location)
		next := nextRunAt(now, s.Schedule)
		zap.L().Info("recbole_trainer_next_run", zap.Time("next_run_at", next), zap.String("config", s.ConfigFile))
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if err := s.runOnce(ctx); err != nil {
			zap.L().Error("recbole_trainer_run_failed", zap.Error(err))
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) error {
	if !s.tryStart() {
		zap.L().Warn("recbole_trainer_skip_overlapping_run")
		return nil
	}
	defer s.finish()

	timeout := s.RunTimeout
	if timeout <= 0 {
		timeout = 6 * time.Hour
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, s.ScriptPath)
	cmd.Dir = s.WorkDir
	cmd.Env = append(os.Environ(), "CONFIG_FILE="+s.ConfigFile)
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		zap.L().Info("recbole_trainer_output", zap.ByteString("output", output))
	}
	if err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return runCtx.Err()
		}
		return err
	}
	zap.L().Info("recbole_trainer_run_succeeded")
	return nil
}

func (s *Scheduler) tryStart() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return false
	}
	s.running = true
	return true
}

func (s *Scheduler) finish() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

func nextRunAt(now time.Time, schedule []DailyTime) time.Time {
	normalized := append([]DailyTime(nil), schedule...)
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Hour == normalized[j].Hour {
			return normalized[i].Minute < normalized[j].Minute
		}
		return normalized[i].Hour < normalized[j].Hour
	})
	for _, item := range normalized {
		candidate := time.Date(now.Year(), now.Month(), now.Day(), item.Hour, item.Minute, 0, 0, now.Location())
		if candidate.After(now) {
			return candidate
		}
	}
	first := normalized[0]
	tomorrow := now.AddDate(0, 0, 1)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), first.Hour, first.Minute, 0, 0, now.Location())
}
