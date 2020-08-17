// Code generated by github.com/whyrusleeping/cbor-gen. DO NOT EDIT.

package paych

import (
	"fmt"
	"io"

	abi "github.com/EpiK-Protocol/go-epik-actors/actors/abi"
	crypto "github.com/EpiK-Protocol/go-epik-actors/actors/crypto"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"
)

var _ = xerrors.Errorf

func (t *State) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{134}); err != nil {
		return err
	}

	// t.From (address.Address) (struct)
	if err := t.From.MarshalCBOR(w); err != nil {
		return err
	}

	// t.To (address.Address) (struct)
	if err := t.To.MarshalCBOR(w); err != nil {
		return err
	}

	// t.ToSend (big.Int) (struct)
	if err := t.ToSend.MarshalCBOR(w); err != nil {
		return err
	}

	// t.SettlingAt (abi.ChainEpoch) (int64)
	if t.SettlingAt >= 0 {
		if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, uint64(t.SettlingAt))); err != nil {
			return err
		}
	} else {
		if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajNegativeInt, uint64(-t.SettlingAt)-1)); err != nil {
			return err
		}
	}

	// t.MinSettleHeight (abi.ChainEpoch) (int64)
	if t.MinSettleHeight >= 0 {
		if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, uint64(t.MinSettleHeight))); err != nil {
			return err
		}
	} else {
		if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajNegativeInt, uint64(-t.MinSettleHeight)-1)); err != nil {
			return err
		}
	}

	// t.LaneStates ([]*paych.LaneState) (slice)
	if len(t.LaneStates) > cbg.MaxLength {
		return xerrors.Errorf("Slice value in field t.LaneStates was too long")
	}

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajArray, uint64(len(t.LaneStates)))); err != nil {
		return err
	}
	for _, v := range t.LaneStates {
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

	if extra != 6 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.From (address.Address) (struct)

	{

		if err := t.From.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.From: %w", err)
		}

	}
	// t.To (address.Address) (struct)

	{

		if err := t.To.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.To: %w", err)
		}

	}
	// t.ToSend (big.Int) (struct)

	{

		if err := t.ToSend.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.ToSend: %w", err)
		}

	}
	// t.SettlingAt (abi.ChainEpoch) (int64)
	{
		maj, extra, err := cbg.CborReadHeader(br)
		var extraI int64
		if err != nil {
			return err
		}
		switch maj {
		case cbg.MajUnsignedInt:
			extraI = int64(extra)
			if extraI < 0 {
				return fmt.Errorf("int64 positive overflow")
			}
		case cbg.MajNegativeInt:
			extraI = int64(extra)
			if extraI < 0 {
				return fmt.Errorf("int64 negative oveflow")
			}
			extraI = -1 - extraI
		default:
			return fmt.Errorf("wrong type for int64 field: %d", maj)
		}

		t.SettlingAt = abi.ChainEpoch(extraI)
	}
	// t.MinSettleHeight (abi.ChainEpoch) (int64)
	{
		maj, extra, err := cbg.CborReadHeader(br)
		var extraI int64
		if err != nil {
			return err
		}
		switch maj {
		case cbg.MajUnsignedInt:
			extraI = int64(extra)
			if extraI < 0 {
				return fmt.Errorf("int64 positive overflow")
			}
		case cbg.MajNegativeInt:
			extraI = int64(extra)
			if extraI < 0 {
				return fmt.Errorf("int64 negative oveflow")
			}
			extraI = -1 - extraI
		default:
			return fmt.Errorf("wrong type for int64 field: %d", maj)
		}

		t.MinSettleHeight = abi.ChainEpoch(extraI)
	}
	// t.LaneStates ([]*paych.LaneState) (slice)

	maj, extra, err = cbg.CborReadHeader(br)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.LaneStates: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		t.LaneStates = make([]*LaneState, extra)
	}

	for i := 0; i < int(extra); i++ {

		var v LaneState
		if err := v.UnmarshalCBOR(br); err != nil {
			return err
		}

		t.LaneStates[i] = &v
	}

	return nil
}

