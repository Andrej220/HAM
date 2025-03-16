package main

import (
	"os"
	"log"
	"github.com/google/uuid"
)

type DSjobStruct struct {
    HostID      int
    ScriptID    int
    UUID        uuid.UUID
	dataChan	chan string
}


func WriteFile(job DSjobStruct) error{

	filename := "testname"
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err!= nil{
		log.Printf("Failed to create filename %s", err)
		return err
	}
	defer file.Close()

	for line := range job.dataChan {
		_, err := file.WriteString(line)
		if err !=  nil {
			log.Printf("Failed to write to file: %s", err)
			return err
		}
	}
	log.Printf("Data successfully saved on disk: %+v", job)
	return nil
}