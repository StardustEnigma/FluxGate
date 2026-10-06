package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
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

	totalRequestFlag := flag.Int(
		"requests",
		1000,
		"Total number of requests",
	)

	concurrencyFlag := flag.Int(
		"concurrency",
		100,
		"Number of concurrent workers",
	)

	targetURL := flag.String(
		"url",
		"http://localhost:8081/rate-limit",
		"Target rate-limiting endpoint URL",
	)

	flag.Parse()

	if *totalRequestFlag <= 0 {
		log.Fatal("total requests must be greater than 0")
	}

	if *concurrencyFlag <= 0 {
		log.Fatal("concurrency must be greater than 0")
	}

	if strings.TrimSpace(*targetURL) == "" {
		log.Fatal("target URL must not be empty")
	}

	totalRequest := *totalRequestFlag
	concurrency := *concurrencyFlag

	// Fix #2 – tune the HTTP transport to match the benchmark's concurrency level.
	//
	// Go's http.DefaultTransport sets MaxIdleConnsPerHost = 2, which means that
	// after each burst of 200 concurrent requests the transport tears down 198
	// keep-alive connections. The next burst must re-dial fresh TCP connections,
	// paying a kernel SYN/SYN-ACK round-trip each time. Inside Docker bridge
	// networking this alone can add 1-5 ms per connection and, because all 200
	// goroutines re-dial simultaneously, causes a thundering-herd that manifests
	// as p95/p99 spikes.
	transport := &http.Transport{
		MaxIdleConnsPerHost: concurrency + 50, // retain enough idle conns for the whole worker pool
		MaxConnsPerHost:     concurrency + 50,
		DisableKeepAlives:   false,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   1 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	// One shared HTTP client for all workers.
	// Its underlying transport reuses connections across workers.
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	}

	latencies := make([]time.Duration, totalRequest)
	jobs := make(chan int)

	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for requestID := range jobs {
				requestStart := time.Now()

				clientID := fmt.Sprintf("client-%d", requestID%1000)
				body := fmt.Sprintf(`{"clientId":"%s"}`, clientID)

				req, err := http.NewRequest(
					http.MethodPost,
					*targetURL,
					strings.NewReader(body),
				)

				if err != nil {
					latencies[requestID] = time.Since(requestStart)
					errors.Add(1)
					continue
				}

				req.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(req)
				latencies[requestID] = time.Since(requestStart)

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

	duration := time.Since(start)

	accounted := allowed.Load() + rejected.Load() + errors.Load()
	accountingOK := accounted == int64(totalRequest)

	var totalLatency time.Duration

	for _, latency := range latencies {
		totalLatency += latency
	}

	avgLatency := totalLatency / time.Duration(totalRequest)
	p50 := percentile(latencies, 50)
	p95 := percentile(latencies, 95)
	p99 := percentile(latencies, 99)

	fmt.Println()
	fmt.Println("========== FluxGate Load Test ==========")
	fmt.Println("Target URL:", *targetURL)
	fmt.Println("Total Requests:", totalRequest)
	fmt.Println("Concurrency:", concurrency)
	fmt.Println("Allowed:", allowed.Load())
	fmt.Println("Rejected:", rejected.Load())
	fmt.Println("Errors:", errors.Load())
	fmt.Println("Accounted:", accounted)
	fmt.Println("Total Duration:", duration)

	fmt.Printf(
		"Requests/sec: %.2f\n",
		float64(totalRequest)/duration.Seconds(),
	)

	fmt.Println()
	fmt.Println("Latency (until response headers):")
	fmt.Println("Average:", avgLatency)
	fmt.Println("p50:", p50)
	fmt.Println("p95:", p95)
	fmt.Println("p99:", p99)

	if accountingOK {
		fmt.Println("\nAccounting Pass")
	} else {
		fmt.Println("\nAccounting failed")
		os.Exit(1)
	}
}
