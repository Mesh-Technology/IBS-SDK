package ibs

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
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
//
// A fresh idempotency key is generated per call; this means each invocation
// is treated as a distinct logical operation by the server. If the network
// drops mid-flight and you call CardBalance again, the server will execute
// the request twice. To get retry-safe semantics, use
// [Client.CardBalanceIdempotent] and persist the key alongside your record
// of the operation so retries reuse the same key.
//
// Returns whether the operation is pending, the transaction ID, and any error.
func (c *Client) CardBalance(amount float64) (bool, string, error) {
	key, err := newIdempotencyKey()
	if err != nil {
		return false, "", err
	}
	return c.CardBalanceIdempotent(amount, key)
}

// CardBalanceIdempotent is the same as CardBalance but takes a caller-supplied
// idempotency key. Reusing the same key for retries of the same logical
// operation guarantees the server will execute it at most once.
//
// On a duplicate key the server returns HTTP 409 and this function returns
// an error wrapping [ErrDuplicateIdempotencyKey]; check with errors.Is or
// [IsDuplicateIdempotencyKey]. The original operation is NOT re-run — you
// must look up its outcome by your own correlation. The idempotency record
// is held for ~24h on the server side; after that a key may be reused.
//
// Pattern for safe retries:
//
//	key := myStore.OrCreateIdempotencyKey(opID)         // stable across retries
//	pending, txID, err := client.CardBalanceIdempotent(amt, key)
//	switch {
//	case ibs.IsDuplicateIdempotencyKey(err):
//	    // First attempt already committed; recover txID from your store.
//	case err != nil:
//	    // Transient or terminal error; safe to retry with the SAME key.
//	default:
//	    myStore.RecordTransactionID(opID, txID)
//	}
func (c *Client) CardBalanceIdempotent(amount float64, idempotencyKey string) (bool, string, error) {
	if idempotencyKey == "" {
		return false, "", errors.New("ibs: idempotency key cannot be empty")
	}

	operation := "add"
	if amount < 0 {
		operation = "dec"
		amount = math.Abs(amount)
	} else if amount == 0 {
		return false, "", errors.New("ibs: amount cannot be zero")
	}

	// Send amount as a JSON string so the server's decimal.Decimal parses
	// without going through a float64 round-trip on its end.
	amountStr := strconv.FormatFloat(amount, 'f', -1, 64)

	respBody, err := c.requestAPIWithIdempotency(
		http.MethodPost,
		"/card/balance/"+operation,
		map[string]any{
			"card_id": c.cardID,
			"user_id": c.userID,
			"amount":  amountStr,
		},
		true,
		idempotencyKey)
	if err != nil {
		return false, "", err
	}

	var resp balanceResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return false, "", err
	}

	return resp.Data.Pending, resp.Data.TransactionID, nil
}
