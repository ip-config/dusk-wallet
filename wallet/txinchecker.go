package wallet

import (
	"github.com/dusk-network/dusk-wallet/block"
	"github.com/dusk-network/dusk-wallet/transactions"
	"github.com/syndtr/goleveldb/leveldb"
)

type keyImage []byte

//TxInChecker contains the necessary information to
// deduce whether a user has spent a tx. This is just the keyImage.
type TxInChecker struct {
	keyImages []keyImage
}

func NewTxInChecker(txs []transactions.Transaction) []TxInChecker {
	txcheckers := make([]TxInChecker, 0, len(txs))

	for _, tx := range txs {
		keyImages := make([]keyImage, 0)
		for _, input := range tx.StandardTx().Inputs {
			keyImages = append(keyImages, input.KeyImage.Bytes())
		}
		txcheckers = append(txcheckers, TxInChecker{keyImages})
	}
	return txcheckers
}

// CheckWireBlockSpent checks if the block has any outputs spent by this wallet
// Returns the number of txs that the sender spent funds in
func (w *Wallet) CheckWireBlockSpent(blk block.Block) (uint64, error) {
	var totalSpentCount uint64
	txInCheckers := NewTxInChecker(blk.Txs)

	for i, txchecker := range txInCheckers {
		spentCount, err := w.removeSpentOutputs(txchecker)
		if err != nil {
			return spentCount, err
		}
		totalSpentCount += spentCount

		if spentCount > 0 {
			_ = w.db.PutTxOut(blk.Txs[i])
		}
	}

	return totalSpentCount, nil
}

// Given a tx checker, this function will remove the inputs associated
// with the keyimages found in the tx checker, as they are now confirmed
// to be spent.
func (w *Wallet) removeSpentOutputs(txChecker TxInChecker) (uint64, error) {
	var didSpendFunds uint64
	for _, keyImage := range txChecker.keyImages {
		pubKey, err := w.db.Get(keyImage)
		if err == leveldb.ErrNotFound {
			continue
		}
		if err != nil {
			return didSpendFunds, err
		}

		didSpendFunds++

		if err := w.db.RemoveInput(pubKey, keyImage); err != nil {
			return didSpendFunds, err
		}
	}

	return didSpendFunds, nil
}
