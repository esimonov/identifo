package embedded

import (
	"log"

	"github.com/madappgang/identifo/model"
	"github.com/madappgang/identifo/server"
)

// ServerSettings are server settings.
var ServerSettings = server.ServerSettings

func init() {
	if ServerSettings.DBType != "boltdb" {
		log.Fatalf("Incorrect database type %s for embedded server", ServerSettings.DBType)
	}
}

// NewServer creates new backend service with BoltDB support.
func NewServer(settings model.ServerSettings, options ...func(*DatabaseComposer) error) (model.Server, error) {
	dbComposer, err := NewComposer(settings, options...)
	if err != nil {
		return nil, err
	}
	return server.NewServer(settings, dbComposer)
}