func (t *LaneState) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{131}); err != nil {
		return err
	}

	// t.ID (uint64) (uint64)

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, uint64(t.ID))); err != nil {
		return err
	}

	// t.Redeemed (big.Int) (struct)
	if err := t.Redeemed.MarshalCBOR(w); err != nil {
		return err
	}

	// t.Nonce (uint64) (uint64)

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, uint64(t.Nonce))); err != nil {
		return err
	}

	return nil
}

func (t *LaneState) UnmarshalCBOR(r io.Reader) error {
	br := cbg.GetPeeker(r)

	maj, extra, err := cbg.CborReadHeader(br)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 3 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.ID (uint64) (uint64)

	{

		maj, extra, err = cbg.CborReadHeader(br)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.ID = uint64(extra)

	}
	// t.Redeemed (big.Int) (struct)

	{

		if err := t.Redeemed.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Redeemed: %w", err)
		}

	}
	// t.Nonce (uint64) (uint64)

	{

		maj, extra, err = cbg.CborReadHeader(br)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.Nonce = uint64(extra)

	}
	return nil
}

func (t *Merge) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{130}); err != nil {
		return err
	}

	// t.Lane (uint64) (uint64)

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, uint64(t.Lane))); err != nil {
		return err
	}

	// t.Nonce (uint64) (uint64)

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, uint64(t.Nonce))); err != nil {
		return err
	}

	return nil
}

func (t *Merge) UnmarshalCBOR(r io.Reader) error {
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

	// t.Lane (uint64) (uint64)

	{

		maj, extra, err = cbg.CborReadHeader(br)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.Lane = uint64(extra)

	}
	// t.Nonce (uint64) (uint64)

	{

		maj, extra, err = cbg.CborReadHeader(br)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.Nonce = uint64(extra)

	}
	return nil
}

func (t *ConstructorParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{130}); err != nil {
		return err
	}

	// t.From (address.Address) (struct)
	if err := t.From.MarshalCBOR(w); err != nil {
		return err
	}

	// t.To (address.Address) (struct)
	if err := t.To.MarshalCBOR(w); err != nil {
		return err
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

	if extra != 2 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.From (address.Address) (struct)

	{

		if err := t.From.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.From: %w", err)
		}

	}
	// t.To (address.Address) (struct)

	{

		if err := t.To.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.To: %w", err)
		}

	}
	return nil
}

func (t *UpdateChannelStateParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{131}); err != nil {
		return err
	}

	// t.Sv (paych.SignedVoucher) (struct)
	if err := t.Sv.MarshalCBOR(w); err != nil {
		return err
	}

	// t.Secret ([]uint8) (slice)
	if len(t.Secret) > cbg.ByteArrayMaxLen {
		return xerrors.Errorf("Byte array in field t.Secret was too long")
	}

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajByteString, uint64(len(t.Secret)))); err != nil {
		return err
	}
	if _, err := w.Write(t.Secret); err != nil {
		return err
	}

	// t.Proof ([]uint8) (slice)
	if len(t.Proof) > cbg.ByteArrayMaxLen {
		return xerrors.Errorf("Byte array in field t.Proof was too long")
	}

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajByteString, uint64(len(t.Proof)))); err != nil {
		return err
	}
	if _, err := w.Write(t.Proof); err != nil {
		return err
	}
	return nil
}

