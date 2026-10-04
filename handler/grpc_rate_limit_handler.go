package handler

import (
	"context"
	"log"
	"time"

	ratelimitv1 "github.com/StardustEnigma/FluxGate/gen/ratelimit/v1"
	"github.com/StardustEnigma/FluxGate/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCRateLimiterHandler struct {
	ratelimitv1.UnimplementedRateLimitServiceServer
	RateLimiter service.RateLimiter
}

func (h *GRPCRateLimiterHandler) CheckRateLimit(
	ctx context.Context,
	req *ratelimitv1.CheckRateLimitRequest,
) (*ratelimitv1.CheckRateLimitResponse, error) {
	if req == nil || req.GetClientId() == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	result, err := h.RateLimiter.RateLimit(
		ctx,
		req.GetClientId(),
		time.Now(),
	)
	if err != nil {
		log.Printf("gRPC rate limit failed: %v", err)
		return nil, status.Error(
			codes.Internal,
			"internal rate limiting error",
		)
	}
	return &ratelimitv1.CheckRateLimitResponse{
		Allowed:    result.Allowed,
		RetryAfter: result.RetryAfter,
	}, nil
}
