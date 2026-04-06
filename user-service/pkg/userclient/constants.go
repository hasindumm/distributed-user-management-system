package userclient

import "time"

const (
	defaultTimeout        = 5 * time.Second
	defaultListLimit      = 50
	cacheStartupBatchSize = 1000
	cacheUpdateBufferSize = 256
	cacheStartupTimeout   = 30 * time.Second
)
