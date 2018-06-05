package main

import (
	"encoding/json"
	rs "github.com/jens18/lgresu/lgresustatus"
	"net/http"
	"net/http/httptest"
	"testing"
)

// https://blog.questionable.services/article/testing-http-handlers-go/
func TestIndex(t *testing.T) {

	// prepare lgResuStatus message
	lgResu := &rs.LgResuStatus{Soc: 78, Soh: 99, Voltage: 54.55, Current: -1, Temp: 26.1}

	// channel to signal request from Index
	httpSigChan := make(chan bool)
	// channel to send data to Index
	recordHttpChan := make(chan rs.LgResuStatus)

	// simulate BrokerRecord
	go func() {
		select {
		case <-httpSigChan: // wait for 'HTTP request pending' signal
			recordHttpChan <- *lgResu // send lrResu object
		}
	}()

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(Index(httpSigChan, recordHttpChan))

	// Call ServeHTTP method directly and pass in Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Index() handler returned wrong status code %v, expect %v",
			status, http.StatusOK)
	}

	// extract JSON message
	b := []byte(rr.Body.String())

	// re-constructed lgResuStatus message
	var status rs.LgResuStatus
	err = json.Unmarshal(b, &status)
	if err != nil {
		t.Error(err)
	}

	// test Soc received against Soc send
	if status.Soc != lgResu.Soc {
		t.Errorf("Index handler() returned Soc value %v, expect Soc value %v",
			status.Soc, lgResu.Soc)
	}
}