func (t *UpdateChannelStateParams) UnmarshalCBOR(r io.Reader) error {
	br := cbg.GetPeeker(r)

	maj, extra, err := cbg.CborReadHeader(br)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 3 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Sv (paych.SignedVoucher) (struct)

	{

		if err := t.Sv.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Sv: %w", err)
		}

	}
	// t.Secret ([]uint8) (slice)

	maj, extra, err = cbg.CborReadHeader(br)
	if err != nil {
		return err
	}

	if extra > cbg.ByteArrayMaxLen {
		return fmt.Errorf("t.Secret: byte array too large (%d)", extra)
	}
	if maj != cbg.MajByteString {
		return fmt.Errorf("expected byte array")
	}
	t.Secret = make([]byte, extra)
	if _, err := io.ReadFull(br, t.Secret); err != nil {
		return err
	}
	// t.Proof ([]uint8) (slice)

	maj, extra, err = cbg.CborReadHeader(br)
	if err != nil {
		return err
	}

	if extra > cbg.ByteArrayMaxLen {
		return fmt.Errorf("t.Proof: byte array too large (%d)", extra)
	}
	if maj != cbg.MajByteString {
		return fmt.Errorf("expected byte array")
	}
	t.Proof = make([]byte, extra)
	if _, err := io.ReadFull(br, t.Proof); err != nil {
		return err
	}
	return nil
}

func (t *SignedVoucher) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{138}); err != nil {
		return err
	}

	// t.TimeLockMin (abi.ChainEpoch) (int64)
	if t.TimeLockMin >= 0 {
		if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, uint64(t.TimeLockMin))); err != nil {
			return err
		}
	} else {
		if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajNegativeInt, uint64(-t.TimeLockMin)-1)); err != nil {
			return err
		}
	}

	// t.TimeLockMax (abi.ChainEpoch) (int64)
	if t.TimeLockMax >= 0 {
		if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, uint64(t.TimeLockMax))); err != nil {
			return err
		}
	} else {
		if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajNegativeInt, uint64(-t.TimeLockMax)-1)); err != nil {
			return err
		}
	}

	// t.SecretPreimage ([]uint8) (slice)
	if len(t.SecretPreimage) > cbg.ByteArrayMaxLen {
		return xerrors.Errorf("Byte array in field t.SecretPreimage was too long")
	}

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajByteString, uint64(len(t.SecretPreimage)))); err != nil {
		return err
	}
	if _, err := w.Write(t.SecretPreimage); err != nil {
		return err
	}

	// t.Extra (paych.ModVerifyParams) (struct)
	if err := t.Extra.MarshalCBOR(w); err != nil {
		return err
	}

	// t.Lane (uint64) (uint64)

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, uint64(t.Lane))); err != nil {
		return err
	}

	// t.Nonce (uint64) (uint64)

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, uint64(t.Nonce))); err != nil {
		return err
	}

	// t.Amount (big.Int) (struct)
	if err := t.Amount.MarshalCBOR(w); err != nil {
		return err
	}

	// t.MinSettleHeight (abi.ChainEpoch) (int64)
	if t.MinSettleHeight >= 0 {
		if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, uint64(t.MinSettleHeight))); err != nil {
			return err
		}
	} else {
		if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajNegativeInt, uint64(-t.MinSettleHeight)-1)); err != nil {
			return err
		}
	}

	// t.Merges ([]paych.Merge) (slice)
	if len(t.Merges) > cbg.MaxLength {
		return xerrors.Errorf("Slice value in field t.Merges was too long")
	}

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajArray, uint64(len(t.Merges)))); err != nil {
		return err
	}
	for _, v := range t.Merges {
		if err := v.MarshalCBOR(w); err != nil {
			return err
		}
	}

	// t.Signature (crypto.Signature) (struct)
	if err := t.Signature.MarshalCBOR(w); err != nil {
		return err
	}
	return nil
}

