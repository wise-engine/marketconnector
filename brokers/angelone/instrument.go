package angelone

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
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

	"github.com/wise-engine/marketconnector/brokers/angelone/internal/util"
	"github.com/wise-engine/marketconnector/model"
)

const instrumentsURL = "https://margincalculator.angelbroking.com/OpenAPI_File/files/OpenAPIScripMaster.json"

const DefaultSavePath = "db/inputs/angelone_instruments.csv"

var instrumentColumns = []string{
	"token", "symbol", "name", "expiry", "strike", "lotsize",
	"instrumenttype", "exch_seg", "tick_size",
}

func GetInstruments(ctx context.Context, savePath string) error {
	if savePath == "" {
		savePath = DefaultSavePath
	}

	content, err := download(ctx, instrumentsURL)
	if err != nil {
		slog.Error("angelone: fetching instrument list", "error", err)
		return err
	}

	if err := os.MkdirAll(filepath.Dir(savePath), 0o755); err != nil {
		slog.Error("angelone: creating output directory", "path", filepath.Dir(savePath), "error", err)
		return err
	}

	// The endpoint returns JSON; convert it to CSV. If it is not JSON (e.g.
	// already CSV), write the raw payload as-is.
	out := content
	if csvBytes, err := instrumentsToCSV(content); err == nil {
		out = csvBytes
	} else {
		slog.Warn("angelone: response is not JSON, writing raw payload", "path", savePath, "error", err)
	}

	if err := os.WriteFile(savePath, out, 0o644); err != nil {
		slog.Error("angelone: writing instrument file", "path", savePath, "error", err)
		return err
	}

	slog.Info("angelone: instruments saved to disk", "path", savePath)
	return nil
}

func instrumentsToCSV(raw []byte) ([]byte, error) {
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if err := w.Write(instrumentColumns); err != nil {
		return nil, fmt.Errorf("writing CSV header: %w", err)
	}

	for _, row := range rows {
		record := make([]string, len(instrumentColumns))
		for i, col := range instrumentColumns {
			record[i] = rawString(row[col])
		}
		if err := w.Write(record); err != nil {
			return nil, fmt.Errorf("writing CSV row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flushing CSV: %w", err)
	}
	return buf.Bytes(), nil
}

func rawString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
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

// GetRawInstruments downloads and returns the raw Angel One instrument list (the
// OpenAPIScripMaster JSON decoded into []model.AngeloneRawInstrument).
func (a *Angelone) GetRawInstruments() (*model.Response[[]model.AngeloneRawInstrument], error) {
	content, err := download(context.Background(), instrumentsURL)
	if err != nil {
		slog.Error("angelone: fetching raw instrument list", "error", err)
		return nil, err
	}
	instruments, err := ParseAngeloneInstruments(content)
	if err != nil {
		return nil, err
	}
	return &model.Response[[]model.AngeloneRawInstrument]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "angelone",
		Data:    instruments,
	}, nil
}

// ParseAngeloneInstruments decodes the OpenAPIScripMaster JSON payload into raw
// instrument rows. Numeric fields are normalized to strings.
func ParseAngeloneInstruments(content []byte) ([]model.AngeloneRawInstrument, error) {
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(content, &rows); err != nil {
		return nil, fmt.Errorf("angelone: parsing instrument JSON: %w", err)
	}

	instruments := make([]model.AngeloneRawInstrument, 0, len(rows))
	for _, row := range rows {
		instruments = append(instruments, model.AngeloneRawInstrument{
			Token:          rawString(row["token"]),
			Symbol:         rawString(row["symbol"]),
			Name:           rawString(row["name"]),
			Expiry:         rawString(row["expiry"]),
			Strike:         rawString(row["strike"]),
			Lotsize:        rawString(row["lotsize"]),
			InstrumentType: rawString(row["instrumenttype"]),
			ExchSeg:        rawString(row["exch_seg"]),
			TickSize:       rawString(row["tick_size"]),
		})
	}
	return instruments, nil
}

// GetProcessedInstruments downloads the raw Angel One instrument list and
// converts it to the standardized form, returning the processed instruments.
func (a *Angelone) GetProcessedInstruments() (*model.Response[[]model.ProcessedInstrument], error) {
	raw, err := a.GetRawInstruments()
	if err != nil {
		return nil, err
	}
	return &model.Response[[]model.ProcessedInstrument]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "angelone",
		Data:    ProcessAngeloneInstruments(raw.Data),
	}, nil
}

