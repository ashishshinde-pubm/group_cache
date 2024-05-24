package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/mailgun/groupcache/v2"
)

// Placeholder for the database fetching logic
func fetchFromDatabase(key, selfAddr string) (string, error) {

	req, _ := http.NewRequest("GET", "http://localhost:5001/"+key, nil)
	req.Header.Set("Source", selfAddr)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error fetching from database: %w", err)
	}
	defer resp.Body.Close()

	value, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	fmt.Println("Fetched value:", string(value))
	return string(value), nil
}

func getGroupCachePool(selfAddr string) *groupcache.HTTPPool {
	return groupcache.NewHTTPPoolOpts(selfAddr, &groupcache.HTTPPoolOptions{Replicas: 2,
		Transport: func(ctx context.Context) http.RoundTripper {
			ctx, cancel := context.WithTimeout(ctx, time.Second*10)
			defer cancel()
			return &http.Transport{
				ResponseHeaderTimeout: 10 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
			}
		},
	})
}

func getGroupCache(selfAddr string) *groupcache.Group {
	// Create a new group cache with a max cache size of 3MB
	return groupcache.NewGroup("myGroup", 3000000, groupcache.GetterFunc(
		func(ctx context.Context, id string, dest groupcache.Sink) error {

			// Returns a protobuf struct `User`
			user, err := fetchFromDatabase(id, selfAddr)
			if err != nil {
				return err
			}

			// Set the user in the groupcache to expire after 5 minutes
			return dest.SetString(user, time.Time{})
		},
	))
}

func main() {

	selfIP := "localhost"
	serverPort := os.Getenv("PORT")
	if serverPort == "" {
		serverPort = "7001"
	}
	selfAddr := fmt.Sprintf("http://%s:%s", selfIP, serverPort)
	peerAddrs := []string{
		"http://localhost:7001",
		"http://localhost:7002",
	}
	log.Printf("Starting server at %s with peers %v", selfAddr, peerAddrs)

	// Initialize groupcache pool
	pool := getGroupCachePool(selfAddr)
	// Set peers
	pool.Set(peerAddrs...)
	// Create a new group cache with a max cache size of 3MB
	group := getGroupCache(selfAddr)

	// Logging handler for /_groupcache/ endpoint
	http.HandleFunc("/_groupcache/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s", r.URL)
		pool.ServeHTTP(w, r)
	})

	// HTTP handler to get values from the cache
	http.HandleFunc("/get/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len("/get/"):]
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		var data string
		err := group.Get(ctx, key, groupcache.StringSink(&data))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, data)
	})

	httpAddr := fmt.Sprintf(":%s", serverPort)

	log.Printf("Server running on %s", httpAddr)
	log.Fatal(http.ListenAndServe(httpAddr, nil))
}
