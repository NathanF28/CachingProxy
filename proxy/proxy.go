package proxy

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"The_Elir.net/cache"
)

type ProxyObject struct {
	Origin string
	Cache  map[string]*cache.CacheObject
	Mutex  sync.RWMutex // for parallel reads but single write at a time
}

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

func (p *ProxyObject) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//should be based on method and url
	key := r.Method + ":" + r.URL.String()

	p.Mutex.RLock()
	value, exists := p.Cache[key]
	if exists {
		p.Mutex.RUnlock()
		RespondWithHeaders(w, value.Response, value.ResponseBody, "HIT", key)
		return
	}
	p.Mutex.RUnlock()
	fmt.Printf("Cache Not Present for key : %s \n", key)
	originUrl := p.Origin + r.URL.String()
	res, err := http.Get(originUrl)
	if err != nil {
		http.Error(w, "Error forwarding Request", 504)
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

func RespondWithHeaders(w http.ResponseWriter, response *http.Response, body []byte, cacheStatus, key string) {
	fmt.Printf("Cache : %s %s", cacheStatus, key)
	w.Header().Set("X-Cache", cacheStatus)
	w.WriteHeader(response.StatusCode)
	for k, v := range response.Header {
		w.Header()[k] = v
	}
	w.Write(body)
}
