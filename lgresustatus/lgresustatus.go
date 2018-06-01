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

// Package lgresustatus provides routines to decode LG Resu 10 LV
// CANBus messages and generate the 'keep alive' message (typically
// send from an CANBus enabled device (for example the Schneider Conext
// Bridge or directly from an inverter) to the LG Resu 10 LV. The decoded
// result can be converted into a JSON message.
//
// Note:
//
// Not all messages can currently be decoded. Support for
// alarm bits (message id 0x359) and CANBus message id 0x354 is missing.
//
// CANBus BMS Message format specifications:
//
// 1) Lithiumate BMS CANBus message format specification:
//
// http://lithiumate.elithion.com/php/controller_can_specs.php
//
// 2) LG Resu 10 LV CANBus message format specification:
//
// https://www.photovoltaikforum.com/speichersysteme-ongrid-netzparallel-f137/reverse-engineering-bms-von-lg-chem-resu-6-4ex-ned-t108629-s10.html
//
package lgresustatus

import (
	"encoding/binary"
	log "github.com/sirupsen/logrus"
)

// Definition of the LG Resu 10 CANBus message id's
const (
	INV_KEEP_ALIVE    uint32 = 0x305
	BMS_LIMITS        uint32 = 0x351
	BMS_SERIAL_NUM    uint32 = 0x354
	BMS_SOC_SOH       uint32 = 0x355
	BMS_VOLT_AMP_TEMP uint32 = 0x356
	BMS_WARN_ALARM    uint32 = 0x359
)

// Github triggers update of godoc documentation.
type Github int

// BitValue contains a single warning/alarm bit mask and definition.
type BitValue struct {
	Description string
	Value       uint16
}

// WarningBitValues defines 16 warning bits.
//
// Raw CANBus message format:
//
// 00000359 8 ww WW aa AA 00 00 00 00
//
//  ww0 WRN_ONLY_SUB_RELAY_COMMAND
//  ww1 BATTERY_HIGH_VOLTAGE
//  ww2 BATTERY_LOW_VOLTAGE
//  ww3 BATTERY_HIGH_TEMP
//  ww4 BATTERY_LOW_TEMP
//  ww5 UNKNOWN
//  ww6 UNKNOWN
//  ww7 BATTERY_HIGH_CURRENT_DISCHARGE
//  WW0 BATTERY_HIGH_CURRENT_CHARGE
//  WW1 UNKNOWN
//  WW2 UNKNOWN
//  WW3 BMS_INTERNAL
//  WW4 CELL_IMBALANCE
//  WW5 ALARM_SUB_PACK2_ERROR
//  WW6 ALARM_SUB_PACK1_ERROR
//  WW7 UNKNOWN
//
// Note:
// Bitmasks are applied after converting the littleEndian representation
// of the first 2 bytes to the bigEndian representation.
var WarningBitValues = []BitValue{
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

// AlarmBitValues defines 16 alarm bits (currently unknown).
//
// Raw CANBus message format:
//
// 00000359 8 ww WW aa AA 00 00 00 00
//
//  aa0-7 UNKNOWN
//  AA0-7 UNKNOWN
//
var AlarmBitValues = []BitValue{
	{"UNKNOWN_ALARM", 0xffff},
}

// LgResuStatus contains metrics send by the LG Resu 10 LV.
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
	// Battery voltage limit (LG Resu 10 LV is a 14S battery, indiv. cell voltage allowed is
	// 4.12V -> 14 * 4.12V = 57.7V)
	MaxVoltage float32 `json:"maxVoltage"`
	// Maximal battery charge current (LG Resu 10 LV is a C = 189Ah battery, C/2 is approx. 90A)
	MaxChargeCurrent float32 `json:"maxChargeCurrent"`
	// Maximal battery discharge current (LG Resu 10 is a C = 189Ah battery, C/2 is approx. 90A)
	MaxDischargeCurrent float32  `json:"maxDischargeCurrent"`
	Warnings            []string `json:"warnings"`
	Alarms              []string `json:"alarms"`
}

