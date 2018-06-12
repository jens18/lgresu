// Copyright 2018 Jens Kaemmerer. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"github.com/brutella/can"
	rs "github.com/jens18/lgresu/lgresustatus"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type MockDatarecorder struct {
	Cnt int
}

func (d *MockDatarecorder) WriteToDatafile(currentTime time.Time, record string) {
	d.Cnt++
}

type MockCanbus struct {
	PublishCnt    int
	DisconnectCnt int
}

func (c *MockCanbus) Publish(can.Frame) error {
	c.PublishCnt++
	return nil
}

func (c *MockCanbus) Disconnect() error {
	c.DisconnectCnt++
	return nil
}

func init() {
	// only log warning severity or above.
	log.SetLevel(log.DebugLevel)
}

// https://blog.questionable.services/article/testing-http-handlers-go/

// TestIndex tests if the HTTP request returns a JSON object.
func TestIndex(t *testing.T) {

	// prepare lgResuStatus message
	lgResu := &rs.LgResuStatus{Soc: 78, Soh: 99, Voltage: 54.55, Current: -1, Temp: 26.1}

	// channel to signal request from Index
	httpSigChan := make(chan bool)
	// channel to send data to Index
	recordHttpChan := make(chan rs.LgResuStatus)

	// simulate BrokerRecord, issue exactly one lgResu object
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
		t.Errorf("Index() handler returned wrong status code %v, expect %v \n",
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
		t.Errorf("Index handler() returned Soc value %v, expect Soc value %v \n",
			status.Soc, lgResu.Soc)
	}
}

// TestWriteRecord tests if a request for an lgResu object is made after 1 second.
func TestWriteRecord(t *testing.T) {

	// prepare lgResuStatus message
	lgResu := &rs.LgResuStatus{Soc: 78, Soh: 99, Voltage: 54.55, Current: -1, Temp: 26.1}

	// channel to signal request from WriteRecord to BrokerRecord
	writeSigChan := make(chan bool)

	// channel to receive data from BrokerRecord to WriteRecord
	recordWriteChan := make(chan rs.LgResuStatus)

	// simulate BrokerRecord
	go func() {
		for {
			select {
			case <-writeSigChan: // wait for 'HTTP request pending' signal
				recordWriteChan <- *lgResu // send lrResu object
			}
		}
	}()

	dataRecorder := &MockDatarecorder{}

	// write record to datafile
	go writeRecord(writeSigChan, recordWriteChan, dataRecorder, 1)

	time.Sleep(3 * time.Second)

	t.Logf("dataRecorder invocations: %d \n", dataRecorder.Cnt)

	if dataRecorder.Cnt != 2 {
		t.Errorf("writeRecorder() requested %d lgResu objects, expect %d requests \n",
			dataRecorder.Cnt, 2)
	}
}

// TestSendKeepAlive tests the periodical generation of keep alive messages.
func TestSendKeepAlive(t *testing.T) {

	// channel to terminate sendKeepAlive goroutine
	termSigChan := make(chan bool)

	_ = termSigChan

	canbus := &MockCanbus{}

	// send keep-alive message to LG Resu 10
	go sendKeepAlive(termSigChan, canbus, 1)

	time.Sleep(3 * time.Second)

	if canbus.PublishCnt != 2 {
		t.Errorf("sendKeepAlive() generated %d keep alive messages, expect %d messages \n",
			canbus.PublishCnt, 2)
	}

	// stop the sendKeepAlive goroutine, there should be no additional keep alive messages
	termSigChan <- true

	time.Sleep(3 * time.Second)

	if canbus.PublishCnt != 2 {
		t.Errorf("sendKeepAlive() generated %d keep alive messages, expect %d messages \n",
			canbus.PublishCnt, 2)
	}
}

// TestDecodeCanFrame tests decoding of a CanBus frame in a closure.
func TestDecodeCanFrame(t *testing.T) {

	recordEmitChan := make(chan rs.LgResuStatus)

	decoder := decodeCanFrame(recordEmitChan)

	frm := can.Frame{
		ID:     rs.BMS_SOC_SOH,
		Length: 8,
		Flags:  0,
		Res0:   0,
		Res1:   0,
		Data:   [8]byte{0x4d, 0x00, 0x63, 0x00, 0x00, 0x00, 0x00, 0x00},
	}

	// simulate the canbus goroutine, executing decoder() as a normal function will cause a deadlock
	go decoder(frm)

	lgResu := rs.LgResuStatus{}
	lgResu = <-recordEmitChan
	t.Logf("lgResu.Soc = %d\n", lgResu.Soc)
	if lgResu.Soc != 77 {
		t.Errorf("decodeCanFrame() produce Soc = %d, expect Soc = 77 \n",
			lgResu.Soc)
	}
}

//
func TestBrokerRecord(t *testing.T) {

	// channel to signal request from WriteRecord to BrokerRecord
	writeSigChan := make(chan bool)

	// channel to signal request from Index to BrokerRecord
	httpSigChan := make(chan bool)

	// channel to receive data from BrokerRecord to WriteRecord
	recordWriteChan := make(chan rs.LgResuStatus)

	// channel to receive data from BrokerRecord to Index
	recordHttpChan := make(chan rs.LgResuStatus)

	recordEmitChan := make(chan rs.LgResuStatus)

	go brokerRecord(recordEmitChan, writeSigChan, recordWriteChan, httpSigChan, recordHttpChan)

	// prepare lgResuStatus message
	lgResu := &rs.LgResuStatus{Soc: 78, Soh: 99, Voltage: 54.55, Current: -1, Temp: 26.1}

	recordEmitChan <- *lgResu

	// send write request signal
	writeSigChan <- true
	// receive lgResu
	lgResuWrite := <-recordWriteChan

	// send http request signal
	httpSigChan <- true
	// receive lgResu
	lgResuHttp := <-recordHttpChan

	if lgResuWrite.Soc != 78 || lgResuHttp.Soc != 78 {
		t.Errorf("brokerRecord() produce Soc = %d, expect Soc = 78 \n",
			lgResuWrite.Soc)
	}
}
