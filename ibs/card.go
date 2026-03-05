package ibs

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

// Card represents an IBS card with all associated metadata.
type Card struct {
	CardID string
	BankID string
	UserID string

	UserEmail    string
	UserFullName string

	CardNumber  string
	Cvv         string
	ExpiryMonth string
	ExpiryYear  string

	Type        string
	Balance     float64
	Currency    string
	PhoneNumber string

	Enabled bool
	ATM     bool

	CreatedAt time.Time
}

func mapCards(raw []getCardsResponseCard) []Card {
	cards := make([]Card, 0, len(raw))
	for _, c := range raw {
		cards = append(cards, cardFromResponse(c))
	}
	return cards
}

func cardFromResponse(c getCardsResponseCard) Card {
	return Card{
		CardID: c.CardID,
		UserID: c.UserID,
		BankID: c.BankID,

		UserEmail:    c.UserEmail,
		UserFullName: c.UserFullName,

		CardNumber:  c.CardNumber,
		Cvv:         c.Cvv,
		ExpiryMonth: c.ExpireMonth,
		ExpiryYear:  c.ExpireYear,

		Type:        c.Type,
		Balance:     c.Balance,
		Currency:    c.Currency,
		PhoneNumber: c.PhoneNumber,

		Enabled: c.Enabled,
		ATM:     c.ATM,

		CreatedAt: time.Unix(c.CreatedAt, 0),
	}
}

// getCardsResponseCard is the JSON shape of a single card inside getCardsResponse.
// Extracted so mapping helpers can reference it by name.
type getCardsResponseCard struct {
	CardID string `json:"card_id"`
	BankID string `json:"bank_id"`
	UserID string `json:"user_id"`

	UserEmail    string `json:"user_email"`
	UserFullName string `json:"user_full_name"`

	CardNumber  string `json:"card_number"`
	Cvv         string `json:"cvv"`
	ExpireMonth string `json:"expire_month"`
	ExpireYear  string `json:"expire_year"`

	Type        string  `json:"type"`
	Balance     float64 `json:"balance"`
	Currency    string  `json:"currency"`
	PhoneNumber string  `json:"phone_number"`

	Enabled bool `json:"enabled"`
	ATM     bool `json:"atm"`

	CreatedAt int64 `json:"createdAt"`
}

// getCardsResponseTyped uses the extracted card struct so mapping helpers work.
type getCardsResponseTyped struct {
	Status bool                   `json:"status"`
	Cards  []getCardsResponseCard `json:"cards"`
	Error  string                 `json:"error,omitempty"`
}

// GetCardInfo retrieves card information for the current user/card context.
func (c *Client) GetCardInfo() ([]Card, error) {
	endpoint := "/card/get"
	if c.userID != "" {
		endpoint += "/" + url.PathEscape(c.userID)
	}
	if c.cardID != "" {
		endpoint += "/" + url.PathEscape(c.cardID)
	}

	respBody, err := c.requestAPI(http.MethodGet, endpoint, nil, true)
	if err != nil {
		return nil, err
	}

	var resp getCardsResponseTyped
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	return mapCards(resp.Cards), nil
}
