package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/brutella/can"
	"github.com/gorilla/mux"
	rs "github.com/jens18/lgresu/lgresustatus"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
)

const (
	KeepAliveInterval time.Duration = 20
)

// decodeCanFrame returns a function that implements the can.Handler interface.
func decodeCanFrame(chRs chan<- rs.LgResuStatus, sig <-chan bool) func(can.Frame) {
	// https://www.calhoun.io/5-useful-ways-to-use-closures-in-go/

	// lgResu holds the current state of metrics from the LG Resu 10.
	// Every update message received will update only parts of LgResuStatus (ie. Soc + Soh but not Voltage).
	// Only the implementation  of the can.Handler interface has access to
	// lgResu.
	lgResu := &rs.LgResuStatus{}

	return func(frm can.Frame) {
		lgResu.DecodeLgResuCanbusMessage(frm.ID, frm.Data[:])

		// check if a HTTP request is pending
		select {
		case <-sig:
			log.Debugf("decodeCanFrame: HTTP request pending.\n")
			log.Debugf("decodeCanFrame: lgResu = %+v \n", *lgResu)

			// send the latest lgResu status update (and block)
			chRs <- *lgResu
		case <-time.After(10 * time.Millisecond):
			log.Debugf("decodeCanFrame: timeout 10[ms] reached.\n")
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
		case <-time.After(KeepAliveInterval * time.Second):
			log.Infof("sendKeepAlive: %d sec time out, sending keep-alive\n", KeepAliveInterval)

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

func Index(chRs <-chan rs.LgResuStatus, sig chan<- bool) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		var lgResu rs.LgResuStatus

		// signal a request has arrived (and block)
		sig <- true

		// receive the latest lgResu status update (and block)
		lgResu = <-chRs
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

	flag.Parse()

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

	// only log warning severity or above.

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
	c := make(chan os.Signal)
	// channel to terminate sendKeepAlive goroutine
	t := make(chan bool)
	// channel
	chRs := make(chan rs.LgResuStatus)
	sig := make(chan bool)

	signal.Notify(c, os.Interrupt)
	signal.Notify(c, os.Kill)

	go func() {
		select {
		case <-c:
			fmt.Printf("main: received SIGKILL/SIGTERM\n")
			// terminate sendKeepAlive goroutine
			t <- true
			bus.Disconnect()
			time.Sleep(time.Second * 1)
			os.Exit(1)
		}
	}()

	// send keep-alive message to LG Resu 10
	go sendKeepAlive(t, bus)

	// receive update messages from LG Resu 10
	bus.SubscribeFunc(decodeCanFrame(chRs, sig))
	go bus.ConnectAndPublish()

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", Index(chRs, sig))
	log.Fatal(http.ListenAndServe(":9090", router))
}