func (t *SignedVoucher) UnmarshalCBOR(r io.Reader) error {
	br := cbg.GetPeeker(r)

	maj, extra, err := cbg.CborReadHeader(br)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 10 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.TimeLockMin (abi.ChainEpoch) (int64)
	{
		maj, extra, err := cbg.CborReadHeader(br)
		var extraI int64
		if err != nil {
			return err
		}
		switch maj {
		case cbg.MajUnsignedInt:
			extraI = int64(extra)
			if extraI < 0 {
				return fmt.Errorf("int64 positive overflow")
			}
		case cbg.MajNegativeInt:
			extraI = int64(extra)
			if extraI < 0 {
				return fmt.Errorf("int64 negative oveflow")
			}
			extraI = -1 - extraI
		default:
			return fmt.Errorf("wrong type for int64 field: %d", maj)
		}

		t.TimeLockMin = abi.ChainEpoch(extraI)
	}
	// t.TimeLockMax (abi.ChainEpoch) (int64)
	{
		maj, extra, err := cbg.CborReadHeader(br)
		var extraI int64
		if err != nil {
			return err
		}
		switch maj {
		case cbg.MajUnsignedInt:
			extraI = int64(extra)
			if extraI < 0 {
				return fmt.Errorf("int64 positive overflow")
			}
		case cbg.MajNegativeInt:
			extraI = int64(extra)
			if extraI < 0 {
				return fmt.Errorf("int64 negative oveflow")
			}
			extraI = -1 - extraI
		default:
			return fmt.Errorf("wrong type for int64 field: %d", maj)
		}

		t.TimeLockMax = abi.ChainEpoch(extraI)
	}
	// t.SecretPreimage ([]uint8) (slice)

	maj, extra, err = cbg.CborReadHeader(br)
	if err != nil {
		return err
	}

	if extra > cbg.ByteArrayMaxLen {
		return fmt.Errorf("t.SecretPreimage: byte array too large (%d)", extra)
	}
	if maj != cbg.MajByteString {
		return fmt.Errorf("expected byte array")
	}
	t.SecretPreimage = make([]byte, extra)
	if _, err := io.ReadFull(br, t.SecretPreimage); err != nil {
		return err
	}
	// t.Extra (paych.ModVerifyParams) (struct)

	{

		pb, err := br.PeekByte()
		if err != nil {
			return err
		}
		if pb == cbg.CborNull[0] {
			var nbuf [1]byte
			if _, err := br.Read(nbuf[:]); err != nil {
				return err
			}
		} else {
			t.Extra = new(ModVerifyParams)
			if err := t.Extra.UnmarshalCBOR(br); err != nil {
				return xerrors.Errorf("unmarshaling t.Extra pointer: %w", err)
			}
		}

	}
	// t.Lane (uint64) (uint64)

	{

		maj, extra, err = cbg.CborReadHeader(br)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.Lane = uint64(extra)

	}
	// t.Nonce (uint64) (uint64)

	{

		maj, extra, err = cbg.CborReadHeader(br)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.Nonce = uint64(extra)

	}
	// t.Amount (big.Int) (struct)

	{

		if err := t.Amount.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Amount: %w", err)
		}

	}
	// t.MinSettleHeight (abi.ChainEpoch) (int64)
	{
		maj, extra, err := cbg.CborReadHeader(br)
		var extraI int64
		if err != nil {
			return err
		}
		switch maj {
		case cbg.MajUnsignedInt:
			extraI = int64(extra)
			if extraI < 0 {
				return fmt.Errorf("int64 positive overflow")
			}
		case cbg.MajNegativeInt:
			extraI = int64(extra)
			if extraI < 0 {
				return fmt.Errorf("int64 negative oveflow")
			}
			extraI = -1 - extraI
		default:
			return fmt.Errorf("wrong type for int64 field: %d", maj)
		}

		t.MinSettleHeight = abi.ChainEpoch(extraI)
	}
	// t.Merges ([]paych.Merge) (slice)

	maj, extra, err = cbg.CborReadHeader(br)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.Merges: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		t.Merges = make([]Merge, extra)
	}

	for i := 0; i < int(extra); i++ {

		var v Merge
		if err := v.UnmarshalCBOR(br); err != nil {
			return err
		}

		t.Merges[i] = v
	}

	// t.Signature (crypto.Signature) (struct)

	{

		pb, err := br.PeekByte()
		if err != nil {
			return err
		}
		if pb == cbg.CborNull[0] {
			var nbuf [1]byte
			if _, err := br.Read(nbuf[:]); err != nil {
				return err
			}
		} else {
			t.Signature = new(crypto.Signature)
			if err := t.Signature.UnmarshalCBOR(br); err != nil {
				return xerrors.Errorf("unmarshaling t.Signature pointer: %w", err)
			}
		}

	}
	return nil
}

