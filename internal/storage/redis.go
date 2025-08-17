package storage

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type DataStore struct {
	R *redis.Client
}

func New(stringConn string, poolSize int) *DataStore {
	return &DataStore{
		R: redis.NewClient(&redis.Options{
			Addr:     stringConn,
			PoolSize: poolSize,
		}),
	}
}

func (ds *DataStore) Ping(ctx context.Context) error {
	return ds.R.Ping(ctx).Err()
}

func (ds *DataStore) MarkIfNew(ctx context.Context, id string, ttl time.Duration) (bool, error) {
	return ds.R.SetNX(ctx, "payment:"+id, "1", ttl).Result()
}

func (ds *DataStore) Aggregation(ctx context.Context, proc string, t time.Time, amount float64) error {
	minute := t.UTC().Format("200601021504")
	pipe := ds.R.Pipeline()
	exp := 25 * time.Hour

	pipe.IncrBy(ctx, "count:"+proc+":"+minute, 1)
	pipe.IncrByFloat(ctx, "sum:"+proc+":"+minute, amount)

	pipe.IncrBy(ctx, "count:"+proc+":all", 1)
	pipe.IncrByFloat(ctx, "sum:"+proc+":all", amount)

	pipe.Expire(ctx, "count:"+proc+":"+minute, exp)
	pipe.Expire(ctx, "sum:"+proc+":"+minute, exp)

	_, err := pipe.Exec(ctx)

	return err
}

func (ds *DataStore) GetTotal(ctx context.Context, proc string) (count int64, sum float64, err error) {
	vals, err := ds.R.MGet(ctx, "count:"+proc+":all", "sum:"+proc+":all").Result()
	if err != nil {
		return
	}

	// Convert count value from Redis, handling both int64 and string types
	if vals[0] != nil {
		count, _ = vals[0].(int64)
		if count == 0 {
			if str, ok := vals[0].(string); ok {
				count, _ = strconv.ParseInt(str, 10, 64)
			}
		}
	}

	// Convert sum value using type switch to handle float64 or string from Redis
	if vals[1] != nil {
		switch v := vals[1].(type) {
		case float64:
			sum = v
		case string:
			sum, _ = strconv.ParseFloat(v, 64)
		}
	}

	return
}

func (ds *DataStore) GetTotalRange(ctx context.Context, proc string, from, to time.Time) (int64, float64, error) {
	keysCount := make([]string, 0, 512)
	keysSum := make([]string, 0, 512)

	for t := from.UTC().Truncate(time.Minute); !t.After(to.UTC()); t = t.Add(time.Minute) {
		mk := t.Format("200601021504")
		keysCount = append(keysCount, "count:"+proc+":"+mk)
		keysSum = append(keysSum, "sum:"+proc+":"+mk)
	}

	pipe := ds.R.Pipeline()
	cnts := pipe.MGet(ctx, keysCount...)
	sums := pipe.MGet(ctx, keysSum...)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, 0, err
	}

	var totalC int64
	var totalS float64

	// Sum up the values for the "count" keys
	for _, v := range cnts.Val() {
		if v == nil {
			continue
		}
		switch vv := v.(type) {
		case int64:
			totalC += vv
		case string:
			x, _ := strconv.ParseInt(vv, 10, 64)
			totalC += x
		}
	}

	// Sum up the values for the "sum" keys
	for _, v := range sums.Val() {
		if v == nil {
			continue
		}
		switch vv := v.(type) {
		case float64:
			totalS += vv
		case string:
			x, _ := strconv.ParseFloat(vv, 64)
			totalS += x
		}
	}

	return totalC, totalS, nil
}

// Health cache control across replicas (cluster-friendly and 1 per 5s).
func (s *DataStore) TryAcquireHealthLock(ctx context.Context, name string) bool {
	ok, _ := s.R.SetNX(ctx, "healthlock:"+name, "1", 5*time.Second).Result()
	return ok
}

func (s *DataStore) PutHealth(ctx context.Context, name string, payload string) error {
	return s.R.Set(ctx, "health:"+name, payload, 5*time.Second).Err()
}

func (s *DataStore) GetHealth(ctx context.Context, name string) (string, error) {
	return s.R.Get(ctx, "health:"+name).Result()
}
