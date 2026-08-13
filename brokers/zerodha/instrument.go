package zerodha

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wise-engine/marketconnector/brokers/zerodha/internal/util"
	"github.com/wise-engine/marketconnector/model"
)

const instrumentsURL = "https://api.kite.trade/instruments"

const DefaultSavePath = "db/inputs/zerodha_instruments.csv"

// GetInstruments downloads the Zerodha instrument list from the public API and
// saves it (as CSV) to savePath. A nil error means the download succeeded.
func GetInstruments(ctx context.Context, savePath string) error {
	if savePath == "" {
		savePath = DefaultSavePath
	}

	content, err := download(ctx, instrumentsURL)
	if err != nil {
		slog.Error("zerodha: fetching instrument list", "error", err)
		return err
	}

	if err := os.MkdirAll(filepath.Dir(savePath), 0o755); err != nil {
		slog.Error("zerodha: creating output directory", "path", filepath.Dir(savePath), "error", err)
		return err
	}

	if err := os.WriteFile(savePath, content, 0o644); err != nil {
		slog.Error("zerodha: writing instrument file", "path", savePath, "error", err)
		return err
	}

	slog.Info("zerodha: instruments saved to disk", "path", savePath)
	return nil
}

func download(ctx context.Context, url string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", url, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %q from %s", resp.Status, url)
	}

	// Peek at the first two bytes to detect a gzip stream (0x1f 0x8b).
	br := bufio.NewReader(resp.Body)
	head, err := br.Peek(2)
	if err == nil && len(head) == 2 && head[0] == 0x1f && head[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return nil, fmt.Errorf("initializing gzip reader: %w", err)
		}
		defer gz.Close()

		body, err := io.ReadAll(gz)
		if err != nil {
			return nil, fmt.Errorf("decompressing response: %w", err)
		}
		return body, nil
	}

	body, err := io.ReadAll(br)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	return body, nil
}

// GetRawInstruments downloads and returns the raw Zerodha instrument list (the public Kite
// instruments CSV decoded into []model.ZerodhaRawInstrument).
func (z *Zerodha) GetRawInstruments(ctx context.Context) (*model.Response[[]model.ZerodhaRawInstrument], error) {
	content, err := download(ctx, instrumentsURL)
	if err != nil {
		slog.Error("zerodha: fetching raw instrument list", "error", err)
		return nil, err
	}
	instruments, err := ParseZerodhaInstruments(content)
	if err != nil {
		return nil, err
	}
	return &model.Response[[]model.ZerodhaRawInstrument]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "zerodha",
		Data:    instruments,
	}, nil
}

// ParseZerodhaInstruments decodes the Kite instruments CSV payload into raw
// instrument rows, mapping columns by header name.
func ParseZerodhaInstruments(content []byte) ([]model.ZerodhaRawInstrument, error) {
	r := csv.NewReader(bytes.NewReader(content))

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("zerodha: reading instrument CSV header: %w", err)
	}
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[strings.TrimSpace(h)] = i
	}

	var instruments []model.ZerodhaRawInstrument
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("zerodha: reading instrument CSV row: %w", err)
		}

		field := func(name string) string {
			if i, ok := idx[name]; ok && i < len(rec) {
				return rec[i]
			}
			return ""
		}

		instruments = append(instruments, model.ZerodhaRawInstrument{
			InstrumentToken: field("instrument_token"),
			ExchangeToken:   field("exchange_token"),
			Tradingsymbol:   field("tradingsymbol"),
			Name:            field("name"),
			LastPrice:       field("last_price"),
			Expiry:          field("expiry"),
			Strike:          field("strike"),
			TickSize:        field("tick_size"),
			LotSize:         field("lot_size"),
			InstrumentType:  field("instrument_type"),
			Segment:         field("segment"),
			Exchange:        field("exchange"),
		})
	}
	return instruments, nil
}

// GetProcessed downloads the raw Zerodha instrument list and converts it to the
// standardized form, returning the processed instruments.
func (z *Zerodha) GetProcessed(ctx context.Context) (*model.Response[[]model.ProcessedInstrument], error) {
	raw, err := z.GetRawInstruments(ctx)
	if err != nil {
		return nil, err
	}
	return &model.Response[[]model.ProcessedInstrument]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "zerodha",
		Data:    ProcessZerodhaInstruments(raw.Data),
	}, nil
}