func (t *ModVerifyParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{131}); err != nil {
		return err
	}

	// t.Actor (address.Address) (struct)
	if err := t.Actor.MarshalCBOR(w); err != nil {
		return err
	}

	// t.Method (abi.MethodNum) (uint64)

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, uint64(t.Method))); err != nil {
		return err
	}

	// t.Data ([]uint8) (slice)
	if len(t.Data) > cbg.ByteArrayMaxLen {
		return xerrors.Errorf("Byte array in field t.Data was too long")
	}

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajByteString, uint64(len(t.Data)))); err != nil {
		return err
	}
	if _, err := w.Write(t.Data); err != nil {
		return err
	}
	return nil
}

func (t *ModVerifyParams) UnmarshalCBOR(r io.Reader) error {
	br := cbg.GetPeeker(r)

	maj, extra, err := cbg.CborReadHeader(br)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 3 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Actor (address.Address) (struct)

	{

		if err := t.Actor.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Actor: %w", err)
		}

	}
	// t.Method (abi.MethodNum) (uint64)

	{

		maj, extra, err = cbg.CborReadHeader(br)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.Method = abi.MethodNum(extra)

	}
	// t.Data ([]uint8) (slice)

	maj, extra, err = cbg.CborReadHeader(br)
	if err != nil {
		return err
	}

	if extra > cbg.ByteArrayMaxLen {
		return fmt.Errorf("t.Data: byte array too large (%d)", extra)
	}
	if maj != cbg.MajByteString {
		return fmt.Errorf("expected byte array")
	}
	t.Data = make([]byte, extra)
	if _, err := io.ReadFull(br, t.Data); err != nil {
		return err
	}
	return nil
}

func (t *PaymentVerifyParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{130}); err != nil {
		return err
	}

	// t.Extra ([]uint8) (slice)
	if len(t.Extra) > cbg.ByteArrayMaxLen {
		return xerrors.Errorf("Byte array in field t.Extra was too long")
	}

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajByteString, uint64(len(t.Extra)))); err != nil {
		return err
	}
	if _, err := w.Write(t.Extra); err != nil {
		return err
	}

	// t.Proof ([]uint8) (slice)
	if len(t.Proof) > cbg.ByteArrayMaxLen {
		return xerrors.Errorf("Byte array in field t.Proof was too long")
	}

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajByteString, uint64(len(t.Proof)))); err != nil {
		return err
	}
	if _, err := w.Write(t.Proof); err != nil {
		return err
	}
	return nil
}

func (t *PaymentVerifyParams) UnmarshalCBOR(r io.Reader) error {
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

	// t.Extra ([]uint8) (slice)

	maj, extra, err = cbg.CborReadHeader(br)
	if err != nil {
		return err
	}

	if extra > cbg.ByteArrayMaxLen {
		return fmt.Errorf("t.Extra: byte array too large (%d)", extra)
	}
	if maj != cbg.MajByteString {
		return fmt.Errorf("expected byte array")
	}
	t.Extra = make([]byte, extra)
	if _, err := io.ReadFull(br, t.Extra); err != nil {
		return err
	}
	// t.Proof ([]uint8) (slice)

	maj, extra, err = cbg.CborReadHeader(br)
	if err != nil {
		return err
	}

	if extra > cbg.ByteArrayMaxLen {
		return fmt.Errorf("t.Proof: byte array too large (%d)", extra)
	}
	if maj != cbg.MajByteString {
		return fmt.Errorf("expected byte array")
	}
	t.Proof = make([]byte, extra)
	if _, err := io.ReadFull(br, t.Proof); err != nil {
		return err
	}
	return nil
}
