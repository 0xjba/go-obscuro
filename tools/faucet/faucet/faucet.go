package faucet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/obscuronet/go-obscuro/go/obsclient"
	"github.com/obscuronet/go-obscuro/go/rpc"
	"github.com/obscuronet/go-obscuro/go/wallet"
)

const (
	_timeout       = 60 * time.Second
	OBXNativeToken = "obx"
	WrappedOBX     = "wobx"
	WrappedEth     = "weth"
	WrappedUSDC    = "usdc"
)

type Faucet struct {
	client    *obsclient.AuthObsClient
	fundMutex sync.Mutex
	wallet    wallet.Wallet
	Logger    log.Logger
}

func NewFaucet(rpcURL string, chainID int64, pkString string) (*Faucet, error) {
	logger := log.New()
	logger.SetHandler(log.StreamHandler(os.Stdout, log.TerminalFormat(false)))
	w := wallet.NewInMemoryWalletFromConfig(pkString, chainID, logger)
	obsClient, err := obsclient.DialWithAuth(rpcURL, w, logger)
	if err != nil {
		return nil, fmt.Errorf("unable to connect with the node: %w", err)
	}

	return &Faucet{
		client: obsClient,
		wallet: w,
		Logger: logger,
	}, nil
}

func (f *Faucet) Fund(address *common.Address, token string, amount int64) error {
	var err error
	var signedTx *types.Transaction

	if token == OBXNativeToken {
		signedTx, err = f.fundNativeToken(address, amount)
	} else {
		return fmt.Errorf("token not fundable atm")
		// signedTx, err = f.fundERC20Token(address, token)
		// todo implement this when contracts are deployable somewhere
	}
	if err != nil {
		return err
	}

	// the faucet should be the only user of the faucet pk
	txMarshal, err := json.Marshal(signedTx)
	if err != nil {
		return err
	}
	f.Logger.Info(fmt.Sprintf("Funded address: %s - tx: %+v\n", address.Hex(), string(txMarshal)))
	// todo handle tx receipt

	if err := f.validateTx(signedTx); err != nil {
		return fmt.Errorf("unable to validate tx %s: %w", signedTx.Hash(), err)
	}

	return nil
}

func (f *Faucet) validateTx(tx *types.Transaction) error {
	for now := time.Now(); time.Since(now) < _timeout; time.Sleep(time.Second) {
		receipt, err := f.client.TransactionReceipt(context.Background(), tx.Hash())
		if err != nil {
			if errors.Is(err, rpc.ErrNilResponse) {
				// tx receipt is not available yet
				continue
			}
			return fmt.Errorf("could not retrieve transaction receipt in eth_getTransactionReceipt request. Cause: %w", err)
		}

		txReceiptBytes, err := receipt.MarshalJSON()
		if err != nil {
			return fmt.Errorf("could not marshal transaction receipt to JSON in eth_getTransactionReceipt request. Cause: %w", err)
		}
		fmt.Println(string(txReceiptBytes))

		if receipt.Status != 1 {
			return fmt.Errorf("tx status is not 0x1")
		}
		return nil
	}
	return fmt.Errorf("unable to fetch tx receipt after %s", _timeout)
}

func (f *Faucet) fundNativeToken(address *common.Address, amount int64) (*types.Transaction, error) {
	// only one funding at the time
	f.fundMutex.Lock()
	defer f.fundMutex.Unlock()

	nonce, err := f.client.NonceAt(context.Background(), nil)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch %s nonce: %w", f.wallet.Address(), err)
	}
	// this isn't great as the tx count might be incremented in between calls
	// but only after removing the pk from other apps can we use a proper counter

	// todo remove hardcoded gas values
	gas := uint64(21000)

	tx := &types.LegacyTx{
		Nonce:    nonce,
		GasPrice: big.NewInt(225),
		Gas:      gas,
		To:       address,
		Value:    new(big.Int).Mul(big.NewInt(amount), big.NewInt(params.Ether)),
	}

	signedTx, err := f.wallet.SignTransaction(tx)
	if err != nil {
		return nil, err
	}

	if err = f.client.SendTransaction(context.Background(), signedTx); err != nil {
		return signedTx, err
	}

	return signedTx, nil
}
