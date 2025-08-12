package main

const SERVICENAME = "dataservice"
const CONFIGFILENAME = "config.yaml"
const PROJECTNAME = "HAM"

type DBConfig struct{
	MongoCollection string	`yaml:"mongoCollection" json:"mongoCollection"`
	MongoDBName 	string	`yaml:"mongoDBName" json:"mongoDBName"`
}

type DataserviceConfig struct {
	Server struct {
		Port 			string `yaml:"port" json:"port"`
		Endpoint        string `yaml:"endpoint" json:"endpoint"`
	}
	DB struct {
		MongoURI  		string   `yaml:"mongoURI" json:"mongoURI"`	
		DBConf 			DBConfig `yaml:"dbConf" json:"dbConf"`
	}
}

func NewDataserviceConfig() *DataserviceConfig{
	return &DataserviceConfig{}
}

//	DATASERVICEPORT = "8082"
//	ENDPOINT = "/dataservice"
//	MongoDBURI = "mongodb://root:BnJlZJ7DOy@localhost:27017/appdb?authSource=admin"
//	mongoDBCollection = "mycollection"
//	mongoDBDatabase = "appdb"`