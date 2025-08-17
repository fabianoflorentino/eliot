package health

import (
	"context"
	"encoding/json"

	"github.com/fabianoflorentino/eliot/internal/core"
	"github.com/fabianoflorentino/eliot/internal/storage"
	"github.com/valyala/fasthttp"
)

type Checker struct {
	Client  *fasthttp.Client
	Store   *storage.DataStore
	Name    string // default|fallback
	BaseURL string
}

// Fetch and cache health (guarded by Redis lock to not break 1/5s limit cluster-wide).
func (c *Checker) Get(ctx context.Context) (core.Health, bool) {
	var h core.Health

	// Try use cache first
	if s, err := c.Store.GetHealth(ctx, c.Name); err == nil && s != "" {
		_ = json.Unmarshal([]byte(s), &h)
		return h, true
	}

	// Try acquire lock; if not possible, return zero+false (caller decides fallback strategy)
	if !c.Store.TryAcquireHealthLock(ctx, c.Name) {
		return h, false
	}

	// Perform HTTP GET /payments/service-health
	var req fasthttp.Request
	var resp fasthttp.Response

	req.SetRequestURI(c.BaseURL + "/payments/service-health")
	req.Header.SetMethod(fasthttp.MethodGet)

	if err := c.Client.Do(&req, &resp); err != nil {
		return h, false
	}

	if resp.StatusCode() != 200 {
		return h, false
	}

	if err := json.Unmarshal(resp.Body(), &h); err != nil {
		return h, false
	}

	b, _ := json.Marshal(h)
	_ = c.Store.PutHealth(ctx, c.Name, string(b))
	return h, true
}
