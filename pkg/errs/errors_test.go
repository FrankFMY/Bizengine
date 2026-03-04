package errs

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError_Error(t *testing.T) {
	err := NewNotFound("entity not found")
	assert.Equal(t, "NOT_FOUND: entity not found", err.Error())
}

func TestError_WithCause(t *testing.T) {
	cause := fmt.Errorf("pg: no rows")
	err := Wrap(cause, CodeNotFound, "entity not found")
	assert.Contains(t, err.Error(), "pg: no rows")
	assert.True(t, errors.Is(err, cause))
}

func TestGetCode(t *testing.T) {
	tests := []struct {
		err  error
		want Code
	}{
		{NewBadRequest("bad"), CodeBadRequest},
		{NewUnauthorized("unauth"), CodeUnauthorized},
		{NewForbidden("forbidden"), CodeForbidden},
		{NewNotFound("not found"), CodeNotFound},
		{NewConflict("conflict"), CodeConflict},
		{NewInternal("internal"), CodeInternal},
		{fmt.Errorf("unknown"), CodeInternal},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, GetCode(tt.err))
	}
}

func TestHTTPStatus(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{NewBadRequest("bad"), http.StatusBadRequest},
		{NewUnauthorized("unauth"), http.StatusUnauthorized},
		{NewForbidden("forbidden"), http.StatusForbidden},
		{NewNotFound("not found"), http.StatusNotFound},
		{NewConflict("conflict"), http.StatusConflict},
		{NewInternal("internal"), http.StatusInternalServerError},
		{fmt.Errorf("unknown"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, HTTPStatus(tt.err))
	}
}

func TestGetMessage(t *testing.T) {
	assert.Equal(t, "entity not found", GetMessage(NewNotFound("entity not found")))
	assert.Equal(t, "internal error", GetMessage(fmt.Errorf("unknown")))
}
