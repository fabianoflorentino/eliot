package queue

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type Stream struct {
	R *redis.Client
}

const (
	StreamKey = "stream:payments"
	GroupName = "workers"
)

func NewStream(r *redis.Client) *Stream {
	return &Stream{R: r}
}

func (s *Stream) EnsureGroup(ctx context.Context) {
	if err := s.R.XGroupCreateMkStream(ctx, StreamKey, GroupName, "0").Err(); err != nil {
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			log.Printf("XGroupCreate: %v", err)
		}
	}
}

func (s *Stream) Enqueue(ctx context.Context, fields map[string]any) error {
	return s.R.XAdd(ctx, &redis.XAddArgs{Stream: StreamKey, Values: fields}).Err()
}

func (s *Stream) Read(ctx context.Context, consumer string, count int64, blockDur time.Duration) ([]redis.XMessage, error) {
	res, err := s.R.XReadGroup(ctx, &redis.XReadGroupArgs{Group: GroupName, Consumer: consumer, Streams: []string{StreamKey, ">"}, Count: count, Block: blockDur}).Result()
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}
	return res[0].Messages, nil
}

func (s *Stream) Ack(ctx context.Context, ids ...string) error {
	return s.R.XAck(ctx, StreamKey, GroupName, ids...).Err()
}

// ReadPending lê mensagens pendentes do grupo para o consumidor
func (s *Stream) ReadPending(ctx context.Context, consumer string, count int64) ([]redis.XMessage, error) {
	res, err := s.R.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    GroupName,
		Consumer: consumer,
		Streams:  []string{StreamKey, "0"},
		Count:    count,
		Block:    0,
	}).Result()
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}
	return res[0].Messages, nil
}
