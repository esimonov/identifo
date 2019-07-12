package server

import (
	"fmt"
	"path"

	"github.com/madappgang/identifo/jwt"
	jwtService "github.com/madappgang/identifo/jwt/service"
	"github.com/madappgang/identifo/model"
)

// DatabaseComposer inits database stack.
type DatabaseComposer interface {
	Compose() (
		model.AppStorage,
		model.UserStorage,
		model.TokenStorage,
		model.VerificationCodeStorage,
		jwtService.TokenService,
		error,
	)
}

// PartialDatabaseComposer can init services backed with different databases.
type PartialDatabaseComposer interface {
	AppStorageComposer() func() (model.AppStorage, error)
	UserStorageComposer() func() (model.UserStorage, error)
	TokenStorageComposer() func() (model.TokenStorage, error)
	VerificationCodeStorageComposer() func() (model.VerificationCodeStorage, error)
}

// Composer is a service composer which is agnostic to particular database implementations.
type Composer struct {
	settings                   model.ServerSettings
	newAppStorage              func() (model.AppStorage, error)
	newUserStorage             func() (model.UserStorage, error)
	newTokenStorage            func() (model.TokenStorage, error)
	newVerificationCodeStorage func() (model.VerificationCodeStorage, error)
}

// Compose composes all services.
func (c *Composer) Compose() (
	model.AppStorage,
	model.UserStorage,
	model.TokenStorage,
	model.VerificationCodeStorage,
	jwtService.TokenService,
	error,
) {
	appStorage, err := c.newAppStorage()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	userStorage, err := c.newUserStorage()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	tokenStorage, err := c.newTokenStorage()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	verificationCodeStorage, err := c.newVerificationCodeStorage()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	tokenServiceAlg, ok := jwt.StrToTokenSignAlg[c.settings.Algorithm]
	if !ok {
		return nil, nil, nil, nil, nil, fmt.Errorf("Unknown token service algorithm %s", c.settings.Algorithm)
	}

	tokenService, err := jwtService.NewJWTokenService(
		path.Join(c.settings.PEMFolderPath, c.settings.PrivateKey),
		path.Join(c.settings.PEMFolderPath, c.settings.PublicKey),
		c.settings.Issuer,
		tokenServiceAlg,
		tokenStorage,
		appStorage,
		userStorage,
	)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	return appStorage, userStorage, tokenStorage, verificationCodeStorage, tokenService, nil
}

// NewComposer returns new database composer based on passed server settings.
func NewComposer(settings model.ServerSettings, partialComposers []PartialDatabaseComposer, options ...func(*Composer) error) (*Composer, error) {
	c := &Composer{settings: settings}

	for _, pc := range partialComposers {
		if pc.AppStorageComposer() != nil {
			c.newAppStorage = pc.AppStorageComposer()
		}
		if pc.UserStorageComposer() != nil {
			c.newUserStorage = pc.UserStorageComposer()
		}
		if pc.TokenStorageComposer() != nil {
			c.newTokenStorage = pc.TokenStorageComposer()
		}
		if pc.VerificationCodeStorageComposer() != nil {
			c.newVerificationCodeStorage = pc.VerificationCodeStorageComposer()
		}
	}

	for _, option := range options {
		if err := option(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}
