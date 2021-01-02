// Code generated by github.com/whyrusleeping/cbor-gen. DO NOT EDIT.

package vote

import (
	"fmt"
	"io"

	abi "github.com/filecoin-project/go-state-types/abi"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"
)

var _ = xerrors.Errorf

var lengthBufState = []byte{134}

func (t *State) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufState); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.Candidates (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.Candidates); err != nil {
		return xerrors.Errorf("failed to write cid field t.Candidates: %w", err)
	}

	// t.Voters (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.Voters); err != nil {
		return xerrors.Errorf("failed to write cid field t.Voters: %w", err)
	}

	// t.TotalVotes (big.Int) (struct)
	if err := t.TotalVotes.MarshalCBOR(w); err != nil {
		return err
	}

	// t.UnownedFunds (big.Int) (struct)
	if err := t.UnownedFunds.MarshalCBOR(w); err != nil {
		return err
	}

	// t.CumEarningsPerVote (big.Int) (struct)
	if err := t.CumEarningsPerVote.MarshalCBOR(w); err != nil {
		return err
	}

	// t.FallbackReceiver (address.Address) (struct)
	if err := t.FallbackReceiver.MarshalCBOR(w); err != nil {
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

	if extra != 6 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Candidates (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.Candidates: %w", err)
		}

		t.Candidates = c

	}
	// t.Voters (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.Voters: %w", err)
		}

		t.Voters = c

	}
	// t.TotalVotes (big.Int) (struct)

	{

		if err := t.TotalVotes.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.TotalVotes: %w", err)
		}

	}
	// t.UnownedFunds (big.Int) (struct)

	{

		if err := t.UnownedFunds.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.UnownedFunds: %w", err)
		}

	}
	// t.CumEarningsPerVote (big.Int) (struct)

	{

		if err := t.CumEarningsPerVote.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.CumEarningsPerVote: %w", err)
		}

	}
	// t.FallbackReceiver (address.Address) (struct)

	{

		if err := t.FallbackReceiver.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.FallbackReceiver: %w", err)
		}

	}
	return nil
}

var lengthBufCandidate = []byte{131}

func (t *Candidate) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufCandidate); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.BlockEpoch (abi.ChainEpoch) (int64)
	if t.BlockEpoch >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.BlockEpoch)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.BlockEpoch-1)); err != nil {
			return err
		}
	}

	// t.BlockCumEarningsPerVote (big.Int) (struct)
	if err := t.BlockCumEarningsPerVote.MarshalCBOR(w); err != nil {
		return err
	}

	// t.Votes (big.Int) (struct)
	if err := t.Votes.MarshalCBOR(w); err != nil {
		return err
	}
	return nil
}

func (t *Candidate) UnmarshalCBOR(r io.Reader) error {
	*t = Candidate{}

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

	// t.BlockEpoch (abi.ChainEpoch) (int64)
	{
		maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
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

		t.BlockEpoch = abi.ChainEpoch(extraI)
	}
	// t.BlockCumEarningsPerVote (big.Int) (struct)

	{

		if err := t.BlockCumEarningsPerVote.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.BlockCumEarningsPerVote: %w", err)
		}

	}
	// t.Votes (big.Int) (struct)

	{

		if err := t.Votes.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Votes: %w", err)
		}

	}
	return nil
}

var lengthBufVoter = []byte{132}

func (t *Voter) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufVoter); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.SettleEpoch (abi.ChainEpoch) (int64)
	if t.SettleEpoch >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.SettleEpoch)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.SettleEpoch-1)); err != nil {
			return err
		}
	}

	// t.SettleCumEarningsPerVote (big.Int) (struct)
	if err := t.SettleCumEarningsPerVote.MarshalCBOR(w); err != nil {
		return err
	}

	// t.UnclaimedFunds (big.Int) (struct)
	if err := t.UnclaimedFunds.MarshalCBOR(w); err != nil {
		return err
	}

	// t.VotingRecords (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.VotingRecords); err != nil {
		return xerrors.Errorf("failed to write cid field t.VotingRecords: %w", err)
	}

	return nil
}

func (t *Voter) UnmarshalCBOR(r io.Reader) error {
	*t = Voter{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 4 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.SettleEpoch (abi.ChainEpoch) (int64)
	{
		maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
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

		t.SettleEpoch = abi.ChainEpoch(extraI)
	}
	// t.SettleCumEarningsPerVote (big.Int) (struct)

	{

		if err := t.SettleCumEarningsPerVote.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.SettleCumEarningsPerVote: %w", err)
		}

	}
	// t.UnclaimedFunds (big.Int) (struct)

	{

		if err := t.UnclaimedFunds.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.UnclaimedFunds: %w", err)
		}

	}
	// t.VotingRecords (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.VotingRecords: %w", err)
		}

		t.VotingRecords = c

	}
	return nil
}

var lengthBufVotingRecord = []byte{131}

func (t *VotingRecord) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufVotingRecord); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.Votes (big.Int) (struct)
	if err := t.Votes.MarshalCBOR(w); err != nil {
		return err
	}

	// t.RescindingVotes (big.Int) (struct)
	if err := t.RescindingVotes.MarshalCBOR(w); err != nil {
		return err
	}

	// t.LastRescindEpoch (abi.ChainEpoch) (int64)
	if t.LastRescindEpoch >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.LastRescindEpoch)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.LastRescindEpoch-1)); err != nil {
			return err
		}
	}
	return nil
}

func (t *VotingRecord) UnmarshalCBOR(r io.Reader) error {
	*t = VotingRecord{}

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

	// t.Votes (big.Int) (struct)

	{

		if err := t.Votes.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Votes: %w", err)
		}

	}
	// t.RescindingVotes (big.Int) (struct)

	{

		if err := t.RescindingVotes.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.RescindingVotes: %w", err)
		}

	}
	// t.LastRescindEpoch (abi.ChainEpoch) (int64)
	{
		maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
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

		t.LastRescindEpoch = abi.ChainEpoch(extraI)
	}
	return nil
}

var lengthBufRescindParams = []byte{130}

func (t *RescindParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufRescindParams); err != nil {
		return err
	}

	// t.Candidate (address.Address) (struct)
	if err := t.Candidate.MarshalCBOR(w); err != nil {
		return err
	}

	// t.Votes (big.Int) (struct)
	if err := t.Votes.MarshalCBOR(w); err != nil {
		return err
	}
	return nil
}

func (t *RescindParams) UnmarshalCBOR(r io.Reader) error {
	*t = RescindParams{}

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

	// t.Candidate (address.Address) (struct)

	{

		if err := t.Candidate.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Candidate: %w", err)
		}

	}
	// t.Votes (big.Int) (struct)

	{

		if err := t.Votes.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Votes: %w", err)
		}

	}
	return nil
}
