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
	"flag"
	"fmt"
	"github.com/brutella/can"
	"github.com/gorilla/mux"
	dr "github.com/jens18/lgresu/datarecorder"
	rs "github.com/jens18/lgresu/lgresustatus"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"
)

const (
	keepAliveInterval time.Duration = 20
)

var (
	version = "undefined"
)

// terminateMonitor receive operating system signal messages (SIGTERM, SIGKILL) via osSigChan and
// issue a message via the termSigChan before disconnecting from the CANBus and terminating the server.
func terminateMonitor(osSigChan <-chan os.Signal, termSigChan chan<- bool, bus *can.Bus) {

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
	dataDirRoot string,
	retentionPeriod int) {

	dr := dr.NewDatarecorder(dataDirRoot, ".csv", retentionPeriod, "Time,Soc,Voltage,Current\n")

	for {
		select {
		case <-time.After(60 * time.Second):
			writeSigChan <- true
			lgResu := <-recordWriteChan

			// convert lgResu to CSV record with timestamp
			now := time.Now()

			lgResuCsvRecord := now.Format("2006/01/02 15:04:05") + "," +
				strconv.Itoa(int(lgResu.Soc)) + "," +
				strconv.FormatFloat(float64(lgResu.Voltage), 'f', 2, 32) + "," +
				strconv.FormatFloat(float64(lgResu.Current), 'f', 2, 32) + "\n"

			log.Infof("WriteRecord: %s ", lgResuCsvRecord)

			dr.WriteToDatafile(now, lgResuCsvRecord)
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
func sendKeepAlive(c <-chan bool, bus *can.Bus) {
	lgResu := &rs.LgResuStatus{}
	frm := can.Frame{}

	for {
		select {
		case <-c:
			log.Debugf("sendKeepAlive: received termination message\n")
			return
		case <-time.After(keepAliveInterval * time.Second):
			log.Infof("sendKeepAlive: %d sec time out, sending keep-alive\n", keepAliveInterval)

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

func main() {
	log.Infof("lgresu_mon:\n")

	// default value is the virtual CANBus interface: vcan0
	i := flag.String("if", "vcan0", "network interface name")
	logLevel := flag.String("d", "info", "log level: debug, info, warn, error")
	port := flag.String("p", "9090", "port number")
	dataDirRoot := flag.String("dr", "/opt/lgresu", "root directory for metric datafiles")
	retentionPeriod := flag.Int("r", 7, "metric datafile retention period in days")
	v := flag.Bool("v", false, "version number")

	flag.Parse()

	if *v == true {
		fmt.Printf("version number: %s \n", version)
		os.Exit(1)
	}

	if len(*i) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	switch *logLevel {
	case "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		flag.Usage()
		os.Exit(1)
	}

	iface, err := net.InterfaceByName(*i)

	if err != nil {
		log.Fatalf("lgresu_mon: Could not find network interface %s (%v)", *i, err)
	}

	// bind to socket
	conn, err := can.NewReadWriteCloserForInterface(iface)

	if err != nil {
		log.Fatal(err)
	}

	bus := can.NewBus(conn)

	// channel to receive os.Kill/SIGKILL(9) and os.Interrupt/SIGTERM(15) notifications
	osSigChan := make(chan os.Signal)
	// channel to terminate sendKeepAlive goroutine
	termSigChan := make(chan bool)

	// channel to signal request from WriteRecord to BrokerRecord
	writeSigChan := make(chan bool)

	// channel to signal request from Index to BrokerRecord
	httpSigChan := make(chan bool)

	// channel to receive data from BrokerRecord to WriteRecord
	recordWriteChan := make(chan rs.LgResuStatus)

	// channel to receive data from BrokerRecord to Index
	recordHttpChan := make(chan rs.LgResuStatus)

	recordEmitChan := make(chan rs.LgResuStatus)

	signal.Notify(osSigChan, os.Interrupt)
	signal.Notify(osSigChan, os.Kill)

	// terminate sendKeepAlive and CANBus
	go terminateMonitor(osSigChan, termSigChan, bus)

	// send keep-alive message to LG Resu 10
	go sendKeepAlive(termSigChan, bus)

	// respond to record requests and receive new records
	go brokerRecord(recordEmitChan, writeSigChan, recordWriteChan, httpSigChan, recordHttpChan)

	// receive update messages from LG Resu 10
	bus.SubscribeFunc(decodeCanFrame(recordEmitChan))
	go bus.ConnectAndPublish()

	// write record to datafile
	go writeRecord(writeSigChan, recordWriteChan, *dataDirRoot, *retentionPeriod)

	router := mux.NewRouter().StrictSlash(true)

	router.PathPrefix("/data").Handler(http.StripPrefix("/data", http.FileServer(http.Dir("data/"))))
	router.HandleFunc("/", Index(httpSigChan, recordHttpChan))

	log.Fatal(http.ListenAndServe(":"+*port, router))
}
