// Package wire holds the mapping functions and constants that describe the
// Zerodha Kite Connect historical data API (interval codes, per-request day
// limits and rate limits). Unlike the Angel One broker, the raw HTTP layer is
// provided by the official gokiteconnect SDK, so no request/response wire types
// are needed here. It is internal to the zerodha broker package.
package wire

import "github.com/wise-engine/marketconnector/model"

// Zerodha historical-data interval codes accepted by the Kite Connect API.
const (
	IntervalMinute  = "minute"
	Interval3Minute = "3minute"
	Interval5Minute = "5minute"
	Interval10Min   = "10minute"
	Interval15Min   = "15minute"
	Interval30Min   = "30minute"
	Interval60Min   = "60minute"
	IntervalDay     = "day"
)

// Historical rate limits (requests/second) for the Kite Connect API v3 and the
// retry settings used by the batch workers. Kept here so every caller uses the
// same values.
const (
	HistoricalRateLimit = 3 // max historical data requests per second
	HistoricalRateBurst = 1 // burst size for the rate limiter
	MaxRetries          = 3 // max retries on rate-limit errors
)

// MapTimeframe converts a broker-agnostic Timeframe to the Kite Connect
// historical data interval.
func MapTimeframe(tf model.Timeframe) string {
	switch tf {
	case model.Timeframe1Minute:
		return IntervalMinute
	case model.Timeframe3Minutes:
		return Interval3Minute
	case model.Timeframe5Minutes:
		return Interval5Minute
	case model.Timeframe10Minutes:
		return Interval10Min
	case model.Timeframe15Minutes:
		return Interval15Min
	case model.Timeframe30Minutes:
		return Interval30Min
	case model.Timeframe1Hour:
		return Interval60Min
	case model.Timeframe1Day:
		return IntervalDay
	default:
		return IntervalDay
	}
}

// IntervalMaxDays defines the maximum number of days Zerodha allows in a single
// historical data request for each interval.
var IntervalMaxDays = map[model.Timeframe]int{
	model.Timeframe1Minute:   60,
	model.Timeframe3Minutes:  100,
	model.Timeframe5Minutes:  100,
	model.Timeframe10Minutes: 100,
	model.Timeframe15Minutes: 200,
	model.Timeframe30Minutes: 200,
	model.Timeframe1Hour:     400,
	model.Timeframe1Day:      2000,
}
