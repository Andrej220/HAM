// pkg/config/config.go
package config

import (
	"errors"
	"fmt"
    "github.com/andrej220/HAM/pkg/config/configstore"
    "github.com/andrej220/HAM/pkg/config/filestore"
    "github.com/andrej220/HAM/pkg/config/mongostore"
	"os"
	"strings"
)

type StoreType int

const (
    FileStore StoreType = iota
    MongoStore
)
const (
	prodRootDir = "/etc/"
	devRootDir  = "./apps/"
)

var (
	ErrInvalidStoreType = errors.New("invalid store type")
)

// Config interface that combines all store capabilities
type Config interface {
	configstore.ConfigStore
	Watch(onChange func()) error // Optional for stores that support watching
}

type FileConfig struct {
	Path string `yaml:"path" json:"path"`
}

type MongoConfig struct {
	URI      string `yaml:"uri" json:"uri"`
	DBName   string `yaml:"dbName" json:"dbName"`
	CollName string `yaml:"collName" json:"collName"`
	ID       string `yaml:"id" json:"id"` // Document ID
}

func NewStore(storeType StoreType, cfg any) (Config, error) {
	switch storeType {
	case FileStore:
		fileCfg, ok := cfg.(*FileConfig)
		if !ok {
			return nil, fmt.Errorf("invalid config type for file store, expected *FileConfig")
		}
		return filestore.New(fileCfg.Path), nil
	case MongoStore:
		mongoCfg, ok := cfg.(*MongoConfig)
		if !ok {
			return nil, fmt.Errorf("invalid config type for mongo store, expected *MongoConfig")
		}
		return mongostore.New(mongoCfg.URI, mongoCfg.DBName, mongoCfg.CollName, mongoCfg.ID)
	default:
		return nil, ErrInvalidStoreType
	}
}

func GetConfigPath(projectName string, serviceName string, filename string) string {
    
    if path := os.Getenv(strings.ToUpper(serviceName) + "_CONFIG_PATH"); path != "" {
        return path
    }
    
    if os.Getenv("ENVIRONMENT") == "production" {
        return prodRootDir + strings.ToUpper(projectName) +"/" + filename  
    }    
	
	if os.Getenv("ENVIRONMENT") == "debug" {
        return "./"  + filename 
    }
    
    // 3. Default development path
    return devRootDir + serviceName +"/" + filename
}