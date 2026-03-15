package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultPageRequest(t *testing.T) {
	p := DefaultPageRequest()

	assert.Equal(t, 50, p.Limit)
	assert.Equal(t, 0, p.Offset)
	assert.Equal(t, "created_at", p.Sort)
	assert.Equal(t, "desc", p.Order)
}

func TestPageRequestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   PageRequest
		want PageRequest
	}{
		{
			name: "valid values unchanged",
			in:   PageRequest{Limit: 25, Offset: 10, Sort: "name", Order: "asc"},
			want: PageRequest{Limit: 25, Offset: 10, Sort: "name", Order: "asc"},
		},
		{
			name: "zero limit gets default",
			in:   PageRequest{Limit: 0, Offset: 0, Sort: "name", Order: "asc"},
			want: PageRequest{Limit: 50, Offset: 0, Sort: "name", Order: "asc"},
		},
		{
			name: "negative limit gets default",
			in:   PageRequest{Limit: -10, Offset: 0, Sort: "name", Order: "asc"},
			want: PageRequest{Limit: 50, Offset: 0, Sort: "name", Order: "asc"},
		},
		{
			name: "limit exceeding max gets capped",
			in:   PageRequest{Limit: 500, Offset: 0, Sort: "name", Order: "asc"},
			want: PageRequest{Limit: 200, Offset: 0, Sort: "name", Order: "asc"},
		},
		{
			name: "limit exactly at max stays",
			in:   PageRequest{Limit: 200, Offset: 0, Sort: "name", Order: "asc"},
			want: PageRequest{Limit: 200, Offset: 0, Sort: "name", Order: "asc"},
		},
		{
			name: "negative offset gets zero",
			in:   PageRequest{Limit: 50, Offset: -5, Sort: "name", Order: "asc"},
			want: PageRequest{Limit: 50, Offset: 0, Sort: "name", Order: "asc"},
		},
		{
			name: "empty sort gets default",
			in:   PageRequest{Limit: 50, Offset: 0, Sort: "", Order: "asc"},
			want: PageRequest{Limit: 50, Offset: 0, Sort: "created_at", Order: "asc"},
		},
		{
			name: "invalid order defaults to desc",
			in:   PageRequest{Limit: 50, Offset: 0, Sort: "name", Order: "invalid"},
			want: PageRequest{Limit: 50, Offset: 0, Sort: "name", Order: "desc"},
		},
		{
			name: "empty order defaults to desc",
			in:   PageRequest{Limit: 50, Offset: 0, Sort: "name", Order: ""},
			want: PageRequest{Limit: 50, Offset: 0, Sort: "name", Order: "desc"},
		},
		{
			name: "all defaults at once",
			in:   PageRequest{},
			want: PageRequest{Limit: 50, Offset: 0, Sort: "created_at", Order: "desc"},
		},
		{
			name: "asc order preserved",
			in:   PageRequest{Limit: 10, Offset: 0, Sort: "updated_at", Order: "asc"},
			want: PageRequest{Limit: 10, Offset: 0, Sort: "updated_at", Order: "asc"},
		},
		{
			name: "desc order preserved",
			in:   PageRequest{Limit: 10, Offset: 0, Sort: "updated_at", Order: "desc"},
			want: PageRequest{Limit: 10, Offset: 0, Sort: "updated_at", Order: "desc"},
		},
		{
			name: "limit 1 is valid",
			in:   PageRequest{Limit: 1, Offset: 0, Sort: "id", Order: "asc"},
			want: PageRequest{Limit: 1, Offset: 0, Sort: "id", Order: "asc"},
		},
		{
			name: "large valid offset preserved",
			in:   PageRequest{Limit: 50, Offset: 10000, Sort: "created_at", Order: "desc"},
			want: PageRequest{Limit: 50, Offset: 10000, Sort: "created_at", Order: "desc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.in
			p.Normalize()

			assert.Equal(t, tt.want.Limit, p.Limit)
			assert.Equal(t, tt.want.Offset, p.Offset)
			assert.Equal(t, tt.want.Sort, p.Sort)
			assert.Equal(t, tt.want.Order, p.Order)
		})
	}
}

func TestNormalizeIdempotent(t *testing.T) {
	p := PageRequest{Limit: 25, Offset: 5, Sort: "name", Order: "asc"}

	p.Normalize()
	first := p

	p.Normalize()
	assert.Equal(t, first, p)
}

func TestDefaultPageRequestAlreadyNormalized(t *testing.T) {
	p := DefaultPageRequest()
	before := p
	p.Normalize()

	assert.Equal(t, before, p)
}
