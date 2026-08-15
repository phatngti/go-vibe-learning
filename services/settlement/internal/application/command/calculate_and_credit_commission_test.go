package command

import (
	"errors"
	"math"
	"testing"

	sharedmoney "github/master_go_cqrs/internal/shared/money"
)

func TestCommissionBasisPointsUseCheckedMinorUnitRounding(t *testing.T) {
	basisPoints, err := commissionBasisPoints("SILVER")
	if err != nil {
		t.Fatal(err)
	}
	transaction, _ := sharedmoney.NewForCurrency(25, "USD")
	commission, err := transaction.MultiplyBasisPoints(basisPoints)
	if err != nil {
		t.Fatal(err)
	}
	// USD 0.25 * 2% = USD 0.005, rounded half away from zero to one cent.
	if commission.AmountMinor != 1 {
		t.Fatalf("commission minor = %d, want 1", commission.AmountMinor)
	}
}

func TestCommissionCalculationRejectsOverflowAndUnknownRank(t *testing.T) {
	transaction, _ := sharedmoney.NewForCurrency(math.MaxInt64, "USD")
	if _, err := transaction.MultiplyBasisPoints(math.MaxInt64); !errors.Is(err, sharedmoney.ErrOverflow) {
		t.Fatalf("overflow error = %v", err)
	}
	if _, err := commissionBasisPoints("VIP"); err == nil {
		t.Fatal("unknown rank was accepted")
	}
}
