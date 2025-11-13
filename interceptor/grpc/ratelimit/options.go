package ratelimit

import "time"

type Option func(*MethodLimitConfig)

func WithMethodCapacity(method string, capacity int) Option {
	return func(c *MethodLimitConfig) {
		c.MethodCapacity[method] = capacity
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *MethodLimitConfig) {
		c.Timeout = timeout
	}
}
