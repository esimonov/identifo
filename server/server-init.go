package server

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	s3Storage "github.com/madappgang/identifo/external_services/storage/s3"
	"github.com/madappgang/identifo/model"
	"go.etcd.io/etcd/clientv3"
	"gopkg.in/yaml.v2"
)

const (
	defaultAdminLogin    = "admin@admin.com"
	defaultAdminPassword = "password"
)

const warningMsg = "WARNING! Config file could not be read, so the default server-config.yaml will be used for the server configuration. Note that when using Docker container, changes made to this file won't survive the container restart."

func init() {
	configFlag := flag.String("config", "", "Path to the file that describes the location of a server configuration file")
	flag.Parse()

	if configFlag == nil || len(*configFlag) == 0 {
		log.Println("Config file path not specified.")
		loadDefaultServerConfiguration(&ServerSettings)
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Cannot get current working directory: %s\n", err)
	}

	initConfigBytes, err := ioutil.ReadFile(filepath.Join(wd, *configFlag))
	if err != nil {
		log.Println("Cannot read init configuration file: ", err, warningMsg)
		loadDefaultServerConfiguration(&ServerSettings)
		return
	}

	ic := new(initialConfig)
	if err = yaml.Unmarshal(initConfigBytes, ic); err != nil {
		log.Println("Cannot unmarshal init configuration file: ", err, warningMsg)
		loadDefaultServerConfiguration(&ServerSettings)
		return
	}

	if err = ic.Validate(); err != nil {
		log.Println("Cannot load initial config: ", err, warningMsg)
		loadDefaultServerConfiguration(&ServerSettings)
		return
	}

	switch ic.Location {
	case "local":
		loadConfigFromFile(ic, &ServerSettings)
	case "etcd":
		loadConfigFromEtcd(ic, &ServerSettings)
	case "s3":
		loadConfigFromS3(ic, &ServerSettings)
	default:
		log.Fatalf("Unknown configuration location %s", ic.Location)
	}
}

func loadConfigFromFile(ic *initialConfig, out *model.ServerSettings) {
	log.Println("Loading server configuration from specified file...")
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalln("Cannot get server configuration file:", err)
	}

	configFile, err := ioutil.ReadFile(filepath.Join(dir, ic.Folder, ic.Filename))
	if err != nil {
		log.Fatalln("Cannot read server configuration file:", err)
	}

	if err = yaml.Unmarshal(configFile, out); err != nil {
		log.Fatalln("Cannot unmarshal server configuration file:", err)
	}

	if err := out.Validate(); err != nil {
		log.Fatalln("Invalid settings.", err)
	}

	loadAdminEnvVars(out.AdminAccount)

	log.Println("Server configuration loaded from the file.")
}

func loadConfigFromEtcd(ic *initialConfig, out *model.ServerSettings) {
	log.Println("Loading server configuration from the etcd...")
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   ic.Endpoints,
		DialTimeout: 5 * time.Second,
	})

	if err != nil {
		log.Fatalf("Cannot get object from S3: %s", err)
	}

	res, err := etcdClient.Get(context.Background(), ic.Key)
	if err != nil {
		log.Fatalf("Cannot get value by key %s: %s", ic.Key, err)
	}
	if len(res.Kvs) == 0 {
		log.Fatalf("Etcd: No value for key %s", ic.Key)
	}

	if err = json.Unmarshal(res.Kvs[0].Value, out); err != nil {
		log.Fatalf("Cannot unmarshal value of key '%s'. %s", ic.Key, err)
	}
}

