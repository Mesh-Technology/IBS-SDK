package ibs

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
)

type CardPriceListQuery struct {
	BankID   string `form:"bank_id"`
	CardType string `form:"card_type"`
	Currency string `form:"currency"`
}

type cardPriceResponse struct {
	BankID            string  `json:"bank_id"`
	BankName          string  `json:"bank_name"`
	CardType          string  `json:"card_type"`
	Currency          string  `json:"currency"`
	Price             float64 `json:"price"`
	CashBack          float64 `json:"cash_back"`
	CashBackCurrency  string  `json:"cash_back_currency"`
	CommissionPercent float64 `json:"commission_percent"`
	CurrencyPrice     float64 `json:"currency_price"`
	MinTopup          float64 `json:"min_topup"`
	MinTopupCurrency  string  `json:"min_topup_currency"`
	PoolAvailable     int64   `json:"pool_available"`
}

type availableBankResponse struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	LogoURL  string `json:"logo_url"`
	Region   string `json:"region"`
	Currency string `json:"currency"`
}

type poolRecommendation struct {
	BankID            string  `json:"bank_id"`
	BankName          string  `json:"bank_name"`
	CardType          string  `json:"card_type"`
	Currency          string  `json:"currency"`
	Price             float64 `json:"price"`
	CashBack          float64 `json:"cash_back"`
	CashBackCurrency  string  `json:"cash_back_currency"`
	CommissionPercent float64 `json:"commission_percent"`
	CurrencyPrice     float64 `json:"currency_price"`
	MinTopup          float64 `json:"min_topup"`
	MinTopupCurrency  string  `json:"min_topup_currency"`
	PoolAvailable     int64   `json:"pool_available"`
}

// CardPriceList holds the combined result of prices, available banks, and the
// pool recommendation returned by the API.
type CardPriceList struct {
	Prices []cardPriceResponse     `json:"prices"`
	Banks  []availableBankResponse `json:"banks"`
	Pool   *poolRecommendation     `json:"pool"`
}

type cardPriceListResponse struct {
	Status bool          `json:"status"`
	Data   CardPriceList `json:"data"`
}

// Prices retrieves card pricing information, available banks, and the pool
// recommendation filtered by the provided query parameters.
func (c *Client) Prices(q CardPriceListQuery) (*CardPriceList, error) {
	query := url.Values{}
	if q.BankID != "" {
		query.Set("bank_id", q.BankID)
	}
	if q.CardType != "" {
		query.Set("card_type", q.CardType)
	}
	if q.Currency != "" {
		query.Set("currency", q.Currency)
	}

	respBody, err := c.requestAPI(
		http.MethodGet,
		"/card/prices?"+query.Encode(),
		nil,
		true,
	)
	if err != nil {
		return nil, err
	}

	var responseMap cardPriceListResponse
	if err := json.Unmarshal(respBody, &responseMap); err != nil {
		return nil, err
	}

	if len(responseMap.Data.Prices) == 0 {
		return nil, errors.New("ibs: no prices found")
	}

	return &responseMap.Data, nil
}
