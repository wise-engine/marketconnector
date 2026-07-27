package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/sunnyme20/marketconnector/brokers"
	models "github.com/sunnyme20/marketconnector/brokers/model"
)

func checkWebsocket(broker brokers.Broker) {
	ws, err := broker.GetWebSocket()
	if err != nil {
		fmt.Println("GetWebSocket error:", err)
		return
	}

	ws.OnConnect(func() {
		fmt.Println("WebSocket connected")

		// Subscribe to SBI (token 3045) on NSE CM in LTP mode
		err := ws.Subscribe(int(models.ModeLTP), []models.WSTokenGroup{
			{
				ExchangeType: int(models.WSExchangeNseCM),
				Tokens:       []string{"3045"},
			},
		})
		if err != nil {
			fmt.Println("Subscribe error:", err)
		}
	})

	ws.OnTick(func(tick models.MarketQuoteResponse) {
		fmt.Printf("Tick: %s | LTP: %.2f | Vol: %d | Time: %d\n",
			tick.SymbolToken, tick.LTP, tick.TradeVolume, tick.ExchangeTime)
	})

	ws.OnError(func(err error) {
		fmt.Println("WS Error:", err)
	})

	ws.OnReconnect(func(attempt int, delay time.Duration) {
		fmt.Printf("WS reconnecting attempt %d in %v\n", attempt, delay)
	})

	// Run the WebSocket in a goroutine so it doesn't block
	go ws.Serve()

	// Let it run for 30 seconds then stop
	time.Sleep(30 * time.Second)
	ws.Stop()
	fmt.Println("WebSocket stopped")
}

func main() {

	clientCode := flag.String("client", "", "Client code")
	apiKey := flag.String("apikey", "", "API key")
	password := flag.String("password", "", "Password")
	totp := flag.String("totp", "", "TOTP")
	flag.Parse()

	if *clientCode == "" || *apiKey == "" || *password == "" || *totp == "" {
		fmt.Println("All flags are required:")
		flag.PrintDefaults()
		return
	}

	broker, err := brokers.NewBroker("angelone")

	if err != nil {
		fmt.Println("error")
	}
	sess, err := broker.NewSession(*clientCode, *apiKey, *password, *totp)
	if err != nil {
		fmt.Println("Login error:", err)
		return
	}

	if sess.Success {
		fmt.Println("login successful")
		fmt.Println("Access token : ", sess.Data.AccessToken)
		fmt.Println("Feed token : ", sess.Data.FeedToken)
		broker.SetAccessToken(sess.Data.AccessToken)
		broker.SetFeedToken(sess.Data.FeedToken)
	}

	profile, err := broker.GetUserProfile()
	if err != nil {
		fmt.Println("Profile error:", err)
		return
	}
	fmt.Println("Profile:", profile)

	holdings, err := broker.GetHoldings()
	if err != nil {
		fmt.Println("Holdings error:", err)
		return
	}
	fmt.Println("Holdings:", holdings)

	quotes, err := broker.GetMarketQuote(models.QuoteModeFull, map[models.Exchange][]string{
		models.ExchangeNSE: {"3045"},
	})

	if err != nil {
		fmt.Println("Quotes error:", err)
	}
	fmt.Println("Quotes:", quotes)

	data, err := broker.GetHistoricalData(models.ExchangeNSE, "1023", models.Timeframe1Day, "2026-07-15 09:00", "2026-07-18 15:30")
	if err == nil {
		fmt.Println(data.Data.Candles)
		fmt.Println(data.Data.OI)
	}

	// // ────────── Worker-Pool Batch Historical Data Demo ──────────
	// // Type-assert to access the batch method (not on the Broker interface)
	// if angelBroker, ok := broker.(*angelone.Angelone); ok {
	// 	batchRequests := []angelone.SymbolRequest{
	// 		{
	// 			Exchange:    models.ExchangeNSE,
	// 			SymbolToken: "3045", // SBI
	// 			Interval:    models.Timeframe1Day,
	// 			FromDate:    "2026-01-01 09:00",
	// 			ToDate:      "2026-07-15 15:30", // > 2000 days → auto-split into batches
	// 		},
	// 		{
	// 			Exchange:    models.ExchangeNSE,
	// 			SymbolToken: "16675", // CANBK (example)
	// 			Interval:    models.Timeframe1Day,
	// 			FromDate:    "2026-01-01 09:00",
	// 			ToDate:      "2026-06-30 15:30",
	// 		},
	// 		{
	// 			Exchange:    models.ExchangeNSE,
	// 			SymbolToken: "1594", // RELIANCE (example)
	// 			Interval:    models.Timeframe1Day,
	// 			FromDate:    "2026-01-01 09:00",
	// 			ToDate:      "2026-06-30 15:30",
	// 		},
	// 	}

	// 	fmt.Println("\n========== Batch Historical Data (Worker Pool) ==========")
	// 	fmt.Printf("Dispatching %d symbols with automatic date-range batching...\n", len(batchRequests))

	// 	start := time.Now()
	// 	batchResp, err := angelBroker.FetchHistoricalDataBatch(batchRequests)
	// 	elapsed := time.Since(start)

	// 	if err != nil {
	// 		fmt.Printf("Batch fetch error: %v\n", err)
	// 	} else {
	// 		fmt.Printf("Completed in %v | Success=%v | Message=%s\n", elapsed, batchResp.Success, batchResp.Message)
	// 		for _, item := range batchResp.Data {
	// 			if item.Error != "" {
	// 				fmt.Printf("  ❌ %s: %s\n", item.SymbolToken, item.Error)
	// 				continue
	// 			}
	// 			fmt.Printf("  ✅ %s: %d candles, %d OI records | success=%v broker=%s\n",
	// 				item.SymbolToken, len(item.Data.Candles), len(item.Data.OI), item.Success, item.Broker)
	// 			if len(item.Data.Candles) > 0 {
	// 				fmt.Printf("     First: %s O=%v H=%v L=%v C=%v V=%d\n",
	// 					item.Data.Candles[0].Timestamp,
	// 					item.Data.Candles[0].Open,
	// 					item.Data.Candles[0].High,
	// 					item.Data.Candles[0].Low,
	// 					item.Data.Candles[0].Close,
	// 					item.Data.Candles[0].Volume,
	// 				)
	// 				fmt.Printf("     Last:  %s O=%v H=%v L=%v C=%v V=%d\n",
	// 					item.Data.Candles[len(item.Data.Candles)-1].Timestamp,
	// 					item.Data.Candles[len(item.Data.Candles)-1].Open,
	// 					item.Data.Candles[len(item.Data.Candles)-1].High,
	// 					item.Data.Candles[len(item.Data.Candles)-1].Low,
	// 					item.Data.Candles[len(item.Data.Candles)-1].Close,
	// 					item.Data.Candles[len(item.Data.Candles)-1].Volume,
	// 				)
	// 			}
	// 		}
	// 	}
	// } else {
	// 	fmt.Println("Note: broker is not an *angelone.Angelone — skipping batch demo")
	// }
	// // ─────────────────────────────────────────────────────────────

	// positions, err := broker.GetPositions()
	// if err == nil {
	// 	for _, p := range positions.Data {
	// 		fmt.Printf("%s: Buy %d @ %.2f\n", p.TradingSymbol, p.BuyQty, p.BuyAvgPrice)
	// 	}
	// }

	// ---------- WebSocket Example (subscribe to SBI token 3045 on NSE) ----------
	checkWebsocket(broker)
}
