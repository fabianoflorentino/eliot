package core

import (
	"context"
	"time"

	"github.com/fabianoflorentino/eliot/internal/storage"
)

type Summarizer struct{ Store *storage.DataStore }

func (s *Summarizer) All(ctx context.Context) (Summary, error) {
	var out Summary

	cd, sd, err := s.Store.GetTotal(ctx, string(DefaultProcessor))
	if err != nil {
		return out, err
	}

	cf, sf, err := s.Store.GetTotal(ctx, string(FallbackProcessor))
	if err != nil {
		return out, err
	}

	out.Default = TSummary{TotalRequests: cd, TotalAmount: sd}
	out.Fallback = TSummary{TotalRequests: cf, TotalAmount: sf}

	return out, nil
}

func (s *Summarizer) Range(ctx context.Context, from, to time.Time) (Summary, error) {
	var out Summary

	cd, sd, err := s.Store.GetTotalRange(ctx, string(DefaultProcessor), from, to)
	if err != nil {
		return out, err
	}

	cf, sf, err := s.Store.GetTotalRange(ctx, string(FallbackProcessor), from, to)
	if err != nil {
		return out, err
	}

	out.Default = TSummary{TotalRequests: cd, TotalAmount: sd}
	out.Fallback = TSummary{TotalRequests: cf, TotalAmount: sf}

	return out, nil
}
