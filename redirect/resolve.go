package redirect

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"reeesolve/config"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Resolver struct {
	c http.Client

	cacheLock    sync.RWMutex
	cache        map[string]cacheItem
	cacheInserts chan cacheInsert

	cachePurgeTime time.Duration

	allowedDomains config.AllowedDomainsMapping
}

type cacheItem struct {
	fullURL   string
	lastCheck time.Time
}

type cacheInsert struct {
	shortURL string
	cacheItem
}

var (
	ErrInvalidScheme   = errors.New("invalid URL scheme")
	ErrForbiddenDomain = errors.New("forbidden domain")
	ErrResolve         = errors.New("error while resolving")
)

func NewResolver(settings config.Settings) (r *Resolver) {
	r = &Resolver{
		c: http.Client{
			Timeout: settings.ResolveTimeout,
		},

		cacheLock:    sync.RWMutex{},
		cache:        make(map[string]cacheItem),
		cacheInserts: make(chan cacheInsert, 50),

		cachePurgeTime: settings.CacheDuration,

		allowedDomains: settings.AllowedDomains,
	}

	go r.run()

	return
}

func (r *Resolver) run() {
	for item := range r.cacheInserts {
		r.cacheLock.Lock()
		r.cache[item.shortURL] = item.cacheItem
		r.cacheLock.Unlock()
	}
}

func (r *Resolver) Resolve(urlStr string, reqURL *url.URL) (fullURL string, err error) {
	// First of all, we try to parse & remove the URL hash
	u, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return
	}

	// Remove the fragment, as it's not sent to the server anyways.
	// This enables better caching
	if u.Fragment != "" {
		u.Fragment = ""
		u.RawFragment = ""
		urlStr = u.String()
	}

	// At first, we check the cache
	cached, ok := r.cached(urlStr)
	if ok {
		return cached.fullURL, nil
	}

	// Make sure we have a valid scheme
	if !(u.Scheme == "http" || u.Scheme == "https") {
		err = ErrInvalidScheme
		return
	}

	// Deny forbidden domains, and also block requests to ourselves. This should prevent loops
	if !r.allowedDomains.Contains(u.Host) || reqURL.Host == u.Host {
		err = ErrForbiddenDomain
		return
	}

	resolved, err := r.resolveNoCache(urlStr)
	if err != nil {
		if !errors.Is(err, ErrResolve) {
			log.Println("resolve error:", err.Error())
			err = ErrResolve
		}
		return
	}

	// Make sure it will be cached at some point
	r.cacheInserts <- cacheInsert{
		shortURL: urlStr,
		cacheItem: cacheItem{
			fullURL:   resolved,
			lastCheck: time.Now(),
		},
	}

	return resolved, nil
}

func (r *Resolver) cached(urlStr string) (item cacheItem, ok bool) {
	r.cacheLock.RLock()
	i, ok := r.cache[urlStr]
	r.cacheLock.RUnlock()

	if ok && time.Since(i.lastCheck) > r.cachePurgeTime {
		return
	}

	return i, ok
}

func (r *Resolver) resolveNoCache(url string) (fullURL string, err error) {
	resp, err := r.c.Head(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		err = ErrResolve
		return
	}

	fullURL = resp.Request.URL.String()

	return
}

func (r *Resolver) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	urlStr := req.URL.Query().Get("u")

	fu, err := r.Resolve(urlStr, req.URL)
	if err != nil {
		// All errors returned from Resolve are defined above and don't leak information

		var statusCode = http.StatusBadRequest
		if errors.Is(err, ErrResolve) {
			statusCode = http.StatusBadGateway
		} else if errors.Is(err, ErrForbiddenDomain) {
			statusCode = http.StatusForbidden
		}

		http.Error(rw, err.Error(), statusCode)
		return
	}

	// Cache the response for a certain amount of time
	if s := int(r.cachePurgeTime.Seconds()); s > 0 {
		rw.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(s))
	}

	// Depending on what format is requested, we encode different things

	switch strings.ToLower(req.URL.Query().Get("f")) {
	case "", "json":
		err = json.NewEncoder(rw).Encode(map[string]string{
			"url": fu,
		})
	case "redirect":
		rw.Header().Set("Location", fu)
		rw.WriteHeader(http.StatusFound)
		fallthrough
	case "txt":
		_, err = rw.Write([]byte(fu))
	default:
		http.Error(rw, "Invalid format string", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Println("[Error] ", err.Error())
	}
}
