// Package ibs provides a Go SDK client for the IBS card provider service.
//
// The SDK handles authentication (HMAC-SHA512 signing), request construction,
// and response parsing for all IBS API endpoints including card creation,
// balance management, PIN operations, and callback verification.
//
// # Getting Started
//
// Initialise the SDK once at program startup with [Configure]:
//
//	ibs.Configure(ibs.Config{
//		APIURL:    os.Getenv("IBS_API_URL"),
//		APIKey:    os.Getenv("IBS_API_KEY"),
//		SecretKey: os.Getenv("IBS_SECRET_KEY"),
//	})
//
// Then create clients scoped to a user/card context anywhere in your code:
//
//	client := ibs.New("user-123", "card-456")
//	cards, err := client.GetCardInfo()
//
// You can also create a bare client and derive scoped ones later:
//
//	base := ibs.New("", "")
//	userClient := base.WithUser("user-123")
//	cardClient := base.WithUserAndCard("user-123", "card-456")
//
// # Functional Options
//
// Pass options to [Configure] to customise SDK-wide behaviour:
//
//	ibs.Configure(cfg,
//		ibs.WithLogger(slog.New(slog.NewJSONHandler(os.Stdout, nil))),
//		ibs.WithUserAgent("MyService/2.0"),
//		ibs.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
//	)
//
// # Card Operations
//
// The SDK supports the following card operations:
//
//   - [Client.VirtualCard] — create a virtual card
//   - [Client.PhysicalCard] — activate a physical card
//   - [Client.GetCardInfo] — retrieve card details
//   - [Client.GetCardLedgers] — retrieve paginated transaction ledgers
//   - [Client.GetCardPendings] — list pending card activations
//   - [Client.CardBalance] — add or deduct balance (positive = add, negative = deduct)
//   - [Client.CardEnable] — enable or disable a card
//   - [Client.CardATM] — enable or disable ATM withdrawals
//   - [Client.ChangePIN] — change the card PIN
//   - [Client.SendPIN] — send the card PIN to the cardholder
//   - [Client.UpdateOwnership] — transfer card ownership to another user
//   - [Client.Prices] — retrieve pricing information
//   - [Client.Callback] — high-level dispatcher for card activation
//
// # Callback Verification
//
// To verify incoming IBS webhook callbacks:
//
//	client := ibs.New("", "")
//	body, err := client.VerifyCallbackRequest(r)
//	if err != nil {
//		// signature invalid
//	}
//	event, _ := ibs.ParseReverseCallbackEvent(body)
//
// # Error Handling
//
// API-level errors (where the IBS service returns status=false) are returned
// as structured errors. Use [IsAPIError] to inspect them:
//
//	_, err := client.GetCardInfo()
//	if code, msg, ok := ibs.IsAPIError(err); ok {
//		log.Printf("IBS API error %d: %s", code, msg)
//	}
//
// # Safety
//
// [Configure] must be called exactly once before any call to [New].
// Calling [Configure] more than once will panic. Calling [New] before
// [Configure] will also panic. Use [Configured] to check whether the
// SDK has been initialised.
package ibs
