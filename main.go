package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"The_Elir.net/proxy"
)

func main() {
	port := flag.Int("port", 0, "Port on which the caching proxy server will run.")
	origin := flag.String("origin", "", " URL of the server to which the requests will be forwarded.")
	clearCache := flag.Bool("clear-cache", false, "Clear the Cache")
	flag.Parse()
	//initalize new router to serveHTTP requests
	p := proxy.NewProxy("http://example.com")

	if *clearCache {
		fmt.Println("Clearing Cache")
		// call the method to clear the cache
	} else if *port != 0 && *origin != "" {
		// Register all routes to proxy(router)
		http.Handle("/", p)
		p.Origin = *origin
		addr := ":" + strconv.Itoa(*port)
		// Start the server
		err := http.ListenAndServe(addr, p)
		if err != nil {
			log.Fatalf("Unable to start server %v", err)
		}
	} else {
		// 4️⃣ Invalid usage
		fmt.Println("Error: You must provide --port and --origin to start the server, or --clear-cache to clear the cache.")
		flag.Usage()
	}
}
