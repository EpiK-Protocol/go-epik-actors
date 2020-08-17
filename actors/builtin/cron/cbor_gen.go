// Code generated by github.com/whyrusleeping/cbor-gen. DO NOT EDIT.

package cron

import (
	"fmt"
	"io"

	abi "github.com/EpiK-Protocol/go-epik-actors/actors/abi"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"
)

var _ = xerrors.Errorf

func (t *State) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{129}); err != nil {
		return err
	}

	// t.Entries ([]cron.Entry) (slice)
	if len(t.Entries) > cbg.MaxLength {
		return xerrors.Errorf("Slice value in field t.Entries was too long")
	}

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajArray, uint64(len(t.Entries)))); err != nil {
		return err
	}
	for _, v := range t.Entries {
		if err := v.MarshalCBOR(w); err != nil {
			return err
		}
	}
	return nil
}

func (t *State) UnmarshalCBOR(r io.Reader) error {
	br := cbg.GetPeeker(r)

	maj, extra, err := cbg.CborReadHeader(br)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 1 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Entries ([]cron.Entry) (slice)

	maj, extra, err = cbg.CborReadHeader(br)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.Entries: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		t.Entries = make([]Entry, extra)
	}

	for i := 0; i < int(extra); i++ {

		var v Entry
		if err := v.UnmarshalCBOR(br); err != nil {
			return err
		}

		t.Entries[i] = v
	}

	return nil
}

func (t *Entry) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{130}); err != nil {
		return err
	}

	// t.Receiver (address.Address) (struct)
	if err := t.Receiver.MarshalCBOR(w); err != nil {
		return err
	}

	// t.MethodNum (abi.MethodNum) (uint64)

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, uint64(t.MethodNum))); err != nil {
		return err
	}

	return nil
}

func (t *Entry) UnmarshalCBOR(r io.Reader) error {
	br := cbg.GetPeeker(r)

	maj, extra, err := cbg.CborReadHeader(br)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 2 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Receiver (address.Address) (struct)

	{

		if err := t.Receiver.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Receiver: %w", err)
		}

	}
	// t.MethodNum (abi.MethodNum) (uint64)

	{

		maj, extra, err = cbg.CborReadHeader(br)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.MethodNum = abi.MethodNum(extra)

	}
	return nil
}

func (t *ConstructorParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{129}); err != nil {
		return err
	}

	// t.Entries ([]cron.Entry) (slice)
	if len(t.Entries) > cbg.MaxLength {
		return xerrors.Errorf("Slice value in field t.Entries was too long")
	}

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajArray, uint64(len(t.Entries)))); err != nil {
		return err
	}
	for _, v := range t.Entries {
		if err := v.MarshalCBOR(w); err != nil {
			return err
		}
	}
	return nil
}

func (t *ConstructorParams) UnmarshalCBOR(r io.Reader) error {
	br := cbg.GetPeeker(r)

	maj, extra, err := cbg.CborReadHeader(br)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 1 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Entries ([]cron.Entry) (slice)

	maj, extra, err = cbg.CborReadHeader(br)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.Entries: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		t.Entries = make([]Entry, extra)
	}

	for i := 0; i < int(extra); i++ {

		var v Entry
		if err := v.UnmarshalCBOR(br); err != nil {
			return err
		}

		t.Entries[i] = v
	}

	return nil
}
