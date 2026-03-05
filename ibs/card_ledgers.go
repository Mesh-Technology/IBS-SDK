package ibs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Ledger represents a single card transaction ledger entry.
type Ledger struct {
	TransactionDate     time.Time
	Amount              float64
	Currency            string
	Category            string
	Description         string
	ResultingBalance    float64
	TransactionAmount   float64
	TransactionCurrency string
}

// Ledgers represents a paginated list of ledger entries.
type Ledgers struct {
	CurrentPage int64
	TotalPage   int64
	Ledgers     []Ledger
}

type getLedgersResponse struct {
	Status bool `json:"status"`
	Data   struct {
		CardID  string `json:"card_id"`
		CurPage int64  `json:"cur_page"`
		MaxPage int64  `json:"max_page"`
		Ledgers []struct {
			Amount              float64 `json:"amount"`
			Currency            string  `json:"currency"`
			Category            string  `json:"category"`
			Description         string  `json:"description"`
			TransactionDate     int64   `json:"transaction_date"`
			ResultingBalance    float64 `json:"resulting_balance"`
			TransactionAmount   float64 `json:"transaction_amount"`
			TransactionCurrency string  `json:"transaction_currency"`
		} `json:"ledgers"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
}

// GetCardLedgers retrieves paginated ledger entries for the current card context.
func (c *Client) GetCardLedgers(pageNumber string) (Ledgers, error) {
	endpoint := fmt.Sprintf("/card/ledgers/%s/%s?page=%s",
		url.PathEscape(c.userID),
		url.PathEscape(c.cardID),
		url.QueryEscape(pageNumber),
	)

	respBody, err := c.requestAPI(http.MethodGet, endpoint, nil, true)
	if err != nil {
		return Ledgers{}, err
	}

	var responseMap getLedgersResponse
	if err := json.Unmarshal(respBody, &responseMap); err != nil {
		return Ledgers{}, fmt.Errorf("ibs: decode ledgers response: %w", err)
	}

	ledgers := make([]Ledger, 0, len(responseMap.Data.Ledgers))
	for _, ledger := range responseMap.Data.Ledgers {
		ledgers = append(ledgers, Ledger{
			TransactionDate:     time.Unix(ledger.TransactionDate, 0),
			Amount:              ledger.Amount,
			Currency:            ledger.Currency,
			Category:            ledger.Category,
			Description:         ledger.Description,
			ResultingBalance:    ledger.ResultingBalance,
			TransactionAmount:   ledger.TransactionAmount,
			TransactionCurrency: ledger.TransactionCurrency,
		})
	}

	return Ledgers{
		CurrentPage: responseMap.Data.CurPage,
		TotalPage:   responseMap.Data.MaxPage,
		Ledgers:     ledgers,
	}, nil
}
