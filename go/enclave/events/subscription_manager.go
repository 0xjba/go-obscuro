package events

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sync"

	"github.com/obscuronet/go-obscuro/go/enclave/core"

	"github.com/obscuronet/go-obscuro/go/enclave/storage"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/obscuronet/go-obscuro/go/common"
	"github.com/obscuronet/go-obscuro/go/enclave/rpc"
	"github.com/obscuronet/go-obscuro/go/enclave/vkhandler"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethlog "github.com/ethereum/go-ethereum/log"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
)

const (
	// The leading zero bytes in a hash indicating that it is possibly an address, since it only has 20 bytes of data.
	zeroBytesHex = "000000000000000000000000"
)

// SubscriptionManager manages the creation/deletion of subscriptions, and the filtering and encryption of logs for
// active subscriptions.
type SubscriptionManager struct {
	rpcEncryptionManager *rpc.EncryptionManager
	storage              storage.Storage

	subscriptions     map[gethrpc.ID]*common.LogSubscription
	subscriptionMutex *sync.RWMutex // the mutex guards the subscriptions/lastHead pair

	logger gethlog.Logger
}

func NewSubscriptionManager(rpcEncryptionManager *rpc.EncryptionManager, storage storage.Storage, logger gethlog.Logger) *SubscriptionManager {
	return &SubscriptionManager{
		rpcEncryptionManager: rpcEncryptionManager,
		storage:              storage,

		subscriptions:     map[gethrpc.ID]*common.LogSubscription{},
		subscriptionMutex: &sync.RWMutex{},
		logger:            logger,
	}
}

// AddSubscription adds a log subscription to the enclave under the given ID, provided the request is authenticated
// correctly. If there is an existing subscription with the given ID, it is overwritten.
func (s *SubscriptionManager) AddSubscription(id gethrpc.ID, encryptedSubscription common.EncryptedParamsLogSubscription) error {
	encodedSubscription, err := s.rpcEncryptionManager.DecryptBytes(encryptedSubscription)
	if err != nil {
		return fmt.Errorf("could not decrypt params in eth_subscribe logs request. Cause: %w", err)
	}

	subscription := &common.LogSubscription{}
	if err = rlp.DecodeBytes(encodedSubscription, subscription); err != nil {
		return fmt.Errorf("could not decocde log subscription from RLP. Cause: %w", err)
	}

	// create viewing key encryption handler for pushing future logs
	encryptor, err := vkhandler.New(subscription.Account, subscription.PublicViewingKey, subscription.Signature)
	if err != nil {
		return fmt.Errorf("unable to create vk encryption for request - %w", err)
	}
	subscription.VkHandler = encryptor

	s.subscriptionMutex.Lock()
	defer s.subscriptionMutex.Unlock()
	s.subscriptions[id] = subscription

	return nil
}

// RemoveSubscription removes the log subscription with the given ID from the enclave. If there is no subscription with
// the given ID, nothing is deleted.
func (s *SubscriptionManager) RemoveSubscription(id gethrpc.ID) {
	s.subscriptionMutex.Lock()
	defer s.subscriptionMutex.Unlock()
	delete(s.subscriptions, id)
}

// FilterLogsForReceipt removes the logs that the sender of a transaction is not allowed to view
func (s *SubscriptionManager) FilterLogsForReceipt(receipt *types.Receipt, account *gethcommon.Address) ([]*types.Log, error) {
	filteredLogs := []*types.Log{}
	stateDB, err := s.storage.CreateStateDB(receipt.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("could not create state DB to filter logs. Cause: %w", err)
	}

	for _, logItem := range receipt.Logs {
		userAddrs := getUserAddrsFromLogTopics(logItem, stateDB)
		if isRelevant(account, userAddrs) {
			filteredLogs = append(filteredLogs, logItem)
		}
	}

	return filteredLogs, nil
}

// GetSubscribedLogsForBatch - Retrieves and encrypts the logs for the batch in live mode.
// The assumption is that this function is called synchronously after the batch is produced
func (s *SubscriptionManager) GetSubscribedLogsForBatch(batch *core.Batch, receipts types.Receipts) (common.EncryptedSubscriptionLogs, error) {
	s.subscriptionMutex.RLock()
	defer s.subscriptionMutex.RUnlock()

	// exit early if there are no subscriptions
	if len(s.subscriptions) == 0 {
		return nil, nil
	}

	relevantLogsPerSubscription := map[gethrpc.ID][]*types.Log{}

	// extract the logs from all receipts
	var allLogs []*types.Log
	for _, receipt := range receipts {
		allLogs = append(allLogs, receipt.Logs...)
	}

	if len(allLogs) == 0 {
		return nil, nil
	}

	// the stateDb is needed to extract the user addresses from the topics
	stateDB, err := s.storage.CreateStateDB(batch.Hash())
	if err != nil {
		return nil, fmt.Errorf("could not create state DB to filter logs. Cause: %w", err)
	}

	// cache for the user addresses extracted from the individual logs
	// this is an expensive operation so we are doing it lazy, and caching the result
	userAddrsForLog := map[*types.Log][]*gethcommon.Address{}

	for id, sub := range s.subscriptions {
		// first filter the logs
		filteredLogs := filterLogs(allLogs, sub.Filter.FromBlock, sub.Filter.ToBlock, sub.Filter.Addresses, sub.Filter.Topics, s.logger)

		relevantLogsForSub := []*types.Log{}
		for _, logItem := range filteredLogs {
			userAddrs, f := userAddrsForLog[logItem]
			if !f {
				userAddrs = getUserAddrsFromLogTopics(logItem, stateDB)
				userAddrsForLog[logItem] = userAddrs
			}
			relevant := isRelevant(sub.Account, userAddrs)
			if relevant {
				relevantLogsForSub = append(relevantLogsForSub, logItem)
			}
			s.logger.Info(fmt.Sprintf("Subscription %s. Account %s. Log %v. Extracted addresses: %v. Relevant: %t", id, sub.Account, logItem, userAddrs, relevant))
		}
		if len(relevantLogsForSub) > 0 {
			relevantLogsPerSubscription[id] = relevantLogsForSub
		}
	}

	// Encrypt the results
	return s.encryptLogs(relevantLogsPerSubscription)
}

