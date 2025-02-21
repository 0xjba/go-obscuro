package container

import (
	"context"
	"fmt"

	gethlog "github.com/ethereum/go-ethereum/log"
	"github.com/obscuronet/go-obscuro/go/common"
	"github.com/obscuronet/go-obscuro/go/common/log"
	"github.com/obscuronet/go-obscuro/go/enclave"

	"github.com/obscuronet/go-obscuro/go/config"

	"github.com/obscuronet/go-obscuro/go/ethadapter/mgmtcontractlib"

	obscuroGenesis "github.com/obscuronet/go-obscuro/go/enclave/genesis"
)

// todo (#1056) - replace with the genesis.json of Obscuro's L1 network.
const hardcodedGenesisJSON = "TODO - REPLACE ME"

type EnclaveContainer struct {
	Enclave   common.Enclave
	RPCServer *enclave.RPCServer
	Logger    gethlog.Logger
}

func (e *EnclaveContainer) Start() error {
	err := e.RPCServer.StartServer()
	if err != nil {
		return err
	}
	e.Logger.Info("obscuro enclave RPC service started.")
	return nil
}

func (e *EnclaveContainer) Stop() error {
	_, err := e.RPCServer.Stop(context.Background(), nil)
	if err != nil {
		e.Logger.Warn("unable to cleanly stop enclave", log.ErrKey, err)
		return err
	}
	return nil
}

// NewEnclaveContainerFromConfig wires up the components of the Enclave and its RPC server. Manages their lifecycle/monitors their status
func NewEnclaveContainerFromConfig(config *config.EnclaveConfig) *EnclaveContainer {
	// todo - improve this wiring, perhaps setup DB etc. at this level and inject into enclave
	// (at that point the WithLogger constructor could be a full DI constructor like the HostContainer tries, for testability)
	logger := log.New(log.EnclaveCmp, config.LogLevel, config.LogPath, log.NodeIDKey, config.HostID)

	// todo - this is for debugging purposes only, should be remove in the future
	fmt.Printf("Building enclave container with config: %+v\n", config)
	logger.Info(fmt.Sprintf("Building enclave container with config: %+v", config))

	return NewEnclaveContainerWithLogger(config, logger)
}

// NewEnclaveContainerWithLogger is useful for testing etc.
func NewEnclaveContainerWithLogger(config *config.EnclaveConfig, logger gethlog.Logger) *EnclaveContainer {
	contractAddr := config.ManagementContractAddress
	mgmtContractLib := mgmtcontractlib.NewMgmtContractLib(&contractAddr, logger)

	if config.ValidateL1Blocks {
		config.GenesisJSON = []byte(hardcodedGenesisJSON)
	}

	genesis, err := obscuroGenesis.New(config.ObscuroGenesis)
	if err != nil {
		logger.Crit("unable to parse obscuro genesis", log.ErrKey, err)
	}

	encl := enclave.NewEnclave(config, genesis, mgmtContractLib, logger)
	rpcServer := enclave.NewEnclaveRPCServer(config.Address, encl, logger)

	return &EnclaveContainer{
		Enclave:   encl,
		RPCServer: rpcServer,
		Logger:    logger,
	}
}