// ProcessAngeloneInstruments applies the Angel One processing rules (exchange
// filter, instrument-type inference and trading symbol formatting) to raw
// instrument rows.
func ProcessAngeloneInstruments(raw []model.AngeloneRawInstrument) []model.ProcessedInstrument {
	allowed := map[string]bool{"NSE": true, "NFO": true, "BSE": true, "BFO": true, "MCX": true}

	processed := make([]model.ProcessedInstrument, 0, len(raw))
	for _, r := range raw {
		if !allowed[r.ExchSeg] {
			continue
		}
		instType := angeloneInstrumentType(r)
		if instType == "" {
			continue
		}

		tradeable := "Y"
		if instType == "INDEX" {
			tradeable = "N"
		}

		processed = append(processed, model.ProcessedInstrument{
			SymbolID:         "",
			BrokerToken:      r.Token,
			TradingSymbol:    formatAngeloneTradingSymbol(r, instType),
			Exchange:         r.ExchSeg,
			Name:             html.UnescapeString(r.Name),
			Expiry:           r.Expiry,
			Strike:           util.ParseFloat64(strings.TrimSpace(r.Strike)) / 100,
			TickSize:         r.TickSize,
			Lot:              r.Lotsize,
			InstrumentType:   instType,
			Tradeable:        tradeable,
			OldTradingsymbol: r.Symbol,
			ActualSymbolName: r.Symbol,
		})
	}
	return processed
}

// angeloneInstrumentType infers the standardized instrument type (INDEX/EQ/
// FUT/OPT) from an Angel One raw row, or returns "" if the row should be
// dropped.
func angeloneInstrumentType(r model.AngeloneRawInstrument) string {
	exchSeg := strings.ToUpper(r.ExchSeg)
	instType := strings.ToUpper(r.InstrumentType)
	lotSize := util.ParseFloat64(strings.TrimSpace(r.Lotsize))
	tick := util.ParseFloat64(strings.TrimSpace(r.TickSize))
	symbol := strings.ToUpper(r.Symbol)

	// INDEX if instrumenttype = AMXIDX
	if instType == "AMXIDX" {
		return "INDEX"
	}

	// EQ if NSE/BSE and instrumenttype is empty and lotsize = 1 and tick > 0
	// and the symbol does not end with -MF.
	isInstTypeEmpty := instType == "" || instType == "NAN"
	if (exchSeg == "NSE" || exchSeg == "BSE") && isInstTypeEmpty && lotSize == 1 && tick > 0 && !strings.HasSuffix(symbol, "-MF") {
		return "EQ"
	}

	// OPT if BFO/NFO/MCX and instrumenttype starts with OPT.
	if (exchSeg == "BFO" || exchSeg == "NFO" || exchSeg == "MCX") && strings.HasPrefix(instType, "OPT") {
		return "OPT"
	}

	// FUT if instrumenttype starts with FUT.
	if strings.HasPrefix(instType, "FUT") {
		return "FUT"
	}

	return ""
}

// formatAngeloneTradingSymbol builds the standardized trading symbol for an
// Angel One row.
func formatAngeloneTradingSymbol(r model.AngeloneRawInstrument, instType string) string {
	symbol := strings.ToUpper(r.Symbol)
	name := strings.ToUpper(r.Name)

	switch instType {
	case "FUT":
		// name + expiry date (DDMONYYYY) + FUT, e.g. HAVELLS28JUL2026FUT
		return name + strings.ToUpper(formatExpiry(r.Expiry, "02Jan2006")) + "FUT"
	case "OPT":
		// name + strike/100 + CE|PE + expiry date (DDMonYYYY), e.g. EICHERMOT6600PE26May2026
		strike := util.ParseFloat64(strings.TrimSpace(r.Strike)) / 100
		optType := ""
		switch {
		case strings.HasSuffix(symbol, "CE"):
			optType = "CE"
		case strings.HasSuffix(symbol, "PE"):
			optType = "PE"
		}
		return name + formatStrike(strike) + optType + formatExpiry(r.Expiry, "02Jan2006")
	case "EQ":
		// Strip the -EQ suffix on NSE, e.g. RELIANCE-EQ -> RELIANCE.
		if r.ExchSeg == "NSE" {
			return strings.TrimSuffix(symbol, "-EQ")
		}
		return symbol
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
