package common

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/obscuronet/go-obscuro/go/enclave/vkhandler"
)

// LogSubscription is an authenticated subscription to logs.
type LogSubscription struct {
	// The account the events relate to.
	Account *common.Address

	// A signature over the account address using a private viewing key. Prevents attackers from subscribing to
	// (encrypted) logs for other accounts to see the pattern of logs.
	// todo - this does not protect against replay attacks, where someone resends an intercepted subscription request.
	Signature []byte

	// PublicViewingKey stores the viewing key used for this subscription
	PublicViewingKey []byte

	// A subscriber-defined filter to apply to the stream of logs.
	Filter *filters.FilterCriteria

	// Handles the viewing key encryption
	VkHandler *vkhandler.VKHandler
}

// IDAndEncLog pairs an encrypted log with the ID of the subscription that generated it.
type IDAndEncLog struct {
	SubID  rpc.ID
	EncLog []byte
}

// IDAndLog pairs a log with the ID of the subscription that generated it.
type IDAndLog struct {
	SubID rpc.ID
	Log   *types.Log
}

// FilterCriteriaJSON is a structure that JSON-serialises to a format that can be successfully deserialised into a
// filters.FilterCriteria object (round-tripping a filters.FilterCriteria to JSON and back doesn't work, due to a
// custom serialiser implemented by filters.FilterCriteria).
type FilterCriteriaJSON struct {
	BlockHash *common.Hash     `json:"blockHash"`
	FromBlock *rpc.BlockNumber `json:"fromBlock"`
	ToBlock   *rpc.BlockNumber `json:"toBlock"`
	Addresses interface{}      `json:"address"`
	Topics    []interface{}    `json:"topics"`
}
