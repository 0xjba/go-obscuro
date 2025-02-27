package network

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/obscuronet/go-obscuro/contracts/generated/ManagementContract"
	"github.com/obscuronet/go-obscuro/go/common/constants"
	"github.com/obscuronet/go-obscuro/go/ethadapter"
	"github.com/obscuronet/go-obscuro/go/wallet"
	"github.com/obscuronet/go-obscuro/integration"
	"github.com/obscuronet/go-obscuro/integration/common/testlog"
	"github.com/obscuronet/go-obscuro/integration/erc20contract"
	"github.com/obscuronet/go-obscuro/integration/eth2network"
	"github.com/obscuronet/go-obscuro/integration/simulation/params"
)

const (
	// These are the addresses that the end-to-end tests expect to be prefunded when run locally. Corresponds to
	// private key hex "f52e5418e349dccdda29b6ac8b0abe6576bb7713886aa85abea6181ba731f9bb".
	e2eTestPrefundedL1Addr = "0x13E23Ca74DE0206C56ebaE8D51b5622EFF1E9944"
)

func SetUpGethNetwork(wallets *params.SimWallets, startPort int, nrNodes int, blockDurationSeconds int) (*params.L1SetupData, []ethadapter.EthClient, eth2network.Eth2Network) {
	eth2Network, err := StartGethNetwork(wallets, startPort, blockDurationSeconds)
	if err != nil {
		panic(err)
	}

	// connect to the first host to deploy
	tmpEthClient, err := ethadapter.NewEthClient(Localhost, uint(startPort+100), DefaultL1RPCTimeout, common.HexToAddress("0x0"), testlog.Logger())
	if err != nil {
		panic(err)
	}

	l1Data, err := DeployObscuroNetworkContracts(tmpEthClient, wallets, true)
	if err != nil {
		panic(err)
	}

	ethClients := make([]ethadapter.EthClient, nrNodes)
	for i := 0; i < nrNodes; i++ {
		ethClients[i] = CreateEthClientConnection(int64(i), uint(startPort+100))
	}

	return l1Data, ethClients, eth2Network
}

func StartGethNetwork(wallets *params.SimWallets, startPort int, blockDurationSeconds int) (eth2network.Eth2Network, error) {
	// make sure the geth network binaries exist
	path, err := eth2network.EnsureBinariesExist()
	if err != nil {
		return nil, err
	}

	// get the node wallet addresses to prefund them with Eth, so they can submit rollups, deploy contracts, deposit to the bridge, etc
	walletAddresses := []string{e2eTestPrefundedL1Addr}
	for _, w := range wallets.AllEthWallets() {
		walletAddresses = append(walletAddresses, w.Address().String())
	}

	// kickoff the network with the prefunded wallet addresses
	eth2Network := eth2network.NewEth2Network(
		path,
		true,
		startPort,
		startPort+integration.DefaultGethWSPortOffset,
		startPort+integration.DefaultGethAUTHPortOffset,
		startPort+integration.DefaultGethNetworkPortOffset,
		startPort+integration.DefaultPrysmHTTPPortOffset,
		startPort+integration.DefaultPrysmP2PPortOffset,
		1337,
		1,
		blockDurationSeconds,
		2,
		2,
		walletAddresses,
		time.Minute,
	)

	err = eth2Network.Start()
	if err != nil {
		return nil, err
	}

	return eth2Network, nil
}

