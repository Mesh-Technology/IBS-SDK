# IBS SDK for Go

A Go module providing a client SDK for the **IBS** (card provider) service. This package handles authentication (HMAC-SHA512 signing), request construction, response parsing, and callback verification for all IBS API endpoints.

## Installation

```sh
go get github.com/Mesh-Technology/IBS-SDK
```

## Project Structure

```
ibs-sdk/
├── go.mod
├── README.md
├── .gitignore
└── ibs/
    ├── doc.go              # Package-level documentation
    ├── client.go           # Config, Options, Configure(), Client, New(), requestAPI
    ├── card.go             # Card type, GetCardInfo
    ├── card_create.go      # Virtual, Physical, PendingCardOrder, VirtualCard(), PhysicalCard()
    ├── card_settings.go    # CardEnable, CardATM, ChangePIN, SendPIN, UpdateOwnership
    ├── card_balance.go     # CardBalance
    ├── card_prices.go      # Price type, Prices()
    ├── card_ledgers.go     # Ledger, Ledgers types, GetCardLedgers()
    ├── card_pendings.go    # PendingCard, CardPendings, GetCardPendings()
    ├── callback.go         # CardActivation types, high-level Callback() dispatcher
    └── callback_verify.go  # Signature verification, callback parsing, sentinel errors
```

## Quick Start

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Mesh-Technology/IBS-SDK/ibs"
)

func main() {
	// Initialise once at startup
	ibs.Configure(ibs.Config{
		APIURL:    os.Getenv("IBS_API_URL"),
		APIKey:    os.Getenv("IBS_API_KEY"),
		SecretKey: os.Getenv("IBS_SECRET_KEY"),
	})

	// Create clients anywhere — credentials are already set
	client := ibs.New("user-123", "")

	cards, err := client.GetCardInfo()
	if err != nil {
		log.Fatal(err)
	}

	for _, card := range cards {
		fmt.Printf("Card %s — Balance: %.2f %s\n", card.CardNumber, card.Balance, card.Currency)
	}
}
```

## Configuration

Call `ibs.Configure()` **once** at program startup. All subsequent calls to `ibs.New()` will share the same credentials, HTTP client, logger, and user-agent.

```go
ibs.Configure(ibs.Config{
	APIURL:    "https://api.ibs.example.com",
	APIKey:    os.Getenv("IBS_API_KEY"),
	SecretKey: os.Getenv("IBS_SECRET_KEY"), // base64-encoded
})
```

| Field       | Description                                       |
|-------------|---------------------------------------------------|
| `APIURL`    | Base URL of the IBS API                           |
| `APIKey`    | API key for authentication and HMAC signing       |
| `SecretKey` | Base64-encoded secret key for HMAC-SHA512 signing |

**Never hardcode secrets** — use environment variables or a secret manager.

### Safety rules

| Scenario                                  | Behaviour          |
|-------------------------------------------|--------------------|
| `Configure()` called once                 | ✅ Normal           |
| `Configure()` called more than once       | ❌ Panics           |
| `New()` called before `Configure()`       | ❌ Panics           |
| Check at runtime whether SDK is ready     | `ibs.Configured()` |

## Functional Options

Pass options to `Configure` to customise SDK-wide behaviour:

```go
ibs.Configure(cfg,
	ibs.WithLogger(slog.New(slog.NewJSONHandler(os.Stdout, nil))),
	ibs.WithUserAgent("MyService/2.0"),
	ibs.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
)
```

| Option           | Description                                |
|------------------|--------------------------------------------|
| `WithLogger`     | Set a custom `*slog.Logger` (nil disables) |
| `WithUserAgent`  | Override the default `User-Agent` header   |
| `WithHTTPClient` | Provide a custom `*http.Client`            |

## Creating Clients

After `Configure`, create lightweight clients scoped to a user/card context:

```go
// User-scoped client
client := ibs.New("user-123", "")

// User + card scoped client
client := ibs.New("user-123", "card-456")

// Bare client (e.g. for pricing or callback verification)
client := ibs.New("", "")
```

### Deriving new contexts

Derive new clients cheaply without re-specifying credentials:

```go
base := ibs.New("user-123", "")

// Narrow to a specific card
cardClient := base.WithCard("card-456")

// Switch user entirely
otherUser := base.WithUserAndCard("user-789", "card-012")

// Read back the context
fmt.Println(cardClient.UserID()) // "user-123"
fmt.Println(cardClient.CardID()) // "card-456"
```

| Method            | Description                           |
|-------------------|---------------------------------------|
| `WithUser`        | Copy with a different user ID         |
| `WithCard`        | Copy with a different card ID         |
| `WithUserAndCard` | Copy with different user and card IDs |
| `UserID`          | Get current user ID                   |
| `CardID`          | Get current card ID                   |

## API Reference

### Card Creation

```go
// Create a virtual card
card, pending, err := client.VirtualCard(ibs.Virtual{
	BankID:       "papara",
	PhoneNumber:  "+905551234567",
	UserEmail:    "user@example.com",
	UserFullName: "John Doe",
	Description:  "Travel card",
})

