package ibs

import (
	"fmt"
	"time"
)

// CardActivation holds the parameters for activating a card through the
// high-level Callback dispatcher.
type CardActivation struct {
	Provider    string
	Description string
	Type        string
	PhoneNumber string
	CardNumber  *string
	Cvv         *string
	ExpireMonth *string
	ExpireYear  *string
}

// CardActivationResponse is the unified response returned by the Callback dispatcher.
type CardActivationResponse struct {
	CardID string
	BankID string

	CardNumber  string
	Cvv         string
	ExpiryMonth string
	ExpiryYear  string

	Type        string
	Balance     float64
	PhoneNumber string

	Enabled bool
	ATM     bool

	CreatedAt time.Time
}

// Callback is a high-level dispatcher that routes a CardActivation request to the
// appropriate card creation method (virtual or physical) based on the provider and type.
func (c *Client) Callback(req *CardActivation) (CardActivationResponse, error) {
	if req == nil {
		return CardActivationResponse{}, fmt.Errorf("ibs: callback request cannot be nil")
	}

	if c.g.logger != nil {
		c.g.logger.Info("ibs: processing card activation callback",
			"provider", req.Provider,
			"type", req.Type,
			"phone_number", req.PhoneNumber,
		)
	}

	switch req.Provider {
	case "papara":
		switch req.Type {
		case "virtual":
			card, pendingOrder, err := c.VirtualCard(Virtual{
				BankID:      req.Provider,
				PhoneNumber: req.PhoneNumber,
				Description: req.Description,
			})
			if err != nil {
				return CardActivationResponse{}, err
			}
			if pendingOrder != nil {
				return CardActivationResponse{}, fmt.Errorf("ibs: virtual card activation is pending: %s", pendingOrder.OrderID)
			}
			if card == nil {
				return CardActivationResponse{}, fmt.Errorf("ibs: virtual card response is empty")
			}
			return CardActivationResponse{
				CardID:      card.CardID,
				BankID:      card.BankID,
				CardNumber:  card.CardNumber,
				Cvv:         card.Cvv,
				ExpiryMonth: card.ExpiryMonth,
				ExpiryYear:  card.ExpiryYear,
				Type:        card.Type,
				Balance:     card.Balance,
				PhoneNumber: card.PhoneNumber,
				Enabled:     card.Enabled,
				ATM:         card.ATM,
				CreatedAt:   card.CreatedAt,
			}, nil

		case "physical":
			if req.CardNumber == nil || req.Cvv == nil || req.ExpireMonth == nil || req.ExpireYear == nil {
				return CardActivationResponse{}, fmt.Errorf("ibs: physical card activation requires card_number, cvv, expire_month and expire_year")
			}
			card, pendingOrder, err := c.PhysicalCard(Physical{
				BankID:      req.Provider,
				PhoneNumber: req.PhoneNumber,
				Description: req.Description,
				CardNumber:  *req.CardNumber,
				Cvv:         *req.Cvv,
				ExpireMonth: *req.ExpireMonth,
				ExpireYear:  *req.ExpireYear,
			})
			if err != nil {
				return CardActivationResponse{}, err
			}
			if pendingOrder != nil {
				return CardActivationResponse{}, fmt.Errorf("ibs: physical card activation is pending: %s", pendingOrder.OrderID)
			}
			if card == nil {
				return CardActivationResponse{}, fmt.Errorf("ibs: physical card response is empty")
			}
			return CardActivationResponse{
				CardID:      card.CardID,
				BankID:      card.BankID,
				CardNumber:  card.CardNumber,
				Cvv:         card.Cvv,
				ExpiryMonth: card.ExpiryMonth,
				ExpiryYear:  card.ExpiryYear,
				Type:        card.Type,
				Balance:     card.Balance,
				PhoneNumber: card.PhoneNumber,
				Enabled:     card.Enabled,
				ATM:         card.ATM,
				CreatedAt:   card.CreatedAt,
			}, nil

		default:
			return CardActivationResponse{}, fmt.Errorf("ibs: unknown card type: %s", req.Type)
		}

	default:
		return CardActivationResponse{}, fmt.Errorf("ibs: unknown provider: %s", req.Provider)
	}
}
