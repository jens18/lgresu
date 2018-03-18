package main

import (
	"flag"
	"fmt"
	"github.com/brutella/can"
	rs "github.com/jens18/lgresu/lgresustatus"
	"log"
	"net"
	"os"
	"os/signal"
	"time"
)

// LG Resu 10 LV CANBus test messages
var CanbusTestMessages = []struct {
	Identifier uint32
	Data       [8]byte
}{
	// volt/amp/temp (LG Resu -> Inverter):
	{
		Identifier: rs.BMS_VOLT_AMP_TEMP,
		Data:       [8]byte{0x4b, 0x15, 0xed, 0xff, 0xba, 0x00, 0x00, 0x00},
	},
	// ? (LG Resu -> Inverter): unknown message type (appears to be a constant)
	{
		Identifier: rs.BMS_SERIAL_NUM,
		Data:       [8]byte{0x04, 0xc0, 0x00, 0x1f, 0x03, 0x00, 0x00, 0x00},
	},
	// configuration parameters (LG Resu -> Inverter):
	{
		Identifier: rs.BMS_LIMITS,
		Data:       [8]byte{0x41, 0x02, 0x96, 0x03, 0x96, 0x03, 0x00, 0x00},
	},
	// state of charge/health (LG Resu -> Inverter):
	{
		Identifier: rs.BMS_SOC_SOH,
		Data:       [8]byte{0x4d, 0x00, 0x63, 0x00, 0x00, 0x00, 0x00, 0x00},
	},
	// warnings/alarms (LG Resu -> Inverter):
	{
		Identifier: rs.BMS_WARN_ALARM,
		Data:       [8]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	},
}

// default value is the virtual CANBus interface: vcan0
var i = flag.String("if", "vcan0", "network interface name")

func main() {

	fmt.Printf("lgresu_sim:\n")

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

	f := can.Frame{}

	for {
		// send all LG Resu 10 test messages in one block
		for _, tm := range CanbusTestMessages {

			f.ID = tm.Identifier
			f.Length = uint8(len(tm.Data))
			f.Data = tm.Data

			bus.Publish(f)

			fmt.Printf("%#4x # % -24X \n", tm.Identifier, tm.Data)
		}

		// wait for 1 second
		<-time.After(time.Second * 1)
	}
}
