package backend

import (
	"fmt"

	"github.com/obscuronet/go-obscuro/go/common"
	"github.com/obscuronet/go-obscuro/go/obsclient"

	gethcommon "github.com/ethereum/go-ethereum/common"
)

type Backend struct {
	obsClient *obsclient.ObsClient
}

func NewBackend(obsClient *obsclient.ObsClient) *Backend {
	return &Backend{
		obsClient: obsClient,
	}
}

func (b *Backend) GetLatestBatch() (*common.BatchHeader, error) {
	return b.obsClient.BatchHeaderByNumber(nil)
}

func (b *Backend) GetLatestRollup() (*common.RollupHeader, error) {
	return &common.RollupHeader{}, nil
}

func (b *Backend) GetNodeCount() (int, error) {
	// return b.obsClient.ActiveNodeCount()
	return 0, nil
}

func (b *Backend) GetTotalContractCount() (int, error) {
	return b.obsClient.GetTotalContractCount()
}

func (b *Backend) GetTotalTransactionCount() (int, error) {
	return b.obsClient.GetTotalTransactionCount()
}

func (b *Backend) GetLatestRollupHeader() (*common.RollupHeader, error) {
	return b.obsClient.GetLatestRollupHeader()
}

func (b *Backend) GetBatch(hash gethcommon.Hash) (*common.BatchHeader, error) {
	return b.obsClient.BatchHeaderByHash(hash)
}

func (b *Backend) GetTransaction(_ gethcommon.Hash) (*common.L2Tx, error) {
	return nil, fmt.Errorf("unable to get encrypted Tx")
}

func (b *Backend) GetPublicTransactions(offset uint64, size uint64) (*common.TransactionListingResponse, error) {
	return b.obsClient.GetPublicTxListing(&common.QueryPagination{
		Offset: offset,
		Size:   uint(size),
	})
}

func (b *Backend) GetBatchesListing(offset uint64, size uint64) (*common.BatchListingResponse, error) {
	return b.obsClient.GetBatchesListing(&common.QueryPagination{
		Offset: offset,
		Size:   uint(size),
	})
}

func (b *Backend) GetBlockListing(offset uint64, size uint64) (*common.BlockListingResponse, error) {
	return b.obsClient.GetBlockListing(&common.QueryPagination{
		Offset: offset,
		Size:   uint(size),
	})
}
