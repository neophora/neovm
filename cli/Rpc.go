package main

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// Request is the function of getting block number
func Request(method string, params []int) (*http.Response, error) {
	client := &http.Client{}
	data := make(map[string]interface{})

	data["jsonrpc"] = "2.0"
	data["method"] = method
	data["params"] = params
	data["id"] = 1
	bytesData, _ := json.Marshal(data)

	req, err := http.NewRequest("POST", "http://seed1.ngd.network:10332", bytes.NewReader(bytesData))
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// RequestString send a request with string
func RequestString(method string, params []string) (*http.Response, error) {
	client := &http.Client{}
	data := make(map[string]interface{})

	data["jsonrpc"] = "2.0"
	data["method"] = method
	data["params"] = params
	data["id"] = 1
	bytesData, _ := json.Marshal(data)

	req, err := http.NewRequest("POST", "http://seed1.ngd.network:10332", bytes.NewReader(bytesData))
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
