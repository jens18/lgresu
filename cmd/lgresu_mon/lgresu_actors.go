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

//+build test

package main

import (
	"encoding/json"
	"fmt"
	"github.com/brutella/can"
	rs "github.com/jens18/lgresu/lgresustatus"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"time"
)

type DatarecorderIf interface {
	WriteToDatafile(time.Time, string)
}

const (
	keepAliveInterval int = 20
)

type CanbusIf interface {
	Publish(can.Frame) error
	Disconnect() error
}

// terminateMonitor receive operating system signal messages (SIGTERM, SIGKILL) via osSigChan and
// issue a message via the termSigChan before disconnecting from the CANBus and terminating the server.
func terminateMonitor(osSigChan <-chan os.Signal, termSigChan chan<- bool, bus CanbusIf) {

	select {
	case <-osSigChan:
		fmt.Printf("main: received SIGKILL/SIGTERM\n")
		// terminate sendKeepAlive goroutine
		termSigChan <- true
		bus.Disconnect()
		time.Sleep(time.Second * 1)
		os.Exit(1)
	}
}

// decodeCanFrame returns a function that implements the can.Handler interface.
func decodeCanFrame(recordEmitChan chan<- rs.LgResuStatus) func(can.Frame) {
	// https://www.calhoun.io/5-useful-ways-to-use-closures-in-go/

	// lgResu holds the current state of metrics from the LG Resu 10.
	// Every update message received will update only parts of LgResuStatus (ie. Soc + Soh but not Voltage).
	// Only the implementation  of the can.Handler interface has access to
	// lgResu.
	lgResu := &rs.LgResuStatus{}

	return func(frm can.Frame) {
		lgResu.DecodeLgResuCanbusMessage(frm.ID, frm.Data[:])

		// send the latest lgResu status update (and block (briefly until BrokerRecord has read lgResu))
		recordEmitChan <- *lgResu
	}
}

// writeRecord writes a new LgResuStatus record to a CSV datafile every minute.
func writeRecord(writeSigChan chan<- bool,
	recordWriteChan <-chan rs.LgResuStatus,
	dataRecorder DatarecorderIf,
	recordingFrequency int) {

	//	dr := dr.NewDatarecorder(dataDirRoot, ".csv", retentionPeriod, lgResu.CsvRecordHeader())

	for {
		select {
		case <-time.After(time.Duration(recordingFrequency) * time.Second):
			writeSigChan <- true
			lgResu := <-recordWriteChan

			// convert lgResu to CSV record with timestamp
			now := time.Now()

			lgResuCsvRecord := lgResu.CsvRecord(now)

			log.Infof("WriteRecord: %s ", lgResuCsvRecord)

			dataRecorder.WriteToDatafile(now, lgResuCsvRecord)
		}
	}

}

// brokerRecord receives LgResuStatus objects at a higher frequency (approx. once per second)
// and responds to lower frequency requests to either persist the LgResuStatus object (writeSigChan to
// signal a request, recordWriteChan to send the LgResuStatus object) or to respond to a pending
// HTTP request (httpSignChan to signal a request, httpWriteChan to send the LgResuStatus object).
func brokerRecord(recordEmitChan <-chan rs.LgResuStatus,
	writeSigChan <-chan bool, recordWriteChan chan<- rs.LgResuStatus,
	httpSigChan <-chan bool, httpWriteChan chan<- rs.LgResuStatus) {

	lgResu := rs.LgResuStatus{}

	for {
		select {
		case lgResu = <-recordEmitChan:
			log.Debugf("BrokerRecord(): received %v\n", lgResu)
		case <-writeSigChan:
			log.Debugf("BrokerRecord(): received writeSigChan\n")
			recordWriteChan <- lgResu
		case <-httpSigChan:
			log.Debugf("BrokerRecord(): received httpSigChan\n")
			httpWriteChan <- lgResu
		}
	}
}

// sendKeepAlive send keep-alive messages in KeepAliveInterval second intervals until it receives termination message.
func sendKeepAlive(c <-chan bool, bus CanbusIf, interval int) {
	lgResu := &rs.LgResuStatus{}
	frm := can.Frame{}

	for {
		select {
		case <-c:
			log.Debugf("sendKeepAlive: received termination message\n")
			return
		case <-time.After(time.Duration(interval) * time.Second):
			log.Infof("sendKeepAlive: %d sec time out, sending keep-alive\n", interval)

			id, data := lgResu.CreateKeepAliveMessage()

			log.Debugf("sendKeepAlive: %#4x # % -24X \n", id, data)

			frm.ID = id
			frm.Length = uint8(len(data))
			// copy must be 'tricked' into treating the array as a slice
			copy(frm.Data[:], data[0:8])

			bus.Publish(frm)
		}
	}
}

// Index processes HTTP requests and generates a JSON response.
func Index(httpSigChan chan<- bool, recordHttpChan <-chan rs.LgResuStatus) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		var lgResu rs.LgResuStatus

		// signal a request has arrived (and block)
		httpSigChan <- true

		// receive the latest lgResu status update (and block)
		lgResu = <-recordHttpChan
		log.Infof("Index: lgResu = %+v \n", lgResu)

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(lgResu)
	}
}
