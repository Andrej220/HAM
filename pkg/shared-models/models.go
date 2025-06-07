package datamodels

import(
	"github.com/google/uuid"	
)


type Request struct {
	HostID   int `json:"hostid"`
	ScriptID int `json:"scriptid"`
}

type datacollectorResponse struct {
	ExecutionUID uuid.UUID `json:"exuid"`
}

