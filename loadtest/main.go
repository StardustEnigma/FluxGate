package main

import (
	
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
)

const (
	totalRequest = 1000
	concurrency  = 50
	url          = "http://localhost:8080/rate-limit"
)

func main() {

	var allowed atomic.Int64
	var rejected atomic.Int64

	start := time.Now()

	jobs := make(chan int)

	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := http.Client{
				Timeout: 5 * time.Second,
			}

			for range jobs {
				req, err := http.NewRequest(
					http.MethodPost,
					url,
					strings.NewReader(`{"clientID":"client123"}`),
				)
				if err != nil {
					continue
				}
				req.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(req)

				if err != nil {
					continue
				}
				var result model.RateLimitingResponse

				err = json.NewDecoder(resp.Body).Decode(&result)
				resp.Body.Close()

				if err != nil {
					continue
				}

				if result.Allowed {
					allowed.Add(1)
				}else {
					rejected.Add(1)
				}
			}
		}()
	}

	for i := 0; i < totalRequest; i++ {
		jobs <- i
	}

	close(jobs)

	wg.Wait()

	duration := time.Since(start)
	fmt.Println("========== FluxGate Load Test ==========")
	fmt.Println("Total Requests:", totalRequest)
	fmt.Println("Concurrency:", concurrency)
	fmt.Println("Allowed:", allowed.Load())
	fmt.Println("Rejected:", rejected.Load())
	fmt.Println("Duration:", duration)
	fmt.Printf("Requests/sec: %.2f\n",
		float64(totalRequest)/duration.Seconds(),
	)
}
