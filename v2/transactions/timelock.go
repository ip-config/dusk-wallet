package transactions

import (
	"bytes"
	"encoding/binary"

	"github.com/dusk-network/dusk-crypto/hash"
)

const MaxLockTime = 250000
const GenesisExpirationHeight = 250001

// TimeLock represents a standard transaction that has an additional time restriction
// What does the time-lock represent?
// For a `Standard TimeLock`; that the TX can only become valid after the time stated.
// This is not the case for others, please check each transaction for the significance of the timelock
type Timelock struct {
	*Standard
	Lock uint64
}

func NewTimelock(ver uint8, netPrefix byte, fee int64, lock uint64) (*Timelock, error) {
	tx, err := NewStandard(ver, netPrefix, fee)
	if err != nil {
		return nil, err
	}

	tx.TxType = TimelockType
	return &Timelock{
		tx,
		lock,
	}, nil
}

func (tl *Timelock) CalculateHash() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := marshalTimelock(buf, tl); err != nil {
		return nil, err
	}

	txid, err := hash.Sha3256(buf.Bytes())
	if err != nil {
		return nil, err
	}

	return txid, nil
}

func (tl *Timelock) StandardTx() *Standard {
	return tl.Standard
}

func (tl *Timelock) Type() TxType {
	return tl.TxType
}

func (tl *Timelock) Prove() error {
	return tl.prove(tl.CalculateHash, true)
}

func (tl *Timelock) Equals(t Transaction) bool {
	other, ok := t.(*Timelock)
	if !ok {
		return false
	}

	if !tl.Standard.Equals(other.Standard) {
		return false
	}

	return true
}

func (tl *Timelock) LockTime() uint64 {
	return tl.Lock
}

func marshalTimelock(b *bytes.Buffer, tl *Timelock) error {
	if err := marshalStandard(b, tl.Standard); err != nil {
		return err
	}

	if err := binary.Write(b, binary.LittleEndian, tl.Lock); err != nil {
		return err
	}

	return nil
}
