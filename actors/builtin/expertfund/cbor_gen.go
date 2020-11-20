// Code generated by github.com/whyrusleeping/cbor-gen. DO NOT EDIT.

package expertfund

import (
	"fmt"
	"io"

	abi "github.com/filecoin-project/go-state-types/abi"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"
)

var _ = xerrors.Errorf

var lengthBufState = []byte{131}

func (t *State) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufState); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.Experts (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.Experts); err != nil {
		return xerrors.Errorf("failed to write cid field t.Experts: %w", err)
	}

	// t.TotalExpertDataSize (abi.PaddedPieceSize) (uint64)

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.TotalExpertDataSize)); err != nil {
		return err
	}

	// t.TotalExpertReward (big.Int) (struct)
	if err := t.TotalExpertReward.MarshalCBOR(w); err != nil {
		return err
	}
	return nil
}

func (t *State) UnmarshalCBOR(r io.Reader) error {
	*t = State{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 3 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Experts (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.Experts: %w", err)
		}

		t.Experts = c

	}
	// t.TotalExpertDataSize (abi.PaddedPieceSize) (uint64)

	{

		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.TotalExpertDataSize = abi.PaddedPieceSize(extra)

	}
	// t.TotalExpertReward (big.Int) (struct)

	{

		if err := t.TotalExpertReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.TotalExpertReward: %w", err)
		}

	}
	return nil
}

var lengthBufExpertInfo = []byte{130}

func (t *ExpertInfo) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufExpertInfo); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.DataSize (abi.PaddedPieceSize) (uint64)

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.DataSize)); err != nil {
		return err
	}

	// t.RewardDebt (big.Int) (struct)
	if err := t.RewardDebt.MarshalCBOR(w); err != nil {
		return err
	}
	return nil
}

func (t *ExpertInfo) UnmarshalCBOR(r io.Reader) error {
	*t = ExpertInfo{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 2 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.DataSize (abi.PaddedPieceSize) (uint64)

	{

		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.DataSize = abi.PaddedPieceSize(extra)

	}
	// t.RewardDebt (big.Int) (struct)

	{

		if err := t.RewardDebt.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.RewardDebt: %w", err)
		}

	}
	return nil
}

var lengthBufExpertDepositParams = []byte{131}

func (t *ExpertDepositParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufExpertDepositParams); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.Expert (address.Address) (struct)
	if err := t.Expert.MarshalCBOR(w); err != nil {
		return err
	}

	// t.PieceID (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.PieceID); err != nil {
		return xerrors.Errorf("failed to write cid field t.PieceID: %w", err)
	}

	// t.PieceSize (abi.PaddedPieceSize) (uint64)

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.PieceSize)); err != nil {
		return err
	}

	return nil
}

func (t *ExpertDepositParams) UnmarshalCBOR(r io.Reader) error {
	*t = ExpertDepositParams{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 3 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Expert (address.Address) (struct)

	{

		if err := t.Expert.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Expert: %w", err)
		}

	}
	// t.PieceID (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.PieceID: %w", err)
		}

		t.PieceID = c

	}
	// t.PieceSize (abi.PaddedPieceSize) (uint64)

	{

		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.PieceSize = abi.PaddedPieceSize(extra)

	}
	return nil
}

var lengthBufClaimFundParams = []byte{130}

func (t *ClaimFundParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufClaimFundParams); err != nil {
		return err
	}

	// t.Expert (address.Address) (struct)
	if err := t.Expert.MarshalCBOR(w); err != nil {
		return err
	}

	// t.Amount (big.Int) (struct)
	if err := t.Amount.MarshalCBOR(w); err != nil {
		return err
	}
	return nil
}

func (t *ClaimFundParams) UnmarshalCBOR(r io.Reader) error {
	*t = ClaimFundParams{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 2 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Expert (address.Address) (struct)

	{

		if err := t.Expert.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Expert: %w", err)
		}

	}
	// t.Amount (big.Int) (struct)

	{

		if err := t.Amount.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Amount: %w", err)
		}

	}
	return nil
}

var lengthBufResetExpertParams = []byte{129}

func (t *ResetExpertParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufResetExpertParams); err != nil {
		return err
	}

	// t.Expert (address.Address) (struct)
	if err := t.Expert.MarshalCBOR(w); err != nil {
		return err
	}
	return nil
}

func (t *ResetExpertParams) UnmarshalCBOR(r io.Reader) error {
	*t = ResetExpertParams{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 1 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Expert (address.Address) (struct)

	{

		if err := t.Expert.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Expert: %w", err)
		}

	}
	return nil
}
