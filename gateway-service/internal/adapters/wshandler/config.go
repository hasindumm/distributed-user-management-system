package wshandler

import "time"

type Config struct {
	WriteWait      time.Duration
	PongWait       time.Duration
	PingPeriod     time.Duration
	MaxMessageSize int64
	SendBufferSize int
}

func DefaultConfig() Config {
	return Config{
		WriteWait:      writeWait,
		PongWait:       pongWait,
		PingPeriod:     pingPeriod,
		MaxMessageSize: maxMessageSize,
		SendBufferSize: sendBufferSize,
	}
}
