// Code generated by github.com/whyrusleeping/cbor-gen. DO NOT EDIT.

package market

import (
	"fmt"
	"io"

	abi "github.com/filecoin-project/go-state-types/abi"
	builtin "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"
)

var _ = xerrors.Errorf

var lengthBufState = []byte{138}

func (t *State) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufState); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.Proposals (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.Proposals); err != nil {
		return xerrors.Errorf("failed to write cid field t.Proposals: %w", err)
	}

	// t.States (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.States); err != nil {
		return xerrors.Errorf("failed to write cid field t.States: %w", err)
	}

	// t.Quotas (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.Quotas); err != nil {
		return xerrors.Errorf("failed to write cid field t.Quotas: %w", err)
	}

	// t.PendingProposals (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.PendingProposals); err != nil {
		return xerrors.Errorf("failed to write cid field t.PendingProposals: %w", err)
	}

	// t.EscrowTable (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.EscrowTable); err != nil {
		return xerrors.Errorf("failed to write cid field t.EscrowTable: %w", err)
	}

	// t.LockedTable (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.LockedTable); err != nil {
		return xerrors.Errorf("failed to write cid field t.LockedTable: %w", err)
	}

	// t.NextID (abi.DealID) (uint64)

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.NextID)); err != nil {
		return err
	}

	// t.DealOpsByEpoch (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.DealOpsByEpoch); err != nil {
		return xerrors.Errorf("failed to write cid field t.DealOpsByEpoch: %w", err)
	}

	// t.LastCron (abi.ChainEpoch) (int64)
	if t.LastCron >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.LastCron)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.LastCron-1)); err != nil {
			return err
		}
	}

	// t.InitialQuota (int64) (int64)
	if t.InitialQuota >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.InitialQuota)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.InitialQuota-1)); err != nil {
			return err
		}
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

	if extra != 10 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Proposals (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.Proposals: %w", err)
		}

		t.Proposals = c

	}
	// t.States (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.States: %w", err)
		}

		t.States = c

	}
	// t.Quotas (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.Quotas: %w", err)
		}

		t.Quotas = c

	}
	// t.PendingProposals (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.PendingProposals: %w", err)
		}

		t.PendingProposals = c

	}
	// t.EscrowTable (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.EscrowTable: %w", err)
		}

		t.EscrowTable = c

	}
	// t.LockedTable (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.LockedTable: %w", err)
		}

		t.LockedTable = c

	}
	// t.NextID (abi.DealID) (uint64)

	{

		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.NextID = abi.DealID(extra)

	}
	// t.DealOpsByEpoch (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.DealOpsByEpoch: %w", err)
		}

		t.DealOpsByEpoch = c

	}
	// t.LastCron (abi.ChainEpoch) (int64)
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

		t.LastCron = abi.ChainEpoch(extraI)
	}
	// t.InitialQuota (int64) (int64)
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

		t.InitialQuota = int64(extraI)
	}
	return nil
}

var lengthBufWithdrawBalanceParams = []byte{130}

func (t *WithdrawBalanceParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufWithdrawBalanceParams); err != nil {
		return err
	}

	// t.ProviderOrClientAddress (address.Address) (struct)
	if err := t.ProviderOrClientAddress.MarshalCBOR(w); err != nil {
		return err
	}

	// t.Amount (big.Int) (struct)
	if err := t.Amount.MarshalCBOR(w); err != nil {
		return err
	}
	return nil
}

func (t *WithdrawBalanceParams) UnmarshalCBOR(r io.Reader) error {
	*t = WithdrawBalanceParams{}

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

	// t.ProviderOrClientAddress (address.Address) (struct)

	{

		if err := t.ProviderOrClientAddress.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.ProviderOrClientAddress: %w", err)
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

var lengthBufPublishStorageDataRef = []byte{131}

func (t *PublishStorageDataRef) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufPublishStorageDataRef); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.RootCID (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.RootCID); err != nil {
		return xerrors.Errorf("failed to write cid field t.RootCID: %w", err)
	}

	// t.Expert (string) (string)
	if len(t.Expert) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.Expert was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len(t.Expert))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.Expert)); err != nil {
		return err
	}

	// t.Bounty (string) (string)
	if len(t.Bounty) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.Bounty was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len(t.Bounty))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.Bounty)); err != nil {
		return err
	}
	return nil
}

