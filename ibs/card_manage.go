package ibs

import (
	"encoding/json"
	"net/http"
)

// ExistingCard holds the details of an existing card to register with IBS.
// The owner is taken from the client's user context.
type ExistingCard struct {
	BankID         string
	UserFullName   string
	UserEmail      string
	ProviderCardID string
	CardNumber     string
	Cvv            string
	ExpireMonth    string
	ExpireYear     string
	Type           string
}

// CardInfoUpdate holds local card information to update. Empty fields are not
// sent. The card is taken from the client's card context.
type CardInfoUpdate struct {
	ProviderCardID string
	UserID         string
	UserFullName   string
	UserEmail      string
	CardNumber     string
	Cvv            string
	ExpireMonth    string
	ExpireYear     string
	Type           string
}

// ManagedCard is the privacy-safe card representation returned by local card
// management endpoints.
type ManagedCard struct {
	CardID         string `json:"card_id"`
	ProviderCardID string `json:"provider_card_id"`
	BankID         string `json:"bank_id"`
	UserID         string `json:"user_id"`
	UserFullName   string `json:"user_full_name"`
	UserEmail      string `json:"user_email"`
	CardLast4      string `json:"card_last4"`
	Type           string `json:"type"`
	Enabled        bool   `json:"enabled"`
}

type managedCardResponse struct {
	Status bool        `json:"status"`
	Data   ManagedCard `json:"data"`
}

// AddCard registers an existing card locally without calling the provider.
func (c *Client) AddCard(data ExistingCard) (*ManagedCard, error) {
	respBody, err := c.requestAPI(
		http.MethodPost,
		"/card/add",
		map[string]any{
			"bank_id":          data.BankID,
			"user_id":          c.userID,
			"user_full_name":   data.UserFullName,
			"user_email":       data.UserEmail,
			"provider_card_id": data.ProviderCardID,
			"card_number":      data.CardNumber,
			"cvv":              data.Cvv,
			"expire_month":     data.ExpireMonth,
			"expire_year":      data.ExpireYear,
			"type":             data.Type,
		},
		true)
	if err != nil {
		return nil, err
	}

	return parseManagedCard(respBody)
}

// UpdateCardInfo updates local card information without calling the provider.
func (c *Client) UpdateCardInfo(data CardInfoUpdate) (*ManagedCard, error) {
	body := map[string]any{"card_id": c.cardID}
	addNonEmptyString(body, "provider_card_id", data.ProviderCardID)
	addNonEmptyString(body, "user_id", data.UserID)
	addNonEmptyString(body, "user_full_name", data.UserFullName)
	addNonEmptyString(body, "user_email", data.UserEmail)
	addNonEmptyString(body, "card_number", data.CardNumber)
	addNonEmptyString(body, "cvv", data.Cvv)
	addNonEmptyString(body, "expire_month", data.ExpireMonth)
	addNonEmptyString(body, "expire_year", data.ExpireYear)
	addNonEmptyString(body, "type", data.Type)

	respBody, err := c.requestAPI(http.MethodPost, "/card/update/info", body, true)
	if err != nil {
		return nil, err
	}

	return parseManagedCard(respBody)
}

// DeleteCard revokes a card without replacement, setting revoked = true and enabled = false.
func (c *Client) DeleteCard() error {
	_, err := c.requestAPI(
		http.MethodPost,
		"/card/delete",
		map[string]any{
			"card_id": c.cardID,
		},
		true)

	return err
}

func addNonEmptyString(body map[string]any, key, value string) {
	if value != "" {
		body[key] = value
	}
}

func parseManagedCard(respBody []byte) (*ManagedCard, error) {
	var resp managedCardResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}
