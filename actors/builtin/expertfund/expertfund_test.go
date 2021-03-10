package expertfund_test

import (
	"testing"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expertfund"
	"github.com/filecoin-project/specs-actors/v2/support/mock"
)

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, expertfund.Actor{})
}
