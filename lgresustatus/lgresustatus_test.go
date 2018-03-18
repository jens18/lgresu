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

package lgresustatus

import (
	"encoding/json"
	"github.com/google/go-cmp/cmp"
	log "github.com/sirupsen/logrus"
	"testing"
)

// LG Resu 10 LV CANBus test messages
var CanbusTestMessages = []struct {
	Identifier uint32
	Data       [8]byte
	Expect     LgResuStatus
}{
	// volt/amp/temp (LG Resu -> Inverter):
	{
		Identifier: BMS_VOLT_AMP_TEMP,
		Data:       [8]byte{0x4b, 0x15, 0xed, 0xff, 0xba, 0x00, 0x00, 0x00},
		Expect:     LgResuStatus{Voltage: 54.51, Current: -1.9, Temp: 18.6},
	},
	// ? (LG Resu -> Inverter): unknown message type (appears to be a constant)
	{
		Identifier: BMS_SERIAL_NUM,
		Data:       [8]byte{0x04, 0xc0, 0x00, 0x1f, 0x03, 0x00, 0x00, 0x00},
		Expect:     LgResuStatus{Voltage: 54.51, Current: -1.9, Temp: 18.6},
	},
	// configuration parameters (LG Resu -> Inverter):
	{
		Identifier: BMS_LIMITS,
		Data:       [8]byte{0x41, 0x02, 0x96, 0x03, 0x96, 0x03, 0x00, 0x00},
		Expect: LgResuStatus{Voltage: 54.51, Current: -1.9, Temp: 18.6,
			MaxVoltage: 57.70, MaxChargeCurrent: 91.80, MaxDischargeCurrent: 91.80},
	},
	// state of charge/health (LG Resu -> Inverter):
	{
		Identifier: BMS_SOC_SOH,
		Data:       [8]byte{0x4d, 0x00, 0x63, 0x00, 0x00, 0x00, 0x00, 0x00},
		Expect: LgResuStatus{Voltage: 54.51, Current: -1.9, Temp: 18.6,
			MaxVoltage: 57.70, MaxChargeCurrent: 91.80, MaxDischargeCurrent: 91.80,
			Soc: 77, Soh: 99},
	},
	// warnings/alarms (LG Resu -> Inverter):
	{
		Identifier: BMS_WARN_ALARM,
		Data:       [8]byte{0xff, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00, 0x00},
		Expect: LgResuStatus{Voltage: 54.51, Current: -1.9, Temp: 18.6,
			MaxVoltage: 57.70, MaxChargeCurrent: 91.80, MaxDischargeCurrent: 91.80,
			Soc: 77, Soh: 99,
			Warnings: []string{"WRN_ONLY_SUB_RELAY_COMMAND", "BATTERY_HIGH_VOLTAGE", "BATTERY_LOW_VOLTAGE",
				"BATTERY_HIGH_TEMP", "BATTERY_LOW_TEMP", "UNKNOWN_ww5", "UNKNOWN_ww6", "BATTERY_HIGH_CURRENT_DISCHARGE",
				"BATTERY_HIGH_CURRENT_CHARGE", "UNKNOWN_WW1", "UNKNOWN_WW2", "BMS_INTERNAL", "CELL_IMBALANCE",
				"ALARM_SUB_PACK2_ERROR", "ALARM_SUB_PACK1_ERROR", "UNKNOWN_WW7"},
			Alarms: []string{"UNKNOWN_ALARM"},
		},
	},
}

var JsonExpectMessage string = `{"soc":77,"soh":99,"voltage":54.51,"current":-1.9,"temp":18.6,"maxVoltage":57.7,"maxChargeCurrent":91.8,"maxDischargeCurrent":91.8,"warnings":["WRN_ONLY_SUB_RELAY_COMMAND","BATTERY_HIGH_VOLTAGE","BATTERY_LOW_VOLTAGE","BATTERY_HIGH_TEMP","BATTERY_LOW_TEMP","UNKNOWN_ww5","UNKNOWN_ww6","BATTERY_HIGH_CURRENT_DISCHARGE","BATTERY_HIGH_CURRENT_CHARGE","UNKNOWN_WW1","UNKNOWN_WW2","BMS_INTERNAL","CELL_IMBALANCE","ALARM_SUB_PACK2_ERROR","ALARM_SUB_PACK1_ERROR","UNKNOWN_WW7"],"alarms":["UNKNOWN_ALARM"]}`

func init() {
	// only log warning severity or above.
	log.SetLevel(log.WarnLevel)
}

func TestDecodeLgResuCanbusMessageToUpdateLgResuStatus(t *testing.T) {

	lgResu := &LgResuStatus{}

	// process all test messages
	for _, tm := range CanbusTestMessages {
		lgResu.DecodeLgResuCanbusMessage(tm.Identifier, tm.Data[:])
		if !cmp.Equal(*lgResu, tm.Expect) {
			t.Errorf("lgResu.DecodeLgResuCanbusMessage(%x, %+v) == %+v, expect %+v", tm.Identifier, tm.Data, *lgResu, tm.Expect)
		}
	}
}

func TestLgResuStatusConversionToJson(t *testing.T) {

	lgResu := &LgResuStatus{}

	// process all test messages
	for _, tm := range CanbusTestMessages {
		lgResu.DecodeLgResuCanbusMessage(tm.Identifier, tm.Data[:])
	}

	jsonMessage, err := json.Marshal(*lgResu)
	if err != nil {
		log.Fatalf("json.MarshalIndent failed with '%s'\n", err)
	}

	if string(jsonMessage) != JsonExpectMessage {
		t.Errorf("LgResuStatus in compact JSON == %s, expect %s\n", string(jsonMessage), JsonExpectMessage)
	}
}

func TestCreateKeepAliveMessage(t *testing.T) {
	lgResu := &LgResuStatus{}

	id, data := lgResu.CreateKeepAliveMessage()

	if (id != INV_KEEP_ALIVE) || (len(data) != 8) {
		t.Errorf("CreateKeepAliveMessage() returned id = %#04x, len(data) = %d, expect id = %#04x, len(data) = 8 \n",
			id, len(data), INV_KEEP_ALIVE)
	}
}
