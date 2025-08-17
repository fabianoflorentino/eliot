package payments

import (
	"context"
	"encoding/json"
	"time"

	"github.com/valyala/fasthttp"
)

type Forwarder struct {
	Client  *fasthttp.Client
	Timeout time.Duration
}

type PaymentReq struct {
	CorrelationID string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
	RequestedAt   string  `json:"requestedAt"`
}

type Result struct {
	OK       bool
	Status   int
	Duration time.Duration
	Err      error
}

func (f *Forwarder) Send(ctx context.Context, baseURL string, body PaymentReq) Result {
	start := time.Now()

	var req fasthttp.Request
	var resp fasthttp.Response

	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI(baseURL + "/payments")

	b, _ := json.Marshal(body)

	req.SetBodyRaw(b)
	req.Header.SetContentType("application/json")

	deadline := time.Now().Add(f.Timeout)
	err := f.Client.DoDeadline(&req, &resp, deadline)

	if err != nil {
		return Result{OK: false, Status: 0, Duration: time.Since(start), Err: err}
	}

	sc := resp.StatusCode()

	return Result{OK: sc >= 200 && sc < 300, Status: sc, Duration: time.Since(start)}
}
