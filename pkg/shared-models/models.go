package datamodels

import(
	"github.com/google/uuid"	
)

type Request struct {
	HostID   int `json:"hostid,string"`
	ScriptID int `json:"scriptid,string"`
	ExecutionUID uuid.UUID `json:"exuid"`
}

type Response struct {
	ExecutionUID uuid.UUID `json:"exuid"`
}

