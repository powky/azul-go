package azul

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"strings"
	"unicode/utf16"
)

// hashFieldOrder is the exact concatenation order for the payment page request hash.
// This order is defined by Azul's HPP specification and MUST NOT be changed.
var hashFieldOrder = []string{
	"MerchantId", "MerchantName", "MerchantType", "CurrencyCode", "OrderNumber",
	"Amount", "ITBIS", "ApprovedUrl", "DeclinedUrl", "CancelUrl",
	"UseCustomField1", "CustomField1Label", "CustomField1Value",
	"UseCustomField2", "CustomField2Label", "CustomField2Value",
}

// signHPPRequest signs the payment page request fields.
//
// Process:
//  1. Concatenate field values in hashFieldOrder + AuthKey (trimmed)
//  2. Compute HMAC-SHA512 using AuthKey as the key
//  3. Return uppercase hex string
func signHPPRequest(secret string, fields map[string]string) string {
	var sb strings.Builder
	for _, k := range hashFieldOrder {
		sb.WriteString(fields[k])
	}
	sb.WriteString(strings.TrimSpace(secret))

	return hmacSHA512Upper(secret, []byte(sb.String()))
}

// verifyHPPReturn checks the return hash from Azul trying multiple field combos
// AND multiple encodings (UTF-8 and UTF-16LE).
//
// Azul's PHP reference implementation uses mb_convert_encoding($str, 'UTF-16LE', 'ASCII')
// before computing HMAC-SHA512, so we must try UTF-16LE as well.
//
// The function tries 4 field orders × 2 encodings = 8 combinations total:
//   - full:       OrderNumber + Amount + AuthorizationCode + DateTime + ResponseCode + ISOCode + ResponseMessage + ErrorDescription + RRN
//   - noDT:       Same but without DateTime
//   - fullNoErr:  Same but without ErrorDescription
//   - noDTNoErr:  Without both DateTime and ErrorDescription
func verifyHPPReturn(secret string, m map[string]string, providedHash string) bool {
	if providedHash == "" {
		return true // no hash to verify — trust by ResponseCode
	}
	provided := strings.ToUpper(strings.TrimSpace(providedHash))

	full := []string{"OrderNumber", "Amount", "AuthorizationCode", "DateTime", "ResponseCode", "ISOCode", "ResponseMessage", "ErrorDescription", "RRN"}
	noDT := []string{"OrderNumber", "Amount", "AuthorizationCode", "ResponseCode", "ISOCode", "ResponseMessage", "ErrorDescription", "RRN"}
	fullNoErr := []string{"OrderNumber", "Amount", "AuthorizationCode", "DateTime", "ResponseCode", "ISOCode", "ResponseMessage", "RRN"}
	noDTNoErr := []string{"OrderNumber", "Amount", "AuthorizationCode", "ResponseCode", "ISOCode", "ResponseMessage", "RRN"}

	for _, order := range [][]string{full, noDT, fullNoErr, noDTNoErr} {
		var sb strings.Builder
		for _, k := range order {
			sb.WriteString(m[k])
		}
		sb.WriteString(strings.TrimSpace(secret))
		raw := sb.String()

		// Try UTF-8
		if strings.EqualFold(hmacSHA512Upper(secret, []byte(raw)), provided) {
			return true
		}
		// Try UTF-16LE (Azul's PHP reference uses this encoding)
		if strings.EqualFold(hmacSHA512Upper(secret, toUTF16LE(raw)), provided) {
			return true
		}
	}
	return false
}

// hmacSHA512Upper computes HMAC-SHA512(key, data) and returns uppercase hex.
func hmacSHA512Upper(secret string, data []byte) string {
	key := []byte(strings.TrimSpace(secret))
	mac := hmac.New(sha512.New, key)
	mac.Write(data)
	return strings.ToUpper(hex.EncodeToString(mac.Sum(nil)))
}

// toUTF16LE converts a string to UTF-16 Little-Endian bytes.
//
// This matches Azul's PHP reference implementation:
//
//	mb_convert_encoding($str, 'UTF-16LE', 'ASCII')
func toUTF16LE(s string) []byte {
	encoded := utf16.Encode([]rune(s))
	b := make([]byte, len(encoded)*2)
	for i, r := range encoded {
		b[i*2] = byte(r)
		b[i*2+1] = byte(r >> 8)
	}
	return b
}
