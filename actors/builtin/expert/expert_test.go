package expert_test

import (
	"testing"

	expert "github.com/filecoin-project/specs-actors/actors/builtin/expert"
	mock "github.com/filecoin-project/specs-actors/support/mock"
)

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, expert.Actor{})
}