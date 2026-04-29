package caddynomadsd

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"

	nomad "github.com/hashicorp/nomad/api"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Upstream = reverseproxy.Upstream

var (
	sds   = make(map[string]sdLookup)
	sdsMu sync.RWMutex
)

func init() {
	caddy.RegisterModule(NomadSDUpstreams{})
}

type NomadSDUpstreams struct {
	Name      string         `json:"name"`
	Tag       string         `json:"tag"`
	Namespace string         `json:"namespace"`
	Refresh   caddy.Duration `json:"refresh,omitempty"`

	client *nomad.Client
	logger *zap.Logger
}

func (NomadSDUpstreams) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.upstreams.nomadsd",
		New: func() caddy.Module { return new(NomadSDUpstreams) },
	}
}

func (u *NomadSDUpstreams) Provision(ctx caddy.Context) error {
	u.logger = ctx.Logger()

	if u.Refresh == 0 {
		u.Refresh = caddy.Duration(time.Minute)
	}

	client, err := nomad.NewClient(nomad.DefaultConfig())
	if err != nil {
		return err
	}
	u.client = client

	return nil
}

func allNew(upstreams []Upstream) []*Upstream {
	results := make([]*Upstream, len(upstreams))
	for i := range upstreams {
		results[i] = &Upstream{Dial: upstreams[i].Dial}
	}
	return results
}

func (u NomadSDUpstreams) GetUpstreams(r *http.Request) ([]*Upstream, error) {
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	tag := repl.ReplaceKnown(u.Tag, "")
	name := repl.ReplaceKnown(u.Name, "")
	namespace := repl.ReplaceKnown(u.Namespace, "")

	key := tag + name + namespace

	// first, use a cheap read-lock to return a cached result quickly
	sdsMu.RLock()
	cached := sds[key]
	sdsMu.RUnlock()
	if cached.isFresh() {
		return allNew(cached.upstreams), nil
	}

	// otherwise, obtain a write-lock to update the cached value
	sdsMu.Lock()
	defer sdsMu.Unlock()

	// check to see if it's still stale, since we're now in a different
	// lock from when we first checked freshness; another goroutine might
	// have refreshed it in the meantime before we re-obtained our lock
	cached = sds[key]
	if cached.isFresh() {
		return allNew(cached.upstreams), nil
	}

	opts := &nomad.QueryOptions{AllowStale: true}
	if namespace != "" {
		opts.Namespace = namespace
	}
	if tag != "" {
		opts.Filter = fmt.Sprintf(`Tags contains "%s"`, tag)
	}

	if c := u.logger.Check(zapcore.DebugLevel, "refreshing nomad-sd upstreams"); c != nil {
		c.Write(zap.String("name", name), zap.Any("opts", opts))
	}

	srvs, _, err := u.client.Services().Get(name, opts)
	if err != nil {
		return nil, err
	}

	upstreams := make([]Upstream, len(srvs))
	for i, srv := range srvs {
		port := strconv.Itoa(srv.Port)
		if c := u.logger.Check(zapcore.DebugLevel, "discovered service records"); c != nil {
			c.Write(zap.String("name", name),
				zap.String("address", srv.Address),
				zap.String("port", port),
				zap.String("namespace", srv.Namespace),
				zap.Strings("tags", srv.Tags))
		}

		upstreams[i] = Upstream{
			Dial: net.JoinHostPort(srv.Address, port),
		}
	}

	// before adding a new one to the cache (as opposed to replacing stale one), make room if cache is full
	if cached.freshness.IsZero() && len(sds) >= 100 {
		for randomKey := range sds {
			delete(sds, randomKey)
			break
		}
	}

	sds[key] = sdLookup{
		sdUpstreams: u,
		freshness:   time.Now(),
		upstreams:   upstreams,
	}

	return allNew(upstreams), nil
}

// UnmarshalCaddyfile deserializes Caddyfile tokens into h.
//
//	dynamic nomadsd [<name>] {
//	    name                <name>
//	    refresh             <interval>
//	}
func (u *NomadSDUpstreams) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next()

	args := d.RemainingArgs()
	if len(args) > 1 {
		return d.ArgErr()
	}
	if len(args) > 0 {
		u.Name = args[0]
	}

	for d.NextBlock(0) {
		switch d.Val() {
		case "name":
			if !d.NextArg() {
				return d.ArgErr()
			}
			u.Name = d.Val()

		case "namespace":
			if !d.NextArg() {
				return d.ArgErr()
			}
			u.Namespace = d.Val()

		case "tag":
			if !d.NextArg() {
				return d.ArgErr()
			}
			u.Tag = d.Val()

		case "refresh":
			if !d.NextArg() {
				return d.ArgErr()
			}
			dur, err := caddy.ParseDuration(d.Val())
			if err != nil {
				return d.Errf("parsing refresh interval duration: %v", err)
			}
			u.Refresh = caddy.Duration(dur)

		default:
			return d.Errf("unrecognized srv option '%s'", d.Val())
		}
	}

	if u.Name == "" {
		return d.Errf("missing required 'name' option")
	}

	return nil
}

type sdLookup struct {
	sdUpstreams NomadSDUpstreams
	freshness   time.Time
	upstreams   []Upstream
}

func (sl sdLookup) isFresh() bool {
	return time.Since(sl.freshness) < time.Duration(sl.sdUpstreams.Refresh)
}

var (
	_ caddy.Module                = (*NomadSDUpstreams)(nil)
	_ caddy.Provisioner           = (*NomadSDUpstreams)(nil)
	_ caddyfile.Unmarshaler       = (*NomadSDUpstreams)(nil)
	_ reverseproxy.UpstreamSource = (*NomadSDUpstreams)(nil)
)
