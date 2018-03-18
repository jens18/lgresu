package main

import (
	"flag"
	"fmt"
	"github.com/brutella/can"
	rs "github.com/jens18/lgresu/lgresustatus"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"os/signal"
	"time"
)

// default value is the virtual CANBus interface: vcan0
var i = flag.String("if", "vcan0", "network interface name")

var chRs = make(chan rs.LgResuStatus)

// the following variable should only be used by decodeCanFrame (should be address with a closure)
var lgResu rs.LgResuStatus

func decodeCanFrame(frm can.Frame) {
	lgResu.DecodeLgResuCanbusMessage(frm.ID, frm.Data[:])
	//log.Infof("lgResu.DecodeLgResuCanbusMessage(%#4x, % -24x) == %+v", frm.ID, frm.Data, lgResu)
	chRs <- lgResu
}

func main() {
	log.Infof("lgresu_mon:\n")

	// only log warning severity or above.
	log.SetLevel(log.WarnLevel)

	flag.Parse()
	if len(*i) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	iface, err := net.InterfaceByName(*i)

	if err != nil {
		log.Fatalf("lgresu_sim: Could not find network interface %s (%v)", i, err)
	}

	// bind to socket
	conn, err := can.NewReadWriteCloserForInterface(iface)

	if err != nil {
		log.Fatal(err)
	}

	bus := can.NewBus(conn)

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, os.Kill)

	go func() {
		select {
		case <-c:
			bus.Disconnect()
			os.Exit(1)
		}
	}()

	bus.SubscribeFunc(decodeCanFrame)

	go bus.ConnectAndPublish()

	for {
		select {
		case rs := <-chRs:
			fmt.Printf("lgResu = %+v \n", rs)
		}
	}

	// stop after 30 second (without it the program would stop immediately!)
	<-time.After(time.Second * 30)
}
