package wpgx

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stumble/wpgx"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	testUser     = "gglib"
	testPassword = "p@ss/w#rd?x" // characters that break an unescaped DSN
	testDB       = "gglib_test"
)

var (
	pgHost string
	pgPort int
)

// TestMain starts one Postgres container for the package. Since Go 1.15 the
// test binary exits with m.Run's result when TestMain returns.
func TestMain(m *testing.M) {
	ctx := context.Background()
	ctr, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithUsername(testUser),
		postgres.WithPassword(testPassword),
		postgres.WithDatabase(testDB),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		panic("start postgres: " + err.Error())
	}
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()

	host, err := ctr.Host(ctx)
	if err != nil {
		panic(err)
	}
	port, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		panic(err)
	}
	pgHost, pgPort = host, int(port.Num())
	m.Run()
}

func testConfig() *wpgx.Config {
	return &wpgx.Config{
		Username:        testUser,
		Password:        testPassword,
		Host:            pgHost,
		Port:            pgPort,
		DBName:          testDB,
		MaxConns:        4,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: time.Minute,
		SSLMode:         "disable",
		AppName:         "gglib-test",
	}
}

func TestLifecycle(t *testing.T) {
	w := NewWPGXWithOptions(WithConfig(testConfig()))
	ctx := t.Context()

	assert.Equal(t, "wpgx", w.Name())
	require.ErrorIs(t, w.OK(ctx), errNotInitialized)
	require.ErrorIs(t, w.Start(ctx), errNotInitialized)
	assert.Nil(t, w.GetPool())

	require.NoError(t, w.Init(ctx))
	require.Error(t, w.Init(ctx), "second Init")
	require.NoError(t, w.Start(ctx))
	require.NoError(t, w.OK(ctx))

	var one int
	require.NoError(t, w.GetPool().RawPool().QueryRow(ctx, "SELECT 1").Scan(&one))
	assert.Equal(t, 1, one)

	require.NoError(t, w.Stop(ctx))
	require.NoError(t, w.Stop(ctx), "Stop is idempotent")
	assert.Nil(t, w.GetPool())
	require.ErrorIs(t, w.OK(ctx), errNotInitialized)
}

// A failed Init used to be reported as success on the second call, with a nil
// pool, and the failed pool was never closed.
func TestInitCanBeRetriedAfterFailure(t *testing.T) {
	cfg := testConfig()
	cfg.Password = "wrong"
	w := NewWPGXWithOptions(WithConfig(cfg))

	err := w.Init(t.Context())
	require.Error(t, err)
	assert.Nil(t, w.GetPool())
	require.ErrorIs(t, w.Start(t.Context()), errNotInitialized)

	cfg.Password = testPassword
	require.NoError(t, w.Init(t.Context()))
	require.NoError(t, w.OK(t.Context()))
	require.NoError(t, w.Stop(t.Context()))
}

func TestInitFailsOnInvalidConfig(t *testing.T) {
	cfg := testConfig()
	cfg.MaxConns = 0
	err := NewWPGXWithOptions(WithConfig(cfg)).Init(t.Context())
	require.ErrorContains(t, err, "wpgx: create pool")
}

// The wrapper's log line used to write "*hidden*" into the caller's replica
// password before the pool was created.
func TestInitLeavesConfigUnchanged(t *testing.T) {
	cfg := testConfig()
	cfg.ReadReplicas = []wpgx.ReadReplicaConfig{{
		Name:            "replica",
		Username:        testUser,
		Password:        testPassword,
		Host:            pgHost,
		Port:            pgPort,
		DBName:          testDB,
		MaxConns:        2,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: time.Minute,
		SSLMode:         "disable",
	}}
	hook := func(context.Context, *pgx.Conn) bool { return true }
	w := NewWPGXWithOptions(WithConfig(cfg), WithBeforeAcquire(hook))
	require.NoError(t, w.Init(t.Context()))
	t.Cleanup(func() { _ = w.Stop(context.Background()) })

	assert.Equal(t, testPassword, cfg.Password)
	assert.Equal(t, testPassword, cfg.ReadReplicas[0].Password)
	assert.Nil(t, cfg.BeforeAcquire, "the hook is set on a copy")
	assert.Len(t, w.GetPool().ReplicaPools(), 1)
}

func TestBeforeAcquireRuns(t *testing.T) {
	var calls atomic.Int32
	w := NewWPGXWithOptions(WithConfig(testConfig()), WithBeforeAcquire(func(context.Context, *pgx.Conn) bool {
		calls.Add(1)
		return true
	}))
	require.NoError(t, w.Init(t.Context()))
	t.Cleanup(func() { _ = w.Stop(context.Background()) })

	var one int
	require.NoError(t, w.GetPool().RawPool().QueryRow(t.Context(), "SELECT 1").Scan(&one))
	assert.Positive(t, calls.Load())
}

// Stop used to block until every connection was released, past the shutdown
// deadline.
func TestStopRespectsDeadline(t *testing.T) {
	w := NewWPGXWithOptions(WithConfig(testConfig()))
	require.NoError(t, w.Init(t.Context()))
	conn, err := w.GetPool().RawPool().Acquire(t.Context())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = w.Stop(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 5*time.Second)
	conn.Release()
}

func TestWithInitTimeout(t *testing.T) {
	ctx, cancel := withInitTimeout(context.Background())
	defer cancel()
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(DefaultInitTimeout), deadline, time.Second)

	parent, cancelParent := context.WithTimeout(context.Background(), time.Minute)
	defer cancelParent()
	ctx, cancel = withInitTimeout(parent)
	defer cancel()
	got, _ := ctx.Deadline()
	want, _ := parent.Deadline()
	assert.Equal(t, want, got, "an existing deadline is kept")
}

func TestConstructors(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "db.example")
	t.Setenv("POSTGRES_PORT", strconv.Itoa(6543))
	t.Setenv("POSTGRES_APPNAME", "svc")
	w := NewWPGXWithDefaultEnvPrefix()
	assert.Equal(t, "db.example", w.config.Host)
	assert.Equal(t, 6543, w.config.Port)

	assert.Panics(t, func() { NewWPGXWithOptions() })
}
