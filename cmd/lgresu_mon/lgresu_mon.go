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
)

var (
	version = "undefined"
)

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
	go sendKeepAlive(termSigChan, bus, keepAliveInterval)

	// respond to record requests and receive new records
	go brokerRecord(recordEmitChan, writeSigChan, recordWriteChan, httpSigChan, recordHttpChan)

	// receive update messages from LG Resu 10
	bus.SubscribeFunc(decodeCanFrame(recordEmitChan))
	go bus.ConnectAndPublish()

	dr := dr.NewDatarecorder(*dataDirRoot, ".csv", *retentionPeriod, rs.CsvRecordHeader())

	// write record to datafile (60 second recordingFrequency)
	go writeRecord(writeSigChan, recordWriteChan, dr, 60)

	router := mux.NewRouter().StrictSlash(true)

	router.PathPrefix("/data").Handler(http.StripPrefix("/data", http.FileServer(http.Dir("data/"))))
	router.HandleFunc("/", Index(httpSigChan, recordHttpChan))

	log.Fatal(http.ListenAndServe(":"+*port, router))
}
