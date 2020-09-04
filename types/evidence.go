package types

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// Evidence represents any provable malicious activity by a validator.
// Verification logic for each evidence is part of the evidence module.
// This interface is designed to ensure compliance with the abci evidence
// that is passed on to the application. 
type Evidence interface {
	Height() int64                  // height of the equivocation
	Time() time.Time                // time of the equivocation
	// addresses of the equivocating validator
	Bytes() []byte                  // bytes which comprise the evidence
	Hash() []byte                   // hash of the evidence
	ValidateBasic() error	        // basic validation	
	Type() abci.EvidenceType        // type of evidence
	String() string		            // string format of the evidence
	
	SetValidatorSet(vals *ValidatorSet)
	ToABCI() []abci.Evidence
}

const (
	// MaxEvidenceBytes is a maximum size of any evidence (including amino overhead).
	MaxEvidenceBytes int64 = 444
)

//--------------------------------------------------------------------------------------

// DuplicateVoteEvidence contains evidence a validator signed two conflicting
// votes.
type DuplicateVoteEvidence struct {
	VoteA *Vote `json:"vote_a"`
	VoteB *Vote `json:"vote_b"`

	timestamp time.Time 
	validator *Validator
}

var _ Evidence = &DuplicateVoteEvidence{}

// NewDuplicateVoteEvidence creates DuplicateVoteEvidence with right ordering given
// two conflicting votes. If one of the votes is nil, evidence returned is nil as well
func NewDuplicateVoteEvidence(vote1, vote2 *Vote, time time.Time) *DuplicateVoteEvidence {
	var voteA, voteB *Vote
	if vote1 == nil || vote2 == nil {
		return nil
	}
	if strings.Compare(vote1.BlockID.Key(), vote2.BlockID.Key()) == -1 {
		voteA = vote1
		voteB = vote2
	} else {
		voteA = vote2
		voteB = vote1
	}
	return &DuplicateVoteEvidence{
		VoteA: voteA,
		VoteB: voteB,

		timestamp: time,
	}
}

// String returns a string representation of the evidence.
func (dve *DuplicateVoteEvidence) String() string {
	return fmt.Sprintf("DuplicateVoteEvidence{VoteA: %v, VoteB: %v, Time: %v}", dve.VoteA, dve.VoteB, dve.Timestamp)
}

// Height returns the height this evidence refers to.
func (dve *DuplicateVoteEvidence) Height() int64 {
	return dve.VoteA.Height
}

// Time returns time of the latest vote.
func (dve *DuplicateVoteEvidence) Time() time.Time {
	return dve.Timestamp
}

// Address returns the address of the validator.
func (dve *DuplicateVoteEvidence) Addresses() []Address {
	return []Address{dve.VoteA.ValidatorAddress}
}

// Hash returns the hash of the evidence.
func (dve *DuplicateVoteEvidence) Bytes() []byte {
	pbe := dve.ToProto()
	bz, err := pbe.Marshal()
	if err != nil {
		panic(err)
	}

	return bz
}

// Hash returns the hash of the evidence.
func (dve *DuplicateVoteEvidence) Hash() []byte {
	return tmhash.Sum(dve.Bytes())
}

// Type returns the type of evidence as a string
func (dve *DuplicateVoteEvidence) Type() abciproto.EvidenceType { 
	return abciproto.EvidenceType_DUPLICATE_VOTE
}

// ValidateBasic performs basic validation.
func (dve *DuplicateVoteEvidence) ValidateBasic() error {
	if dve == nil {
		return errors.New("empty duplicate vote evidence")
	}

	if dve.VoteA == nil || dve.VoteB == nil {
		return fmt.Errorf("one or both of the votes are empty %v, %v", dve.VoteA, dve.VoteB)
	}
	if err := dve.VoteA.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid VoteA: %w", err)
	}
	if err := dve.VoteB.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid VoteB: %w", err)
	}
	// Enforce Votes are lexicographically sorted on blockID
	if strings.Compare(dve.VoteA.BlockID.Key(), dve.VoteB.BlockID.Key()) >= 0 {
		return errors.New("duplicate votes in invalid order")
	}
	return nil
}

func (dve *DuplicateVoteEvidence) ToProto() *tmproto.DuplicateVoteEvidence {
	voteB := dve.VoteB.ToProto()
	voteA := dve.VoteA.ToProto()
	tp := tmproto.DuplicateVoteEvidence{
		VoteA:     voteA,
		VoteB:     voteB,
		Timestamp: dve.Timestamp,
	}
	return &tp
}

func DuplicateVoteEvidenceFromProto(pb *tmproto.DuplicateVoteEvidence) (*DuplicateVoteEvidence, error) {
	if pb == nil {
		return nil, errors.New("nil duplicate vote evidence")
	}

	vA, err := VoteFromProto(pb.VoteA)
	if err != nil {
		return nil, err
	}

	vB, err := VoteFromProto(pb.VoteB)
	if err != nil {
		return nil, err
	}

	dve := NewDuplicateVoteEvidence(vA, vB, pb.Timestamp)

	return dve, dve.ValidateBasic()
}


