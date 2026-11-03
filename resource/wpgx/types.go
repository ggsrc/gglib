package wpgx

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kelseyhightower/envconfig"

	"github.com/ggsrc/gglib/zerolog/log"
)

const defaultTimeout = 5 * time.Second

// TypeLoaderParam configures a Loader: the Postgres type names to load and
// the timeout for loading them on one connection.
type TypeLoaderParam struct {
	Timeout time.Duration `default:"5s"`
	Types   []string
}

var defaultLoader *Loader

// InitTypeLoader sets up the package-level Loader. A nil p reads
// WPGX_TYPE_LOADER_* environment variables.
func InitTypeLoader(p *TypeLoaderParam) {
	if p == nil {
		cfg := TypeLoaderParam{}
		envconfig.MustProcess("WPGX_TYPE_LOADER", &cfg)
		log.Info().Msgf("type loader config: %+v", cfg)
		p = &cfg
	}
	defaultLoader = NewLoader(p)
}

// LoadTypes registers the configured types on conn with the package-level
// Loader. It always returns true, so it can be used as a BeforeAcquire hook.
func LoadTypes(ctx context.Context, conn *pgx.Conn) bool {
	if defaultLoader == nil {
		// no type loader, skip
		log.Error().Msg("type loader not initialized")
		return true
	}
	return defaultLoader.LoadTypes(ctx, conn)
}

// Reset clears the package-level Loader.
func Reset() {
	if defaultLoader == nil {
		// no type loader, skip
		log.Error().Msg("type loader not initialized")
		return
	}
	defaultLoader.Reset()
}

// NewLoader returns a Loader for the de-duplicated type names in p.
func NewLoader(p *TypeLoaderParam) *Loader {
	l := Loader{
		ptMap:   map[string]*pgtype.Type{},
		timeout: defaultTimeout,
	}
	if p.Timeout > 0 {
		l.timeout = p.Timeout
	}
	l.ts = make([]string, 0, len(p.Types))
	dedup := map[string]struct{}{}
	for _, t := range p.Types {
		if _, ok := dedup[t]; !ok {
			dedup[t] = struct{}{}
			l.ts = append(l.ts, t)
		}
	}
	return &l
}

// Loader loads custom Postgres types once and registers them on every
// connection.
type Loader struct {
	ptMap   map[string]*pgtype.Type
	timeout time.Duration
	ts      []string
}

// LoadTypes registers the configured types on conn, loading each one from the
// database the first time. It always returns true, so it can be used as a
// BeforeAcquire hook.
func (l *Loader) LoadTypes(ctx context.Context, conn *pgx.Conn) bool {
	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()
	for _, t := range l.ts {
		// reuse type if already registered in conn
		if _, ok := conn.TypeMap().TypeForName(t); ok {
			continue
		}
		if pt, ok := l.ptMap[t]; ok {
			// reuse type if already loaded in ptMap
			conn.TypeMap().RegisterType(pt)
		} else {
			var err error
			// load type from database
			if pt, err = conn.LoadType(ctx, t); err != nil {
				log.Ctx(ctx).Err(err).Msgf("failed to load type %s", t)
				return true
			}
			log.Ctx(ctx).Info().Msgf("loaded type %s", t)
			conn.TypeMap().RegisterType(pt)
			l.ptMap[t] = pt
		}
	}
	return true
}

// Reset forgets the configured and loaded types.
func (l *Loader) Reset() {
	l.ptMap = map[string]*pgtype.Type{}
	l.ts = []string{}
	l.timeout = defaultTimeout
}
