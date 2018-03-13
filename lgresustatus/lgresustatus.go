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

// Package lgresu provides routines for decoding LG Resu 10 LV
// CANBus messages.
//
// NOTE: Not all messages can currently be decoded. Support for
// warnings and alarms is missing.

package lgresustatus

import (
	"encoding/binary"
	log "github.com/sirupsen/logrus"
)

type bitValue struct {
	description string
	value       uint16
}

// decoding for 16 warning bits is known
var warningBitValues = []bitValue{
	{"WRN_ONLY_SUB_RELAY_COMMAND", 0x0001},
	{"BATTERY_HIGH_VOLTAGE", 0x0002},
	{"BATTERY_LOW_VOLTAGE", 0x0004},
	{"BATTERY_HIGH_TEMP", 0x0008},
	{"BATTERY_LOW_TEMP", 0x0010},
	{"UNKNOWN_ww5", 0x0020},
	{"UNKNOWN_ww6", 0x0040},
	{"BATTERY_HIGH_CURRENT_DISCHARGE", 0x0080},
	{"BATTERY_HIGH_CURRENT_CHARGE", 0x0100},
	{"UNKNOWN_WW1", 0x0200},
	{"UNKNOWN_WW2", 0x0400},
	{"BMS_INTERNAL", 0x0800},
	{"CELL_IMBALANCE", 0x1000},
	{"ALARM_SUB_PACK2_ERROR", 0x2000},
	{"ALARM_SUB_PACK1_ERROR", 0x4000},
	{"UNKNOWN_WW7", 0x8000},
}

// decoding for 16 alarm bits is unknown
var alarmBitValues = []bitValue{
	{"UNKNOWN_ALARM", 0xffff},
}

// LGResuBmsStatus contains all of the metrics send by the LG Resu 10 LV
// that can currently be decoded
type LgResuStatus struct {
	// State Of Charge
	Soc uint16 `json:"soc"`
	// State Of Health
	Soh uint16 `json:"soh"`
	// Current battery voltage
	Voltage float32 `json:"voltage"`
	// Current battery current (positive value: battery charge current,
	// negative value: battery discharge current)
	Current float32 `json:"current"`
	// Battery temperature
	Temp float32 `json:"temp"`
	// Battery voltage limit (LG Resu 10 is a 14S battery, indiv. cell voltage allowed is
	// 4.12V -> 14 * 4.12V = 57.7V)
	MaxVoltage float32 `json:"maxVoltage"`
	// Maximal battery charge current (LG Resu 10 is a C = 189Ah battery, C/2 is approx. 90A)
	MaxChargeCurrent float32 `json:"maxChargeCurrent"`
	// Maximal battery discharge current (LG Resu 10 is a C = 189Ah battery, C/2 is approx. 90A)
	MaxDischargeCurrent float32 `json:"maxDischargeCurrent"`
	/*
	   00000359 8 ww WW aa AA 00 00 00 00

	   16 warning and alarm bits:

	   ww0 WRN_ONLY_SUB_RELAY_COMMAND
	   ww1 BATTERY_HIGH_VOLTAGE
	   ww2 BATTERY_LOW_VOLTAGE
	   ww3 BATTERY_HIGH_TEMP
	   ww4 BATTERY_LOW_TEMP
	   ww5 UNKNOWN
	   ww6 UNKNOWN
	   ww7 BATTERY_HIGH_CURRENT_DISCHARGE
	   WW0 BATTERY_HIGH_CURRENT_CHARGE
	   WW1 UNKNOWN
	   WW2 UNKNOWN
	   WW3 BMS_INTERNAL
	   WW4 CELL_IMBALANCE
	   WW5 ALARM_SUB_PACK2_ERROR
	   WW6 ALARM_SUB_PACK1_ERROR
	   WW7 UNKNOWN

	   Example:

	   LG Resu 10 low voltage situation (42VDC):

	   can0  359   [8]  00 00 00 08 00 00 00 00
	*/
	Warnings []string `json:"warnings"`
	Alarms   []string `json:"alarms"`
}

// lg resu 10 CANBus messages decoder updates LgResuStatus with the latest metric values.
func (lgResu *LgResuStatus) DecodeLgResuCanbusMessage(id uint32, s []byte) {

	log.Debugf("%-4x % -24X\n", id, s)

	switch id {
	case 0x356:
		log.Info("BMS: volt/amp/temp (0x356)\n")

		// unsigned: voltage is always positive
		data := binary.LittleEndian.Uint16(s[0:2])
		lgResu.Voltage = float32(data) / 100
		log.Infof("voltage = %.2f [VDC]\n", lgResu.Voltage)

		// signed: - battery is discharged, + battery is charged
		data = binary.LittleEndian.Uint16(s[2:4])
		lgResu.Current = float32(int16(data)) / 10
		log.Infof("current = %.2f [ADC]\n", lgResu.Current)

		// signed: temperature in Celsius
		data = binary.LittleEndian.Uint16(s[4:6])
		lgResu.Temp = float32(data) / 10
		log.Infof("temperature = %.1f [Celsius]\n\n", float32(data)/10)

	case 0x355:
		log.Infof("BMS: state of charge/health (0x355):\n")

		data := binary.LittleEndian.Uint16(s[0:2])
		lgResu.Soc = data
		log.Infof("soc = %d %%\n", lgResu.Soc)

		data = binary.LittleEndian.Uint16(s[2:4])
		lgResu.Soh = data
		log.Infof("soh = %d %%\n\n", lgResu.Soh)

	case 0x351:
		log.Infof("BMS: configuration parameters (0x351):\n")

		// unsigned: voltage is always positive
		data := binary.LittleEndian.Uint16(s[0:2])
		lgResu.MaxVoltage = float32(data) / 10
		log.Infof("max voltage = %.2f [VDC]\n", lgResu.MaxVoltage)

		// unsigned: ADC
		data = binary.LittleEndian.Uint16(s[2:4])
		lgResu.MaxChargeCurrent = float32(data) / 10
		log.Infof("max charge current = %.2f [ADC]\n", lgResu.MaxChargeCurrent)

		// unsigned: ADC
		data = binary.LittleEndian.Uint16(s[4:6])
		lgResu.MaxDischargeCurrent = float32(data) / 10
		log.Infof("max discharge current = %.2f [ADC]\n\n", lgResu.MaxDischargeCurrent)

	case 0x354:
		log.Infof("BMS: unknown (0x354):\n\n")

	case 0x305:
		log.Infof("INV: keep alive (0x305):\n\n")

	case 0x359:
		log.Infof("BMS: warnings/alarms (0x359):\n\n")

		// decode warnings
		data := binary.LittleEndian.Uint16(s[0:2])
		for _, bv := range warningBitValues {
			if data&bv.value != 0 {
				lgResu.Warnings = append(lgResu.Warnings, bv.description)
			}
		}

		// decode alarms
		data = binary.LittleEndian.Uint16(s[2:4])
		for _, bv := range alarmBitValues {
			if data&bv.value != 0 {
				lgResu.Alarms = append(lgResu.Alarms, bv.description)
			}
		}

	}
}
