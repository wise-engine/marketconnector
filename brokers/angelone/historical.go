package angelone

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sunnyme20/marketconnector/brokers/angelone/internal/client"
	"github.com/sunnyme20/marketconnector/brokers/angelone/internal/endpoints"
	"github.com/sunnyme20/marketconnector/brokers/angelone/internal/util"
	"github.com/sunnyme20/marketconnector/brokers/angelone/internal/wire"
	"github.com/sunnyme20/marketconnector/model"
	"golang.org/x/time/rate"
)

// retryBaseDelay is the initial backoff delay for 403 rate-limit retries.
const retryBaseDelay = 500 * time.Millisecond

// postWithRetry calls c.Post and retries with exponential backoff when the
// server returns a 403 (rate limit exceeded). This prevents transient
// rate-limit errors from failing an entire batch.
func (a *Angelone) postWithRetry(c *client.Client, url string, body, result any) error {
	var lastErr error
	for attempt := 0; attempt <= wire.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 500ms, 1s, 2s.
			delay := retryBaseDelay * time.Duration(1<<(attempt-1))
			time.Sleep(delay)
		}

		err := c.Post(url, body, result)
		if err == nil {
			return nil
		}

		// Only retry on 403 rate-limit errors; fail fast on everything else.
		errMsg := err.Error()
		if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "exceeding access rate") {
			lastErr = err
			continue
		}
		return err
	}
	return fmt.Errorf("rate limit exceeded after %d retries: %w", wire.MaxRetries, lastErr)
}

// fetchSingleBatch performs one raw API call (candles + OI) for a single batch.
// It retries up to wire.MaxRetries times with exponential backoff on 403
// rate-limit errors.
func (a *Angelone) fetchSingleBatch(batch symbolBatch) (*model.Response[model.HistoricalResponse], error) {
	httpClient := a.httpClient()
	httpClient.SetAccessToken(a.accessToken)

	req := wire.HistoricalRequest{
		Exchange:    wire.MapExchange(batch.Exchange),
		SymbolToken: batch.SymbolToken,
		Interval:    wire.MapTimeframe(batch.Interval),
		FromDate:    batch.FromDate,
		ToDate:      batch.ToDate,
	}

	// Fetch candle data with retry on 403.
	var candleResp wire.HistoricalCandleData
	if err := a.postWithRetry(httpClient, endpoints.API.Historical, req, &candleResp); err != nil {
		return nil, fmt.Errorf("fetch candle data: %w", err)
	}

	records, err := candleResp.Candles()
	if err != nil {
		return nil, fmt.Errorf("fetch candle data: %w", err)
	}
	candles := make([]model.HistoricalCandle, 0, len(records))
	for _, record := range records {
		if len(record) < 6 {
			continue
		}
		candles = append(candles, model.HistoricalCandle{
			Timestamp: record[0].(string),
			Open:      util.ToFloat64OrZero(record[1]),
			High:      util.ToFloat64OrZero(record[2]),
			Low:       util.ToFloat64OrZero(record[3]),
			Close:     util.ToFloat64OrZero(record[4]),
			Volume:    util.ToInt64OrZero(record[5]),
		})
	}

	// OI data is only relevant for derivatives (NFO/BFO/MCX). Skip it for cash
	// equity (NSE/BSE) to avoid unnecessary API calls and stay within the rate
	// limit.
	var oiItems []model.HistoricalOIItem
	if batch.Exchange != model.ExchangeNSE && batch.Exchange != model.ExchangeBSE {
		// Rate-limit between the candle and OI calls — both count toward the
		// broker's rate limit.
		time.Sleep(time.Duration(float64(time.Second) / wire.HistoricalRateLimit))

		var oiResp wire.HistoricalOIData
		if err := a.postWithRetry(httpClient, endpoints.API.HistoricalOI, req, &oiResp); err != nil {
			return nil, fmt.Errorf("fetch OI data: %w", err)
		}

		oiRecords, err := oiResp.OIRecords()
		if err != nil {
			return nil, fmt.Errorf("fetch OI data: %w", err)
		}
		oiItems = make([]model.HistoricalOIItem, 0, len(oiRecords))
		for _, item := range oiRecords {
			oiItems = append(oiItems, model.HistoricalOIItem{
				Timestamp: item.Time,
				OI:        item.OI,
			})
		}
	}

	return &model.Response[model.HistoricalResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "angelone",
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
func (a *Angelone) GetHistoricalData(exchange model.Exchange, symbolToken string, interval model.Timeframe, fromDate, toDate string) (*model.Response[model.HistoricalResponse], error) {
	maxDays, ok := wire.IntervalMaxDays[interval]
	if !ok {
		return nil, fmt.Errorf("unknown interval %q: no max-days mapping", interval)
	}

	from, err := time.Parse(util.DateFormat, fromDate)
	if err != nil {
		return nil, fmt.Errorf("invalid fromDate %q: %w", fromDate, err)
	}
	to, err := time.Parse(util.DateFormat, toDate)
	if err != nil {
		return nil, fmt.Errorf("invalid toDate %q: %w", toDate, err)
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
		return a.fetchSingleBatch(symbolBatch{
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
				data, err := a.fetchSingleBatch(batch)
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
		Broker:  "angelone",
		Data: model.HistoricalResponse{
			Candles: allCandles,
			OI:      allOI,
		},
	}, nil
}
