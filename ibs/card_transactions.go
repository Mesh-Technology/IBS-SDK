package ibs

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Transaction type constants accepted by [Client.ListTransactions] in the
// TransactionListQuery.Type field.
const (
	TransactionTypeTopup    = "topup"
	TransactionTypeWithdraw = "withdraw"
)

// Transaction status constants accepted by [Client.ListTransactions] in the
// TransactionListQuery.Status field.
const (
	TransactionStatusPending  = "pending"
	TransactionStatusSuccess  = "success"
	TransactionStatusFailed   = "failed"
	TransactionStatusReversed = "reversed"
)

// Pagination defaults for [Client.ListTransactions].
const (
	transactionDefaultPage  = 1
	transactionDefaultLimit = 50
	transactionMaxLimit     = 500
)

// uuidPattern matches the standard 8-4-4-4-12 hex UUID format. The SDK uses
// this only to give callers a friendlier error before the request is sent;
// the server performs its own authoritative validation.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isValidUUID(s string) bool {
	return uuidPattern.MatchString(strings.TrimSpace(s))
}

// defaultReverseReason is substituted when the caller passes an empty reason
// to [Client.ReverseTransaction].
const defaultReverseReason = "reversed by service"

// Transaction represents a single card transaction returned by the IBS API.
type Transaction struct {
	ID                string  `json:"id"`
	CardID            string  `json:"card_id"`
	ServiceID         string  `json:"service_id"`
	UserID            string  `json:"user_id"`
	CardNumberMasked  string  `json:"card_number_masked"`
	Type              string  `json:"type"`
	Status            string  `json:"status"`
	Amount            float64 `json:"amount"`
	BalanceBefore     float64 `json:"balance_before"`
	BalanceAfter      float64 `json:"balance_after"`
	CommissionPercent float64 `json:"commission_percent"`
	CommissionAmount  float64 `json:"commission_amount"`
	GrossAmount       float64 `json:"gross_amount"`
	Currency          string  `json:"currency"`
	ExternalRef       string  `json:"external_ref"`
	Description       string  `json:"description"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

// TransactionList is a paginated list of card transactions.
type TransactionList struct {
	Total        int           `json:"total"`
	Page         int           `json:"page"`
	Limit        int           `json:"limit"`
	Transactions []Transaction `json:"transactions"`
}

// TransactionListQuery holds the optional filter parameters for
// [Client.ListTransactions]. Zero values mean "do not filter on this field".
//
// UserID and CardID fall back to the values carried by the client context when
// left blank, mirroring the pattern used by [Client.GetCardPendings].
type TransactionListQuery struct {
	// BankID is the bank code (e.g. "papara"). Optional.
	BankID string
	// CardID is the card UUID. Optional; falls back to the client context.
	CardID string
	// UserID is the user ID. Optional; falls back to the client context.
	UserID string
	// Type narrows the result to topup or withdraw transactions. Optional.
	// Use the TransactionType* constants.
	Type string
	// Status narrows the result to one of the four lifecycle states.
	// Optional; omit to receive all states including pending. Use the
	// TransactionStatus* constants.
	Status string
	// FromDate is the inclusive lower bound for the transaction's
	// creation time, formatted as RFC3339 in the request. Optional.
	FromDate time.Time
	// ToDate is the inclusive upper bound for the transaction's
	// creation time, formatted as RFC3339 in the request. Optional.
	ToDate time.Time
	// Page is the 1-based page number. Defaults to 1; values < 1 are
	// coerced to 1.
	Page int
	// Limit is the page size. Defaults to 50; values < 1 are coerced to
	// 50; values > 500 are clamped to 500.
	Limit int
}

type listTransactionsResponse struct {
	Status bool            `json:"status"`
	Data   TransactionList `json:"data"`
}

// ListTransactions returns a paginated list of card transactions for the
// calling service. Pending transactions are included in the default result
// set, so callers do not need to opt in.
//
// All fields of [TransactionListQuery] are optional. CardID and UserID fall
// back to the values carried by the client context when left blank.
//
// Returns an error wrapping one of the package's sentinel errors when the
// IBS API rejects the request (use [IsAPIError] to inspect HTTP status and
// message).
func (c *Client) ListTransactions(q TransactionListQuery) (TransactionList, error) {
	cardID := strings.TrimSpace(q.CardID)
	if cardID == "" {
		cardID = strings.TrimSpace(c.cardID)
	}
	if cardID != "" && !isValidUUID(cardID) {
		return TransactionList{}, errors.New("ibs: card_id must be a valid UUID")
	}

	userID := strings.TrimSpace(q.UserID)
	if userID == "" {
		userID = strings.TrimSpace(c.userID)
	}

	txType := strings.ToLower(strings.TrimSpace(q.Type))
	if txType != "" {
		switch txType {
		case TransactionTypeTopup, TransactionTypeWithdraw:
			// ok
		default:
			return TransactionList{}, errors.New("ibs: type must be one of: topup, withdraw")
		}
	}

	status := strings.ToLower(strings.TrimSpace(q.Status))
	if status != "" {
		switch status {
		case TransactionStatusPending, TransactionStatusSuccess,
			TransactionStatusFailed, TransactionStatusReversed:
			// ok
		default:
			return TransactionList{}, errors.New("ibs: status must be one of: pending, success, failed, reversed")
		}
	}

	page := q.Page
	if page < 1 {
		page = transactionDefaultPage
	}
	limit := q.Limit
	if limit < 1 {
		limit = transactionDefaultLimit
	} else if limit > transactionMaxLimit {
		limit = transactionMaxLimit
	}

	query := url.Values{}
	if bankID := strings.TrimSpace(q.BankID); bankID != "" {
		query.Set("bank_id", bankID)
	}
	if cardID != "" {
		query.Set("card_id", cardID)
	}
	if userID != "" {
		query.Set("user_id", userID)
	}
	if txType != "" {
		query.Set("type", txType)
	}
	if status != "" {
		query.Set("status", status)
	}
	if !q.FromDate.IsZero() {
		query.Set("from_date", q.FromDate.UTC().Format(time.RFC3339))
	}
	if !q.ToDate.IsZero() {
		query.Set("to_date", q.ToDate.UTC().Format(time.RFC3339))
	}
	query.Set("page", strconv.Itoa(page))
	query.Set("limit", strconv.Itoa(limit))

	endpoint := "/v1/card/transactions"
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	respBody, err := c.requestAPI(http.MethodGet, endpoint, nil, true)
	if err != nil {
		return TransactionList{}, err
	}

	var responseMap listTransactionsResponse
	if err := json.Unmarshal(respBody, &responseMap); err != nil {
		return TransactionList{}, fmt.Errorf("ibs: decode transactions response: %w", err)
	}

	// Ensure callers can range over Transactions without a nil check.
	if responseMap.Data.Transactions == nil {
		responseMap.Data.Transactions = []Transaction{}
	}

	return responseMap.Data, nil
}

// ConfirmTransactionResult is the data returned by a successful
// [Client.ConfirmTransaction] call.
type ConfirmTransactionResult struct {
	TransactionID string  `json:"transaction_id"`
	CardID        string  `json:"card_id"`
	Amount        float64 `json:"amount"`
	Balance       float64 `json:"balance"`
	Status        string  `json:"status"`
}

type confirmTransactionResponse struct {
	Status bool                     `json:"status"`
	Data   ConfirmTransactionResult `json:"data"`
}

// ConfirmTransaction approves a pending topup. The card is locked for the
// duration, the balance is incremented, and the transaction's status flips
// from "pending" to "success". Only works on the calling service's own
// pending topup rows; the bank must report that it supports topup
// confirmation (e.g. Alaan-style).
//
// The server may fire a signed "pending_transaction_done" HMAC callback to
// the service's configured TransactionDoneEndpoint after the call returns.
// That fan-out is handled server-side; the SDK does not need to do
// anything extra here.
//
// Errors are returned as *APIError. Common HTTP status codes:
//   - 400: invalid id, not pending, not topup, or the bank does not
//     support topup confirmation
//   - 404: not found or owned by another service
//   - 409 / 503: card busy (lock contention)
//   - 500: internal error
func (c *Client) ConfirmTransaction(transactionID string) (ConfirmTransactionResult, error) {
	trimmed := strings.TrimSpace(transactionID)
	if !isValidUUID(trimmed) {
		return ConfirmTransactionResult{}, errors.New("ibs: transaction_id must be a valid UUID")
	}

	respBody, err := c.requestAPI(
		http.MethodPost,
		"/v1/card/transactions/confirm",
		map[string]any{
			"transaction_id": trimmed,
		},
		true)
	if err != nil {
		return ConfirmTransactionResult{}, err
	}

	var resp confirmTransactionResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return ConfirmTransactionResult{}, fmt.Errorf("ibs: decode confirm response: %w", err)
	}

	return resp.Data, nil
}

// ReverseTransactionResult is the data returned by a successful
// [Client.ReverseTransaction] call.
type ReverseTransactionResult struct {
	TransactionID      string `json:"transaction_id"`
	Status             string `json:"status"`
	WalletRefunded     bool   `json:"wallet_refunded"`
	WalletRefundAmount string `json:"wallet_refund_amount"`
	WalletCurrency     string `json:"wallet_currency"`
}

type reverseTransactionResponse struct {
	Status bool                     `json:"status"`
	Data   ReverseTransactionResult `json:"data"`
}

// ReverseTransaction rejects a pending or failed topup. When the row was
// a local reservation (external_ref == "" and description == "Card TopUp")
// the reserved service wallet is refunded. If the service has a
// TransactionReverseEndpoint configured, a signed
// "pending_transaction_reverse" HMAC callback is fired before the
// database transaction; the payload includes expected_refund and
// expected_refund_type ("service_wallet" | "none"). All of that is handled
// server-side.
//
// An empty reason is normalised to "reversed by service".
//
// Errors are returned as *APIError. Common HTTP status codes:
//   - 400: invalid id, not pending/failed, not topup, or the upstream
//     reported "already reversed"
//   - 404: not found or owned by another service
//   - 502: the upstream reverse endpoint rejected the request
//   - 500: internal error
func (c *Client) ReverseTransaction(transactionID, reason string) (ReverseTransactionResult, error) {
	trimmedID := strings.TrimSpace(transactionID)
	if !isValidUUID(trimmedID) {
		return ReverseTransactionResult{}, errors.New("ibs: transaction_id must be a valid UUID")
	}

	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		trimmedReason = defaultReverseReason
	}

	respBody, err := c.requestAPI(
		http.MethodPost,
		"/v1/card/transactions/reverse",
		map[string]any{
			"transaction_id": trimmedID,
			"reason":         trimmedReason,
		},
		true)
	if err != nil {
		return ReverseTransactionResult{}, err
	}

	var resp reverseTransactionResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return ReverseTransactionResult{}, fmt.Errorf("ibs: decode reverse response: %w", err)
	}

	return resp.Data, nil
}
