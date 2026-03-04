package money

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdd(t *testing.T) {
	assert.Equal(t, int64(300), Add(100, 200))
	assert.Equal(t, int64(-100), Add(100, -200))
}

func TestSub(t *testing.T) {
	assert.Equal(t, int64(100), Sub(300, 200))
}

func TestMulQty(t *testing.T) {
	// 250 RUB * 3 pieces = 750 RUB
	assert.Equal(t, int64(75000), MulQty(25000, 3))

	// 100 RUB/kg * 1.5 kg = 150 RUB
	assert.Equal(t, int64(15000), MulQty(10000, 1.5))

	// Rounding: 33.33 RUB * 3 = 99.99 RUB
	assert.Equal(t, int64(9999), MulQty(3333, 3))
}

func TestFormat(t *testing.T) {
	tests := []struct {
		amount   int64
		currency string
		want     string
	}{
		{150025, "RUB", "1 500,25 ₽"},
		{10000, "RUB", "100,00 ₽"},
		{50, "RUB", "0,50 ₽"},
		{0, "RUB", "0,00 ₽"},
		{-150025, "RUB", "-1 500,25 ₽"},
		{100000000, "RUB", "1 000 000,00 ₽"},
		{10000, "USD", "100,00 $"},
		{10000, "EUR", "100,00 €"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, Format(tt.amount, tt.currency))
	}
}

func TestFromFloat(t *testing.T) {
	assert.Equal(t, int64(10000), FromFloat(100.00))
	assert.Equal(t, int64(15050), FromFloat(150.50))
	assert.Equal(t, int64(10), FromFloat(0.1))
	// Classic floating-point trap: 0.1 + 0.2
	assert.Equal(t, int64(30), FromFloat(0.1+0.2))
}

func TestToFloat(t *testing.T) {
	assert.Equal(t, 100.0, ToFloat(10000))
	assert.Equal(t, 150.5, ToFloat(15050))
}
