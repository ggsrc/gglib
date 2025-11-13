package ratelimit

import (
	"context"
	"errors"
	"time"

	"github.com/kelseyhightower/envconfig"
	"golang.org/x/time/rate"
)

type MethodLimitConfig struct {
	MethodCapacity map[string]int `required:"true"`
	Timeout        time.Duration  `default:"500ms"`
}

type RateLimitManager struct {
	methodLimitter map[string]*rate.Limiter
	conf           *MethodLimitConfig
}

func NewRateLimitManagerWithOptions(opts ...Option) *RateLimitManager {
	conf := &MethodLimitConfig{}
	for _, opt := range opts {
		opt(conf)
	}
	return &RateLimitManager{conf: conf}
}

func NewRateLimitManagerWithDefaultEnvPrefix() *RateLimitManager {
	return NewRateLimitManager("ratelimit")
}

func NewRateLimitManager(envPrefix string) *RateLimitManager {
	config := &MethodLimitConfig{}
	envconfig.MustProcess(envPrefix, config)

	rlm := &RateLimitManager{
		methodLimitter: make(map[string]*rate.Limiter),
	}
	for method, capacity := range config.MethodCapacity {
		rlm.methodLimitter[method] = rate.NewLimiter(rate.Limit(capacity), 10)
	}
	return rlm
}

func (rlm *RateLimitManager) Allow(ctx context.Context, method string) bool {
	limiter := rlm.methodLimitter[method]

	if limiter.Allow() {
		return true
	}

	ctx, cancel := context.WithTimeout(ctx, rlm.conf.Timeout)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return false
			}
		case <-ticker.C:
			if limiter.Allow() {
				return true
			}
		}
	}
}
