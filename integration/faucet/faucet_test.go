package faucet

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/obscuronet/go-obscuro/go/obsclient"
	"github.com/obscuronet/go-obscuro/go/wallet"
	"github.com/obscuronet/go-obscuro/integration"
	"github.com/obscuronet/go-obscuro/integration/common/testlog"
	"github.com/obscuronet/go-obscuro/integration/datagenerator"
	"github.com/obscuronet/go-obscuro/integration/ethereummock"
	"github.com/obscuronet/go-obscuro/integration/simulation/network"
	"github.com/obscuronet/go-obscuro/integration/simulation/params"
	"github.com/obscuronet/go-obscuro/tools/faucet/container"
	"github.com/obscuronet/go-obscuro/tools/faucet/faucet"
	"github.com/stretchr/testify/assert"
)

func init() { //nolint:gochecknoinits
	testlog.Setup(&testlog.Cfg{
		LogDir:      testLogs,
		TestType:    "faucet",
		TestSubtype: "test",
		LogLevel:    log.LvlInfo,
	})
}

const (
	contractDeployerPrivateKeyHex = "4bfe14725e685901c062ccd4e220c61cf9c189897b6c78bd18d7f51291b2b8f8"
	testLogs                      = "../.build/faucet/"
)

func TestFaucet(t *testing.T) {
	startPort := integration.StartPortFaucetUnitTest
	createObscuroNetwork(t, startPort)
	// This sleep is required to ensure the initial rollup exists, and thus contract deployer can check its balance.
	time.Sleep(2 * time.Second)

	faucetConfig := &faucet.Config{
		Port:       startPort,
		Host:       "localhost",
		HTTPPort:   startPort + integration.DefaultHostRPCHTTPOffset,
		PK:         "0x" + contractDeployerPrivateKeyHex,
		JWTSecret:  "This_is_secret",
		ChainID:    big.NewInt(integration.ObscuroChainID),
		ServerPort: integration.StartPortFaucetHTTPUnitTest,
	}
	faucetContainer, err := container.NewFaucetContainerFromConfig(faucetConfig)
	assert.NoError(t, err)

	err = faucetContainer.Start()
	assert.NoError(t, err)

	rndWallet := datagenerator.RandomWallet(integration.ObscuroChainID)
	err = fundWallet(faucetConfig.ServerPort, rndWallet)
	assert.NoError(t, err)

	obsClient, err := obsclient.DialWithAuth(fmt.Sprintf("http://%s:%d", network.Localhost, startPort+integration.DefaultHostRPCHTTPOffset), rndWallet, testlog.Logger())
	assert.NoError(t, err)

	currentBalance, err := obsClient.BalanceAt(context.Background(), nil)
	assert.NoError(t, err)

	if currentBalance.Cmp(big.NewInt(0)) <= 0 {
		t.Fatalf("Unexpected balance, got: %d, expected > 0", currentBalance.Int64())
	}
}

// Creates a single-node Obscuro network for testing.
func createObscuroNetwork(t *testing.T, startPort int) {
	// Create the Obscuro network.
	numberOfNodes := 1
	wallets := params.NewSimWallets(1, numberOfNodes, integration.EthereumChainID, integration.ObscuroChainID)
	simParams := params.SimParams{
		NumberOfNodes:    numberOfNodes,
		AvgBlockDuration: 1 * time.Second,
		MgmtContractLib:  ethereummock.NewMgmtContractLibMock(),
		ERC20ContractLib: ethereummock.NewERC20ContractLibMock(),
		Wallets:          wallets,
		StartPort:        startPort,
	}

	obscuroNetwork := network.NewNetworkOfSocketNodes(wallets)
	t.Cleanup(obscuroNetwork.TearDown)
	_, err := obscuroNetwork.Create(&simParams, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create test Obscuro network. Cause: %s", err))
	}
}

func fundWallet(port int, w wallet.Wallet) error {
	url := fmt.Sprintf("http://localhost:%d/fund/obx", port)
	method := "POST"

	payload := strings.NewReader(fmt.Sprintf(`{"address":"%s"}`, w.Address()))

	client := &http.Client{}
	req, err := http.NewRequestWithContext(context.Background(), method, url, payload)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}
