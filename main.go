package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	ratelimitv1 "github.com/StardustEnigma/FluxGate/gen/ratelimit/v1"
	"github.com/StardustEnigma/FluxGate/handler"
	"github.com/StardustEnigma/FluxGate/metrics"
	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/repository"
	"github.com/StardustEnigma/FluxGate/service"
	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found; using environment variables")
	}
	store := repository.NewRedisStore()
	if err := store.Ping(context.Background()); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Redis Connected")

	var limiter service.RateLimiter
	algorithm := flag.String("algorithm", "", "Rate Limiting Algorithm")

	capacity := flag.Float64("capacity", 10, "Token Bucket Capacity")
	refillRate := flag.Float64("refill-rate", 2, "Token Bucket Refill Rate")

	limit := flag.Int("limit", 10, "Sliding Window Request Limit")
	timeWindow := flag.Duration("window", 60*time.Second, "Sliding Window")
	flag.Parse()
	if *algorithm == "" {
		log.Fatal("algorithm is required")
	}
	fmt.Println("algorithm =", *algorithm)

	switch *algorithm {
	case "token-bucket":
		if *capacity <= 0 {
			log.Fatal("capacity must be greater than 0")
		}
		if *refillRate <= 0 {
			log.Fatal("refill rate must be greater than 0")
		}
		tokenBucketpolicy := model.TokenBucketPolicy{
			Capacity:   *capacity,
			RefillRate: *refillRate,
		}
		limiter = service.NewTokenBucketLimiter(tokenBucketpolicy, store)

	case "sliding-window":
		if *limit <= 0 {
			log.Fatal("limit must be greater than 0")
		}
		if *timeWindow <= 0 {
			log.Fatal("window must be greater than 0")
		}
		slidingWindowPolicy := model.SlidingWindowPolicy{
			Limit:      *limit,
			TimeWindow: *timeWindow,
		}
		limiter = service.NewSlidingWindowLimiter(slidingWindowPolicy, store)

	default:
		log.Fatalf("unknown algorithm : %s (use token-bucket or sliding-window)", *algorithm)
	}

	appMetrics := metrics.NewMetrics()
	limiter = service.NewInstrumentedLimiter(limiter, appMetrics)

	// Fix #5 – pass appMetrics into the handler so it can record full HTTP time
	restHandler := &handler.RateLimiterHandler{
		RateLimiter: limiter,
		Metrics:     appMetrics,
	}

	router := chi.NewRouter()
	router.Post("/rate-limit", restHandler.RateLimit)

	// Fix #4 – add HTTP server timeouts.
	// Without these, slow or stalled clients hold goroutines open indefinitely,
	// exhausting the scheduler's thread pool and inflating latency for healthy
	// requests running concurrently.

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	metricsHandler := promhttp.HandlerFor(
		appMetrics.Resgistry,
		promhttp.HandlerOpts{},
	)
	router.Handle("/metrics", metricsHandler)
	serverMode := os.Getenv("SERVER_MODE")
	if serverMode == "" {
		serverMode = "both"
	}

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	grpcServer := grpc.NewServer()
	ratelimitv1.RegisterRateLimitServiceServer(
		grpcServer,
		&handler.GRPCRateLimiterHandler{
			RateLimiter: limiter,
		},
	)

	switch serverMode {
	case "rest":
		log.Printf("REST server listening on :%s", port)
		if err := httpServer.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			log.Fatalf("REST server failed: %v", err)
		}

	case "grpc":
		grpcListener, err := net.Listen("tcp", ":"+port)
		if err != nil {
			log.Fatalf("failed to listen for gRPC: %v", err)
		}
		log.Printf("gRPC server listening on :%s", port)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}

	case "both":
		grpcListener, err := net.Listen("tcp", ":9090")
		if err != nil {
			log.Fatalf("failed to listen for gRPC: %v", err)
		}

		go func() {
			log.Printf("REST server listening on :%s", port)
			if err := httpServer.ListenAndServe(); err != nil &&
				err != http.ErrServerClosed {
				log.Fatalf("REST server failed: %v", err)
			}
		}()

		log.Println("gRPC server listening on :9090")
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}

	default:
		log.Fatalf(
			"unknown SERVER_MODE %q (use rest, grpc, or both)",
			serverMode,
		)
	}

}