func DeployObscuroNetworkContracts(client ethadapter.EthClient, wallets *params.SimWallets, deployERC20s bool) (*params.L1SetupData, error) {
	bytecode, err := constants.Bytecode()
	if err != nil {
		return nil, err
	}
	mgmtContractReceipt, err := DeployContract(client, wallets.MCOwnerWallet, bytecode)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy management contract. Cause: %w", err)
	}

	managementContract, err := ManagementContract.NewManagementContract(mgmtContractReceipt.ContractAddress, client.EthClient())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate management contract. Cause: %w", err)
	}

	l1BusAddress, err := managementContract.MessageBus(&bind.CallOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MessageBus address. Cause: %w", err)
	}

	fmt.Println("Deployed Management Contract successfully",
		"address: ", mgmtContractReceipt.ContractAddress, "txHash: ", mgmtContractReceipt.TxHash,
		"blockHash: ", mgmtContractReceipt.BlockHash, "l1BusAddress: ", l1BusAddress)

	if !deployERC20s {
		return &params.L1SetupData{
			ObscuroStartBlock:   mgmtContractReceipt.BlockHash,
			MgmtContractAddress: mgmtContractReceipt.ContractAddress,
			MessageBusAddr:      l1BusAddress,
		}, nil
	}

	erc20ContractAddr := make([]common.Address, 0)
	for _, token := range wallets.Tokens {
		erc20receipt, err := DeployContract(client, token.L1Owner, erc20contract.L1BytecodeWithDefaultSupply(string(token.Name), mgmtContractReceipt.ContractAddress))
		if err != nil {
			return nil, fmt.Errorf("failed to deploy ERC20 contract. Cause: %w", err)
		}
		token.L1ContractAddress = &erc20receipt.ContractAddress
		erc20ContractAddr = append(erc20ContractAddr, erc20receipt.ContractAddress)
	}

	return &params.L1SetupData{
		ObscuroStartBlock:   mgmtContractReceipt.BlockHash,
		MgmtContractAddress: mgmtContractReceipt.ContractAddress,
		ObxErc20Address:     erc20ContractAddr[0],
		EthErc20Address:     erc20ContractAddr[1],
		MessageBusAddr:      l1BusAddress,
	}, nil
}

func StopEth2Network(clients []ethadapter.EthClient, netw eth2network.Eth2Network) {
	// Stop the clients first
	for _, c := range clients {
		if c != nil {
			c.Stop()
		}
	}
	// Stop the nodes second
	if netw != nil { // If network creation failed, we may be attempting to tear down the Geth network before it even exists.
		err := netw.Stop()
		if err != nil {
			fmt.Println(err)
		}
	}
}

// DeployContract returns receipt of deployment
// todo (@matt) - this should live somewhere else
func DeployContract(workerClient ethadapter.EthClient, w wallet.Wallet, contractBytes []byte) (*types.Receipt, error) {
	deployContractTx, err := workerClient.PrepareTransactionToSend(&types.LegacyTx{
		Data: contractBytes,
	}, w.Address(), w.GetNonceAndIncrement())
	if err != nil {
		w.SetNonce(w.GetNonce() - 1)
		return nil, err
	}

	signedTx, err := w.SignTransaction(deployContractTx)
	if err != nil {
		return nil, err
	}

	err = workerClient.SendTransaction(signedTx)
	if err != nil {
		return nil, err
	}

	var start time.Time
	var receipt *types.Receipt
	// todo (@matt) these timings should be driven by the L2 batch times and L1 block times
	for start = time.Now(); time.Since(start) < 80*time.Second; time.Sleep(2 * time.Second) {
		receipt, err = workerClient.TransactionReceipt(signedTx.Hash())
		if err == nil && receipt != nil {
			if receipt.Status != types.ReceiptStatusSuccessful {
				return nil, errors.New("unable to deploy contract")
			}
			testlog.Logger().Info(fmt.Sprintf("Contract successfully deployed to %s", receipt.ContractAddress))
			return receipt, nil
		}

		testlog.Logger().Info(fmt.Sprintf("Contract deploy tx (%s) has not been mined into a block after %s...", signedTx.Hash(), time.Since(start)))
	}

	return nil, fmt.Errorf("failed to mine contract deploy tx (%s) into a block after %s. Aborting", signedTx.Hash(), time.Since(start))
}

func CreateEthClientConnection(id int64, port uint) ethadapter.EthClient {
	ethnode, err := ethadapter.NewEthClient(Localhost, port, DefaultL1RPCTimeout, common.BigToAddress(big.NewInt(id)), testlog.Logger())
	if err != nil {
		panic(err)
	}
	return ethnode
}
