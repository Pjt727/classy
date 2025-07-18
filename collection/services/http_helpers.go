package services

import (
	"context"
	"net/http"
	"sync"

	"github.com/hashicorp/go-retryablehttp"
	log "github.com/sirupsen/logrus"
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

func addRateLimiter(client *http.Client, limiter *RateLimiter) {
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

// TODO: fix logs when this gets merged https://github.com/hashicorp/go-retryablehttp/pull/231
func retryLog(l retryablehttp.Logger, req *http.Request, retryCount int) {
	if retryCount == 0 {
		return
	}
	switch v := l.(type) {
	case *LogrusLogger:
		v.Get().Warnf("try %d for %s: %s", retryCount, req.Method, req.URL)
	default:
		log.Warnf("FAILED TO TYPE LOGGER: try %d for %s: %s", retryCount, req.Method, req.URL)
		log.Warnf("FAILED COOKIES: %s", req.Cookies())
	}
}

func responseLog(l retryablehttp.Logger, res *http.Response) {
	switch v := l.(type) {
	case LogrusLogger:
		v.Get().Tracef("%s: %s", res.Status, res.Request.URL)
	default:
		log.Tracef("(FAILED TO TYPE LOGGER) %s: %s", res.Status, res.Request.URL)
	}
}

func NewRetryClientWithLimiter(logger *log.Entry, limiter *RateLimiter) *http.Client {
	client := retryablehttp.NewClient()
	var l retryablehttp.LeveledLogger = LogrusLogger{Entry: logger}
	client.Logger = l

	client.ResponseLogHook = responseLog
	client.RequestLogHook = retryLog
	stdClient := client.StandardClient()
	addRateLimiter(stdClient, limiter)
	return stdClient
}

// wrapper make the logrus logger a LeveledLogger
type LogrusLogger struct {
	Entry *log.Entry
}

func (l LogrusLogger) Error(msg string, keysAndValues ...any) {
	l.Entry.Errorln(msg, keysAndValues)
}

func (l LogrusLogger) Info(msg string, keysAndValues ...any) {
	l.Entry.Infoln(msg, keysAndValues)
}

func (l LogrusLogger) Debug(msg string, keysAndValues ...any) {
	l.Entry.Debugln(msg, keysAndValues)
}

func (l LogrusLogger) Warn(msg string, keysAndValues ...any) {
	l.Entry.Warnln(msg, keysAndValues)
}

func (l LogrusLogger) Printf(msg string, keysAndValues ...any) {
	l.Entry.Printf(msg)
}

func (l LogrusLogger) Get() *log.Entry {
	return l.Entry
}
