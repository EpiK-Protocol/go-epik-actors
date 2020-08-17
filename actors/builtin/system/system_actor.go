package system

import (
	abi "github.com/EpiK-Protocol/go-epik-actors/actors/abi"
	builtin "github.com/EpiK-Protocol/go-epik-actors/actors/builtin"
	runtime "github.com/EpiK-Protocol/go-epik-actors/actors/runtime"
	adt "github.com/EpiK-Protocol/go-epik-actors/actors/util/adt"
)

type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
	}
}

var _ abi.Invokee = Actor{}

func (a Actor) Constructor(rt runtime.Runtime, _ *adt.EmptyValue) *adt.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	rt.State().Create(&State{})
	return nil
}

type State struct{}