func (t *PublishStorageDataRef) UnmarshalCBOR(r io.Reader) error {
	*t = PublishStorageDataRef{}

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

	// t.RootCID (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.RootCID: %w", err)
		}

		t.RootCID = c

	}
	// t.Expert (string) (string)

	{
		sval, err := cbg.ReadStringBuf(br, scratch)
		if err != nil {
			return err
		}

		t.Expert = string(sval)
	}
	// t.Bounty (string) (string)

	{
		sval, err := cbg.ReadStringBuf(br, scratch)
		if err != nil {
			return err
		}

		t.Bounty = string(sval)
	}
	return nil
}

var lengthBufPublishStorageDealsParams = []byte{130}

func (t *PublishStorageDealsParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufPublishStorageDealsParams); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.Deals ([]market.ClientDealProposal) (slice)
	if len(t.Deals) > cbg.MaxLength {
		return xerrors.Errorf("Slice value in field t.Deals was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajArray, uint64(len(t.Deals))); err != nil {
		return err
	}
	for _, v := range t.Deals {
		if err := v.MarshalCBOR(w); err != nil {
			return err
		}
	}

	// t.DataRef (market.PublishStorageDataRef) (struct)
	if err := t.DataRef.MarshalCBOR(w); err != nil {
		return err
	}
	return nil
}

func (t *PublishStorageDealsParams) UnmarshalCBOR(r io.Reader) error {
	*t = PublishStorageDealsParams{}

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

	// t.Deals ([]market.ClientDealProposal) (slice)

	maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.Deals: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		t.Deals = make([]ClientDealProposal, extra)
	}

	for i := 0; i < int(extra); i++ {

		var v ClientDealProposal
		if err := v.UnmarshalCBOR(br); err != nil {
			return err
		}

		t.Deals[i] = v
	}

	// t.DataRef (market.PublishStorageDataRef) (struct)

	{

		if err := t.DataRef.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.DataRef: %w", err)
		}

	}
	return nil
}

var lengthBufPublishStorageDealsReturn = []byte{129}

func (t *PublishStorageDealsReturn) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufPublishStorageDealsReturn); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.IDs ([]abi.DealID) (slice)
	if len(t.IDs) > cbg.MaxLength {
		return xerrors.Errorf("Slice value in field t.IDs was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajArray, uint64(len(t.IDs))); err != nil {
		return err
	}
	for _, v := range t.IDs {
		if err := cbg.CborWriteHeader(w, cbg.MajUnsignedInt, uint64(v)); err != nil {
			return err
		}
	}
	return nil
}

func (t *PublishStorageDealsReturn) UnmarshalCBOR(r io.Reader) error {
	*t = PublishStorageDealsReturn{}

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

	// t.IDs ([]abi.DealID) (slice)

	maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.IDs: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		t.IDs = make([]abi.DealID, extra)
	}

	for i := 0; i < int(extra); i++ {

		maj, val, err := cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return xerrors.Errorf("failed to read uint64 for t.IDs slice: %w", err)
		}

		if maj != cbg.MajUnsignedInt {
			return xerrors.Errorf("value read for array t.IDs was not a uint, instead got %d", maj)
		}

		t.IDs[i] = abi.DealID(val)
	}

	return nil
}

var lengthBufActivateDealsParams = []byte{129}

func (t *ActivateDealsParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufActivateDealsParams); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.DealIDs ([]abi.DealID) (slice)
	if len(t.DealIDs) > cbg.MaxLength {
		return xerrors.Errorf("Slice value in field t.DealIDs was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajArray, uint64(len(t.DealIDs))); err != nil {
		return err
	}
	for _, v := range t.DealIDs {
		if err := cbg.CborWriteHeader(w, cbg.MajUnsignedInt, uint64(v)); err != nil {
			return err
		}
	}
	return nil
}

func (t *ActivateDealsParams) UnmarshalCBOR(r io.Reader) error {
	*t = ActivateDealsParams{}

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

	// t.DealIDs ([]abi.DealID) (slice)

	maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.DealIDs: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		t.DealIDs = make([]abi.DealID, extra)
	}

	for i := 0; i < int(extra); i++ {

		maj, val, err := cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return xerrors.Errorf("failed to read uint64 for t.DealIDs slice: %w", err)
		}

		if maj != cbg.MajUnsignedInt {
			return xerrors.Errorf("value read for array t.DealIDs was not a uint, instead got %d", maj)
		}

		t.DealIDs[i] = abi.DealID(val)
	}

	return nil
}

