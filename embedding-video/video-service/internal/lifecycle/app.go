package lifecycle

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type App struct {
	name            string
	ctx             context.Context
	cancel          context.CancelFunc
	shutdownTimeout time.Duration

	mu      sync.Mutex
	closers []func(context.Context) error

	wg       sync.WaitGroup
	errOnce  sync.Once
	firstErr error
}

// New 创建带信号监听的生命周期容器。
// 收到 Ctrl+C 或 SIGTERM 后，会触发统一关闭流程。
func New(name string, shutdownTimeout time.Duration) *App {
	baseCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	return &App{
		name:            name,
		ctx:             baseCtx,
		cancel:          stop,
		shutdownTimeout: shutdownTimeout,
	}
}

// Context 返回整个应用共享的根上下文。
func (a *App) Context() context.Context {
	return a.ctx
}

// AddCloser 注册一个按 LIFO 顺序执行的关闭动作。
func (a *App) AddCloser(fn func(context.Context) error) {
	if fn == nil {
		return
	}
	a.mu.Lock()
	a.closers = append(a.closers, fn)
	a.mu.Unlock()
}

// Go 在应用上下文中启动一个后台任务。
// 任一任务返回非取消错误时，会触发整个应用进入关闭流程。
func (a *App) Go(fn func(context.Context) error) {
	if fn == nil {
		return
	}
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := fn(a.ctx); err != nil && !errors.Is(err, context.Canceled) {
			a.errOnce.Do(func() {
				a.firstErr = err
				a.cancel()
			})
		}
	}()
}

// Run 启动主任务并阻塞，直到收到退出信号或某个任务返回致命错误。
func (a *App) Run(mainFn func(ctx context.Context) error) error {
	if mainFn != nil {
		a.Go(mainFn)
	}

	<-a.ctx.Done()

	shutdownCtx := context.Background()
	var cancel context.CancelFunc
	if a.shutdownTimeout > 0 {
		shutdownCtx, cancel = context.WithTimeout(shutdownCtx, a.shutdownTimeout)
		defer cancel()
	}

	a.mu.Lock()
	closers := append([]func(context.Context) error(nil), a.closers...)
	a.mu.Unlock()

	var closeErr error
	for i := len(closers) - 1; i >= 0; i-- {
		if err := closers[i](shutdownCtx); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}

	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-shutdownCtx.Done():
		closeErr = errors.Join(closeErr, shutdownCtx.Err())
	}

	if a.firstErr == nil {
		return closeErr
	}
	if closeErr != nil {
		return errors.Join(a.firstErr, closeErr)
	}
	return a.firstErr
}