// DecodeLgResuCanbusMessage decodes messages send by the LG Resu 10 LV BMS and updates lgResu with new metric values.
func (lgResu *LgResuStatus) DecodeLgResuCanbusMessage(id uint32, s []byte) {

	log.Debugf("%-4x % -24X\n", id, s)

	switch id {
	case BMS_VOLT_AMP_TEMP:
		log.Debugf("BMS: volt/amp/temp (%#04x)\n", BMS_VOLT_AMP_TEMP)

		// unsigned: voltage is always positive
		data := binary.LittleEndian.Uint16(s[0:2])
		lgResu.Voltage = float32(data) / 100
		log.Debugf("voltage = %.2f [VDC]\n", lgResu.Voltage)

		// signed: - battery is discharged, + battery is charged
		data = binary.LittleEndian.Uint16(s[2:4])
		lgResu.Current = float32(int16(data)) / 10
		log.Debugf("current = %.2f [ADC]\n", lgResu.Current)

		// signed: temperature in Celsius
		data = binary.LittleEndian.Uint16(s[4:6])
		lgResu.Temp = float32(data) / 10
		log.Debugf("temperature = %.1f [Celsius]\n\n", float32(data)/10)

	case BMS_SOC_SOH:
		log.Debugf("BMS: state of charge/health (%#04x):\n", BMS_SOC_SOH)

		data := binary.LittleEndian.Uint16(s[0:2])
		lgResu.Soc = data
		log.Debugf("soc = %d %%\n", lgResu.Soc)

		data = binary.LittleEndian.Uint16(s[2:4])
		lgResu.Soh = data
		log.Debugf("soh = %d %%\n\n", lgResu.Soh)

	case BMS_LIMITS:
		log.Debugf("BMS: configuration parameters (%#04x):\n", BMS_LIMITS)

		// unsigned: voltage is always positive
		data := binary.LittleEndian.Uint16(s[0:2])
		lgResu.MaxVoltage = float32(data) / 10
		log.Debugf("max voltage = %.2f [VDC]\n", lgResu.MaxVoltage)

		// unsigned: ADC
		data = binary.LittleEndian.Uint16(s[2:4])
		lgResu.MaxChargeCurrent = float32(data) / 10
		log.Debugf("max charge current = %.2f [ADC]\n", lgResu.MaxChargeCurrent)

		// unsigned: ADC
		data = binary.LittleEndian.Uint16(s[4:6])
		lgResu.MaxDischargeCurrent = float32(data) / 10
		log.Debugf("max discharge current = %.2f [ADC]\n\n", lgResu.MaxDischargeCurrent)

	case BMS_SERIAL_NUM:
		log.Debugf("BMS: serial number (?) (%#04x):\n\n", BMS_SERIAL_NUM)

	case INV_KEEP_ALIVE:
		log.Debugf("INV: keep alive (%#04x):\n\n", INV_KEEP_ALIVE)

	case BMS_WARN_ALARM:
		log.Debugf("BMS: warnings/alarms (%#04x):\n\n", BMS_WARN_ALARM)

		// decode warnings
		data := binary.LittleEndian.Uint16(s[0:2])
		for _, bv := range WarningBitValues {
			if data&bv.Value != 0 {
				lgResu.Warnings = append(lgResu.Warnings, bv.Description)
			}
		}

		// decode alarms
		data = binary.LittleEndian.Uint16(s[2:4])
		for _, bv := range AlarmBitValues {
			if data&bv.Value != 0 {
				lgResu.Alarms = append(lgResu.Alarms, bv.Description)
			}
		}

	}
}

// CreateKeepAliveMessage creates one 'keep alive' message (to be send to the LG Resu 10 LV).
func (lgResu *LgResuStatus) CreateKeepAliveMessage() (id uint32, s []byte) {
	id = INV_KEEP_ALIVE
	s = []byte{0x00, 0x0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	return id, s
}
