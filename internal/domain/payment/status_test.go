package payment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanTransition_AllowedTransitions(t *testing.T) {
	cases := []struct {
		from PaymentStatus
		to   PaymentStatus
	}{
		{New, Processing},
		{New, Canceled},
		{Processing, Completed},
		{Processing, Failed},
		{Processing, Canceled},
	}

	for _, tc := range cases {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			assert.True(t, CanTransition(tc.from, tc.to))
		})
	}
}

func TestCanTransition_ForbiddenTransitions(t *testing.T) {
	cases := []struct {
		from PaymentStatus
		to   PaymentStatus
	}{
		{New, Completed},
		{New, Failed},
		{Processing, New},
		{Completed, Processing},
		{Completed, Canceled},
		{Completed, Failed},
		{Failed, Completed},
		{Failed, Processing},
		{Canceled, Processing},
		{Canceled, Completed},
	}

	for _, tc := range cases {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			assert.False(t, CanTransition(tc.from, tc.to))
		})
	}
}
