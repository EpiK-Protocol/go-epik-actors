// Code generated by github.com/whyrusleeping/cbor-gen. DO NOT EDIT.

package reward

import (
	"fmt"
	"io"

	abi "github.com/filecoin-project/go-state-types/abi"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"
)

var _ = xerrors.Errorf

var lengthBufState = []byte{136}

func (t *State) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufState); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.ThisEpochReward (big.Int) (struct)
	if err := t.ThisEpochReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.Epoch (abi.ChainEpoch) (int64)
	if t.Epoch >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.Epoch)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.Epoch-1)); err != nil {
			return err
		}
	}

	// t.TotalStoragePowerReward (big.Int) (struct)
	if err := t.TotalStoragePowerReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.TotalExpertReward (big.Int) (struct)
	if err := t.TotalExpertReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.TotalVoteReward (big.Int) (struct)
	if err := t.TotalVoteReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.TotalKnowledgeReward (big.Int) (struct)
	if err := t.TotalKnowledgeReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.TotalRetrievalReward (big.Int) (struct)
	if err := t.TotalRetrievalReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.TotalSendFailed (big.Int) (struct)
	if err := t.TotalSendFailed.MarshalCBOR(w); err != nil {
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

	if extra != 8 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.ThisEpochReward (big.Int) (struct)

	{

		if err := t.ThisEpochReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.ThisEpochReward: %w", err)
		}

	}
	// t.Epoch (abi.ChainEpoch) (int64)
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

		t.Epoch = abi.ChainEpoch(extraI)
	}
	// t.TotalStoragePowerReward (big.Int) (struct)

	{

		if err := t.TotalStoragePowerReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.TotalStoragePowerReward: %w", err)
		}

	}
	// t.TotalExpertReward (big.Int) (struct)

	{

		if err := t.TotalExpertReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.TotalExpertReward: %w", err)
		}

	}
	// t.TotalVoteReward (big.Int) (struct)

	{

		if err := t.TotalVoteReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.TotalVoteReward: %w", err)
		}

	}
	// t.TotalKnowledgeReward (big.Int) (struct)

	{

		if err := t.TotalKnowledgeReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.TotalKnowledgeReward: %w", err)
		}

	}
	// t.TotalRetrievalReward (big.Int) (struct)

	{

		if err := t.TotalRetrievalReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.TotalRetrievalReward: %w", err)
		}

	}
	// t.TotalSendFailed (big.Int) (struct)

	{

		if err := t.TotalSendFailed.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.TotalSendFailed: %w", err)
		}

	}
	return nil
}

var lengthBufAwardBlockRewardParams = []byte{134}

func (t *AwardBlockRewardParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufAwardBlockRewardParams); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.Miner (address.Address) (struct)
	if err := t.Miner.MarshalCBOR(w); err != nil {
		return err
	}

	// t.Penalty (big.Int) (struct)
	if err := t.Penalty.MarshalCBOR(w); err != nil {
		return err
	}

	// t.GasReward (big.Int) (struct)
	if err := t.GasReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.WinCount (int64) (int64)
	if t.WinCount >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.WinCount)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.WinCount-1)); err != nil {
			return err
		}
	}

	// t.ShareCount (int64) (int64)
	if t.ShareCount >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.ShareCount)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.ShareCount-1)); err != nil {
			return err
		}
	}

	// t.RetrievalPledged (big.Int) (struct)
	if err := t.RetrievalPledged.MarshalCBOR(w); err != nil {
		return err
	}
	return nil
}

func (t *AwardBlockRewardParams) UnmarshalCBOR(r io.Reader) error {
	*t = AwardBlockRewardParams{}

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

	// t.Miner (address.Address) (struct)

	{

		if err := t.Miner.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Miner: %w", err)
		}

	}
	// t.Penalty (big.Int) (struct)

	{

		if err := t.Penalty.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Penalty: %w", err)
		}

	}
	// t.GasReward (big.Int) (struct)

	{

		if err := t.GasReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.GasReward: %w", err)
		}

	}
	// t.WinCount (int64) (int64)
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

		t.WinCount = int64(extraI)
	}
	// t.ShareCount (int64) (int64)
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

		t.ShareCount = int64(extraI)
	}
	// t.RetrievalPledged (big.Int) (struct)

	{

		if err := t.RetrievalPledged.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.RetrievalPledged: %w", err)
		}

	}
	return nil
}

var lengthBufAwardBlockRewardReturn = []byte{135}

