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

// Package datafile contains functions to write metrics to a datafile.
//
// Datafiles can be of any format (JSON, CSV, ...). Filenames and directory
// name reflect the date when the file was created and have the format:
//
//     <RootPath>/YYYY/MM/YYYYMMDD<Extension>
//
// RootPath and Extension are define in the constructor for datafile.
// The first line of the datafiles contains a header describing the data
// columns.
//
// Files older than RetentionPeriod days are automatically deleted to
// maintain a constant number of files.
//
// Example:
//
// Metrics from 01/08/2006 would have been written to a CSV file
// with the filename: 20060108.csv
//
//     time,soc,soh,current,voltage,temp
//     2006/01/02 15:04:05,97,99,-3.2,57.43,21.3
//
// Directory hierarchy with RetentionPeriod set to 7 days:
//
//     data
//     └── 2006
//         └── 01
//             ├── 20060102.csv
//             ├── 20060103.csv
//             ├── 20060104.csv
//             ├── 20060105.csv
//             ├── 20060106.csv
//             ├── 20060107.csv
//             └── 20060108.csv
//
//
package datarecorder

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Datafile contains configuration data for datafile management (RootPath, Extension, RetentionPeriod)
// and state information (FileName, FileDesc).
type Datarecorder struct {
	RootPath        string
	Extension       string
	RetentionPeriod int
	Header          string
	FileName        string
	FileDesc        *os.File
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

// exists reports whether the named file or directory exists.
func exists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// deleteExpiredFiles removes all files that are older than the retentionPeriod days.
func deleteExpiredFiles(currentTime time.Time, rootDir string, retentionPeriod int) ([]string, error) {

	fileList := make([]string, 0)

	// define datafile cutoff date

	// substract retentionPeriod days from currentTime
	cutoff := currentTime.AddDate(0, 0, -(retentionPeriod - 1))

	log.Debugf("deleteExpiredFiles: cutoff = %v, currentTime = %v \n", cutoff, currentTime)

	err := filepath.Walk(rootDir, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			// find files that are older than retentionPeriod days
			// decode filename and compare with current timestamp

			// split filename into basename and extension
			log.Debugf("deleteExpiredFiles: datafile %s \n", f.Name())

			str := strings.Split(f.Name(), ".")

			// basename without extension
			basename := str[0]

			// parse date string
			layout := "20060102"
			datafileTime, err := time.Parse(layout, basename)
			check(err)

			if cutoff.After(datafileTime) {
				// add file to list of files to be deleted
				log.Debugf("deleteExpiredFiles: to be deleted datafile %s \n", f.Name())
				fileList = append(fileList, path)
			}
		}
		return err
	})
	check(err)

	for _, file := range fileList {

		log.Infof("deleteExpiredFiles: finally deleting datafile %s \n", file)
		// delete file
		err := os.Remove(file)
		check(err)
	}

	return fileList, nil
}

// NewDatarecorder is the constructor for Datarecorder.
func NewDatarecorder(rootPath string, extension string, retentionPeriod int, header string) *Datarecorder {
	dr := &Datarecorder{rootPath, extension, retentionPeriod, header, "", nil}
	return dr
}

// WriteToDatarecorder writes a new record to a CSV datafile.
func (dr *Datarecorder) WriteToDatafile(currentTime time.Time, record string) {

	var err error

	// extract year, month, day strings from date
	year := strconv.Itoa(currentTime.Year())
	month := fmt.Sprintf("%02d", currentTime.Month()) // padding with 0's
	day := fmt.Sprintf("%02d", currentTime.Day())     // padding with 0's

	// construct filename
	fileName := year + month + day + dr.Extension

	// test if file already exists
	if fileName != dr.FileName {
		// platform independent path name concatenation
		folderPath := filepath.Join(dr.RootPath, year, month)

		// create new directory if it does not already exist
		os.MkdirAll(folderPath, 0755)

		// construct absolute file path
		filePath := filepath.Join(folderPath, fileName)
		log.Debugf("write to new datafile at: %s \n", filePath)

		// check if file has previously been created
		fileExists := exists(filePath)

		// update Datarecorder struct
		dr.FileName = fileName

		// close previous file descriptor
		if dr.FileDesc != nil {
			dr.FileDesc.Close()
		}

		// create new file or append to existing file
		dr.FileDesc, err = os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		check(err)

		// write header information only if the file has been newly created
		if !fileExists {
			_, err = dr.FileDesc.WriteString(dr.Header)
			check(err)
		}

		// check if aged out files need to be deleted
		_, err = deleteExpiredFiles(currentTime, dr.RootPath, dr.RetentionPeriod)
		check(err)
	}

	_, err = dr.FileDesc.WriteString(record)
	check(err)
}
