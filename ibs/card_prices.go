package ibs

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
)

type pricesResponse struct {
	Status bool `json:"status"`
	Data   struct {
		Total  int     `json:"total"`
		Prices []Price `json:"prices"`
	} `json:"data"`
}

// Price represents pricing information for a card from a specific bank.
type Price struct {
	BankID   string `json:"bank_id"`
	BankName string `json:"bank_name"`
	CardType string `json:"card_type"`

	Currency string  `json:"currency"`
	Price    float64 `json:"price"`

	CashBack         float64 `json:"cash_back"`
	CashBackCurrency string  `json:"cash_back_currency"`

	CommissionPercent float64 `json:"commission_percent"`

	CurrencyPrice float64 `json:"currency_price"`

	MinTopup         float64 `json:"min_topup"`
	MinTopupCurrency string  `json:"min_topup_currency"`
}

// Prices retrieves pricing information for a given bank and card type.
func (c *Client) Prices(bankID, cardType string) (*Price, error) {
	query := url.Values{}
	query.Set("bank_id", bankID)
	query.Set("card_type", cardType)

	respBody, err := c.requestAPI(
		http.MethodGet,
		"/card/prices?"+query.Encode(),
		nil,
		true)
	if err != nil {
		return nil, err
	}

	var responseMap pricesResponse
	if err := json.Unmarshal(respBody, &responseMap); err != nil {
		return nil, err
	}

	if len(responseMap.Data.Prices) == 0 {
		return nil, errors.New("ibs: no prices found")
	}

	return &responseMap.Data.Prices[0], nil
}
