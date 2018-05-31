package datarecorder

import (
	"bufio"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const (
	extension          string = ".csv"
	retentionPeriod    int    = 7
	header             string = "time,soc,soh,current,voltage,temp\n"
	sampleRecord       string = "2006/01/02 15:04:05,97,99,-3.2,57.43,21.3\n"
	sampleYear         string = "2006"
	sampleMonth        string = "01"
	sampleFileBasename string = "20060102"
)

var (
	rootDir     string
	absFileName string
	err         error
	recordTime  time.Time
)

func countLines(fileName string) int {
	file, _ := os.Open(fileName)
	fileScanner := bufio.NewScanner(file)
	lineCount := 0
	for fileScanner.Scan() {
		lineCount++
	}
	return lineCount
}

func countFiles(rootDir string) (int, error) {
	fileList := make([]string, 0)

	err := filepath.Walk(rootDir, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			fileList = append(fileList, path)
		}
		return err
	})

	return len(fileList), err
}

func init() {
	// only log warning severity or above.
	log.SetLevel(log.WarnLevel)
	//log.SetLevel(log.DebugLevel)
	// get current working directory
	workingDir, err := os.Getwd()
	check(err)

	rootDir = filepath.Join(workingDir, "data")

	absFileName = filepath.Join(rootDir, sampleYear, sampleMonth, sampleFileBasename+extension)

	// convert layout timestamp to time object
	layout := "20060102"
	recordTime, _ = time.Parse(layout, sampleFileBasename)
}

// TestDatarecorderWriteToDatafileSingleRecord writes a single records.
func TestDatarecorderWriteToDatafileSingleRecord(t *testing.T) {

	df := NewDatarecorder(rootDir, extension, retentionPeriod, header)

	// establish that there is no datafile
	_ = os.Remove(absFileName)

	// datafile does exist yet
	if exists(absFileName) {
		t.Errorf("expect to find no datafile at %s\n", absFileName)
	}

	df.WriteToDatafile(recordTime, sampleRecord)

	// check if datafile has been created
	if !exists(absFileName) {
		t.Errorf("expect to find new datafile at %s\n", absFileName)
	}

	// cleanup: remove single datafile
	err := os.Remove(absFileName)
	check(err)
}

// TestDatarecorderWriteToDatafileSingleRecord writes multiple records.
func TestDatarecorderWriteToDatafileMultipleRecord(t *testing.T) {

	maxRecords := 20

	df := NewDatarecorder(rootDir, extension, retentionPeriod, header)

	for i := 0; i < maxRecords; i++ {
		df.WriteToDatafile(recordTime, sampleRecord)
	}

	lines := countLines(absFileName)

	if lines != maxRecords+1 {
		t.Errorf("expect to find %d lines in datafile (1*header, 2*datarecords), found %d lines\n",
			maxRecords, lines)
	}

	// cleanup: remove single datafile
	err := os.Remove(absFileName)
	check(err)
}

// TestDatarecorderWriteToDatafileForRangeOfDays creates a range of datafiles
// and verifies that the total number of datafiles never exceeds retentionPeriodtests.
func TestDatarecorderWriteToDatafileForRangeOfDays(t *testing.T) {

	df := NewDatarecorder(rootDir, extension, retentionPeriod, header)

	// test start time: 12/20/2005
	layout := "20060102"
	startTime, _ := time.Parse(layout, "20051220")

	// create 20 datafiles for 12/20/2005 to 01/08/2006
	for i := 0; i < 20; i++ {
		recordTime := startTime.AddDate(0, 0, i)
		year := strconv.Itoa(recordTime.Year())

		// write a record
		df.WriteToDatafile(recordTime, sampleRecord)
		// verify that the total number of datafiles never exceeds retentionPeriod
		fileCnt, _ := countFiles(filepath.Join(rootDir, year))
		log.Debugf("TestDatarecorderWriteToDatafileForRangeOfDays: fileCnt = %d \n", fileCnt)

		if fileCnt > retentionPeriod {
			t.Errorf("expect to find %d datafiles or less, found %d datafiles\n",
				retentionPeriod, fileCnt)
		}
	}

	// cleanup: remove entire directory hierarchy
	err = os.RemoveAll(rootDir)
	check(err)

}