// ProcessZerodhaInstruments applies the Zerodha processing rules (exchange
// filter, instrument-type inference and trading symbol formatting) to raw
// instrument rows.
func ProcessZerodhaInstruments(raw []model.ZerodhaRawInstrument) []model.ProcessedInstrument {
	allowed := map[string]bool{"NSE": true, "NFO": true, "BSE": true, "BFO": true, "MCX": true}

	processed := make([]model.ProcessedInstrument, 0, len(raw))
	for _, r := range raw {
		if !allowed[r.Exchange] {
			continue
		}
		instType := zerodhaInstrumentType(r)
		if instType == "" {
			continue
		}

		tradeable := "Y"
		if instType == "INDEX" {
			tradeable = "N"
		}

		processed = append(processed, model.ProcessedInstrument{
			SymbolID:         "",
			BrokerToken:      r.InstrumentToken,
			TradingSymbol:    formatZerodhaTradingSymbol(r, instType),
			Exchange:         r.Exchange,
			Name:             html.UnescapeString(r.Name),
			Expiry:           r.Expiry,
			Strike:           util.ParseFloat64(strings.TrimSpace(r.Strike)),
			TickSize:         r.TickSize,
			Lot:              r.LotSize,
			InstrumentType:   instType,
			Tradeable:        tradeable,
			OldTradingsymbol: r.Tradingsymbol,
			ActualSymbolName: r.Tradingsymbol,
		})
	}
	return processed
}

// zerodhaInstrumentType infers the standardized instrument type (INDEX/EQ/
// FUT/OPT) from a Zerodha raw row, or returns "" if the row should be dropped.
func zerodhaInstrumentType(r model.ZerodhaRawInstrument) string {
	segment := strings.ToUpper(r.Segment)
	exchange := strings.ToUpper(r.Exchange)
	instType := strings.ToUpper(r.InstrumentType)
	lotSize := util.ParseFloat64(strings.TrimSpace(r.LotSize))

	// INDEX if segment = INDICES and exchange = NSE/BSE/MCX.
	if segment == "INDICES" && (exchange == "NSE" || exchange == "BSE" || exchange == "MCX") {
		return "INDEX"
	}

	// EQ if segment and exchange are NSE/BSE and lot_size = 1.
	if (segment == "NSE" || segment == "BSE") && (exchange == "NSE" || exchange == "BSE") && lotSize == 1 {
		return "EQ"
	}

	// FUT if exchange = NFO/BFO/MCX and instrument_type = FUT.
	if (exchange == "NFO" || exchange == "BFO" || exchange == "MCX") && instType == "FUT" {
		return "FUT"
	}

	// OPT if exchange = NFO/BFO/MCX and instrument_type = CE or PE.
	if (exchange == "NFO" || exchange == "BFO" || exchange == "MCX") && (instType == "CE" || instType == "PE") {
		return "OPT"
	}

	return ""
}

// formatZerodhaTradingSymbol builds the standardized trading symbol for a
// Zerodha row.
func formatZerodhaTradingSymbol(r model.ZerodhaRawInstrument, instType string) string {
	symbol := strings.ToUpper(r.Tradingsymbol)
	name := strings.ToUpper(r.Name)

	switch instType {
	case "FUT":
		// name + expiry date (DDMONYYYY) + FUT, e.g. HAVELLS28JUL2026FUT
		return name + strings.ToUpper(formatExpiry(r.Expiry, "02Jan2006")) + "FUT"
	case "OPT":
		// name + strike + CE|PE + expiry date (DDMonYYYY), e.g. EICHERMOT6600PE26May2026
		strike := util.ParseFloat64(strings.TrimSpace(r.Strike))
		return name + formatStrike(strike) + strings.ToUpper(r.InstrumentType) + formatExpiry(r.Expiry, "02Jan2006")
	default:
		return symbol
	}
}

// formatExpiry renders an expiry string in the given Go layout. On parse
// failure the original value is returned unchanged.
func formatExpiry(expiry, layout string) string {
	if strings.TrimSpace(expiry) == "" {
		return ""
	}
	t, err := parseInstrumentExpiry(expiry)
	if err != nil {
		return expiry
	}
	return t.Format(layout)
}

// parseInstrumentExpiry parses the expiry formats emitted by the Angel One /
// Zerodha instrument sources.
func parseInstrumentExpiry(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	layouts := []string{
		"2006-01-02",          // 2026-05-26 (Zerodha kite CSV)
		"02-Jan-2006",         // 26-May-2026 (Angel One)
		"02-Jan-06",           // 26-May-26
		"2006-01-02 15:04:05", // full timestamp
		"02-01-2006",          // 26-05-2026
		"01/02/2006",          // 05/26/2026
		"02/01/2006",          // 26/05/2026
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse expiry %q", s)
}

// formatStrike renders a strike price so whole numbers are written without a
// decimal point and fractional values are kept as-is.
func formatStrike(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
