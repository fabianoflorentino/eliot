package http

import (
	"context"
	"net"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fasthttp"

	"github.com/fabianoflorentino/eliot/internal/core"
	"github.com/fabianoflorentino/eliot/internal/health"
	"github.com/fabianoflorentino/eliot/internal/payments"
	"github.com/fabianoflorentino/eliot/internal/queue"
	"github.com/fabianoflorentino/eliot/internal/storage"
)

var jsonFast = jsoniter.ConfigFastest

type Server struct {
	Store       *storage.DataStore
	Stream      *queue.Stream
	Forwarder   *payments.Forwarder
	DefaultURL  string
	FallbackURL string
	DefaultHC   *health.Checker
	FallbackHC  *health.Checker
	HedgeAfter  time.Duration
}

func writeJSON(ctx *fasthttp.RequestCtx, status int, v any) {
	ctx.SetStatusCode(status)
	if v == nil {
		return
	}
	b, _ := jsonFast.Marshal(v)

	ctx.SetContentType("application/json")
	if _, err := ctx.Write(b); err != nil {
		return
	}
}

func isUUID(s string) bool {
	return len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}

func (s *Server) PostPayments(ctx *fasthttp.RequestCtx) {
	var in core.Payment
	if err := jsonFast.Unmarshal(ctx.PostBody(), &in); err != nil || in.Amount <= 0 || !isUUID(in.CorrelationID) {
		ctx.Error("bad request", fasthttp.StatusBadRequest)
		return
	}
	ok, err := s.Store.MarkIfNew(context.Background(), in.CorrelationID, 24*time.Hour)
	if err != nil {
		ctx.Error("server error", 500)
		return
	}
	if !ok {
		writeJSON(ctx, fasthttp.StatusOK, map[string]string{"message": "duplicate ignored"})
		return
	}
	// Enqueue and return immediately — sub-10ms path
	err = s.Stream.Enqueue(context.Background(), map[string]any{"correlationId": in.CorrelationID, "amount": in.Amount})
	if err != nil {
		ctx.Error("server error", 500)
		return
	}
	writeJSON(ctx, fasthttp.StatusAccepted, map[string]string{"status": "queued"})
}

func (s *Server) GetPaymentsSummary(ctx *fasthttp.RequestCtx) {
	q := ctx.QueryArgs()
	fromStr := strings.TrimSpace(string(q.Peek("from")))
	toStr := strings.TrimSpace(string(q.Peek("to")))

	var out core.Summary
	var err error

	if fromStr == "" || toStr == "" {
		out, err = (&core.Summarizer{Store: s.Store}).All(context.Background())
	} else {
		from, e1 := time.Parse(time.RFC3339Nano, fromStr)
		to, e2 := time.Parse(time.RFC3339Nano, toStr)

		if e1 != nil || e2 != nil {
			ctx.Error("bad request", 400)
			return
		}

		out, err = (&core.Summarizer{Store: s.Store}).Range(context.Background(), from, to)
	}

	if err != nil {
		ctx.Error("server error", 500)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, out)
}

func (s *Server) NewClient() *fasthttp.Client {
	return &fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		ReadTimeout:              200 * time.Millisecond,
		WriteTimeout:             200 * time.Millisecond,
		MaxConnsPerHost:          1024,
		Dial: func(addr string) (net.Conn, error) {
			return fasthttp.DialTimeout(addr, 200*time.Millisecond)
		},
	}
}
