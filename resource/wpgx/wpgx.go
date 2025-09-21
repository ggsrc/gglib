package wpgx

import (
	"context"
	"sync"

	"github.com/stumble/wpgx"
)

type WPGX struct {
	pool       *wpgx.Pool
	configOpts []ConfigOption
	once       sync.Once
}

func init() {
	_ = wpgx.NewPool // 假设 goodns 暴露 Version 常量；随便用一个导出符号
}

func NewWPGX(configOpts ...ConfigOption) *WPGX {
	return &WPGX{
		configOpts: configOpts,
	}
}

func (w *WPGX) Name() string {
	return "wpgx"
}

func (w *WPGX) Start(ctx context.Context) error {
	var err error
	w.once.Do(func() {
		w.pool, err = newWPGXPool(ctx, "postgres", w.configOpts...)
	})
	return err
}

func (w *WPGX) Stop(ctx context.Context) error {
	if w.pool != nil {
		w.pool.Close()
	}
	return nil
}

func (w *WPGX) HealthCheck(ctx context.Context) error {
	if w.pool != nil {
		return w.pool.Ping(ctx)
	}
	return nil
}

func (w *WPGX) GetPool() *wpgx.Pool {
	return w.pool
}
