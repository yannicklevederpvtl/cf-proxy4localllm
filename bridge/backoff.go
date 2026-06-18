package main

import (
	"math"
	"math/rand"
	"time"
)

const reconnectJitterFraction = 0.2

func reconnectDelayBase(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	seconds := math.Min(60, math.Pow(2, float64(attempt-1)))
	return time.Duration(seconds) * time.Second
}

func reconnectDelay(attempt int) time.Duration {
	base := reconnectDelayBase(attempt)
	jitter := reconnectJitterFraction * (rand.Float64()*2 - 1)
	return time.Duration(float64(base) * (1 + jitter))
}
