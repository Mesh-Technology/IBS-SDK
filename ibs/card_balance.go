package ibs

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
)

type balanceResponse struct {
	Status bool `json:"status"`
	Data   struct {
		Amount        float64 `json:"amount"`
		Pending       bool    `json:"pending"`
		TransactionID string  `json:"transaction_id"`
	} `json:"data"`
}

// CardBalance adds or deducts balance from the card.
// Pass a positive amount to add, or a negative amount to deduct.
// Returns whether the operation is pending, the transaction ID, and any error.
func (c *Client) CardBalance(amount float64) (bool, string, error) {
	operation := "add"
	if amount < 0 {
		operation = "dec"
		amount = math.Abs(amount)
	} else if amount == 0 {
		return false, "", errors.New("ibs: amount cannot be zero")
	}

	respBody, err := c.requestAPI(
		http.MethodPost,
		"/card/balance/"+operation,
		map[string]any{
			"card_id": c.cardID,
			"user_id": c.userID,
			"amount":  amount,
		},
		true)
	if err != nil {
		return false, "", err
	}

	var resp balanceResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return false, "", err
	}

	return resp.Data.Pending, resp.Data.TransactionID, nil
}
