package main

import (
	"encoding/json"
	"os"
)


func LoadConfigs() ([]SSHConfig, error) {
    file, err := os.Open("config.json")
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var configs []SSHConfig
    decoder := json.NewDecoder(file)
    err = decoder.Decode(&configs)
    if err != nil {
        return nil, err
    }
    return configs, nil
}