package zerodha

import (
	"context"
	"fmt"
	"sync"

	"github.com/wise-engine/marketconnector/brokers/zerodha/internal/util"
	"github.com/wise-engine/marketconnector/brokers/zerodha/internal/wire"
	"github.com/wise-engine/marketconnector/model"
	"golang.org/x/time/rate"
)

// symbolBatch is a single API-call-sized chunk for one symbol.
type symbolBatch struct {
	Exchange    model.Exchange
	SymbolToken string
	Interval    model.Timeframe
	FromDate    string
	ToDate      string
}

// batchResult holds the outcome of fetching one symbolBatch.
type batchResult struct {
	SymbolToken string
	FromDate    string
	ToDate      string
	Data        *model.Response[model.HistoricalResponse]
	Err         error
}

// splitDateRangeIntoBatches splits a single (fromDate, toDate) range into
// multiple symbolBatch entries, each respecting the interval's max-days limit.
func splitDateRangeIntoBatches(req model.HistoricalBatchRequest) ([]symbolBatch, error) {
	maxDays, ok := wire.IntervalMaxDays[req.Interval]
	if !ok {
		return nil, fmt.Errorf("unknown interval %q: no max-days mapping", req.Interval)
	}

	from, err := util.ParseTime(req.FromDate)
	if err != nil {
		return nil, fmt.Errorf("invalid fromDate %q: %w", req.FromDate, err)
	}
	to, err := util.ParseTime(req.ToDate)
	if err != nil {
		return nil, fmt.Errorf("invalid toDate %q: %w", req.ToDate, err)
	}
	if to.Before(from) {
		return nil, fmt.Errorf("toDate %q is before fromDate %q", req.ToDate, req.FromDate)
	}

	if util.DaysBetween(from, to) <= maxDays {
		return []symbolBatch{{
			Exchange:    req.Exchange,
			SymbolToken: req.SymbolToken,
			Interval:    req.Interval,
			FromDate:    req.FromDate,
			ToDate:      req.ToDate,
		}}, nil
	}

	var batches []symbolBatch
	currentFrom := from
	for currentFrom.Before(to) {
		currentTo := currentFrom.AddDate(0, 0, maxDays)
		if currentTo.After(to) {
			currentTo = to
		}

		batches = append(batches, symbolBatch{
			Exchange:    req.Exchange,
			SymbolToken: req.SymbolToken,
			Interval:    req.Interval,
			FromDate:    currentFrom.Format(util.DateFormat),
			ToDate:      currentTo.Format(util.DateFormat),
		})

		// Next batch starts one day after the previous chunk's end.
		currentFrom = currentTo.AddDate(0, 0, 1)
	}

	return batches, nil
}

// FetchHistoricalDataBatch fetches historical data for multiple symbols
// concurrently using a bounded worker pool under a global rate limit.
//
// Each request's date range is automatically split into per-interval max-day
// batches. Results are aggregated per symbol, merging candles and OI data.
func (z *Zerodha) FetchHistoricalDataBatch(requests []model.HistoricalBatchRequest) (*model.Response[[]model.HistoricalBatchItem], error) {
	if len(requests) == 0 {
		return &model.Response[[]model.HistoricalBatchItem]{
			Success: true,
			Message: "SUCCESS",
			Broker:  "zerodha",
			Data:    []model.HistoricalBatchItem{},
		}, nil
	}

	// Flatten every request into individual day-bounded batches.
	var allBatches []symbolBatch
	for _, req := range requests {
		batches, err := splitDateRangeIntoBatches(req)
		if err != nil {
			return nil, fmt.Errorf("split batch for %s: %w", req.SymbolToken, err)
		}
		allBatches = append(allBatches, batches...)
	}

	if len(allBatches) == 0 {
		return &model.Response[[]model.HistoricalBatchItem]{
			Success: true,
			Message: "SUCCESS",
			Broker:  "zerodha",
			Data:    []model.HistoricalBatchItem{},
		}, nil
	}

	const numWorkers = 3
	jobCh := make(chan symbolBatch, len(allBatches))
	resultCh := make(chan batchResult, len(allBatches))

	// Strict rate limiter — wire.HistoricalRateLimit req/s with burst=1 so only
	// one request fires at a time.
	limiter := rate.NewLimiter(wire.HistoricalRateLimit, wire.HistoricalRateBurst)

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range jobCh {
				_ = limiter.Wait(context.Background())
				// Call fetchSingleBatch directly since batches are already
				// split into per-day chunks — no further splitting needed.
				data, err := z.fetchSingleBatch(batch)
				resultCh <- batchResult{
					SymbolToken: batch.SymbolToken,
					FromDate:    batch.FromDate,
					ToDate:      batch.ToDate,
					Data:        data,
					Err:         err,
				}
			}
		}()
	}

	for _, batch := range allBatches {
		jobCh <- batch
	}
	close(jobCh)
	wg.Wait()
	close(resultCh)

	// Aggregate results per symbol.
	itemsMap := make(map[string]*model.HistoricalBatchItem, len(requests))
	for res := range resultCh {
		item, ok := itemsMap[res.SymbolToken]
		if !ok {
			item = &model.HistoricalBatchItem{
				SymbolToken: res.SymbolToken,
				Broker:      "zerodha",
			}
			itemsMap[res.SymbolToken] = item
		}
		if res.Err != nil {
			item.Success = false
			item.Message = "FAILED"
			item.Error = res.Err.Error()
			continue
		}
		if res.Data != nil && res.Data.Success {
			item.Success = true
			item.Message = "SUCCESS"
			if item.Data == nil {
				item.Data = &model.HistoricalResponse{}
			}
			item.Data.Candles = append(item.Data.Candles, res.Data.Data.Candles...)
			item.Data.OI = append(item.Data.OI, res.Data.Data.OI...)
		}
	}

	items := make([]model.HistoricalBatchItem, 0, len(itemsMap))
	for _, item := range itemsMap {
		items = append(items, *item)
	}

	return &model.Response[[]model.HistoricalBatchItem]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "zerodha",
		Data:    items,
	}, nil
}
