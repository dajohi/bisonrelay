package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/decred/slog"
)

type Config struct {
	HTTPClient *http.Client
	Log        slog.Logger
}

type Rates struct {
	cfg Config

	mtx      sync.Mutex
	dcrPrice float64
	btcPrice float64
}

func New(cfg Config) *Rates {
	return &Rates{
		cfg: cfg,
	}
}

func (r *Rates) Run(ctx context.Context) {
	const shortTimeout = time.Second * 30
	const longTimeout = time.Minute * 10
	const triesBeforeErr = 20
	const requestTimeout = shortTimeout

	t := time.NewTicker(1)
	defer t.Stop()

	var failedTries int

	var err error
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			t.Stop()

			// Try from dcrdata.
			rctx, cancel := context.WithTimeout(ctx, requestTimeout)
			if err = r.dcrData(rctx); err == nil {
				cancel()
				failedTries = 0
				t.Reset(longTimeout)
				continue
			}
			cancel()
			r.cfg.Log.Debugf("Unable to fetch rate from dcrdata: %v", err)

			// Try from bittrex.
			rctx, cancel = context.WithTimeout(ctx, requestTimeout)
			if err = r.bittrex(rctx); err == nil {
				cancel()
				failedTries = 0
				t.Reset(longTimeout)
				continue
			}
			cancel()
			r.cfg.Log.Debugf("Unable to fetch rate from bittrex: %v", err)

			// Only log these at a higher warning level once after
			// the rate has been successfully fetched. This prevents
			// spam in the UI.
			failedTries++
			if failedTries == triesBeforeErr {
				r.cfg.Log.Warnf("Unable to fetch rate from dcrdata: %v", err)
				r.cfg.Log.Warnf("Unable to fetch rate from bittrex: %v", err)
				r.cfg.Log.Errorf("Unable to fetch recent exchange rate. Will keep retrying.")
			}
			t.Reset(shortTimeout)
		}
	}
}

// Get returns the last fetched USD/DCR and USD/BTC prices.
func (r *Rates) Get() (float64, float64) {
	r.mtx.Lock()
	dcrPrice, btcPrice := r.dcrPrice, r.btcPrice
	r.mtx.Unlock()

	return dcrPrice, btcPrice
}

// Set manually sets the USD/DCR and USD/BTC prices.
func (r *Rates) Set(dcrPrice, btcPrice float64) {
	r.cfg.Log.Infof("Setting manual exchange rate: DCR:%0.2f BTC:%0.2f",
		dcrPrice, btcPrice)

	r.mtx.Lock()
	r.dcrPrice = dcrPrice
	r.btcPrice = btcPrice
	r.mtx.Unlock()
}

func (r *Rates) dcrData(ctx context.Context) error {
	dcrDataExchange := struct {
		DCRPrice float64 `json:"dcrPrice"`
		BTCPrice float64 `json:"btcPrice"`
	}{}

	const apiURL = "https://explorer.dcrdata.org/api/exchangerate"
	b, err := r.getRaw(ctx, apiURL)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &dcrDataExchange); err != nil {
		return fmt.Errorf("failed to decode exchange rate: %v", err)
	}

	r.cfg.Log.Infof("Current dcrdata exchange rate: DCR:%0.2f BTC:%0.2f",
		dcrDataExchange.DCRPrice, dcrDataExchange.BTCPrice)

	r.mtx.Lock()
	r.dcrPrice = dcrDataExchange.DCRPrice
	r.btcPrice = dcrDataExchange.BTCPrice
	r.mtx.Unlock()

	return nil
}

func (r *Rates) bittrex(ctx context.Context) error {
	bittrexExchange := struct {
		LastTradeRate string `json:"lastTradeRate"`
	}{}

	const dcrAPI = "https://api.bittrex.com/v3/markets/DCR-USD/ticker"
	b, err := r.getRaw(ctx, dcrAPI)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &bittrexExchange); err != nil {
		return fmt.Errorf("failed to decode exchange rate: %w", err)
	}
	dcrPrice, err := strconv.ParseFloat(bittrexExchange.LastTradeRate, 64)
	if err != nil {
		return fmt.Errorf("failed to parse exchange rate: %w", err)
	}

	const btcAPI = "https://api.bittrex.com/v3/markets/BTC-USDT/ticker"
	b, err = r.getRaw(ctx, btcAPI)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &bittrexExchange); err != nil {
		return fmt.Errorf("failed to decode exchange rate: %v", err)
	}
	btcPrice, err := strconv.ParseFloat(bittrexExchange.LastTradeRate, 64)
	if err != nil {
		return fmt.Errorf("failed to parse exchange rate: %w", err)
	}

	r.cfg.Log.Infof("Current bittrex exchange rate: DCR:%0.2f BTC:%0.2f",
		dcrPrice, btcPrice)

	r.mtx.Lock()
	r.dcrPrice = dcrPrice
	r.btcPrice = btcPrice
	r.mtx.Unlock()

	return nil
}

func (r *Rates) getRaw(ctx context.Context, exchangeAPI string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		exchangeAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new http request: %v", err)
	}
	req.Header.Del("User-Agent")

	resp, err := r.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange rate: %v", err)
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read exchange rate response: %v", err)
	}
	return b, nil
}
