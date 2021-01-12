package fake

import (
	"github.com/madappgang/identifo/model"
	"github.com/madappgang/identifo/plugin/shared"
	"github.com/madappgang/identifo/storage/mem"
)

// NewComposer creates new database composer with in-memory storage support.
func NewComposer(settings model.ServerSettings, plugins shared.Plugins) (*DatabaseComposer, error) {
	c := DatabaseComposer{
		settings:                   settings,
		newAppStorage:              mem.NewAppStorage,
		userStorage:                plugins.UserStorage,
		newTokenStorage:            mem.NewTokenStorage,
		newTokenBlacklist:          mem.NewTokenBlacklist,
		newVerificationCodeStorage: mem.NewVerificationCodeStorage,
	}
	return &c, nil
}

// DatabaseComposer composes in-memory services.
type DatabaseComposer struct {
	settings                   model.ServerSettings
	newAppStorage              func() (model.AppStorage, error)
	userStorage                shared.UserStorage
	newTokenStorage            func() (model.TokenStorage, error)
	newTokenBlacklist          func() (model.TokenBlacklist, error)
	newVerificationCodeStorage func() (model.VerificationCodeStorage, error)
}

// Compose composes all services with in-memory storage support.
func (dc *DatabaseComposer) Compose() (
	model.AppStorage,
	shared.UserStorage,
	model.TokenStorage,
	model.TokenBlacklist,
	model.VerificationCodeStorage,
	error,
) {
	appStorage, err := dc.newAppStorage()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	tokenStorage, err := dc.newTokenStorage()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	tokenBlacklist, err := dc.newTokenBlacklist()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	verificationCodeStorage, err := dc.newVerificationCodeStorage()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	return appStorage, dc.userStorage, tokenStorage, tokenBlacklist, verificationCodeStorage, nil
}

// NewPartialComposer returns new partial composer with in-memory storage support.
func NewPartialComposer(settings model.StorageSettings, options ...func(*PartialDatabaseComposer) error) (*PartialDatabaseComposer, error) {
	pc := &PartialDatabaseComposer{}

	if settings.AppStorage.Type == model.DBTypeFake {
		pc.newAppStorage = mem.NewAppStorage
	}

	if settings.TokenStorage.Type == model.DBTypeFake {
		pc.newTokenStorage = mem.NewTokenStorage
	}

	if settings.TokenBlacklist.Type == model.DBTypeFake {
		pc.newTokenBlacklist = mem.NewTokenBlacklist
	}

	if settings.VerificationCodeStorage.Type == model.DBTypeFake {
		pc.newVerificationCodeStorage = mem.NewVerificationCodeStorage
	}

	for _, option := range options {
		if err := option(pc); err != nil {
			return nil, err
		}
	}
	return pc, nil
}

// PartialDatabaseComposer composes only those services that support in-memory storage.
type PartialDatabaseComposer struct {
	newAppStorage              func() (model.AppStorage, error)
	userStorage                shared.UserStorage
	newTokenStorage            func() (model.TokenStorage, error)
	newTokenBlacklist          func() (model.TokenBlacklist, error)
	newVerificationCodeStorage func() (model.VerificationCodeStorage, error)
}

// AppStorageComposer returns app storage composer.
func (pc *PartialDatabaseComposer) AppStorageComposer() func() (model.AppStorage, error) {
	if pc.newAppStorage != nil {
		return func() (model.AppStorage, error) {
			return pc.newAppStorage()
		}
	}
	return nil
}

// TokenStorageComposer returns token storage composer.
func (pc *PartialDatabaseComposer) TokenStorageComposer() func() (model.TokenStorage, error) {
	if pc.newTokenStorage != nil {
		return func() (model.TokenStorage, error) {
			return pc.newTokenStorage()
		}
	}
	return nil
}

// TokenBlacklistComposer returns token blacklist composer.
func (pc *PartialDatabaseComposer) TokenBlacklistComposer() func() (model.TokenBlacklist, error) {
	if pc.newTokenBlacklist != nil {
		return func() (model.TokenBlacklist, error) {
			return pc.newTokenBlacklist()
		}
	}
	return nil
}

// VerificationCodeStorageComposer returns verification code storage composer.
func (pc *PartialDatabaseComposer) VerificationCodeStorageComposer() func() (model.VerificationCodeStorage, error) {
	if pc.newVerificationCodeStorage != nil {
		return func() (model.VerificationCodeStorage, error) {
			return pc.newVerificationCodeStorage()
		}
	}
	return nil
}
