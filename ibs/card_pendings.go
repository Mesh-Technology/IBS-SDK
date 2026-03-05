package ibs

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// PendingCard represents a card activation that is still pending.
type PendingCard struct {
	PendingCardID string `json:"pending_card_id"`
	OrderID       string `json:"order_id"`
	UserID        string `json:"user_id"`
	UserFullName  string `json:"user_full_name"`
	UserEmail     string `json:"user_email"`
	BankID        string `json:"bank_id"`
	CardType      string `json:"card_type"`
	Status        string `json:"status"`
	PhoneNumber   string `json:"phone_number"`
	Description   string `json:"description"`
	PriceAmount   string `json:"price_amount"`
	PriceCurrency string `json:"price_currency"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// CardPendings holds a list of pending card activations along with a total count.
type CardPendings struct {
	Total int           `json:"total"`
	Cards []PendingCard `json:"cards"`
}

type getPendingsResponse struct {
	Status bool         `json:"status"`
	Data   CardPendings `json:"data"`
}

// GetCardPendings fetches pending card activations and excludes rejected items.
// Supported filters are optional: userID, bankID and cardType (virtual|physical).
func (c *Client) GetCardPendings(userID, bankID, cardType string) (CardPendings, error) {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		trimmedUserID = strings.TrimSpace(c.userID)
	}

	trimmedBankID := strings.TrimSpace(bankID)
	trimmedCardType := strings.ToLower(strings.TrimSpace(cardType))
	if trimmedCardType != "" && trimmedCardType != "virtual" && trimmedCardType != "physical" {
		return CardPendings{}, errors.New("ibs: invalid card type filter")
	}

	query := url.Values{}
	if trimmedUserID != "" {
		query.Set("user_id", trimmedUserID)
	}
	if trimmedBankID != "" {
		query.Set("bank_id", trimmedBankID)
	}
	if trimmedCardType != "" {
		query.Set("card_type", trimmedCardType)
	}

	endpoint := "/card/pendings"
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	respBody, err := c.requestAPI(http.MethodGet, endpoint, nil, true)
	if err != nil {
		return CardPendings{}, err
	}

	var responseMap getPendingsResponse
	if err := json.Unmarshal(respBody, &responseMap); err != nil {
		return CardPendings{}, err
	}

	pendingCards := make([]PendingCard, 0, len(responseMap.Data.Cards))
	for _, card := range responseMap.Data.Cards {
		if strings.EqualFold(strings.TrimSpace(card.Status), "rejected") {
			continue
		}
		pendingCards = append(pendingCards, card)
	}

	return CardPendings{
		Total: responseMap.Data.Total,
		Cards: pendingCards,
	}, nil
}
