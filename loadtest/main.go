package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
)

const (
	url = "http://localhost:8080/rate-limit"
)

func percentile(latency []time.Duration, p float64) time.Duration {
	if len(latency) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(latency))
	copy(sorted, latency)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	index := int(math.Ceil((p/100)*float64(len(sorted)))) - 1

	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

func main() {

	var allowed atomic.Int64
	var rejected atomic.Int64
	var errors atomic.Int64

	TotalRequest := flag.Int("requests", 1000, "Total Requests")
	Concurrency := flag.Int("concurrency", 100, "concurrent workers")
	flag.Parse()

	if *TotalRequest <= 0 {
		log.Fatal("total request must be greater than 0")
	}

	if *Concurrency <= 0 {
		log.Fatal("workers must be greater than 0")
	}
	totalRequest := *TotalRequest
	concurrency := *Concurrency

	latencies := make([]time.Duration, totalRequest)

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

			for requestId := range jobs {

				requestStart := time.Now()

				req, err := http.NewRequest(
					http.MethodPost,
					url,
					strings.NewReader(`{"clientId":"client123"}`),
				)
				if err != nil {
					errors.Add(1)
					continue
				}
				req.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(req)
				latencies[requestId] = time.Since(requestStart)

				if err != nil {
					errors.Add(1)
					continue
				}
				if (resp.StatusCode < 200 || resp.StatusCode >= 300) &&
					resp.StatusCode != http.StatusTooManyRequests {
					resp.Body.Close()
					errors.Add(1)
					continue
				}
				var result model.RateLimitingResponse

				err = json.NewDecoder(resp.Body).Decode(&result)
				resp.Body.Close()

				if err != nil {
					errors.Add(1)
					continue
				}

				if result.Allowed {
					allowed.Add(1)
				} else {
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
	accounted := allowed.Load() + rejected.Load() + errors.Load()
	accountingOk := accounted == int64(totalRequest)

	var totalLatency time.Duration
	duration := time.Since(start)
	for _, latency := range latencies {
		totalLatency += latency
	}

	avgLatency := totalLatency / time.Duration(totalRequest)
	p50 := percentile(latencies, 50)
	p95 := percentile(latencies, 95)
	p99 := percentile(latencies, 99)

	fmt.Println()
	fmt.Println("========== FluxGate Load Test ==========")
	fmt.Println("Total Requests:", totalRequest)
	fmt.Println("Concurrency:", concurrency)
	fmt.Println("Allowed:", allowed.Load())
	fmt.Println("Rejected:", rejected.Load())
	fmt.Println("Errors:", errors.Load())
	fmt.Println("Accounted:", accounted)
	fmt.Println("Total Duration:", duration)

	fmt.Printf("Requests/sec: %.2f\n",
		float64(totalRequest)/float64(duration.Seconds()),
	)

	fmt.Println()
	fmt.Println("Latency:")
	fmt.Println("Average:", avgLatency)
	fmt.Println("p50:", p50)
	fmt.Println("p95:", p95)
	fmt.Println("p99:", p99)
	if accountingOk{
		fmt.Println("\nAccounting Pass")
	}else{
		fmt.Println("\nAccounting failed")
		os.Exit(1)
	}
}
