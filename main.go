package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/sunnyme20/marketconnector/brokers"
	models "github.com/sunnyme20/marketconnector/brokers/model"
	"github.com/xlzd/gotp"
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
		fmt.Printf("Tick: %s | LTP: %.2f | Vol: %d | Time: %s\n",
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
	totpCode := flag.String("totpcode", "", "TOTP Code")

	flag.Parse()

	if *totpCode != "" {
		totpAuto := gotp.NewDefaultTOTP(*totpCode)
		*totp = totpAuto.Now()
	}

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
		// fmt.Println("Access token : ", sess.Data.AccessToken)
		// fmt.Println("Feed token : ", sess.Data.FeedToken)
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

	data, err := broker.GetHistoricalData(models.ExchangeNSE, "3045", models.Timeframe30Minutes, "2026-07-15 09:00", "2026-08-01 15:30")
	if err == nil {
		for _, d := range data.Data.Candles {
			fmt.Println(d.Timestamp, d.Close, d.Volume)
		}
	}

	// ---------- WebSocket Example (subscribe to SBI token 3045 on NSE) ----------
	// checkWebsocket(broker)
}
