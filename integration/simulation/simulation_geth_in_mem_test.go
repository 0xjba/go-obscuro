package simulation

import (
	"os"
	"testing"
	"time"

	"github.com/obscuronet/go-obscuro/integration"

	"github.com/obscuronet/go-obscuro/integration/simulation/network"
	"github.com/obscuronet/go-obscuro/integration/simulation/params"
)

const gethTestEnv = "GETH_TEST_ENABLED"

// TestGethSimulation runs the simulation against a private geth network using Clique (PoA)
func TestGethSimulation(t *testing.T) {
	if os.Getenv(gethTestEnv) == "" {
		t.Skipf("set the variable to run this test: `%s=true`", gethTestEnv)
	}
	setupSimTestLog("geth-in-mem")

	numberOfNodes := 5
	numberOfSimWallets := 5

	wallets := params.NewSimWallets(numberOfSimWallets, numberOfNodes, integration.EthereumChainID, integration.ObscuroChainID)

	simParams := &params.SimParams{
		NumberOfNodes:         numberOfNodes,
		AvgBlockDuration:      1 * time.Second,
		SimulationTime:        35 * time.Second,
		L1EfficiencyThreshold: 0.2,
		Wallets:               wallets,
		StartPort:             integration.StartPortSimulationGethInMem,
		IsInMem:               true,
		ReceiptTimeout:        30 * time.Second,
		StoppingDelay:         10 * time.Second,
	}

	simParams.AvgNetworkLatency = simParams.AvgBlockDuration / 15

	testSimulation(t, network.NewNetworkInMemoryGeth(wallets), simParams)
}
