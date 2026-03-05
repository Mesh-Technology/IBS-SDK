package ibs

import "net/http"

// CardEnable enables or disables the card.
func (c *Client) CardEnable(enabled bool) error {
	_, err := c.requestAPI(
		http.MethodPost,
		"/card/setting/enabled",
		map[string]any{
			"card_id": c.cardID,
			"user_id": c.userID,
			"enabled": enabled,
		},
		true)

	return err
}

// CardATM enables or disables ATM withdrawals for the card.
func (c *Client) CardATM(atmWithdrawEnabled bool) error {
	_, err := c.requestAPI(
		http.MethodPost,
		"/card/setting/atm",
		map[string]any{
			"card_id": c.cardID,
			"user_id": c.userID,
			"enabled": atmWithdrawEnabled,
		},
		true)

	return err
}

// ChangePIN changes the card PIN to the given value.
func (c *Client) ChangePIN(newPIN string) error {
	_, err := c.requestAPI(
		http.MethodPost,
		"/card/pin/change",
		map[string]any{
			"card_id": c.cardID,
			"user_id": c.userID,
			"new_pin": newPIN,
		},
		true)

	return err
}

// SendPIN sends the card PIN to the cardholder.
func (c *Client) SendPIN() error {
	_, err := c.requestAPI(
		http.MethodPost,
		"/card/pin/send",
		map[string]any{
			"card_id": c.cardID,
			"user_id": c.userID,
		},
		true)

	return err
}

// UpdateOwnership transfers card ownership to a different user.
func (c *Client) UpdateOwnership(newUserID string) error {
	_, err := c.requestAPI(
		http.MethodPost,
		"/card/update/ownership",
		map[string]any{
			"card_id":     c.cardID,
			"user_id":     c.userID,
			"new_user_id": newUserID,
		},
		true)

	return err
}