func (t *AwardBlockRewardReturn) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufAwardBlockRewardReturn); err != nil {
		return err
	}

	// t.PowerReward (big.Int) (struct)
	if err := t.PowerReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.GasReward (big.Int) (struct)
	if err := t.GasReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.VoteReward (big.Int) (struct)
	if err := t.VoteReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.ExpertReward (big.Int) (struct)
	if err := t.ExpertReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.RetrievalReward (big.Int) (struct)
	if err := t.RetrievalReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.KnowledgeReward (big.Int) (struct)
	if err := t.KnowledgeReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.SendFailed (big.Int) (struct)
	if err := t.SendFailed.MarshalCBOR(w); err != nil {
		return err
	}
	return nil
}

func (t *AwardBlockRewardReturn) UnmarshalCBOR(r io.Reader) error {
	*t = AwardBlockRewardReturn{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 7 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.PowerReward (big.Int) (struct)

	{

		if err := t.PowerReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.PowerReward: %w", err)
		}

	}
	// t.GasReward (big.Int) (struct)

	{

		if err := t.GasReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.GasReward: %w", err)
		}

	}
	// t.VoteReward (big.Int) (struct)

	{

		if err := t.VoteReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.VoteReward: %w", err)
		}

	}
	// t.ExpertReward (big.Int) (struct)

	{

		if err := t.ExpertReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.ExpertReward: %w", err)
		}

	}
	// t.RetrievalReward (big.Int) (struct)

	{

		if err := t.RetrievalReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.RetrievalReward: %w", err)
		}

	}
	// t.KnowledgeReward (big.Int) (struct)

	{

		if err := t.KnowledgeReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.KnowledgeReward: %w", err)
		}

	}
	// t.SendFailed (big.Int) (struct)

	{

		if err := t.SendFailed.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.SendFailed: %w", err)
		}

	}
	return nil
}

var lengthBufThisEpochRewardReturn = []byte{136}

func (t *ThisEpochRewardReturn) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufThisEpochRewardReturn); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.Epoch (abi.ChainEpoch) (int64)
	if t.Epoch >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.Epoch)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.Epoch-1)); err != nil {
			return err
		}
	}

	// t.ThisEpochReward (big.Int) (struct)
	if err := t.ThisEpochReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.TotalStoragePowerReward (big.Int) (struct)
	if err := t.TotalStoragePowerReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.TotalExpertReward (big.Int) (struct)
	if err := t.TotalExpertReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.TotalVoteReward (big.Int) (struct)
	if err := t.TotalVoteReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.TotalKnowledgeReward (big.Int) (struct)
	if err := t.TotalKnowledgeReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.TotalRetrievalReward (big.Int) (struct)
	if err := t.TotalRetrievalReward.MarshalCBOR(w); err != nil {
		return err
	}

	// t.TotalSendFailed (big.Int) (struct)
	if err := t.TotalSendFailed.MarshalCBOR(w); err != nil {
		return err
	}
	return nil
}

func (t *ThisEpochRewardReturn) UnmarshalCBOR(r io.Reader) error {
	*t = ThisEpochRewardReturn{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 8 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Epoch (abi.ChainEpoch) (int64)
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

		t.Epoch = abi.ChainEpoch(extraI)
	}
	// t.ThisEpochReward (big.Int) (struct)

	{

		if err := t.ThisEpochReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.ThisEpochReward: %w", err)
		}

	}
	// t.TotalStoragePowerReward (big.Int) (struct)

	{

		if err := t.TotalStoragePowerReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.TotalStoragePowerReward: %w", err)
		}

	}
	// t.TotalExpertReward (big.Int) (struct)

	{

		if err := t.TotalExpertReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.TotalExpertReward: %w", err)
		}

	}
	// t.TotalVoteReward (big.Int) (struct)

	{

		if err := t.TotalVoteReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.TotalVoteReward: %w", err)
		}

	}
	// t.TotalKnowledgeReward (big.Int) (struct)

	{

		if err := t.TotalKnowledgeReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.TotalKnowledgeReward: %w", err)
		}

	}
	// t.TotalRetrievalReward (big.Int) (struct)

	{

		if err := t.TotalRetrievalReward.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.TotalRetrievalReward: %w", err)
		}

	}
	// t.TotalSendFailed (big.Int) (struct)

	{

		if err := t.TotalSendFailed.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.TotalSendFailed: %w", err)
		}

	}
	return nil
}
