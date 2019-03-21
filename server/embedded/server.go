package embedded

import (
	"log"
	"os"

	"github.com/madappgang/identifo/model"
	"github.com/madappgang/identifo/server"
)

const databaseConfigPath = "./database-config.yaml"

// Settings are the extended settings for BoltDB server.
type Settings struct {
	model.ServerSettings
	DBPath string `yaml:"path"`
}

// ServerSettings are default server settings.
var ServerSettings = Settings{}

func init() {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalln("Cannot get database configuration file:", err)
	}

	server.LoadConfiguration(dir, databaseConfigPath, &ServerSettings)
	ServerSettings.ServerSettings = server.ServerSettings
}

// NewServer creates new backend service with BoltDB support.
func NewServer(settings Settings, options ...func(*server.Server) error) (model.Server, error) {
	dbComposer, err := NewComposer(settings)
	if err != nil {
		return nil, err
	}
	return server.NewServer(settings.ServerSettings, dbComposer, options...)
}
