package wshandler

import "time"

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024
	sendBufferSize = 256

	created = "created"
	deleted = "deleted"
	updated = "updated"

	ActionUserCreate      = "user.create"
	ActionUserGetByID     = "user.get_by_id"
	ActionUserGetByEmail  = "user.get_by_email"
	ActionUserList        = "user.list"
	ActionUserUpdate      = "user.update"
	ActionUserDelete      = "user.delete"
	ActionUserSubscribe   = "user.subscribe"
	ActionUserUnsubscribe = "user.unsubscribe"
)