var lengthBufActivateDealsReturn = []byte{129}

func (t *ActivateDealsReturn) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufActivateDealsReturn); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.DealWins ([]builtin.BoolValue) (slice)
	if len(t.DealWins) > cbg.MaxLength {
		return xerrors.Errorf("Slice value in field t.DealWins was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajArray, uint64(len(t.DealWins))); err != nil {
		return err
	}
	for _, v := range t.DealWins {
		if err := v.MarshalCBOR(w); err != nil {
			return err
		}
	}
	return nil
}

func (t *ActivateDealsReturn) UnmarshalCBOR(r io.Reader) error {
	*t = ActivateDealsReturn{}

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

	// t.DealWins ([]builtin.BoolValue) (slice)

	maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.DealWins: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		t.DealWins = make([]builtin.BoolValue, extra)
	}

	for i := 0; i < int(extra); i++ {

		var v builtin.BoolValue
		if err := v.UnmarshalCBOR(br); err != nil {
			return err
		}

		t.DealWins[i] = v
	}

	return nil
}

var lengthBufVerifyDealsForActivationParams = []byte{130}

func (t *VerifyDealsForActivationParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufVerifyDealsForActivationParams); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.DealIDs ([]abi.DealID) (slice)
	if len(t.DealIDs) > cbg.MaxLength {
		return xerrors.Errorf("Slice value in field t.DealIDs was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajArray, uint64(len(t.DealIDs))); err != nil {
		return err
	}
	for _, v := range t.DealIDs {
		if err := cbg.CborWriteHeader(w, cbg.MajUnsignedInt, uint64(v)); err != nil {
			return err
		}
	}

	// t.SectorStart (abi.ChainEpoch) (int64)
	if t.SectorStart >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.SectorStart)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.SectorStart-1)); err != nil {
			return err
		}
	}
	return nil
}

func (t *VerifyDealsForActivationParams) UnmarshalCBOR(r io.Reader) error {
	*t = VerifyDealsForActivationParams{}

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

	// t.DealIDs ([]abi.DealID) (slice)

	maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.DealIDs: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		t.DealIDs = make([]abi.DealID, extra)
	}

	for i := 0; i < int(extra); i++ {

		maj, val, err := cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return xerrors.Errorf("failed to read uint64 for t.DealIDs slice: %w", err)
		}

		if maj != cbg.MajUnsignedInt {
			return xerrors.Errorf("value read for array t.DealIDs was not a uint, instead got %d", maj)
		}

		t.DealIDs[i] = abi.DealID(val)
	}

	// t.SectorStart (abi.ChainEpoch) (int64)
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

		t.SectorStart = abi.ChainEpoch(extraI)
	}
	return nil
}

var lengthBufVerifyDealsForActivationReturn = []byte{129}

func (t *VerifyDealsForActivationReturn) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufVerifyDealsForActivationReturn); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.PieceSizes ([]uint64) (slice)
	if len(t.PieceSizes) > cbg.MaxLength {
		return xerrors.Errorf("Slice value in field t.PieceSizes was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajArray, uint64(len(t.PieceSizes))); err != nil {
		return err
	}
	for _, v := range t.PieceSizes {
		if err := cbg.CborWriteHeader(w, cbg.MajUnsignedInt, uint64(v)); err != nil {
			return err
		}
	}
	return nil
}

func (t *VerifyDealsForActivationReturn) UnmarshalCBOR(r io.Reader) error {
	*t = VerifyDealsForActivationReturn{}

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

	// t.PieceSizes ([]uint64) (slice)

	maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.PieceSizes: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		t.PieceSizes = make([]uint64, extra)
	}

	for i := 0; i < int(extra); i++ {

		maj, val, err := cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return xerrors.Errorf("failed to read uint64 for t.PieceSizes slice: %w", err)
		}

		if maj != cbg.MajUnsignedInt {
			return xerrors.Errorf("value read for array t.PieceSizes was not a uint, instead got %d", maj)
		}

		t.PieceSizes[i] = uint64(val)
	}

	return nil
}

var lengthBufComputeDataCommitmentParams = []byte{130}

