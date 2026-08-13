package zerodha

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/wise-engine/marketconnector/brokers/zerodha/internal/util"
	"github.com/wise-engine/marketconnector/brokers/zerodha/internal/wire"
	"github.com/wise-engine/marketconnector/model"
	"golang.org/x/time/rate"
)

// fetchSingleBatch performs one SDK historical data call for a single batch.
// Kite Connect returns candles and open interest together, so both are mapped
// in a single call (unlike Angel One, which needs a second OI endpoint).
func (z *Zerodha) fetchSingleBatch(batch symbolBatch) (*model.Response[model.HistoricalResponse], error) {
	token, err := strconv.Atoi(batch.SymbolToken)
	if err != nil {
		return nil, fmt.Errorf("invalid symbol token %q: %w", batch.SymbolToken, err)
	}

	from, err := util.ParseTime(batch.FromDate)
	if err != nil {
		return nil, err
	}
	to, err := util.ParseTime(batch.ToDate)
	if err != nil {
		return nil, err
	}

	// OI=true always: for cash equities the OI column is 0 and yields no OI
	// items, while derivatives get their open interest populated.
	records, err := z.kite().GetHistoricalData(token, wire.MapTimeframe(batch.Interval), from, to, false, true)
	if err != nil {
		return nil, fmt.Errorf("fetch historical data: %w", err)
	}

	candles := make([]model.HistoricalCandle, 0, len(records))
	oiItems := make([]model.HistoricalOIItem, 0)
	for _, record := range records {
		ts := record.Date.Format(util.TimestampFormat)
		candles = append(candles, model.HistoricalCandle{
			Timestamp: ts,
			Open:      record.Open,
			High:      record.High,
			Low:       record.Low,
			Close:     record.Close,
			Volume:    int64(record.Volume),
		})
		if record.OI > 0 {
			oiItems = append(oiItems, model.HistoricalOIItem{
				Timestamp: ts,
				OI:        int64(record.OI),
			})
		}
	}

	return &model.Response[model.HistoricalResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "zerodha",
		Data: model.HistoricalResponse{
			Candles: candles,
			OI:      oiItems,
		},
	}, nil
}

// GetHistoricalData fetches historical data for a single symbol. If the date
// range exceeds the broker's per-request limit for the given interval, it is
// split into day-bounded batches fetched concurrently by a small worker pool
// under a rate limit, and the results are merged.
func (z *Zerodha) GetHistoricalData(exchange model.Exchange, symbolToken string, interval model.Timeframe, fromDate, toDate string) (*model.Response[model.HistoricalResponse], error) {
	maxDays, ok := wire.IntervalMaxDays[interval]
	if !ok {
		return nil, fmt.Errorf("unknown interval %q: no max-days mapping", interval)
	}

	from, err := util.ParseTime(fromDate)
	if err != nil {
		return nil, fmt.Errorf("invalid fromDate: %w", err)
	}
	to, err := util.ParseTime(toDate)
	if err != nil {
		return nil, fmt.Errorf("invalid toDate: %w", err)
	}

	req := model.HistoricalBatchRequest{
		Exchange:    exchange,
		SymbolToken: symbolToken,
		Interval:    interval,
		FromDate:    fromDate,
		ToDate:      toDate,
	}

	// Single batch — no splitting needed.
	if util.DaysBetween(from, to) <= maxDays {
		return z.fetchSingleBatch(symbolBatch{
			Exchange:    exchange,
			SymbolToken: symbolToken,
			Interval:    interval,
			FromDate:    fromDate,
			ToDate:      toDate,
		})
	}

	// Split into day-bounded batches.
	batches, err := splitDateRangeIntoBatches(req)
	if err != nil {
		return nil, fmt.Errorf("split date range: %w", err)
	}

	const numWorkers = 3
	jobCh := make(chan symbolBatch, len(batches))
	resultCh := make(chan batchResult, len(batches))

	limiter := rate.NewLimiter(wire.HistoricalRateLimit, wire.HistoricalRateBurst)

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range jobCh {
				// Block until the rate limiter allows the next request.
				_ = limiter.Wait(context.Background())
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

	for _, batch := range batches {
		jobCh <- batch
	}
	close(jobCh)
	wg.Wait()
	close(resultCh)

	// Merge results.
	var allCandles []model.HistoricalCandle
	var allOI []model.HistoricalOIItem
	var errs []string
	for res := range resultCh {
		if res.Err != nil {
			errs = append(errs, fmt.Sprintf("%s [%s → %s]: %v", res.SymbolToken, res.FromDate, res.ToDate, res.Err))
			continue
		}
		if res.Data != nil && res.Data.Success {
			allCandles = append(allCandles, res.Data.Data.Candles...)
			allOI = append(allOI, res.Data.Data.OI...)
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("batch errors (%d/%d failed): %s", len(errs), len(batches), util.JoinErrors(errs))
	}

	return &model.Response[model.HistoricalResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "zerodha",
		Data: model.HistoricalResponse{
			Candles: allCandles,
			OI:      allOI,
		},
	}, nil
}
