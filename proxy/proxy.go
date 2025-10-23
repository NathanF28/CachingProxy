package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"The_Elir.net/cache"
)

type ProxyObject struct {
	Origin string
	Cache  map[string]*cache.CacheObject
	Mutex  sync.RWMutex // for parallel reads but single write at a time
}

const DefaultTTL = 5 * time.Minute

func NewProxy(origin string) *ProxyObject {
	return &ProxyObject{
		Origin: origin,
		Cache:  make(map[string]*cache.CacheObject),
	}
}

func (p *ProxyObject) ClearCache() {
	p.Mutex.Lock()
	p.Cache = make(map[string]*cache.CacheObject)
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
	//should be based on method and url
	key := r.Method + ":" + r.URL.String()
	// honor client side preferences
	cc := r.Header.Get("Cache-Control")
	if strings.Contains(cc, "no-store") || strings.Contains(cc, "no-cache") {
		//fetch straight from the server
		fmt.Println("Fetching directly from server because of client preferences")
		p.proxyPassThrough(w, r)
	} else {

		p.Mutex.RLock()
		value, exists := p.Cache[key]
		if exists {
			if time.Since(value.Created) <= DefaultTTL {
				p.Mutex.RUnlock()
				p.Mutex.RUnlock()
				RespondWithHeaders(w, value.Response, value.ResponseBody, "HIT", key)
				return
			}
			p.Mutex.RUnlock()
			p.Mutex.Lock()
			delete(p.Cache, key)
			p.Mutex.Unlock()
		}
		p.Mutex.RUnlock()
		fmt.Printf("Cache Not Present for key : %s \n", key)
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
			http.Error(w, "Error Forwarding Request Body", http.StatusInternalServerError)
			return
		}
		p.Mutex.Lock()
		p.Cache[key] = &cache.CacheObject{
			Response:     res,
			ResponseBody: body,
			Created:      time.Now(),
		}
		p.Mutex.Unlock()
		RespondWithHeaders(w, res, body, "MISS", key)
		// respond with header MISS
	}
}

func RespondWithHeaders(w http.ResponseWriter, response *http.Response, body []byte, cacheStatus, key string) {
	fmt.Printf("Cache : %s %s", cacheStatus, key)
	w.Header().Set("X-Cache", cacheStatus)
	w.WriteHeader(response.StatusCode)
	for k, v := range response.Header {
		w.Header()[k] = v
	}
	w.Write(body)
}
