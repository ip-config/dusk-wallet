package wallet

import (
	"bytes"
	"math/big"
	"os"
	"testing"

	"github.com/dusk-network/dusk-wallet/block"
	"github.com/dusk-network/dusk-wallet/database"
	"github.com/dusk-network/dusk-wallet/key"
	"github.com/dusk-network/dusk-wallet/transactions"

	"github.com/bwesterb/go-ristretto"
	"github.com/stretchr/testify/assert"
)

const dbPath = "testDb"
const walletPath = "wallet.dat"

func TestNewWallet(t *testing.T) {
	netPrefix := byte(1)

	db, err := database.New(dbPath)
	assert.Nil(t, err)
	defer os.RemoveAll(dbPath)
	defer os.Remove(walletPath)

	w, err := New(randReader, netPrefix, db, GenerateDecoys, GenerateInputs, "pass", walletPath)
	assert.Nil(t, err)

	// wrong wallet password
	loadedWallet, err := LoadFromFile(netPrefix, db, GenerateDecoys, GenerateInputs, "wrongPass", walletPath)
	assert.NotNil(t, err)

	// correct wallet password
	loadedWallet, err = LoadFromFile(netPrefix, db, GenerateDecoys, GenerateInputs, "pass", walletPath)
	assert.Nil(t, err)

	assert.Equal(t, w.PublicKey(), loadedWallet.PublicKey())

	assert.Equal(t, w.consensusKeys.EdSecretKey, loadedWallet.consensusKeys.EdSecretKey)
	assert.Equal(t, w.consensusKeys.BLSSecretKey, loadedWallet.consensusKeys.BLSSecretKey)
	assert.True(t, bytes.Equal(w.consensusKeys.BLSPubKeyBytes, loadedWallet.consensusKeys.BLSPubKeyBytes))

}

func TestReceivedTx(t *testing.T) {
	netPrefix := byte(1)
	fee := int64(0)

	db, err := database.New(dbPath)
	assert.Nil(t, err)
	defer os.RemoveAll(dbPath)
	defer os.Remove(walletPath)

	w, err := New(randReader, netPrefix, db, GenerateDecoys, GenerateInputs, "pass", walletPath)
	assert.Nil(t, err)

	tx, err := w.NewStandardTx(fee)
	assert.Nil(t, err)

	var tenDusk ristretto.Scalar
	tenDusk.SetBigInt(big.NewInt(10))

	sendersAddr := generateSendAddr(t, netPrefix, w.keyPair)
	assert.Nil(t, err)

	err = tx.AddOutput(sendersAddr, tenDusk)
	assert.Nil(t, err)

	err = w.Sign(tx)
	assert.Nil(t, err)

	for _, output := range tx.Outputs {
		_, ok := w.keyPair.DidReceiveTx(tx.R, output.PubKey, output.Index)
		assert.True(t, ok)
	}

	var destKeys []ristretto.Point
	for _, output := range tx.Outputs {
		destKeys = append(destKeys, output.PubKey.P)
	}
	assert.False(t, hasDuplicates(destKeys))
}

func TestCheckBlock(t *testing.T) {
	netPrefix := byte(1)

	alice := generateWallet(t, netPrefix, "alice")
	bob := generateWallet(t, netPrefix, "bob")
	defer os.Remove(walletPath)
	bobAddr, err := bob.keyPair.PublicKey().PublicAddress(netPrefix)
	assert.Nil(t, err)

	var numTxs = 3 // numTxs to send to Bob

	var blk block.Block
	for i := 0; i < numTxs; i++ {
		tx := generateStandardTx(t, *bobAddr, 20, alice)
		assert.Nil(t, err)
		blk.AddTx(tx)
	}

	count, _, err := bob.CheckWireBlockReceived(blk, true)
	assert.Nil(t, err)
	assert.Equal(t, uint64(numTxs), count)

	_, err = alice.CheckWireBlockSpent(blk, true)
	assert.Nil(t, err)
}

func TestSpendLockedInputs(t *testing.T) {
	netPrefix := byte(1)
	alice := generateWallet(t, netPrefix, "alice")
	defer os.Remove(walletPath)

	var blk block.Block
	tx := generateStakeTx(t, 20, alice, 100000)
	blk.AddTx(tx)

	_, _, err := alice.CheckWireBlockReceived(blk, true)
	assert.Nil(t, err)

	// Attempt to send a Standard tx with this single input we received.
	// Set our FetchInputs function to a proper one, so that we actually
	// check the database.
	alice.fetchInputs = fetchInputs
	standard, err := alice.NewStandardTx(100)
	assert.NoError(t, err)

	var amount ristretto.Scalar
	amount.SetBigInt(big.NewInt(5000))

	pubAddr, err := alice.keyPair.PublicKey().PublicAddress(netPrefix)
	assert.NoError(t, err)
	assert.NoError(t, standard.AddOutput(*pubAddr, amount))

	// Should fail
	assert.Error(t, alice.Sign(standard))
}

func generateWallet(t *testing.T, netPrefix byte, path string) *Wallet {

	db, err := database.New(path)
	assert.Nil(t, err)
	defer os.RemoveAll(path)

	os.Remove(walletPath)
	w, err := New(randReader, netPrefix, db, GenerateDecoys, GenerateInputs, "pass", walletPath)
	assert.Nil(t, err)
	return w
}

func generateStandardTx(t *testing.T, receiver key.PublicAddress, amount int64, sender *Wallet) *transactions.Standard {
	tx, err := sender.NewStandardTx(0)
	assert.Nil(t, err)

	var duskAmount ristretto.Scalar
	duskAmount.SetBigInt(big.NewInt(amount))

	err = tx.AddOutput(receiver, duskAmount)
	assert.Nil(t, err)

	err = sender.Sign(tx)
	assert.Nil(t, err)

	return tx
}

func generateStakeTx(t *testing.T, amount int64, sender *Wallet, lockTime uint64) *transactions.Stake {
	var duskAmount ristretto.Scalar
	duskAmount.SetBigInt(big.NewInt(amount))

	tx, err := sender.NewStakeTx(0, lockTime, duskAmount)
	assert.Nil(t, err)

	err = sender.Sign(tx)
	assert.Nil(t, err)

	return tx
}

func generateSendAddr(t *testing.T, netPrefix byte, randKeyPair *key.Key) key.PublicAddress {
	pubAddr, err := randKeyPair.PublicKey().PublicAddress(netPrefix)
	assert.Nil(t, err)
	return *pubAddr
}

// https://www.dotnetperls.com/duplicates-go
func hasDuplicates(elements []ristretto.Point) bool {
	encountered := map[ristretto.Point]bool{}

	for v := range elements {
		if encountered[elements[v]] == true {
			return true
		}
		encountered[elements[v]] = true
	}
	return false
}

func sliceToPoint(t *testing.T, b []byte) ristretto.Point {
	if len(b) != 32 {
		t.Fatal("slice to point must be given a 32 byte slice")
	}
	var c ristretto.Point
	var byts [32]byte
	copy(byts[:], b)
	c.SetBytes(&byts)
	return c
}

func randReader(b []byte) (n int, err error) {
	return len(b), nil
}

func fetchInputs(netPrefix byte, db *database.DB, totalAmount int64, key *key.Key) ([]*transactions.Input, int64, error) {
	// Fetch all inputs from database that are >= totalAmount
	// returns error if inputs do not add up to total amount
	privSpend, err := key.PrivateSpend()
	if err != nil {
		return nil, 0, err
	}
	return db.FetchInputs(privSpend.Bytes(), totalAmount)
}
