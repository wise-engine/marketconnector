package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/sunnyme20/marketconnector"
	"github.com/sunnyme20/marketconnector/model"
	"github.com/xlzd/gotp"
)

func checkWebsocket(broker marketconnector.Broker) {
	ws, err := broker.GetWebSocket()
	if err != nil {
		fmt.Println("GetWebSocket error:", err)
		return
	}

	ws.OnConnect(func() {
		fmt.Println("WebSocket connected")

		// Subscribe to SBI (token 3045) on NSE CM in LTP mode
		err := ws.Subscribe(int(model.ModeLTP), []model.WSTokenGroup{
			{
				ExchangeType: int(model.WSExchangeNseCM),
				Tokens:       []string{"3045"},
			},
		})
		if err != nil {
			fmt.Println("Subscribe error:", err)
		}
	})

	ws.OnTick(func(tick model.MarketQuoteResponse) {
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
	// Load .env if present (e.g. CLIENT_CODE, API_KEY, PASSWORD, TOTP_SECRET).
	_ = godotenv.Load()

	clientCode := flag.String("client", os.Getenv("CLIENT_CODE"), "Client code")
	apiKey := flag.String("apikey", os.Getenv("API_KEY"), "API key")
	password := flag.String("password", os.Getenv("PASSWORD"), "Password")
	totp := flag.String("totp", os.Getenv("TOTP"), "TOTP code")
	totpCode := flag.String("totpcode", os.Getenv("TOTP_SECRET"), "TOTP secret to generate the code")
	ws := flag.Bool("ws", false, "Run the WebSocket streaming example")

	flag.Parse()

	if *totp == "" && *totpCode != "" {
		totpAuto := gotp.NewDefaultTOTP(*totpCode)
		*totp = totpAuto.Now()
	}

	if *clientCode == "" || *apiKey == "" || *password == "" || *totp == "" {
		fmt.Println("client, apikey, password and totp (or TOTP_SECRET) are required — pass them as flags or in a .env file:")
		flag.PrintDefaults()
		return
	}

	broker, err := marketconnector.NewBroker(model.BrokerAngelOne)

	if err != nil {
		fmt.Println("error:", err)
		return
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

	quotes, err := broker.GetMarketQuote(model.QuoteModeFull, map[model.Exchange][]string{
		model.ExchangeNSE: {"3045"},
	})

	if err != nil {
		fmt.Println("Quotes error:", err)
	}
	fmt.Println("Quotes:", quotes)

	data, err := broker.GetHistoricalData(model.ExchangeNSE, "3045", model.Timeframe30Minutes, "2026-07-15 09:00", "2026-07-20 15:30")
	if err == nil {
		for _, d := range data.Data.Candles {
			fmt.Println(d.Timestamp, d.Close, d.Volume)
		}
	}

	if *ws {
		checkWebsocket(broker)
	}
}
