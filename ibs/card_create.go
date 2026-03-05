package ibs

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Virtual holds the parameters for creating a virtual card.
type Virtual struct {
	BankID       string
	PhoneNumber  string
	UserEmail    string
	UserFullName string
	Description  string
}

// Physical holds the parameters for activating a physical card.
type Physical struct {
	UserEmail   string
	BankID      string
	CardNumber  string
	ExpireMonth string
	ExpireYear  string
	Cvv         string
	PhoneNumber string
	Description string
}

// PendingCardOrder represents a card activation that is pending processing.
type PendingCardOrder struct {
	PendingActivation bool   `json:"pending_activation"`
	PendingCardID     string `json:"pending_card_id,omitempty"`
	OrderID           string `json:"order_id"`
	OrderStatus       string `json:"order_status"`
	Message           string `json:"message,omitempty"`
}

type pendingCardOrderResponse struct {
	Status bool             `json:"status"`
	Data   PendingCardOrder `json:"data"`
}

// VirtualCard creates a new virtual card. It returns either a Card on immediate
// activation or a PendingCardOrder when the activation is queued.
func (c *Client) VirtualCard(data Virtual) (*Card, *PendingCardOrder, error) {
	respBody, err := c.requestAPI(
		http.MethodPost,
		"/card/create/virtual",
		map[string]any{
			"user_id":        c.userID,
			"user_full_name": data.UserFullName,
			"user_email":     data.UserEmail,
			"bank_id":        data.BankID,
			"phone_number":   data.PhoneNumber,
			"description":    data.Description,
		},
		true)
	if err != nil {
		return nil, nil, err
	}

	return parseCardOrPending(respBody)
}

// PhysicalCard activates a physical card. It returns either a Card or a
// PendingCardOrder when the activation requires asynchronous processing.
func (c *Client) PhysicalCard(data Physical) (*Card, *PendingCardOrder, error) {
	respBody, err := c.requestAPI(
		http.MethodPost,
		"/card/create/physical",
		map[string]any{
			"user_id":      c.userID,
			"user_email":   data.UserEmail,
			"bank_id":      data.BankID,
			"card_number":  data.CardNumber,
			"expire_month": data.ExpireMonth,
			"expire_year":  data.ExpireYear,
			"cvv":          data.Cvv,
			"phone_number": data.PhoneNumber,
			"description":  data.Description,
		},
		true)
	if err != nil {
		return nil, nil, err
	}

	return parseCardOrPending(respBody)
}

// parseCardOrPending attempts to parse the response as a pending order first,
// and falls back to parsing it as a card creation response.
func parseCardOrPending(respBody []byte) (*Card, *PendingCardOrder, error) {
	var pending pendingCardOrderResponse
	if err := json.Unmarshal(respBody, &pending); err == nil {
		if pending.Data.PendingActivation {
			return nil, &pending.Data, nil
		}
	}

	var resp getCardsResponseTyped
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, nil, err
	}

	if len(resp.Cards) < 1 {
		return nil, nil, errors.New("ibs: no card created")
	}

	card := cardFromResponse(resp.Cards[0])
	return &card, nil, nil
}
