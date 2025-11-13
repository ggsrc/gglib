package metric

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var counters = map[string]*prometheus.CounterVec{}

type MetricEvent struct {
	Name   string
	Labels map[string]string
	Value  float64
}

type Server struct {
	conf       *Config
	httpServer *http.Server
	errCh      chan error
}

type Config struct {
	Port int `default:"4014"`
}

func NewWithOptions(opts ...Option) *Server {
	conf := &Config{}
	for _, opt := range opts {
		opt(conf)
	}
	return &Server{conf: conf}
}

func NewWithDefaultEnvPrefix() *Server {
	return New("metric")
}

func New(envPrefix string) *Server {
	conf := &Config{}
	envconfig.MustProcess(envPrefix, conf)
	return &Server{conf: conf}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", s.conf.Port),
		Handler:           mux,
		ReadHeaderTimeout: time.Second * 5,
	}
	s.httpServer = server
	s.errCh = make(chan error)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.errCh <- err
		}
		close(s.errCh)
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) OK(ctx context.Context) error {
	select {
	case err := <-s.errCh:
		return err
	default:
		return nil
	}
}

// RecordEvent auto register counter
func RecordEvent(e MetricEvent) {
	// 1. 对 labelKeys 排序
	labelKeys := make([]string, 0, len(e.Labels))
	for k := range e.Labels {
		labelKeys = append(labelKeys, k)
	}
	sort.Strings(labelKeys)

	// 2. 根据排序后的 keys 构造 labelValues
	labelValues := make([]string, 0, len(labelKeys))
	for _, k := range labelKeys {
		labelValues = append(labelValues, e.Labels[k])
	}

	// 3. 注册并缓存 CounterVec
	counter, ok := counters[e.Name]
	if !ok {
		counter = prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: e.Name, Help: "auto generated"},
			labelKeys,
		)
		prometheus.MustRegister(counter)
		counters[e.Name] = counter
	}

	// 4. 打点
	counter.WithLabelValues(labelValues...).Add(e.Value)
}
