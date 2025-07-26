package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"

	"golang.org/x/time/rate"
)

const (
	decreaseFactor = 0.8 // Reduce aggressively on failure
	increaseFactor = 0.2 // Increase conservatively on success
	minLimit       = 1   // Minimum requests per second
)

type AdaptiveRateLimiter struct {
	mu          sync.Mutex
	limit       rate.Limit
	burst       int
	limiter     *rate.Limiter
	maxIncrease rate.Limit
}

func (a *AdaptiveRateLimiter) Fail() {
	a.mu.Lock()
	defer a.mu.Unlock()

	newLimit := max(rate.Limit(float64(a.limit)*(1-decreaseFactor)), minLimit)
	a.setLimit(newLimit)
}

func (a *AdaptiveRateLimiter) Succeed() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Increase limit more conservatively, up to maxIncrease
	newLimit := min(rate.Limit(float64(a.limit)*(1+increaseFactor)), a.limit+a.maxIncrease)

	a.setLimit(newLimit)
}

func (a *AdaptiveRateLimiter) Wait(ctx context.Context) error {
	return a.limiter.Wait(ctx)
}

func (a *AdaptiveRateLimiter) Pause(ctx context.Context) error {
	a.limiter.SetLimit(0)
	return a.limiter.Wait(ctx)
}

func (a *AdaptiveRateLimiter) Cotinue(ctx context.Context) error {
	a.limiter.SetLimit(a.limit)
	return a.limiter.Wait(ctx)
}

func (a *AdaptiveRateLimiter) setLimit(newLimit rate.Limit) {
	a.limit = newLimit
	a.limiter.SetLimit(a.limit)
}

func NewAdaptiveRateLimiter(startingLimit rate.Limit, startingBurst int, maxIncrease rate.Limit) *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		limit:       startingLimit,
		burst:       startingBurst,
		limiter:     rate.NewLimiter(startingLimit, startingBurst),
		mu:          sync.Mutex{},
		maxIncrease: maxIncrease,
	}
}

type RateLimiter interface {
	Succeed()
	Fail()
	Wait(context.Context) error
}

type rateLimitedRoundTripper struct {
	transport http.RoundTripper
	limiter   RateLimiter
}

func (rt *rateLimitedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := rt.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}

	resp, err := rt.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		rt.limiter.Fail()
	} else {
		rt.limiter.Succeed()
	}

	return resp, nil
}

func AddRateLimiter(client *http.Client, limiter *RateLimiter) {
	rt := &rateLimitedRoundTripper{
		limiter: *limiter,
	}
	if client.Transport == nil {
		rt.transport = http.DefaultTransport
	} else {
		rt.transport = client.Transport
	}
	client.Transport = rt
}

type loggerRoundTripper struct {
	logger    slog.Logger
	transport http.RoundTripper
	requestID int32
}

func (rt *loggerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if !rt.logger.Enabled(req.Context(), LevelHttpReport.Level()) {
		return rt.transport.RoundTrip(req)
	}

	// it might be hard to distinguish requests from the same term collection from each other
	// when they are happening in parralel
	currentID := atomic.AddInt32(&rt.requestID, 1)

	rt.logger.Log(req.Context(), LevelHttpReport, "outgoing request", "method", req.Method, "url", req.URL.String(), "id", currentID)

	resp, err := rt.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	rt.logger.Log(req.Context(), LevelHttpReport, "response received", "status", resp.Status, "url", req.URL.String(), "id", currentID)

	return resp, nil
}

func AddHttpReporting(client *http.Client, logger slog.Logger) {
	rt := &loggerRoundTripper{
		logger:    logger,
		requestID: 0,
	}
	if client.Transport == nil {
		rt.transport = http.DefaultTransport
	} else {
		rt.transport = client.Transport
	}
	client.Transport = rt
}

// shorthand to check if a response is within 200-299
func IsOk(r *http.Response) bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// returns a ErrTemporaryNetworkFailure wrapped error of either
// the respErr if not nill or status code if non "Ok"
func RespOrStatusErr(r *http.Response, respErr error) error {
	if respErr != nil {
		return errors.Join(ErrTemporaryNetworkFailure, respErr)
	}
	if !IsOk(r) {
		return fmt.Errorf(
			"%w Got status code %d",
			ErrTemporaryNetworkFailure,
			r.StatusCode,
		)
	}
	return nil
}
