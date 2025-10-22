package proxy

import (
	"fmt"
	"net/http"
	"sync"
)

type ProxyObject struct {
	Origin string
	Cache  map[string]string
	Mutex  sync.RWMutex // for parallel reads but single write at a time
}

func NewProxy(origin string) *ProxyObject {
	return &ProxyObject{
		Origin: origin,
		Cache:  make(map[string]string),
	}
}

func (p *ProxyObject) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//should be based on method and url
	key := r.Method + ":" + r.URL.String()

	p.Mutex.RLock()
	value, exists := p.Cache[key]
	if exists {
		p.Mutex.RUnlock()
		//respond with header
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

}
