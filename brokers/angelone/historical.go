package angelone

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	models "github.com/sunnyme20/marketconnector/brokers/model"
	"golang.org/x/time/rate"
)

// retryBaseDelay is the initial backoff delay for 403 rate-limit retries.
const retryBaseDelay = 500 * time.Millisecond

// postWithRetry calls client.Post and retries with exponential backoff when the
// server returns a 403 (rate limit exceeded). This prevents transient rate-limit
// errors from failing the entire batch.
func (a *Angelone) postWithRetry(client *Client, url string, body, result any) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 500ms, 1s, 2s
			delay := retryBaseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
			fmt.Printf("    ⏳ rate limited, retrying in %v (attempt %d/%d)...\n", delay, attempt, maxRetries)
			time.Sleep(delay)
		}

		err := client.Post(url, body, result)
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
	return fmt.Errorf("rate limit exceeded after %d retries: %w", maxRetries, lastErr)
}

// fetchSingleBatch performs one raw API call (candles + OI) for a single batch.
// Retries up to maxRetries times with exponential backoff on 403 rate-limit errors.
func (a *Angelone) fetchSingleBatch(batch SymbolBatch) (*models.Response[models.HistoricalResponse], error) {
	client := NewClient(a.ApiKey)
	client.AccessToken = a.AccessToken

	req := HistoricalRequest{
		Exchange:    MapExchange(batch.Exchange),
		SymbolToken: batch.SymbolToken,
		Interval:    MapTimeframe(batch.Interval),
		FromDate:    batch.FromDate,
		ToDate:      batch.ToDate,
	}

	// Fetch candle data with retry on 403
	var candleResp *HistoricalCandleData
	if err := a.postWithRetry(client, Api.Historical, req, &candleResp); err != nil {
		return nil, fmt.Errorf("failed to fetch candle data: %w", err)
	}

	records, _ := candleResp.ParseData()
	var candles []models.HistoricalCandle
	for _, record := range records {
		if len(record) < 6 {
			continue
		}
		candles = append(candles, models.HistoricalCandle{
			Timestamp: record[0].(string),
			Open:      toFloat64OrZero(record[1]),
			High:      toFloat64OrZero(record[2]),
			Low:       toFloat64OrZero(record[3]),
			Close:     toFloat64OrZero(record[4]),
			Volume:    toInt64OrZero(record[5]),
		})
	}

	// OI data is only relevant for derivatives (NFO/BFO/MCX). Skip it for
	// cash equity (NSE/BSE) to avoid unnecessary API calls and stay within
	// the rate limit.
	var oiItems []models.HistoricalOIItem
	if batch.Exchange != models.ExchangeNSE && batch.Exchange != models.ExchangeBSE {
		// Rate-limit between the candle and OI calls — both count toward the
		// broker's rate limit.
		time.Sleep(time.Duration(float64(time.Second) / HistoricalRateLimit))

		var oiResp *HistoricalOIData
		if err := a.postWithRetry(client, Api.HistoricalOI, req, &oiResp); err != nil {
			return nil, fmt.Errorf("failed to fetch OI data: %w", err)
		}

		oiRecords, _ := oiResp.ParseOIData()
		oiItems = make([]models.HistoricalOIItem, 0, len(oiRecords))
		for _, item := range oiRecords {
			oiItems = append(oiItems, models.HistoricalOIItem{
				Timestamp: item.Time,
				OI:        item.OI,
			})
		}
	}

	return &models.Response[models.HistoricalResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "angelone",
		Data: models.HistoricalResponse{
			Candles: candles,
			OI:      oiItems,
		},
	}, nil
}

