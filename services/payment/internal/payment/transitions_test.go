package payment_test

import (
	"testing"

	"github.com/remodov/marketplace-system-go/services/payment/internal/payment"
)

func TestTransitions_allowedExactlyAsDescribed(t *testing.T) {
	allowed := [][2]payment.Status{
		{payment.StatusAuthorized, payment.StatusCaptured},
		{payment.StatusAuthorized, payment.StatusRefunded},
		{payment.StatusAuthorized, payment.StatusFailed},
		{payment.StatusCaptured, payment.StatusRefunded},
	}
	for _, pair := range allowed {
		if !pair[0].CanMoveTo(pair[1]) {
			t.Errorf("переход %s -> %s должен быть разрешён", pair[0], pair[1])
		}
	}
}

func TestTransitions_terminalStatesLeadNowhere(t *testing.T) {
	forbidden := [][2]payment.Status{
		{payment.StatusRefunded, payment.StatusCaptured},
		{payment.StatusRefunded, payment.StatusAuthorized},
		{payment.StatusFailed, payment.StatusCaptured},
		{payment.StatusCaptured, payment.StatusAuthorized},
	}
	for _, pair := range forbidden {
		if pair[0].CanMoveTo(pair[1]) {
			t.Errorf("переход %s -> %s должен быть запрещён", pair[0], pair[1])
		}
	}
}

func TestTransitions_selfTransitionIsNotATransition(t *testing.T) {
	for _, s := range []payment.Status{payment.StatusAuthorized, payment.StatusCaptured, payment.StatusRefunded, payment.StatusFailed} {
		if s.CanMoveTo(s) {
			t.Errorf("переход %s -> %s не переход: повтор обрабатывается выше, а не в автомате", s, s)
		}
	}
}
