package boltdb

import (
	"github.com/boltdb/bolt"
	"github.com/madappgang/identifo/model"
	"github.com/madappgang/identifo/plugin/shared"
	"github.com/madappgang/identifo/storage/boltdb"
)

// NewComposer creates new database composer with BoltDB support.
func NewComposer(settings model.ServerSettings, plugins shared.Plugins) (*DatabaseComposer, error) {
	c := DatabaseComposer{
		settings:                   settings,
		newAppStorage:              boltdb.NewAppStorage,
		userStorage:                plugins.UserStorage,
		newTokenStorage:            boltdb.NewTokenStorage,
		newTokenBlacklist:          boltdb.NewTokenBlacklist,
		newVerificationCodeStorage: boltdb.NewVerificationCodeStorage,
	}
	return &c, nil
}

// DatabaseComposer composes BoltDB services.
type DatabaseComposer struct {
	settings                   model.ServerSettings
	newAppStorage              func(*bolt.DB) (model.AppStorage, error)
	userStorage                shared.UserStorage
	newTokenStorage            func(*bolt.DB) (model.TokenStorage, error)
	newTokenBlacklist          func(*bolt.DB) (model.TokenBlacklist, error)
	newVerificationCodeStorage func(*bolt.DB) (model.VerificationCodeStorage, error)
}

// Compose composes all services with BoltDB support.
func (dc *DatabaseComposer) Compose() (
	model.AppStorage,
	shared.UserStorage,
	model.TokenStorage,
	model.TokenBlacklist,
	model.VerificationCodeStorage,
	error,
) {
	// We assume that all BoltDB-backed storages share the same filepath, so we can pick any of them.
	db, err := boltdb.InitDB(dc.settings.Storage.AppStorage.Path)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	appStorage, err := dc.newAppStorage(db)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	tokenStorage, err := dc.newTokenStorage(db)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	tokenBlacklist, err := dc.newTokenBlacklist(db)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	verificationCodeStorage, err := dc.newVerificationCodeStorage(db)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return appStorage, dc.userStorage, tokenStorage, tokenBlacklist, verificationCodeStorage, nil
}

// NewPartialComposer returns new partial composer with BoltDB support.
func NewPartialComposer(settings model.StorageSettings, plugins shared.Plugins, options ...func(*PartialDatabaseComposer) error) (*PartialDatabaseComposer, error) {
	pc := &PartialDatabaseComposer{}
	// We assume that all BoltDB-backed storages share the same filepath, so we can pick any of them.
	var dbPath string

	pc.userStorage = plugins.UserStorage

	if settings.AppStorage.Type == model.DBTypeBoltDB {
		pc.newAppStorage = boltdb.NewAppStorage
		dbPath = settings.AppStorage.Path
	}

	if settings.TokenStorage.Type == model.DBTypeBoltDB {
		pc.newTokenStorage = boltdb.NewTokenStorage
		dbPath = settings.TokenStorage.Path
	}

	if settings.TokenBlacklist.Type == model.DBTypeBoltDB {
		pc.newTokenBlacklist = boltdb.NewTokenBlacklist
		dbPath = settings.TokenBlacklist.Path
	}

	if settings.VerificationCodeStorage.Type == model.DBTypeBoltDB {
		pc.newVerificationCodeStorage = boltdb.NewVerificationCodeStorage
		dbPath = settings.VerificationCodeStorage.Path
	}

	db, err := boltdb.InitDB(dbPath)
	if err != nil {
		return nil, err
	}
	pc.db = db

	for _, option := range options {
		if err := option(pc); err != nil {
			return nil, err
		}
	}
	return pc, nil
}

// PartialDatabaseComposer composes only BoltDB-supporting services.
type PartialDatabaseComposer struct {
	db                         *bolt.DB
	newAppStorage              func(*bolt.DB) (model.AppStorage, error)
	userStorage                shared.UserStorage
	newTokenStorage            func(*bolt.DB) (model.TokenStorage, error)
	newTokenBlacklist          func(*bolt.DB) (model.TokenBlacklist, error)
	newVerificationCodeStorage func(*bolt.DB) (model.VerificationCodeStorage, error)
}

// AppStorageComposer returns app storage composer.
func (pc *PartialDatabaseComposer) AppStorageComposer() func() (model.AppStorage, error) {
	if pc.newAppStorage != nil {
		return func() (model.AppStorage, error) {
			return pc.newAppStorage(pc.db)
		}
	}
	return nil
}

// TokenStorageComposer returns token storage composer.
func (pc *PartialDatabaseComposer) TokenStorageComposer() func() (model.TokenStorage, error) {
	if pc.newTokenStorage != nil {
		return func() (model.TokenStorage, error) {
			return pc.newTokenStorage(pc.db)
		}
	}
	return nil
}

// TokenBlacklistComposer returns token blacklist composer.
func (pc *PartialDatabaseComposer) TokenBlacklistComposer() func() (model.TokenBlacklist, error) {
	if pc.newTokenBlacklist != nil {
		return func() (model.TokenBlacklist, error) {
			return pc.newTokenBlacklist(pc.db)
		}
	}
	return nil
}

// VerificationCodeStorageComposer returns verification code storage composer.
func (pc *PartialDatabaseComposer) VerificationCodeStorageComposer() func() (model.VerificationCodeStorage, error) {
	if pc.newVerificationCodeStorage != nil {
		return func() (model.VerificationCodeStorage, error) {
			return pc.newVerificationCodeStorage(pc.db)
		}
	}
	return nil
}
