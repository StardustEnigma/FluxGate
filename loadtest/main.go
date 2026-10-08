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

type BenchmarkResult struct {
	Metadata  BenchmarkMetadata `json:"metadata"`
	Benchmark BenchmarkConfig   `json:"benchmark"`
	Results   BenchmarkMetrics  `json:"results"`
}

type BenchmarkMetadata struct {
	Algorithm string `json:"algorithm"`
	Commit    string `json:"commit,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

type BenchmarkConfig struct {
	TargetURL   string `json:"target_url"`
	Requests    int    `json:"requests"`
	Concurrency int    `json:"concurrency"`

	Capacity   float64 `json:"capacity,omitempty"`
	RefillRate float64 `json:"refill_rate,omitempty"`
	Limit      int     `json:"limit,omitempty"`
	Window     string  `json:"window,omitempty"`
}

type BenchmarkMetrics struct {
	Allowed       int64         `json:"allowed"`
	Rejected      int64         `json:"rejected"`
	Errors        int64         `json:"errors"`
	Duration      string        `json:"duration"`
	ThroughputRPS float64       `json:"throughput_rps"`
	Latency       LatencyResult `json:"latency"`
}

type LatencyResult struct {
	Average string `json:"average"`
	P50     string `json:"p50"`
	P95     string `json:"p95"`
	P99     string `json:"p99"`
}

func main() {
	jsonOutputFlag := flag.Bool(
		"json",
		false,
		"Output benchmark result as json",
	)

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

	algorithm := flag.String(
		"algorithm",
		"",
		"Rate limiting algorithm",
	)

	capacity := flag.Float64(
		"capacity",
		0,
		"Token Bucket capacity",
	)

	refillRate := flag.Float64(
		"refill-rate",
		0,
		"Token Bucket refill rate",
	)

	limit := flag.Int(
		"limit",
		0,
		"Sliding Window request limit",
	)

	window := flag.Duration(
		"window",
		0,
		"Sliding Window duration",
	)

	commit := flag.String(
		"commit",
		"",
		"Git commit SHA",
	)

	timestamp := flag.String(
		"timestamp",
		"",
		"Benchmark timestamp",
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

	if strings.TrimSpace(*algorithm) == "" {
		log.Fatal("algorithm must not be empty")
	}

	totalRequest := *totalRequestFlag
	concurrency := *concurrencyFlag

	// Tune the HTTP transport to match the benchmark's concurrency level.
	transport := &http.Transport{
		MaxIdleConnsPerHost: concurrency + 50,
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

	allowedCount := allowed.Load()
	rejectedCount := rejected.Load()
	errorCount := errors.Load()

	accounted := allowedCount + rejectedCount + errorCount
	accountingOK := accounted == int64(totalRequest)

	var totalLatency time.Duration

	for _, latency := range latencies {
		totalLatency += latency
	}

	avgLatency := totalLatency / time.Duration(totalRequest)

	p50 := percentile(latencies, 50)
	p95 := percentile(latencies, 95)
	p99 := percentile(latencies, 99)

	throughput := float64(totalRequest) / duration.Seconds()

	result := BenchmarkResult{
		Metadata: BenchmarkMetadata{
			Algorithm: *algorithm,
			Commit:    *commit,
			Timestamp: *timestamp,
		},

		Benchmark: BenchmarkConfig{
			TargetURL:   *targetURL,
			Requests:    totalRequest,
			Concurrency: concurrency,

			Capacity:   *capacity,
			RefillRate: *refillRate,
			Limit:      *limit,
			Window:     window.String(),
		},

		Results: BenchmarkMetrics{
			Allowed:       allowedCount,
			Rejected:      rejectedCount,
			Errors:        errorCount,
			Duration:      duration.String(),
			ThroughputRPS: throughput,

			Latency: LatencyResult{
				Average: avgLatency.String(),
				P50:     p50.String(),
				P95:     p95.String(),
				P99:     p99.String(),
			},
		},
	}

	if *jsonOutputFlag {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", " ")
		_ = encoder.Encode(result)
		return
	}

	fmt.Println()
	fmt.Println("========== FluxGate Load Test ==========")
	fmt.Println("Algorithm:", *algorithm)
	fmt.Println("Target URL:", *targetURL)
	fmt.Println("Total Requests:", totalRequest)
	fmt.Println("Concurrency:", concurrency)
	fmt.Println("Allowed:", allowedCount)
	fmt.Println("Rejected:", rejectedCount)
	fmt.Println("Errors:", errorCount)
	fmt.Println("Accounted:", accounted)
	fmt.Println("Total Duration:", duration)

	fmt.Printf(
		"Requests/sec: %.2f\n",
		throughput,
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
