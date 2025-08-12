package main

const SERVICENAME = "datacollectorProducer"
const CONFIGFILENAME = "config.yaml"
const PROJECTNAME = "HAM"

type DatacollectorProducerConfig struct{
	Service struct{
		Name 	 	string	`yaml:"name" json:"name"`
		Port	 	string	`yaml:"port" json:"port"`
		HTTPpath	string  `yaml:"http_path" json:"http_path"`
	} `yaml:"service" json:"service"`
	
	Kafka struct {
		Brokers 	string `yaml:"brokers" json:"brokers"`
		Topic		string `yaml:"topic" json:"topic"`
	} `yaml:"kafka" json:"kafka"`
}

func NewDatacollectorProducerConfig() DatacollectorProducerConfig{
	return DatacollectorProducerConfig{}
}