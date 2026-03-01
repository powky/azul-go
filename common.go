// Package azul provides Go clients for the Azul payment gateway (Dominican Republic).
//
// Two clients are available:
//
//   - HPPClient: Hosted Payment Page (browser redirect) — builds form fields + HMAC
//   - APIClient: Webservices API (server-to-server) — direct JSON POST with TLS mutual auth
//
// This file contains shared utilities used by both clients.
package azul

import (
	"fmt"
	"math"
	"time"
)

// ============================================================================
// Amount formatting
// ============================================================================

// FormatAmount converts a monetary amount (e.g. 1500.00) to Azul's string format.
//
// Azul expects the amount as a string where the last 2 digits represent cents,
// with no decimal separator. Minimum length is 3 characters (zero-padded).
//
// Examples:
//
//	FormatAmount(0)       → "000"
//	FormatAmount(0.50)    → "050"
//	FormatAmount(15.00)   → "1500"
//	FormatAmount(1500.00) → "150000"
func FormatAmount(amount float64) string {
	cents := int64(math.Round(amount * 100))
	s := fmt.Sprintf("%d", cents)
	if len(s) < 3 {
		return fmt.Sprintf("%03d", cents)
	}
	return s
}

// ParseAmount converts an Azul amount string back to a monetary amount.
//
// This is the inverse of FormatAmount. Returns 0 if the string is invalid.
//
// Examples:
//
//	ParseAmount("000")    → 0.00
//	ParseAmount("1500")   → 15.00
//	ParseAmount("150000") → 1500.00
func ParseAmount(s string) float64 {
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return float64(n) / 100.0
}

// ============================================================================
// Order number generation
// ============================================================================

// GenerateOrderNumber creates a unique order number using the given prefix
// and the current Unix timestamp in milliseconds.
//
// Example: GenerateOrderNumber("ORD") → "ORD-1709234567890"
func GenerateOrderNumber(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixMilli())
}
