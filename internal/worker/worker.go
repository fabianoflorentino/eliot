package worker

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/fabianoflorentino/eliot/internal/core"
	"github.com/fabianoflorentino/eliot/internal/health"
	"github.com/fabianoflorentino/eliot/internal/payments"
	"github.com/fabianoflorentino/eliot/internal/queue"
	"github.com/fabianoflorentino/eliot/internal/storage"
)

type Job struct {
	CorrelationID string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
	EnqueuedAt    string  `json:"enqueuedAt"`
}

type Worker struct {
	Store  *storage.DataStore
	Stream *queue.Stream
	Client *payments.Forwarder
	DefHC  *health.Checker
	FalHC  *health.Checker
	DefURL string
	FalURL string
}

func (w *Worker) Run(ctx context.Context, concurrency int) {
	consumer := time.Now().Format("150405.000")
	for i := 0; i < concurrency; i++ {
		go w.loop(ctx, consumer+"-"+strconv.Itoa(i))
	}
}

func (w *Worker) loop(ctx context.Context, consumer string) {
	for ctx.Err() == nil {
		msgs, err := w.Stream.Read(ctx, consumer, 32, 1*time.Second)
		if err != nil && err != redis.Nil {
			log.Println("xreadgroup", err)
			continue
		}
		if len(msgs) == 0 {
			continue
		}
		ids := make([]string, 0, len(msgs))
		for _, m := range msgs {
			ids = append(ids, m.ID)
			var j Job
			if err := mapToStruct(m.Values, &j); err != nil {
				continue
			}
			w.process(ctx, j)
		}
		_ = w.Stream.Ack(ctx, ids...)
	}
}

func (w *Worker) process(ctx context.Context, j Job) {
	hd, _ := w.DefHC.Get(ctx)
	hf, _ := w.FalHC.Get(ctx)
	primaryURL, primaryLabel := w.DefURL, string(core.DefaultProcessor)
	backupURL, backupLabel := w.FalURL, string(core.FallbackProcessor)
	if hd.Failing && !hf.Failing {
		primaryURL, backupURL = backupURL, primaryURL
		primaryLabel, backupLabel = backupLabel, primaryLabel
	}

	body := payments.PaymentReq{CorrelationID: j.CorrelationID, Amount: j.Amount, RequestedAt: core.NowUTCISO()}
	res := w.Client.Send(ctx, primaryURL, body)

	if !res.OK {
		res2 := w.Client.Send(ctx, backupURL, body)
		if !res2.OK {
			return
		}
		primaryLabel = backupLabel
	}

	// Use enqueued timestamp for consistency
	var aggTime time.Time
	if j.EnqueuedAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, j.EnqueuedAt); err == nil {
			aggTime = parsed
		} else {
			aggTime = time.Now().UTC()
		}
	} else {
		aggTime = time.Now().UTC()
	}

	_ = w.Store.Aggregation(ctx, primaryLabel, aggTime, j.Amount)
}

func mapToStruct(m map[string]any, j *Job) error {
	// Handle Redis stream values that come as strings
	if corrId, ok := m["correlationId"].(string); ok {
		j.CorrelationID = corrId
	}

	if amt, ok := m["amount"].(string); ok {
		amount, err := strconv.ParseFloat(amt, 64)
		if err != nil {
			return err
		}
		j.Amount = amount
	} else if amt, ok := m["amount"].(float64); ok {
		j.Amount = amt
	}

	if enqAt, ok := m["enqueuedAt"].(string); ok {
		j.EnqueuedAt = enqAt
	}

	return nil
}
