package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"golang.org/x/sync/singleflight"

	"The_Elir.net/cache"
)

type ProxyObject struct {
	Origin string
	Cache  *lru.Cache
	Mutex  sync.RWMutex // for parallel reads but single write at a time
}

const DefaultTTL = 5 * time.Minute

var group singleflight.Group

func NewProxy(origin string) *ProxyObject {
	cache, _ := lru.New(1000)
	return &ProxyObject{
		Origin: origin,
		Cache:  cache,
	}
}

func (p *ProxyObject) ClearCache() {
	p.Mutex.Lock()
	p.Cache.Purge()
	p.Mutex.Unlock()
	fmt.Println("Cache cleared successfully")
}

func (p *ProxyObject) proxyPassThrough(w http.ResponseWriter, r *http.Request) {
	originUrl := p.Origin + r.URL.String()
	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Get(originUrl)
	if err != nil {
		http.Error(w, "Error forwarding Request", http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		http.Error(w, "Error reading origin body", http.StatusInternalServerError)
		return
	}
	RespondWithHeaders(w, res, body, "BYPASS", r.URL.String())
}

func (p *ProxyObject) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only cache GET requests
	if r.Method != http.MethodGet {
		p.proxyPassThrough(w, r)
		return
	}

	key := r.Method + ":" + r.URL.String()
	cc := r.Header.Get("Cache-Control")
	// Honor Client preferences
	if strings.Contains(cc, "no-store") || strings.Contains(cc, "no-cache") {
		fmt.Println("Client requested no-cache")
		p.proxyPassThrough(w, r)
		return
	}

	// Try cache (use lru API)
	raw, exists := p.Cache.Get(key)
	if exists {
		if obj, ok := raw.(*cache.CacheObject); ok {
			if time.Since(obj.Created) <= DefaultTTL {
				RespondWithHeaders(w, obj.Response, obj.ResponseBody, "HIT", key)
				return
			}
			// expired, remove
			p.Mutex.Lock()
			p.Cache.Remove(key)
			p.Mutex.Unlock()
		}
	}

	// Singleflight prevents duplicate origin requests
	result, err, _ := group.Do(key, func() (interface{}, error) {
		originUrl := p.Origin + r.URL.String()
		client := &http.Client{Timeout: 10 * time.Second}
		res, err := client.Get(originUrl)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		obj := &cache.CacheObject{
			Response:     res,
			ResponseBody: body,
			Created:      time.Now(),
		}

		p.Mutex.Lock()
		p.Cache.Add(key, obj)
		p.Mutex.Unlock()

		return obj, nil
	})

	if err != nil {
		http.Error(w, "Error forwarding Request", http.StatusBadGateway)
		return
	}

	cachedObj, ok := result.(*cache.CacheObject)
	if !ok || cachedObj == nil {
		http.Error(w, "Error caching response", http.StatusInternalServerError)
		return
	}

	RespondWithHeaders(w, cachedObj.Response, cachedObj.ResponseBody, "MISS", key)
}

func RespondWithHeaders(w http.ResponseWriter, response *http.Response, body []byte, cacheStatus, key string) {
	fmt.Printf("Cache : %s %s\n", cacheStatus, key)

	// hop-by-hop headers that should not be forwarded
	hopByHop := map[string]struct{}{
		"Connection":          {},
		"Keep-Alive":          {},
		"Proxy-Authenticate":  {},
		"Proxy-Authorization": {},
		"TE":                  {},
		"Trailers":            {},
		"Transfer-Encoding":   {},
		"Upgrade":             {},
	}

	// copy headers first (skip hop-by-hop)
	for k, vals := range response.Header {
		if _, skip := hopByHop[k]; skip {
			continue
		}
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}

	// ensure X-Cache is set/overwrites origin
	w.Header().Set("X-Cache", cacheStatus)

	// write status and body
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(body)
}
