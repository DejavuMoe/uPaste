package abuse

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	Create = iota
	Mutation
	Read
)

const (
	ClientIdleTTL = 10 * time.Minute
	MaxClients    = 16384
	UploadLimit   = 4
	DownloadLimit = 32
	SweepInterval = time.Minute
)

type Config struct {
	Trusted  []netip.Prefix
	Now      func() time.Time
	Max      int
	Disabled bool
}

type client struct {
	last     time.Time
	limiters [3]*rate.Limiter
}

type Control struct {
	trusted   []netip.Prefix
	now       func() time.Time
	max       int
	mu        sync.Mutex
	clients   map[string]*client
	global    *rate.Limiter
	uploads   chan struct{}
	downloads chan struct{}
	disabled  bool
	lastSweep time.Time
}

func New(config Config) *Control {
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.Max == 0 {
		config.Max = MaxClients
	}
	return &Control{
		trusted: config.Trusted, now: config.Now, max: config.Max, clients: make(map[string]*client),
		global:  rate.NewLimiter(rate.Every(time.Minute/1000), 200),
		uploads: make(chan struct{}, UploadLimit), downloads: make(chan struct{}, DownloadLimit), disabled: config.Disabled,
	}
}

func (control *Control) Allow(request *http.Request, kind int) bool {
	if control.disabled {
		return true
	}
	now := control.now()
	key := RateKey(ClientIP(request.RemoteAddr, request.Header.Get("X-Forwarded-For"), control.trusted))
	control.mu.Lock()
	defer control.mu.Unlock()
	if now.Sub(control.lastSweep) >= SweepInterval {
		control.sweep(now)
	}
	value := control.clients[key]
	if value == nil {
		if len(control.clients) >= control.max {
			control.sweep(now)
			if len(control.clients) >= control.max {
				return false
			}
		}
		value = &client{last: now, limiters: [3]*rate.Limiter{
			rate.NewLimiter(rate.Every(time.Minute/10), 5),
			rate.NewLimiter(rate.Every(time.Minute/30), 10),
			rate.NewLimiter(rate.Every(time.Minute/120), 60),
		}}
		control.clients[key] = value
	}
	value.last = now
	return control.global.AllowN(now, 1) && value.limiters[kind].AllowN(now, 1)
}

func (control *Control) sweep(now time.Time) {
	for key, value := range control.clients {
		if now.Sub(value.last) >= ClientIdleTTL {
			delete(control.clients, key)
		}
	}
	control.lastSweep = now
}

// ClientIP resolves the effective client identity using the same trusted-proxy
// rules as rate limiting. An invalid peer resolves to the zero address.
func (control *Control) ClientIP(request *http.Request) netip.Addr {
	if control == nil {
		return netip.Addr{}
	}
	return ClientIP(request.RemoteAddr, request.Header.Get("X-Forwarded-For"), control.trusted)
}

func (control *Control) TryUpload() bool {
	if control.disabled {
		return true
	}
	select {
	case control.uploads <- struct{}{}:
		return true
	default:
		return false
	}
}
func (control *Control) ReleaseUpload() {
	if !control.disabled {
		<-control.uploads
	}
}
func (control *Control) TryDownload() bool {
	if control.disabled {
		return true
	}
	select {
	case control.downloads <- struct{}{}:
		return true
	default:
		return false
	}
}
func (control *Control) ReleaseDownload() {
	if !control.disabled {
		<-control.downloads
	}
}

func ClientIP(remoteAddr, xff string, trusted []netip.Prefix) netip.Addr {
	peer, ok := parsePeer(remoteAddr)
	if !ok {
		return netip.Addr{}
	}
	if !isTrusted(peer, trusted) {
		return peer
	}
	var chain []netip.Addr
	for _, part := range strings.Split(xff, ",") {
		address, err := netip.ParseAddr(strings.TrimSpace(part))
		if err != nil {
			return peer
		}
		chain = append(chain, address.Unmap())
	}
	for index := len(chain) - 1; index >= 0; index-- {
		if !isTrusted(chain[index], trusted) {
			return chain[index]
		}
	}
	return peer
}

func RateKey(address netip.Addr) string {
	address = address.Unmap()
	if !address.IsValid() {
		return "invalid"
	}
	if address.Is4() {
		return address.String()
	}
	return netip.PrefixFrom(address, 64).Masked().String()
}

func parsePeer(value string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(value)
	if err != nil {
		host = value
	}
	address, err := netip.ParseAddr(host)
	return address.Unmap(), err == nil
}
func isTrusted(address netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