func (t *ComputeDataCommitmentParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufComputeDataCommitmentParams); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.DealIDs ([]abi.DealID) (slice)
	if len(t.DealIDs) > cbg.MaxLength {
		return xerrors.Errorf("Slice value in field t.DealIDs was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajArray, uint64(len(t.DealIDs))); err != nil {
		return err
	}
	for _, v := range t.DealIDs {
		if err := cbg.CborWriteHeader(w, cbg.MajUnsignedInt, uint64(v)); err != nil {
			return err
		}
	}

	// t.SectorType (abi.RegisteredSealProof) (int64)
	if t.SectorType >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.SectorType)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.SectorType-1)); err != nil {
			return err
		}
	}
	return nil
}

func (t *ComputeDataCommitmentParams) UnmarshalCBOR(r io.Reader) error {
	*t = ComputeDataCommitmentParams{}

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

	// t.DealIDs ([]abi.DealID) (slice)

	maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.DealIDs: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		t.DealIDs = make([]abi.DealID, extra)
	}

	for i := 0; i < int(extra); i++ {

		maj, val, err := cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return xerrors.Errorf("failed to read uint64 for t.DealIDs slice: %w", err)
		}

		if maj != cbg.MajUnsignedInt {
			return xerrors.Errorf("value read for array t.DealIDs was not a uint, instead got %d", maj)
		}

		t.DealIDs[i] = abi.DealID(val)
	}

	// t.SectorType (abi.RegisteredSealProof) (int64)
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

		t.SectorType = abi.RegisteredSealProof(extraI)
	}
	return nil
}

var lengthBufOnMinerSectorsTerminateParams = []byte{130}

func (t *OnMinerSectorsTerminateParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufOnMinerSectorsTerminateParams); err != nil {
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

	// t.DealIDs ([]abi.DealID) (slice)
	if len(t.DealIDs) > cbg.MaxLength {
		return xerrors.Errorf("Slice value in field t.DealIDs was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajArray, uint64(len(t.DealIDs))); err != nil {
		return err
	}
	for _, v := range t.DealIDs {
		if err := cbg.CborWriteHeader(w, cbg.MajUnsignedInt, uint64(v)); err != nil {
			return err
		}
	}
	return nil
}

func (t *OnMinerSectorsTerminateParams) UnmarshalCBOR(r io.Reader) error {
	*t = OnMinerSectorsTerminateParams{}

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
	// t.DealIDs ([]abi.DealID) (slice)

	maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.DealIDs: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		t.DealIDs = make([]abi.DealID, extra)
	}

	for i := 0; i < int(extra); i++ {

		maj, val, err := cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return xerrors.Errorf("failed to read uint64 for t.DealIDs slice: %w", err)
		}

		if maj != cbg.MajUnsignedInt {
			return xerrors.Errorf("value read for array t.DealIDs was not a uint, instead got %d", maj)
		}

		t.DealIDs[i] = abi.DealID(val)
	}

	return nil
}

var lengthBufNewQuota = []byte{130}

func (t *NewQuota) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufNewQuota); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.PieceCID (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.PieceCID); err != nil {
		return xerrors.Errorf("failed to write cid field t.PieceCID: %w", err)
	}

	// t.Quota (int64) (int64)
	if t.Quota >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.Quota)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.Quota-1)); err != nil {
			return err
		}
	}
	return nil
}

func (t *NewQuota) UnmarshalCBOR(r io.Reader) error {
	*t = NewQuota{}

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

	// t.PieceCID (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.PieceCID: %w", err)
		}

		t.PieceCID = c

	}
	// t.Quota (int64) (int64)
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

		t.Quota = int64(extraI)
	}
	return nil
}

var lengthBufResetQuotasParams = []byte{129}

func (t *ResetQuotasParams) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufResetQuotasParams); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.NewQuotas ([]market.NewQuota) (slice)
	if len(t.NewQuotas) > cbg.MaxLength {
		return xerrors.Errorf("Slice value in field t.NewQuotas was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajArray, uint64(len(t.NewQuotas))); err != nil {
		return err
	}
	for _, v := range t.NewQuotas {
		if err := v.MarshalCBOR(w); err != nil {
			return err
		}
	}
	return nil
}

func (t *ResetQuotasParams) UnmarshalCBOR(r io.Reader) error {
	*t = ResetQuotasParams{}

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

	// t.NewQuotas ([]market.NewQuota) (slice)

	maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.NewQuotas: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		t.NewQuotas = make([]NewQuota, extra)
	}

	for i := 0; i < int(extra); i++ {

		var v NewQuota
		if err := v.UnmarshalCBOR(br); err != nil {
			return err
		}

		t.NewQuotas[i] = v
	}

	return nil
}

