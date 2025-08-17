package http

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/fabianoflorentino/eliot/internal/health"
	"github.com/fabianoflorentino/eliot/internal/payments"
	"github.com/fabianoflorentino/eliot/internal/queue"
	"github.com/fabianoflorentino/eliot/internal/storage"
	"github.com/fabianoflorentino/eliot/internal/worker"
)

type App struct {
	Server *fasthttp.Server
}

func NewApp() (*App, error) {
	redisAddr := env("REDIS_ADDR", "redis:6379")
	pool := envInt("REDIS_POOL", 150)
	defaultURL := env("PROCESSOR_DEFAULT_URL", "http://payment-processor-default:8080")
	fallbackURL := env("PROCESSOR_FALLBACK_URL", "http://payment-processor-fallback:8080")
	hedgeMS := envInt("HEDGE_MS", 40)
	toMS := envInt("TIMEOUT_MS", 120)
	workerConc := envInt("WORKER_CONCURRENCY", 8)

	store := storage.New(redisAddr, pool)
	_ = store.Ping(context.Background())
	stream := queue.NewStream(store.R)
	stream.EnsureGroup(context.Background())

	srv := &Server{
		Store:  store,
		Stream: stream,
		Forwarder: &payments.Forwarder{
			Client:  (&Server{}).NewClient(),
			Timeout: time.Duration(toMS) * time.Millisecond,
		},
		DefaultURL:  defaultURL,
		FallbackURL: fallbackURL,
		HedgeAfter:  time.Duration(hedgeMS) * time.Millisecond,
	}

	srv.DefaultHC = &health.Checker{
		Client:  srv.NewClient(),
		Store:   store,
		Name:    "default",
		BaseURL: defaultURL,
	}
	srv.FallbackHC = &health.Checker{
		Client:  srv.NewClient(),
		Store:   store,
		Name:    "fallback",
		BaseURL: fallbackURL,
	}

	w := &worker.Worker{
		Store:  store,
		Stream: stream,
		Client: srv.Forwarder,
		DefHC:  srv.DefaultHC,
		FalHC:  srv.FallbackHC,
		DefURL: defaultURL,
		FalURL: fallbackURL,
	}

	w.Run(context.Background(), workerConc)

	server := &fasthttp.Server{
		Name:                         "eliot",
		DisablePreParseMultipartForm: true,
		ReduceMemoryUsage:            true,
		ReadBufferSize:               4096,
		WriteTimeout:                 250 * time.Millisecond,
		ReadTimeout:                  250 * time.Millisecond,
		IdleTimeout:                  30 * time.Second,
		Handler: func(ctx *fasthttp.RequestCtx) {
			switch string(ctx.Path()) {
			case "/payments":
				if ctx.IsPost() {
					srv.PostPayments(ctx)
					return
				}
				ctx.Error("method not allowed", 405)
			case "/payments-summary":
				if ctx.IsGet() {
					srv.GetPaymentsSummary(ctx)
					return
				}
				ctx.Error("method not allowed", 405)
			default:
				ctx.Error("not found", 404)
			}
		}}
	return &App{Server: server}, nil
}

func env(k, v string) string {
	if s := os.Getenv(k); s != "" {
		return s
	}

	return v
}

func envInt(k string, v int) int {
	if s := os.Getenv(k); s != "" {
		if i, err := strconv.Atoi(s); err == nil {
			return i
		}
	}

	return v
}