func isRelevant(sub *gethcommon.Address, userAddrs []*gethcommon.Address) bool {
	// If there are no user addresses, this is a lifecycle event, and is therefore relevant to everyone.
	if len(userAddrs) == 0 {
		return true
	}
	for _, addr := range userAddrs {
		if *addr == *sub {
			return true
		}
	}
	return false
}

// Encrypts each log with the appropriate viewing key.
func (s *SubscriptionManager) encryptLogs(logsByID map[gethrpc.ID][]*types.Log) (map[gethrpc.ID][]byte, error) {
	encryptedLogsByID := map[gethrpc.ID][]byte{}

	for subID, logs := range logsByID {
		subscription, found := s.subscriptions[subID]
		if !found {
			continue // The subscription has been removed, so there's no need to return anything.
		}

		jsonLogs, err := json.Marshal(logs)
		if err != nil {
			return nil, fmt.Errorf("could not marshal logs to JSON. Cause: %w", err)
		}

		encryptedLogs, err := subscription.VkHandler.Encrypt(jsonLogs)
		if err != nil {
			return nil, fmt.Errorf("unable to encrypt logs - %w", err)
		}

		encryptedLogsByID[subID] = encryptedLogs
	}

	return encryptedLogsByID, nil
}

// Of the log's topics, returns those that are (potentially) user addresses. A topic is considered a user address if:
//   - It has 12 leading zero bytes (since addresses are 20 bytes long, while hashes are 32)
//   - It has a non-zero nonce (to prevent accidental or malicious creation of the address matching a given topic,
//     forcing its events to become permanently private
//   - It does not have associated code (meaning it's a smart-contract address)
func getUserAddrsFromLogTopics(log *types.Log, db *state.StateDB) []*gethcommon.Address {
	var userAddrs []*gethcommon.Address

	// We skip over the first topic, which is always the hash of the event.
	for _, topic := range log.Topics[1:len(log.Topics)] {
		if topic.Hex()[2:len(zeroBytesHex)+2] != zeroBytesHex {
			continue
		}

		potentialAddr := gethcommon.BytesToAddress(topic.Bytes())

		// A user address must have a non-zero nonce. This prevents accidental or malicious sending of funds to an
		// address matching a topic, forcing its events to become permanently private.
		if db.GetNonce(potentialAddr) != 0 {
			// If the address has code, it's a smart contract address instead.
			if db.GetCode(potentialAddr) == nil {
				userAddrs = append(userAddrs, &potentialAddr)
			}
		}
	}

	return userAddrs
}

// Lifted from eth/filters/filter.go in the go-ethereum repository.
// filterLogs creates a slice of logs matching the given criteria.
func filterLogs(logs []*types.Log, fromBlock, toBlock *big.Int, addresses []gethcommon.Address, topics [][]gethcommon.Hash, logger gethlog.Logger) []*types.Log { //nolint:gocognit
	var ret []*types.Log
Logs:
	for _, logItem := range logs {
		if fromBlock != nil && fromBlock.Int64() >= 0 && fromBlock.Uint64() > logItem.BlockNumber {
			logger.Info(fmt.Sprintf("Skipping log = %v", logItem), "reason", "In the past. The starting block num for filter is bigger than log")
			continue
		}
		if toBlock != nil && toBlock.Int64() > 0 && toBlock.Uint64() < logItem.BlockNumber {
			logger.Info(fmt.Sprintf("Skipping log = %v", logItem), "reason", "In the future. The ending block num for filter is smaller than log")
			continue
		}

		if len(addresses) > 0 && !includes(addresses, logItem.Address) {
			logger.Info(fmt.Sprintf("Skipping log = %v", logItem), "reason", "The contract address of the log is not an address of interest")
			continue
		}
		// If the to filtered topics is greater than the amount of topics in logs, skip.
		if len(topics) > len(logItem.Topics) {
			logger.Info(fmt.Sprintf("Skipping log = %v", logItem), "reason", "Insufficient topics. The log has less topics than the required one to satisfy the query")
			continue
		}
		for i, sub := range topics {
			match := len(sub) == 0 // empty rule set == wildcard
			for _, topic := range sub {
				if logItem.Topics[i] == topic {
					match = true
					break
				}
			}
			if !match {
				logger.Info(fmt.Sprintf("Skipping log = %v", logItem), "reason", "Topics do not match.")
				continue Logs
			}
		}
		ret = append(ret, logItem)
	}
	return ret
}

// Lifted from eth/filters/filter.go in the go-ethereum repository.
func includes(addresses []gethcommon.Address, a gethcommon.Address) bool {
	for _, addr := range addresses {
		if addr == a {
			return true
		}
	}

	return false
}