//------------------------------------ LIGHT EVIDENCE --------------------------------------

// LightClientAttackEvidence is a generalized evidence that captures all forms of known attacks on
// a light client such that a full node can verify, propose and commit the evidence on-chain for
// punishment of the malicious validators. There are three forms of attacks: Lunatic, Equivocation
// and Amnesia. You can find a more detailed overview of this at
// tendermint/docs/architecture/adr-047-handling-evidence-from-light-client.md
type LightClientAttackEvidence struct {
	ConflictingBlock *LightBlock
	CommonHeight     int64
	Timestamp        time.Time
	AttackType       tmproto.LightClientAttackType
}

var _ Evidence = &LightClientAttackEvidence{}

func (l *LightClientAttackEvidence) Height() int64 {
	return l.CommonHeight
}

func (l *LightClientAttackEvidence) Time() time.Time {
	return l.Timestamp
}

func (l *LightClientAttackEvidence) Addresses() []Address {
	addresses := make([]Address, len(l.ConflictingBlock.ValidatorSet.Validators))
	for idx, val := range l.ConflictingBlock.ValidatorSet.Validators {
		addresses[idx] = val.Address
	}
	return addresses
}

func (l *LightClientAttackEvidence) Bytes() []byte {
	pbe, err := l.ToProto()
	if err != nil {
		panic(err)
	}
	bz, err := pbe.Marshal()
	if err != nil {
		panic(err)
	}
	return bz
}

func (l *LightClientAttackEvidence) Hash() []byte {
	bz := make([]byte, tmhash.Size+8)
	copy(bz[:tmhash.Size-1], l.ConflictingBlock.Hash().Bytes())
	copy(bz[tmhash.Size:], []byte(strconv.Itoa(int(l.CommonHeight))))
	return tmhash.Sum(bz)
}

func (l *LightClientAttackEvidence) ValidateBasic() error {
	if l.ConflictingBlock == nil {
		return errors.New("conflicting block is nil")
	}

	// this check needs to be done before we can run validate basic
	if l.ConflictingBlock.Header == nil {
		return errors.New("conflicting block missing header")
	}

	if err := l.ConflictingBlock.ValidateBasic(l.ConflictingBlock.ChainID); err != nil {
		return fmt.Errorf("invalid conflicting light block: %w", err)
	}
	
	if l.CommonHeight <= 0 {
		fmt.Errorf("incorrect common height (#%d <= 0)", l.CommonHeight)
	}
	
	if l.CommonHeight >= l.ConflictingBlock.Height {
		return fmt.Errorf("common height is ahead of the conflicting block height (%d > %d)",
		l.CommonHeight, l.ConflictingBlock.Height)
	}

	return nil
}

func (l *LightClientAttackEvidence) Type() abciproto.EvidenceType {
	return abciproto.EvidenceType_LIGHT_CLIENT_ATTACK
}

func (l *LightClientAttackEvidence) String() string {
	return fmt.Sprintf("LightClientAttackEvidence{ConflictingBlock: %v, CommonHeight: %d, Timestamp: %v, AttackType: %v}", 
	l.ConflictingBlock.String(), l.CommonHeight, l.Timestamp.String(), l.AttackType.String())
}

func (l *LightClientAttackEvidence) ToProto() (*tmproto.LightClientAttackEvidence, error) {
	conflictingBlock, err := l.ConflictingBlock.ToProto()
	if err != nil {
		return nil, err
	}
	
	return &tmproto.LightClientAttackEvidence{
		ConflictingBlock: conflictingBlock,
		CommonHeight: l.CommonHeight,
		Timestamp: l.Timestamp,
		AttackType: l.AttackType,
	}, nil
}

func LightClientAttackEvidenceFromProto(l *tmproto.LightClientAttackEvidence) (*LightClientAttackEvidence, error) {
	if l == nil {
		return nil, errors.New("empty light client attack evidence")
	}

	conflictingBlock, err := LightBlockFromProto(l.ConflictingBlock)
	if err != nil {
		return nil, err
	}
	
	le := &LightClientAttackEvidence{
		ConflictingBlock: conflictingBlock,
		CommonHeight: l.CommonHeight,
		Timestamp: l.Timestamp,
		AttackType: l.AttackType,
	}
	
	return le, le.ValidateBasic()
}

//------------------------------------------------------------------------------------------

// EvidenceList is a list of Evidence. Evidences is not a word.
type EvidenceList []Evidence

// Hash returns the simple merkle root hash of the EvidenceList.
func (evl EvidenceList) Hash() []byte {
	// These allocations are required because Evidence is not of type Bytes, and
	// golang slices can't be typed cast. This shouldn't be a performance problem since
	// the Evidence size is capped.
	evidenceBzs := make([][]byte, len(evl))
	for i := 0; i < len(evl); i++ {
		evidenceBzs[i] = evl[i].Bytes()
	}
	return merkle.HashFromByteSlices(evidenceBzs)
}

func (evl EvidenceList) String() string {
	s := ""
	for _, e := range evl {
		s += fmt.Sprintf("%s\t\t", e)
	}
	return s
}

