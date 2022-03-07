package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"

	shell "github.com/ipfs/go-ipfs-api"
)

type CacheEntry struct {
	Cid     string
	Allowed bool
}

// NewProxy takes target host and creates a reverse proxy
func NewProxy(targetHost string) (*httputil.ReverseProxy, error) {
	url, err := url.Parse(targetHost)
	if err != nil {
		return nil, err
	}

	return httputil.NewSingleHostReverseProxy(url), nil
}

// ProxyRequestHandler handles the http request using proxy
func ProxyRequestHandler(proxy *httputil.ReverseProxy, sh *shell.Shell) func(http.ResponseWriter, *http.Request) {
	totalAllowedSize, err := strconv.ParseInt(os.Getenv("MAX_SIZE_MB"), 10, 64)
	totalAllowedSize = totalAllowedSize * 1024 * 1024
	cache := []CacheEntry{}

	if err != nil {
		log.Println("Error:", err)
		log.Panicln("Failed to parse MAX_SIZE_MB")
		panic(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		splitted := strings.Split(path, "/")
		if len(splitted) > 1 {
			if splitted[1] == "ipfs" {
				if len(splitted) > 2 {
					// log.Println("IPFS request: " + path)
					// log.Println("CID: " + splitted[2])
					cid := splitted[2]

					// check if the cid is in the cache
					for _, entry := range cache {
						if entry.Cid == cid {
							if entry.Allowed {
								// log.Println("CID is allowed")
								proxy.ServeHTTP(w, r)
								return
							} else {
								log.Println("CID is not allowed by cache " + cid)
								w.WriteHeader(http.StatusForbidden)
								return
							}
						}
					}

					_, size, err := getBlockSizeRecursive(cid, sh)
					if err != nil {
						log.Println(err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					if int64(size) > totalAllowedSize {
						log.Println("Total size of " + strconv.Itoa(size) + " exceeded for CID: " + cid)
						w.WriteHeader(http.StatusForbidden)
						cache = append(cache, CacheEntry{Cid: cid, Allowed: false})
						return
					}
					cache = append(cache, CacheEntry{Cid: cid, Allowed: true})
					// log.Println("Total size: " + strconv.Itoa(size))

					if len(cache) > 2000 {
						cache = cache[1:]
					}
				}
			}
		}

		proxy.ServeHTTP(w, r)
	}
}

func getBlockSizeRecursive(block string, sh *shell.Shell) (string, int, error) {
	refsChan, err := sh.Refs(block, true)
	if err != nil {
		log.Println(err)
		return "", 0, err
	}
	totalSize := 0
	for {
		select {
		case ref, ok := <-refsChan:
			if !ok {
				return block, totalSize, nil
			}
			// log.Println("New ref: " + ref)
			_, size, err := sh.BlockStat(ref)
			if err != nil {
				log.Println(err)
			}
			totalSize += size
			break
		}
	}
}

func main() {
	// create new IPFS shell
	sh := shell.NewShell("localhost:5001")
	backend := os.Getenv("BACKEND")
	port := os.Getenv("PORT")
	// initialize a reverse proxy and pass the actual backend server url here
	proxy, err := NewProxy(backend)
	if err != nil {
		panic(err)
	}

	// handle all requests to your server using the proxy
	http.HandleFunc("/", ProxyRequestHandler(proxy, sh))
	log.Println("Proxy server starting. Listening on " + port + ". Redirecting to " + backend)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
