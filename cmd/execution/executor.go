package main

import (
	//"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
//	"strings"
)

type executorResponse struct{
	ExecutionUID int `json:"exuid"`
}

type executorRequest struct{
	HostID 		int `json:"hostid"`
	ScriptID 	int  `json:"scriptid"`
}

type validationHandler struct{
	next http.Handler
}

func newValidationHandler(next http.Handler) http.Handler {
	return validationHandler{next: next}
}

func (h validationHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request){
	var request executorRequest
	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&request)
	if err != nil{
		http.Error(rw, "Bad request", http.StatusBadRequest)
	}

	GetRemoteConfig(request.HostID, request.ScriptID)

	// get results and store them in DB

	//

	h.next.ServeHTTP(rw, r)
}

type executorHandler struct{}

func newExecutorHandler() http.Handler {
	return executorHandler{}
}

func (h executorHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request){
	response := executorResponse{ ExecutionUID: 1}
	encoder := json.NewEncoder(rw)
	encoder.Encode(response)
}


func main(){
	port := 8081

	handler := newValidationHandler(newExecutorHandler() )

	http.Handle("/executor", handler)

	log.Printf("Server starting on port %v\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", port), nil))
}