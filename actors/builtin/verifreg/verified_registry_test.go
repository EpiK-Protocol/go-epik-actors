package verifreg_test

import (
	"testing"

	"github.com/EpiK-Protocol/go-epik-actors/actors/builtin/verifreg"
	"github.com/EpiK-Protocol/go-epik-actors/support/mock"
)

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, verifreg.Actor{})
}
