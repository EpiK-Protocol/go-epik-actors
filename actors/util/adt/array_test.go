package adt_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/require"

	"github.com/EpiK-Protocol/go-epik-actors/actors/util/adt"
	"github.com/EpiK-Protocol/go-epik-actors/support/mock"
)

func TestArrayNotFound(t *testing.T) {
	rt := mock.NewBuilder(context.Background(), address.Undef).Build(t)
	store := adt.AsStore(rt)
	arr := adt.MakeEmptyArray(store)

	found, err := arr.Get(7, nil)
	require.NoError(t, err)
	require.False(t, found)
}