var lengthBufDealProposal = []byte{134}

func (t *DealProposal) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufDealProposal); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.PieceCID (cid.Cid) (struct)

	if err := cbg.WriteCidBuf(scratch, w, t.PieceCID); err != nil {
		return xerrors.Errorf("failed to write cid field t.PieceCID: %w", err)
	}

	// t.PieceSize (abi.PaddedPieceSize) (uint64)

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.PieceSize)); err != nil {
		return err
	}

	// t.Client (address.Address) (struct)
	if err := t.Client.MarshalCBOR(w); err != nil {
		return err
	}

	// t.Provider (address.Address) (struct)
	if err := t.Provider.MarshalCBOR(w); err != nil {
		return err
	}

	// t.Label (string) (string)
	if len(t.Label) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.Label was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len(t.Label))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.Label)); err != nil {
		return err
	}

	// t.StartEpoch (abi.ChainEpoch) (int64)
	if t.StartEpoch >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.StartEpoch)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.StartEpoch-1)); err != nil {
			return err
		}
	}
	return nil
}

func (t *DealProposal) UnmarshalCBOR(r io.Reader) error {
	*t = DealProposal{}

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

	// t.PieceCID (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.PieceCID: %w", err)
		}

		t.PieceCID = c

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
	// t.Client (address.Address) (struct)

	{

		if err := t.Client.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Client: %w", err)
		}

	}
	// t.Provider (address.Address) (struct)

	{

		if err := t.Provider.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Provider: %w", err)
		}

	}
	// t.Label (string) (string)

	{
		sval, err := cbg.ReadStringBuf(br, scratch)
		if err != nil {
			return err
		}

		t.Label = string(sval)
	}
	// t.StartEpoch (abi.ChainEpoch) (int64)
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

		t.StartEpoch = abi.ChainEpoch(extraI)
	}
	return nil
}

var lengthBufClientDealProposal = []byte{130}

func (t *ClientDealProposal) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufClientDealProposal); err != nil {
		return err
	}

	// t.Proposal (market.DealProposal) (struct)
	if err := t.Proposal.MarshalCBOR(w); err != nil {
		return err
	}

	// t.ClientSignature (crypto.Signature) (struct)
	if err := t.ClientSignature.MarshalCBOR(w); err != nil {
		return err
	}
	return nil
}

func (t *ClientDealProposal) UnmarshalCBOR(r io.Reader) error {
	*t = ClientDealProposal{}

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

	// t.Proposal (market.DealProposal) (struct)

	{

		if err := t.Proposal.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Proposal: %w", err)
		}

	}
	// t.ClientSignature (crypto.Signature) (struct)

	{

		if err := t.ClientSignature.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.ClientSignature: %w", err)
		}

	}
	return nil
}

var lengthBufDealState = []byte{131}

func (t *DealState) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufDealState); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.SectorStartEpoch (abi.ChainEpoch) (int64)
	if t.SectorStartEpoch >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.SectorStartEpoch)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.SectorStartEpoch-1)); err != nil {
			return err
		}
	}

	// t.LastUpdatedEpoch (abi.ChainEpoch) (int64)
	if t.LastUpdatedEpoch >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.LastUpdatedEpoch)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.LastUpdatedEpoch-1)); err != nil {
			return err
		}
	}

	// t.SlashEpoch (abi.ChainEpoch) (int64)
	if t.SlashEpoch >= 0 {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.SlashEpoch)); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajNegativeInt, uint64(-t.SlashEpoch-1)); err != nil {
			return err
		}
	}
	return nil
}

func (t *DealState) UnmarshalCBOR(r io.Reader) error {
	*t = DealState{}

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

	// t.SectorStartEpoch (abi.ChainEpoch) (int64)
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

		t.SectorStartEpoch = abi.ChainEpoch(extraI)
	}
	// t.LastUpdatedEpoch (abi.ChainEpoch) (int64)
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

		t.LastUpdatedEpoch = abi.ChainEpoch(extraI)
	}
	// t.SlashEpoch (abi.ChainEpoch) (int64)
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

		t.SlashEpoch = abi.ChainEpoch(extraI)
	}
	return nil
}
