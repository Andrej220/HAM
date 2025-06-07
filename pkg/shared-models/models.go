package datamodels

import(
	"github.com/google/uuid"	
)


type Request struct {
	HostID   int `json:"hostid"`
	ScriptID int `json:"scriptid"`
}

type Response struct {
	ExecutionUID uuid.UUID `json:"exuid"`
}