// Activate a physical card
card, pending, err := client.PhysicalCard(ibs.Physical{
	BankID:      "papara",
	CardNumber:  "4111111111111111",
	ExpireMonth: "12",
	ExpireYear:  "2027",
	Cvv:         "123",
	PhoneNumber: "+905551234567",
	Description: "Main card",
})
```

Both methods return a `*Card` on immediate activation, or a `*PendingCardOrder` when the activation is queued for asynchronous processing.

### Card Information

```go
cards, err := client.GetCardInfo()
for _, c := range cards {
	fmt.Printf("%s — %s — Enabled: %v\n", c.CardID, c.CardNumber, c.Enabled)
}
```

### Balance Operations

```go
// Add balance (positive amount)
pending, txID, err := client.CardBalance(100.0)

// Deduct balance (negative amount)
pending, txID, err := client.CardBalance(-50.0)
```

### Card Settings

```go
// Enable / disable card
err := client.CardEnable(true)

// Enable / disable ATM withdrawals
err := client.CardATM(true)
```

### PIN Management

```go
// Change PIN
err := client.ChangePIN("1234")

// Send PIN to cardholder
err := client.SendPIN()
```

### Ownership Transfer

```go
err := client.UpdateOwnership("new-user-456")
```

### Transaction Ledgers

```go
ledgers, err := client.GetCardLedgers("1") // page number
fmt.Printf("Page %d of %d\n", ledgers.CurrentPage, ledgers.TotalPage)
for _, l := range ledgers.Ledgers {
	fmt.Printf("  %s  %+.2f %s  %s\n", l.TransactionDate, l.Amount, l.Currency, l.Description)
}
```

### Pending Card Activations

```go
pendings, err := client.GetCardPendings("user-123", "papara", "virtual")
for _, p := range pendings.Cards {
	fmt.Printf("Order %s — Status: %s\n", p.OrderID, p.Status)
}
```

### Pricing

```go
price, err := client.Prices("papara", "virtual")
fmt.Printf("Price: %.2f %s (cashback: %.2f %s)\n",
	price.Price, price.Currency,
	price.CashBack, price.CashBackCurrency,
)
```

### High-Level Callback Dispatcher

The `Callback` method routes a `CardActivation` request to the correct creation method:

```go
resp, err := client.Callback(&ibs.CardActivation{
	Provider:    "papara",
	Type:        "virtual",
	PhoneNumber: "+905551234567",
	Description: "New card",
})
```

## Webhook Callback Verification

Verify incoming IBS webhook signatures in your HTTP handler:

```go
func webhookHandler(w http.ResponseWriter, r *http.Request) {
	client := ibs.New("", "")

	body, err := client.VerifyCallbackRequest(r)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	event, err := ibs.ParseReverseCallbackEvent(body)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	switch event {
	case "pending_transaction_reverse":
		cb, err := ibs.ParsePendingTransactionReverseCallback(body)
		// handle transaction reversal...

	case "pending_card_reverse":
		cb, err := ibs.ParsePendingCardReverseCallback(body)
		// handle card reversal...
	}

	w.WriteHeader(http.StatusOK)
}
```

You can also verify signatures manually:

```go
err := client.VerifyCallbackSignature(apiKey, signature, timestamp, bodyBytes)
```

## Error Handling

API-level errors (where IBS returns `status: false`) are returned as structured errors. Use `IsAPIError` to inspect them:

```go
cards, err := client.GetCardInfo()
if err != nil {
	if statusCode, msg, ok := ibs.IsAPIError(err); ok {
		log.Printf("IBS API error (HTTP %d): %s", statusCode, msg)
	} else {
		log.Printf("unexpected error: %v", err)
	}
}
```

All errors from this package are prefixed with `ibs:` for easy identification in logs.

## Sentinel Errors

The following sentinel errors are exported for callback verification:

| Error                         | Description                               |
|-------------------------------|-------------------------------------------|
| `ErrNotConfigured`            | `New()` called before `Configure()`       |
| `ErrMissingCallbackHeaders`   | Required HMAC headers are missing         |
| `ErrInvalidCallbackTimestamp` | Timestamp header is not a valid integer   |
| `ErrStaleCallbackTimestamp`   | Timestamp is outside the 60-second window |
| `ErrInvalidCallbackAPIKey`    | API key does not match the configured key |
| `ErrInvalidCallbackSignature` | HMAC signature verification failed        |
| `ErrInvalidCallbackPayload`   | Callback body is missing required fields  |

## Migration from the old `cms` package

```go
// ─── Before ───────────────────────────────────
import "papara/cms"
c := cms.New(userID, cardID)

// ─── After ────────────────────────────────────
import "github.com/Mesh-Technology/IBS-SDK/ibs"

// once at startup:
ibs.Configure(ibs.Config{
	APIURL:    os.Getenv("IBS_API_URL"),
	APIKey:    os.Getenv("IBS_API_KEY"),
	SecretKey: os.Getenv("IBS_SECRET_KEY"),
})

// everywhere else:
c := ibs.New(userID, cardID)
```

Everything else (`GetCardInfo`, `CardBalance`, `VirtualCard`, etc.) keeps the same method signatures.

## Requirements

- Go 1.26 or later
- No external dependencies (stdlib only)

## License

Copyright (c) 2025-2026 InnovateX.
