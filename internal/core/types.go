package core

import "time"

type Payment struct {
	CorrelationID string  `json:"correlationId" binding:"required"`
	Amount        float64 `json:"amount" binding:"required,gt=0"`
}

type Processor string

const (
	DefaultProcessor  Processor = "default"
	FallbackProcessor Processor = "fallback"
)

type Health struct {
	Failing       bool `json:"failing"`
	MinResponseMS int  `json:"minResponseTime"`
}

type TSummary struct {
	TotalRequests int64   `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

type Summary struct {
	Default  TSummary `json:"default"`
	Fallback TSummary `json:"fallback"`
}

func NowUTCISO() string { return time.Now().UTC().Format(time.RFC3339) }

func MinuteKey(t time.Time) string { return t.UTC().Format("200601021504") }