// GetHistoricalData fetches historical data for a single symbol.
// If the date range exceeds AngelOne's per-request limit for the given interval,
// it automatically splits the range into batches and fetches them concurrently
// using a 3-worker pool with a 3 req/s rate limiter, then merges the results.
func (a *Angelone) GetHistoricalData(exchange models.Exchange, symbolToken string, interval models.Timeframe, fromDate, toDate string) (*models.Response[models.HistoricalResponse], error) {
	fmt.Printf("Fetching historical data for %s\n", a.ClientCode)

	// Check if the date range needs batching
	maxDays, ok := IntervalMaxDays[interval]
	if !ok {
		return nil, fmt.Errorf("unknown interval %q — no max-days mapping", interval)
	}

	from, err := time.Parse(angeloneDateFormat, fromDate)
	if err != nil {
		return nil, fmt.Errorf("invalid fromDate %q: %w", fromDate, err)
	}
	to, err := time.Parse(angeloneDateFormat, toDate)
	if err != nil {
		return nil, fmt.Errorf("invalid toDate %q: %w", toDate, err)
	}

	totalDays := int(ceilDiv(to.Sub(from).Hours(), 24))
	if totalDays <= maxDays {
		// Single batch — use the helper directly
		return a.fetchSingleBatch(SymbolBatch{
			Exchange:    exchange,
			SymbolToken: symbolToken,
			Interval:    interval,
			FromDate:    fromDate,
			ToDate:      toDate,
		})
	}

	// ── Split into day-bounded batches ──
	batches, err := a.splitDateRangeIntoBatches(models.HistoricalBatchRequest{
		Exchange:    exchange,
		SymbolToken: symbolToken,
		Interval:    interval,
		FromDate:    fromDate,
		ToDate:      toDate,
	})
	if err != nil {
		return nil, fmt.Errorf("split date range: %w", err)
	}

	fmt.Printf("Date range exceeds %d-day limit — split into %d batches, fetching with worker pool\n", maxDays, len(batches))
	for i, b := range batches {
		fmt.Printf("  batch %d/%d: [%s] %s → %s\n", i+1, len(batches), b.SymbolToken, b.FromDate, b.ToDate)
	}

	// ── Worker pool (3 workers, 3 req/s) ──
	const numWorkers = 3
	jobCh := make(chan SymbolBatch, len(batches))
	resultCh := make(chan BatchResult, len(batches))

	limiter := rate.NewLimiter(HistoricalRateLimit, HistoricalRateBurst)

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for batch := range jobCh {
				// Strict rate limit: blocks until a token is available
				if err := limiter.Wait(context.Background()); err != nil {
					fmt.Printf("  [worker %d] ⚠️ rate limiter error: %v\n", workerID, err)
				}
				batchStart := time.Now()
				fmt.Printf("  [worker %d] fetching %s [%s → %s]\n", workerID, batch.SymbolToken, batch.FromDate, batch.ToDate)
				data, err := a.fetchSingleBatch(batch)
				elapsed := time.Since(batchStart).Truncate(time.Millisecond)
				if err != nil {
					fmt.Printf("  [worker %d] ❌ %s [%s → %s] after %v: %v\n", workerID, batch.SymbolToken, batch.FromDate, batch.ToDate, elapsed, err)
				} else {
					fmt.Printf("  [worker %d] ✅ %s [%s → %s] in %v: %d candles\n", workerID, batch.SymbolToken, batch.FromDate, batch.ToDate, elapsed, len(data.Data.Candles))
				}
				resultCh <- BatchResult{
					SymbolToken: batch.SymbolToken,
					FromDate:    batch.FromDate,
					ToDate:      batch.ToDate,
					Data:        data,
					Err:         err,
				}
			}
		}(i)
	}

	for _, batch := range batches {
		jobCh <- batch
	}
	close(jobCh)

	wg.Wait()
	close(resultCh)

	// ── Merge results ──
	var allCandles []models.HistoricalCandle
	var allOI []models.HistoricalOIItem
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
		return nil, fmt.Errorf("batch errors (%d/%d failed): %s", len(errs), len(batches), fmtErrList(errs))
	}

	return &models.Response[models.HistoricalResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "angelone",
		Data: models.HistoricalResponse{
			Candles: allCandles,
			OI:      allOI,
		},
	}, nil
}

func toFloat64OrZero(val any) float64 {
	v, _ := toFloat64(val)
	return v
}

func toInt64OrZero(val any) int64 {
	v, _ := toInt64(val)
	return v
}

func toFloat64(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	default:
		return 0, false
	}
}

func toInt64(val any) (int64, bool) {
	switch v := val.(type) {
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}

// ceilDiv rounds up n/d.
func ceilDiv(n, d float64) int {
	if n == float64(int(n/d)) {
		return int(n / d)
	}
	if n/d > 0 {
		return int(n/d) + 1
	}
	return int(n / d)
}

// fmtErrList joins error strings with "; " for readable merge-failure messages.
func fmtErrList(errs []string) string {
	s := ""
	for i, e := range errs {
		if i > 0 {
			s += "; "
		}
		s += e
	}
	return s
}
