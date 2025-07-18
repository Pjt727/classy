package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"slices"
)

const (
	decreaseFactor          = 0.5
	increaseFactor          = 0.1
	failureBurstTimeSeconds = 3
)

// there are probably a lot of improvements that can be made to
//
//	the algorithm
type AdaptiveRateLimiter struct {
	mu            sync.Mutex
	startingLimit rate.Limit
	// TODO: add a longer time interval which which slowly resets this value
	//    when there are now more failures
	newStartingLimit rate.Limit
	startingBurst    int
	currentLimit     rate.Limit
	maxLimit         rate.Limit
	limiter          *rate.Limiter
	// try to prevent bursts of errors increasing the newStartingLimit
	//  by so much
	lastFailureGroup time.Time
}

func (a *AdaptiveRateLimiter) Fail() {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	newLimit := max(rate.Limit(float64(a.currentLimit)*(1-decreaseFactor)), 1)

	if now.After(a.lastFailureGroup) {
		// we've failed making requests here so assume that the rate limit of the server is higher
		a.newStartingLimit = newLimit
		a.lastFailureGroup = time.Now().Add(failureBurstTimeSeconds * time.Second)
	}

	a.currentLimit = newLimit
	a.limiter.SetLimit(a.currentLimit)
}

func (a *AdaptiveRateLimiter) Succeed() {
	a.mu.Lock()
	defer a.mu.Unlock()

	newLimit := min(rate.Limit(float64(a.currentLimit)*(1+increaseFactor)), a.newStartingLimit)
	a.currentLimit = newLimit
	a.limiter.SetLimit(a.currentLimit)
}

func (a *AdaptiveRateLimiter) Wait(ctx context.Context) error {
	return a.limiter.Wait(ctx)
}

func NewAdaptiveRateLimiter(startingLimit rate.Limit, startingBurst int) *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		startingLimit:    startingLimit,
		startingBurst:    startingBurst,
		limiter:          rate.NewLimiter(startingLimit, startingBurst),
		mu:               sync.Mutex{},
		currentLimit:     0,
		maxLimit:         startingLimit,
		newStartingLimit: startingLimit,
		lastFailureGroup: time.Now(),
	}
}

type RateLimiter interface {
	Succeed()
	Fail()
	Wait(context.Context) error
}

type RetryRequester struct {
	rateLimiter *RateLimiter
	maxRetries  uint
}

// NewHTTPHelper creates a new HTTPHelper with the given options.
func NewRetryRequest(retryCounter uint, rateLimiter RateLimiter) *RetryRequester {
	helper := &RetryRequester{
		maxRetries:  retryCounter,
		rateLimiter: &rateLimiter,
	}

	return helper
}

func dumpRequestInfo(req *http.Request) {
	for key, values := range req.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", key, value)
		}
	}
}

func (r *RetryRequester) Do(ctx context.Context, logger *log.Entry, client *http.Client, req *http.Request) (*http.Response, error) {
	/// creates a deep clone of the request by copying the body reader
	var (
		resp *http.Response
		err  error
	)
	retriesLeft := r.maxRetries

	for r.maxRetries > 0 {

		err = (*r.rateLimiter).Wait(ctx)
		if err != nil {
			return nil, err
		}

		reqCopy, _ := deepCopyRequest(req)

		resp, err = client.Do(reqCopy)

		if err != nil {
			logger.Errorf("Failed %s request to `%s`: %s", req.Method, req.URL.String(), err)
			retriesLeft -= 1
			continue
		}
		if resp.StatusCode >= 400 {
			logger.Warnf(
				"Status error %d for %s request to `%s`",
				resp.StatusCode,
				req.Method,
				req.URL.String(),
			)
			retriesLeft -= 1
			continue
		}
		if retriesLeft < r.maxRetries {

			logger.Warnf(
				"Retried %s request to `%s`",
				req.Method,
				req.URL.String(),
			)
		}
		break
	}
	if retriesLeft == 0 || resp == nil {
		return nil, fmt.Errorf("Failed %s request to `%s` after %d tries",
			req.Method, req.URL.String(), r.maxRetries)
	}

	return resp, nil
}

func deepCopyRequest(r *http.Request) (*http.Request, error) {
	r2 := new(http.Request)
	*r2 = *r

	if r.Body != nil {
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(r.Body); err != nil {
			return nil, err
		}
		r.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
		r2.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
	}

	if r.URL != nil {
		urlCopy := *r.URL
		r2.URL = &urlCopy
	}
	if r.Header != nil {
		headerCopy := make(http.Header)
		for k, v := range r.Header {
			headerCopy[k] = slices.Clone(v)
		}
		r2.Header = headerCopy
	}

	return r2, nil
}
