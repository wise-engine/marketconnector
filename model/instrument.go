package model

// AngeloneRawInstrument mirrors a single row of the Angel One OpenAPIScripMaster
// instrument list. Values are kept as strings because the source JSON encodes
// numeric fields as strings.
type AngeloneRawInstrument struct {
	Token          string `json:"token"`
	Symbol         string `json:"symbol"`
	Name           string `json:"name"`
	Expiry         string `json:"expiry"`
	Strike         string `json:"strike"`
	Lotsize        string `json:"lotsize"`
	InstrumentType string `json:"instrumenttype"`
	ExchSeg        string `json:"exch_seg"`
	TickSize       string `json:"tick_size"`
}

// ZerodhaRawInstrument mirrors a single row of the Zerodha Kite instruments
// CSV.
type ZerodhaRawInstrument struct {
	InstrumentToken string `json:"instrument_token"`
	ExchangeToken   string `json:"exchange_token"`
	Tradingsymbol   string `json:"tradingsymbol"`
	Name            string `json:"name"`
	LastPrice       string `json:"last_price"`
	Expiry          string `json:"expiry"`
	Strike          string `json:"strike"`
	TickSize        string `json:"tick_size"`
	LotSize         string `json:"lot_size"`
	InstrumentType  string `json:"instrument_type"`
	Segment         string `json:"segment"`
	Exchange        string `json:"exchange"`
}

// ProcessedInstrument is the standardized, broker-agnostic instrument row
// produced by the broker GetProcessed helpers.
type ProcessedInstrument struct {
	SymbolID         string  `json:"symbolId"`
	BrokerToken      string  `json:"brokerToken"`
	TradingSymbol    string  `json:"tradingSymbol"`
	Exchange         string  `json:"exchange"`
	Name             string  `json:"name"`
	Expiry           string  `json:"expiry"`
	Strike           float64 `json:"strike"`
	TickSize         string  `json:"tickSize"`
	Lot              string  `json:"lot"`
	InstrumentType   string  `json:"instrumentType"`
	Tradeable        string  `json:"tradeable"`
	OldTradingsymbol string  `json:"old_tradingsymbol"`
	ActualSymbolName string  `json:"actualSymbolName"`
}
