package antspool

import (
	"context"
	"sync"
	"time"

	ants "github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
)

type Recorder interface {
	OnPoolCreated(name string, size int)
	OnSubmit(name string)
	OnSubmitError(name string, err error)
	OnTaskDone(name string, dur time.Duration)
	OnTaskError(name string, err error, dur time.Duration)
}

type Options struct {
	Name     string
	Size     int
	Logger   *zap.Logger
	Recorder Recorder
}

type Group struct {
	ctx      context.Context
	name     string
	pool     *ants.Pool
	logger   *zap.Logger
	recorder Recorder
	wg       sync.WaitGroup
	errMu    sync.Mutex
	first    error

	statsMu       sync.Mutex
	submitted     int
	completed     int
	failed        int
	waitStartedAt time.Time
}

func New(ctx context.Context, opts Options) (*Group, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Size <= 0 {
		opts.Size = 1
	}
	pool, err := ants.NewPool(opts.Size)
	if err != nil {
		return nil, err
	}
	g := &Group{ctx: ctx, name: opts.Name, pool: pool, logger: opts.Logger, recorder: opts.Recorder}
	if g.recorder == nil {
		g.recorder = getDefaultRecorder()
	}
	if g.recorder != nil {
		g.recorder.OnPoolCreated(g.name, opts.Size)
	}
	if g.logger != nil {
		g.logger.Info("ants_pool_created", zap.String("pool_name", g.name), zap.Int("size", opts.Size))
	}
	return g, nil
}

func (g *Group) Submit(fn func() error) error {
	if fn == nil {
		return nil
	}
	select {
	case <-g.ctx.Done():
		if g.recorder != nil {
			g.recorder.OnSubmitError(g.name, g.ctx.Err())
		}
		return g.ctx.Err()
	default:
	}

	g.statsMu.Lock()
	g.submitted++
	g.statsMu.Unlock()
	if g.recorder != nil {
		g.recorder.OnSubmit(g.name)
	}

	g.wg.Add(1)
	err := g.pool.Submit(func() {
		startedAt := time.Now()
		defer g.wg.Done()
		if err := fn(); err != nil {
			g.errMu.Lock()
			if g.first == nil {
				g.first = err
			}
			g.errMu.Unlock()
			g.statsMu.Lock()
			g.failed++
			g.statsMu.Unlock()
			if g.recorder != nil {
				g.recorder.OnTaskError(g.name, err, time.Since(startedAt))
			}
			if g.logger != nil {
				g.logger.Warn("ants_pool_task_failed", zap.String("pool_name", g.name), zap.Duration("cost", time.Since(startedAt)), zap.Error(err))
			}
			return
		}
		g.statsMu.Lock()
		g.completed++
		g.statsMu.Unlock()
		if g.recorder != nil {
			g.recorder.OnTaskDone(g.name, time.Since(startedAt))
		}
	})
	if err != nil {
		g.wg.Done()
		if g.recorder != nil {
			g.recorder.OnSubmitError(g.name, err)
		}
		if g.logger != nil {
			g.logger.Warn("ants_pool_submit_failed", zap.String("pool_name", g.name), zap.Error(err))
		}
		return err
	}
	return nil
}

func (g *Group) Wait() error {
	g.statsMu.Lock()
	g.waitStartedAt = time.Now()
	g.statsMu.Unlock()
	g.wg.Wait()

	g.statsMu.Lock()
	submitted := g.submitted
	completed := g.completed
	failed := g.failed
	waitCost := time.Since(g.waitStartedAt)
	g.statsMu.Unlock()

	g.errMu.Lock()
	defer g.errMu.Unlock()
	if g.logger != nil {
		g.logger.Info("ants_pool_wait_done", zap.String("pool_name", g.name), zap.Int("submitted", submitted), zap.Int("completed", completed), zap.Int("failed", failed), zap.Duration("wait_cost", waitCost))
	}
	return g.first
}

func (g *Group) Release() {
	if g == nil || g.pool == nil {
		return
	}
	g.pool.Release()
}