func loadConfigFromS3(ic *initialConfig, out *model.ServerSettings) {
	log.Println("Loading server configuration from the S3 bucket...")
	s3client, err := s3Storage.NewS3Client(ic.Region)
	if err != nil {
		log.Fatalf("Cannot initialize S3 client: %s.", err)
	}
	getObjInput := &s3.GetObjectInput{
		Bucket: aws.String(ic.Bucket),
		Key:    aws.String(filepath.Join(ic.Folder, ic.Filename)),
	}

	resp, err := s3client.GetObject(getObjInput)
	if err != nil {
		log.Fatalf("Cannot get object from S3: %s", err)
	}
	defer resp.Body.Close()

	if err = yaml.NewDecoder(resp.Body).Decode(out); err != nil {
		log.Fatalf("Cannot decode S3 response: %s", err)
	}
	log.Println("Server configuration loaded from the S3 bucket.")
}

// initialConfig is for settings required by the server on the start.
type initialConfig struct {
	Location  string   `yaml:"location"`
	Folder    string   `yaml:"folder"`
	Filename  string   `yaml:"filename"`
	Bucket    string   `yaml:"bucket"`
	Region    string   `yaml:"region"`
	Key       string   `yaml:"key"`
	Endpoints []string `yaml:"endpoints"`
}

func (ic *initialConfig) Validate() error {
	subject := "Initial config"

	if ic == nil {
		return fmt.Errorf("Nil initial server config")
	}

	switch ic.Location {
	case "local":
		if len(ic.Filename) == 0 {
			return fmt.Errorf("%s. Empty filename", subject)
		}
	case "s3":
		if len(ic.Filename) == 0 {
			return fmt.Errorf("%s. Empty filename", subject)
		}
		if len(ic.Bucket) == 0 {
			log.Fatalf("%s. Empty bucket", subject)
		}
		if len(ic.Region) == 0 {
			log.Fatalf("%s. Empty region", subject)
		}
	case "etcd":
		if len(ic.Key) == 0 {
			log.Fatalf("%s. Empty key", subject)
		}
		if len(ic.Endpoints) == 0 {
			log.Fatalf("%s. Empty endpoints", subject)
		}
	default:
		return fmt.Errorf("Unknown location '%s'", ic.Location)
	}
	return nil
}

const serverConfigPathEnvName = "SERVER_CONFIG_PATH"

// loadDefaultServerConfiguration loads configuration from the yaml file and writes it to out variable.
func loadDefaultServerConfiguration(out *model.ServerSettings) {
	log.Println(warningMsg, "\n", "Loading default server configuration...")
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalln("Cannot get server configuration file:", err)
	}

	// Iterate through possible config paths until we find the valid one.
	configPaths := []string{
		os.Getenv(serverConfigPathEnvName),
		"./server-config.yaml",
		"../../server/server-config.yaml",
	}

	var configFile []byte

	for _, p := range configPaths {
		if p == "" {
			continue
		}
		configFile, err = ioutil.ReadFile(filepath.Join(dir, p))
		if err == nil {
			break
		}
	}

	if err != nil {
		log.Fatalln("Cannot read server configuration file:", err)
	}

	if err = yaml.Unmarshal(configFile, out); err != nil {
		log.Fatalln("Cannot unmarshal server configuration file:", err)
	}

	if err := out.Validate(); err != nil {
		log.Fatalln(err)
	}
	loadAdminEnvVars(out.AdminAccount)
	log.Println("Default server configuration loaded.")
}

func loadAdminEnvVars(vars model.AdminAccountSettings) {
	if len(os.Getenv(vars.LoginEnvName)) == 0 {
		if err := os.Setenv(vars.LoginEnvName, defaultAdminLogin); err != nil {
			log.Fatalf("Could not set default %s: %s\n", vars.LoginEnvName, err)
		}
		log.Printf("WARNING! %s not set. Default value will be used.\n", vars.LoginEnvName)
	}
	if len(os.Getenv(vars.PasswordEnvName)) == 0 {
		if err := os.Setenv(vars.PasswordEnvName, defaultAdminPassword); err != nil {
			log.Fatalf("Could not set default %s: %s\n", vars.PasswordEnvName, err)
		}
		log.Printf("WARNING! %s not set. Default value will be used.\n", vars.PasswordEnvName)
	}
}
