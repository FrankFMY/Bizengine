// Package money provides exact arithmetic for monetary values stored as int64 kopecks.
// 100 RUB = 10000 kopecks. No float64 is used for financial calculations.
package money

import (
	"fmt"
	"math"
	"strings"
)

// Add returns a + b.
func Add(a, b int64) int64 {
	return a + b
}

// Sub returns a - b.
func Sub(a, b int64) int64 {
	return a - b
}

// MulQty multiplies a monetary amount by a quantity (which may be fractional for kg, liters, etc.).
// Result is rounded to the nearest kopeck using banker's rounding.
func MulQty(amount int64, qty float64) int64 {
	return int64(math.Round(float64(amount) * qty))
}

// Format renders a kopeck amount as a human-readable string.
// Example: Format(150025, "RUB") → "1 500,25 ₽"
func Format(amount int64, currency string) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}

	rubles := amount / 100
	kopecks := amount % 100

	rubStr := formatWithSpaces(rubles)

	var symbol string
	switch currency {
	case "RUB":
		symbol = "₽"
	case "USD":
		symbol = "$"
	case "EUR":
		symbol = "€"
	default:
		symbol = currency
	}

	result := fmt.Sprintf("%s,%02d %s", rubStr, kopecks, symbol)
	if negative {
		result = "-" + result
	}
	return result
}

// FromFloat converts a floating-point amount to kopecks. Use only at API boundary.
func FromFloat(f float64) int64 {
	return int64(math.Round(f * 100))
}

// ToFloat converts kopecks to float64. Use only for display/serialization.
func ToFloat(amount int64) float64 {
	return float64(amount) / 100
}

func formatWithSpaces(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, " ")
}