// Has returns true if the evidence is in the EvidenceList.
func (evl EvidenceList) Has(evidence Evidence) bool {
	for _, ev := range evl {
		if bytes.Equal(evidence.Hash(), ev.Hash()) {
			return true
		}
	}
	return false
}

//------------------------------------------ PROTO --------------------------------------

func EvidenceToProto(evidence Evidence) (*tmproto.Evidence, error) {
	if evidence == nil {
		return nil, errors.New("nil evidence")
	}

	switch evi := evidence.(type) {
	case *DuplicateVoteEvidence:
		pbev := evi.ToProto()
		return &tmproto.Evidence{
			Sum: &tmproto.Evidence_DuplicateVoteEvidence{
				DuplicateVoteEvidence: pbev,
			},
		}, nil
		
	case *LightClientAttackEvidence:
		pbev, err := evi.ToProto()
		if err != nil {
			return nil, err
		}
		return &tmproto.Evidence{
			Sum: &tmproto.Evidence_LightClientAttackEvidence{
				LightClientAttackEvidence: pbev,
			},
		}, nil 

	default:
		return nil, fmt.Errorf("toproto: evidence is not recognized: %T", evi)
	}
}

func EvidenceFromProto(evidence *tmproto.Evidence) (Evidence, error) {
	if evidence == nil {
		return nil, errors.New("nil evidence")
	}

	switch evi := evidence.Sum.(type) {
	case *tmproto.Evidence_DuplicateVoteEvidence:
		return DuplicateVoteEvidenceFromProto(evi.DuplicateVoteEvidence)
	case *tmproto.Evidence_LightClientAttackEvidence:
		return LightClientAttackEvidenceFromProto(evi.LightClientAttackEvidence)
	default:
		return nil, errors.New("evidence is not recognized")
	}
}

func init() {
	tmjson.RegisterType(&DuplicateVoteEvidence{}, "tendermint/DuplicateVoteEvidence")
	tmjson.RegisterType(&LightClientAttackEvidence{}, "tendermint/LightClientAttackEvidence")
}

//-------------------------------------------- ERRORS --------------------------------------

// ErrEvidenceInvalid wraps a piece of evidence and the error denoting how or why it is invalid.
type ErrEvidenceInvalid struct {
	Evidence   Evidence
	ErrorValue error
}

// NewErrEvidenceInvalid returns a new EvidenceInvalid with the given err.
func NewErrEvidenceInvalid(ev Evidence, err error) *ErrEvidenceInvalid {
	return &ErrEvidenceInvalid{ev, err}
}

// Error returns a string representation of the error.
func (err *ErrEvidenceInvalid) Error() string {
	return fmt.Sprintf("Invalid evidence: %v. Evidence: %v", err.ErrorValue, err.Evidence)
}

// ErrEvidenceOverflow is for when there is too much evidence in a block.
type ErrEvidenceOverflow struct {
	MaxNum int
	GotNum int
}

// NewErrEvidenceOverflow returns a new ErrEvidenceOverflow where got > max.
func NewErrEvidenceOverflow(max, got int) *ErrEvidenceOverflow {
	return &ErrEvidenceOverflow{max, got}
}

// Error returns a string representation of the error.
func (err *ErrEvidenceOverflow) Error() string {
	return fmt.Sprintf("Too much evidence: Max %d, got %d", err.MaxNum, err.GotNum)
}

//-------------------------------------------- MOCKING --------------------------------------

// unstable - use only for testing

// assumes the round to be 0 and the validator index to be 0
func NewMockDuplicateVoteEvidence(height int64, time time.Time, chainID string) *DuplicateVoteEvidence {
	val := NewMockPV()
	return NewMockDuplicateVoteEvidenceWithValidator(height, time, val, chainID)
}

func NewMockDuplicateVoteEvidenceWithValidator(height int64, time time.Time,
	pv PrivValidator, chainID string) *DuplicateVoteEvidence {
	pubKey, _ := pv.GetPubKey()
	voteA := makeMockVote(height, 0, 0, pubKey.Address(), randBlockID(), time)
	vA := voteA.ToProto()
	_ = pv.SignVote(chainID, vA)
	voteA.Signature = vA.Signature
	voteB := makeMockVote(height, 0, 0, pubKey.Address(), randBlockID(), time)
	vB := voteB.ToProto()
	_ = pv.SignVote(chainID, vB)
	voteB.Signature = vB.Signature
	return NewDuplicateVoteEvidence(voteA, voteB, time)
}

func makeMockVote(height int64, round, index int32, addr Address,
	blockID BlockID, time time.Time) *Vote {
	return &Vote{
		Type:             tmproto.SignedMsgType(2),
		Height:           height,
		Round:            round,
		BlockID:          blockID,
		Timestamp:        time,
		ValidatorAddress: addr,
		ValidatorIndex:   index,
	}
}

func randBlockID() BlockID {
	return BlockID{
		Hash: tmrand.Bytes(tmhash.Size),
		PartSetHeader: PartSetHeader{
			Total: 1,
			Hash:  tmrand.Bytes(tmhash.Size),
		},
	}
}
