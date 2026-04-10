package userclient

import "time"

const (
	defaultTimeout        = 5 * time.Second
	defaultListLimit      = 50
	cacheUpdateBufferSize = 256

	created = "created"
	updated = "updated"
	deleted = "deleted"
)
