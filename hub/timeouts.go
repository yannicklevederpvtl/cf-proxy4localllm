package main

import "time"

var (
	upstreamRequestTimeout = 300 * time.Second
	httpServerReadTimeout  = 310 * time.Second
	httpServerWriteTimeout = 310 * time.Second
	httpServerIdleTimeout  = 120 * time.Second
)
