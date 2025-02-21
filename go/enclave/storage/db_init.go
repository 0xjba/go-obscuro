package storage

import (
	"fmt"

	"github.com/obscuronet/go-obscuro/go/enclave/storage/enclavedb"

	"github.com/obscuronet/go-obscuro/go/enclave/storage/init/edgelessdb"
	"github.com/obscuronet/go-obscuro/go/enclave/storage/init/sqlite"

	gethlog "github.com/ethereum/go-ethereum/log"

	"github.com/obscuronet/go-obscuro/go/config"
)

// CreateDBFromConfig creates an appropriate ethdb.Database instance based on your config
func CreateDBFromConfig(cfg *config.EnclaveConfig, logger gethlog.Logger) (enclavedb.EnclaveDB, error) {
	if err := validateDBConf(cfg); err != nil {
		return nil, err
	}
	if cfg.UseInMemoryDB {
		logger.Info("UseInMemoryDB flag is true, data will not be persisted. Creating in-memory database...")
		// this creates a temporary sqlite sqldb
		return sqlite.CreateTemporarySQLiteDB(cfg.HostID.String(), "mode=memory&cache=shared&_foreign_keys=on", logger)
	}

	if !cfg.WillAttest {
		// persistent but not secure in an enclave, we'll connect to a throwaway sqlite DB and test out persistence/sql implementations
		logger.Warn("Attestation is disabled, using a basic sqlite DB for persistence")
		// when we want to test persistence after node restart the SqliteDBPath should be set
		// (if empty string then a temp sqldb file will be created for the lifetime of the enclave)
		return sqlite.CreateTemporarySQLiteDB(cfg.SqliteDBPath, "_foreign_keys=on", logger)
	}

	// persistent and with attestation means connecting to edgeless DB in a trusted enclave from a secure enclave
	logger.Info(fmt.Sprintf("Preparing Edgeless DB connection to %s...", cfg.EdgelessDBHost))
	return getEdgelessDB(cfg, logger)
}

// validateDBConf high-level checks that you have a valid configuration for DB creation
func validateDBConf(cfg *config.EnclaveConfig) error {
	if cfg.UseInMemoryDB && cfg.EdgelessDBHost != "" {
		return fmt.Errorf("invalid db config, useInMemoryDB=true so EdgelessDB host not expected, but EdgelessDBHost=%s", cfg.EdgelessDBHost)
	}
	if !cfg.WillAttest && cfg.EdgelessDBHost != "" {
		return fmt.Errorf("invalid db config, willAttest=false so EdgelessDB host not supported, but EdgelessDBHost=%s", cfg.EdgelessDBHost)
	}
	if !cfg.UseInMemoryDB && cfg.WillAttest && cfg.EdgelessDBHost == "" {
		return fmt.Errorf("useInMemoryDB=false, willAttest=true so expected an EdgelessDB host but none was provided")
	}
	if cfg.SqliteDBPath != "" && cfg.UseInMemoryDB {
		return fmt.Errorf("useInMemoryDB=true so sqlite database will not be used and no path is needed, but sqliteDBPath=%s", cfg.SqliteDBPath)
	}
	if cfg.SqliteDBPath != "" && cfg.WillAttest {
		return fmt.Errorf("willAttest=true so sqlite database will not be used and no path is needed, but sqliteDBPath=%s", cfg.SqliteDBPath)
	}
	return nil
}

func getEdgelessDB(cfg *config.EnclaveConfig, logger gethlog.Logger) (enclavedb.EnclaveDB, error) {
	if cfg.EdgelessDBHost == "" {
		return nil, fmt.Errorf("failed to prepare EdgelessDB connection - EdgelessDBHost was not set on enclave config")
	}
	dbConfig := edgelessdb.Config{Host: cfg.EdgelessDBHost}
	return edgelessdb.Connector(&dbConfig, logger)
}
