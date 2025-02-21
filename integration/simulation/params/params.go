package params

import (
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/obscuronet/go-obscuro/go/ethadapter/erc20contractlib"
	"github.com/obscuronet/go-obscuro/go/ethadapter/mgmtcontractlib"
)

// SimParams are the parameters for setting up the simulation.
type SimParams struct {
	NumberOfNodes int

	// A critical parameter of the simulation. The value should be as low as possible, as long as the test is still meaningful
	AvgBlockDuration  time.Duration
	AvgNetworkLatency time.Duration // artificial latency injected between sending and receiving messages on the mock network

	SimulationTime time.Duration // how long the simulations should run for

	L1EfficiencyThreshold float64

	// MgmtContractLib allows parsing MgmtContract txs to and from the eth txs
	MgmtContractLib mgmtcontractlib.MgmtContractLib
	// ERC20ContractLib allows parsing ERC20Contract txs to and from the eth txs
	ERC20ContractLib erc20contractlib.ERC20ContractLib

	L1SetupData *L1SetupData

	// Contains all the wallets required by the simulation
	Wallets *SimWallets

	StartPort int  // The port from which to start allocating ports. Must be unique across all simulations.
	IsInMem   bool // Denotes that the sim does not have a full RPC layer.

	ReceiptTimeout time.Duration // How long to wait for transactions to be confirmed.

	StoppingDelay              time.Duration // How long to wait between injection and verification
	NodeWithInboundP2PDisabled int
}

type L1SetupData struct {
	// ObscuroStartBlock is the L1 block hash where the Obscuro network activity begins (e.g. mgmt contract deployment)
	ObscuroStartBlock common.Hash
	// MgmtContractAddr defines the management contract address
	MgmtContractAddress common.Address
	// ObxErc20Address - the address of the "OBX" ERC20
	ObxErc20Address common.Address
	// EthErc20Address - the address of the "ETH" ERC20
	EthErc20Address common.Address
	// MessageBusAddr - the address of the L1 message bus.
	MessageBusAddr common.Address
}
