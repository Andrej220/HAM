package main

const SERVICENAME = "datacollector"
const CONFIGFILENAME = "config.yaml"
const PROJECTNAME = "HAM"

type DataCollectorConfig struct{
	Server struct {
		Port int `yaml:"port" json:"port"`
	} `yaml:"server" json:"server"`
	
	Kafka struct {
		Brokers []string `yaml:"brokers" json:"brokers"`
		Topic   string   `yaml:"topic" json:"topic"`
		GroupID string   `yaml:"groupID" json:"groupID"`
	} `yaml:"kafka" json:"kafka"`
	
	Database struct {
		MongoURI string `yaml:"mongoURI" json:"mongoURI"`
		DBName   string `yaml:"dbName" json:"dbName"`
	} `yaml:"database" json:"database"`
}

func NewDataCollectorConfig() *DataCollectorConfig{
	return &DataCollectorConfig{}
}

