// Copyright (c) 2022 Alec Randazzo

package windows

import (
	"io"
	"os"

	"github.com/AlecRandazzo/Packrat/pkg/windows/mft"
	"github.com/AlecRandazzo/Packrat/pkg/windows/volume"
	log "github.com/sirupsen/logrus"
)

// dataRunsReader contains all the information needed to support the data runs reader function
type dataRunsReader struct {
	Handler                       volume.Handler
	DataRuns                      mft.DataRuns
	fileName                      string
	dataRunTracker                int
	dataRunBytesLeftToReadTracker int64
	totalFileSize                 int64
	totalByesRead                 int64
	initialized                   bool
}

// Read is an io.Reader that reads data found at a set of dataruns.
func (dataRunReader *dataRunsReader) Read(byteSliceToPopulate []byte) (numberOfBytesRead int, err error) {
	bufferSize := int64(len(byteSliceToPopulate))

	// Sanity checking
	if len(dataRunReader.DataRuns) == 0 {
		log.Warnf("failed to read %s, received: %v", dataRunReader.fileName, err)
		return 0, io.ErrUnexpectedEOF
	}

	// Check if this reader has been initialized, if not, do so.
	if !dataRunReader.initialized {
		if dataRunReader.totalFileSize == 0 {
			for _, dataRun := range dataRunReader.DataRuns {
				dataRunReader.totalFileSize += dataRun.Length
			}
		}
		dataRunReader.dataRunTracker = 0
		dataRunReader.dataRunBytesLeftToReadTracker = dataRunReader.DataRuns[dataRunReader.dataRunTracker].Length
		_, _ = dataRunReader.Handler.Handle().Seek(dataRunReader.DataRuns[dataRunReader.dataRunTracker].AbsoluteOffset, 0)
		newOffset := dataRunReader.Handler.LastOffset() - bufferSize
		dataRunReader.Handler.UpdateLastOffset(newOffset)
		dataRunReader.initialized = true

		// These are for debug purposes
		if log.GetLevel() == log.DebugLevel {
			totalSize := int64(0)
			for _, dataRun := range dataRunReader.DataRuns {
				totalSize += dataRun.Length
			}
			log.Debugf("Reading data run number 1 of %d for file '%s' which has a length of %d bytes at absolute offset %d",
				len(dataRunReader.DataRuns),
				dataRunReader.fileName,
				totalSize,
				dataRunReader.DataRuns[0].AbsoluteOffset,
			)
		}

	}

	// Figure out how many bytes are left to read
	if dataRunReader.dataRunBytesLeftToReadTracker-bufferSize == 0 {
		dataRunReader.dataRunBytesLeftToReadTracker -= bufferSize
	} else if dataRunReader.dataRunBytesLeftToReadTracker-bufferSize < 0 {
		bufferSize = dataRunReader.dataRunBytesLeftToReadTracker
		dataRunReader.dataRunBytesLeftToReadTracker = 0
	} else {
		dataRunReader.dataRunBytesLeftToReadTracker -= bufferSize
	}

	// Read from the data run
	if dataRunReader.totalByesRead+bufferSize > dataRunReader.totalFileSize {
		bufferSize = dataRunReader.totalFileSize - dataRunReader.totalByesRead
	}
	buffer := make([]byte, bufferSize)
	newOffset := dataRunReader.Handler.LastOffset() + bufferSize
	dataRunReader.Handler.UpdateLastOffset(newOffset)
	numberOfBytesRead, _ = dataRunReader.Handler.Handle().Read(buffer)
	copy(byteSliceToPopulate, buffer)
	dataRunReader.totalByesRead += bufferSize
	if dataRunReader.totalFileSize == dataRunReader.totalByesRead {
		return 0, io.EOF
	}

	// Check to see if there are any bytes left to read in the current data run
	if dataRunReader.dataRunBytesLeftToReadTracker == 0 {
		// Increment our tracker
		dataRunReader.dataRunTracker++

		// Get the size of the next datarun
		dataRunReader.dataRunBytesLeftToReadTracker = dataRunReader.DataRuns[dataRunReader.dataRunTracker].Length

		// Seek to the offset of the next datarun
		newOffset, _ = dataRunReader.Handler.Handle().Seek(dataRunReader.DataRuns[dataRunReader.dataRunTracker].AbsoluteOffset, 0)
		newOffset -= bufferSize
		dataRunReader.Handler.UpdateLastOffset(newOffset)

		log.Debugf("Reading data run number %d of %d for file '%s' which has a length of %d bytes at absolute offset %d",
			dataRunReader.dataRunTracker+1,
			len(dataRunReader.DataRuns),
			dataRunReader.fileName,
			dataRunReader.DataRuns[dataRunReader.dataRunTracker].Length,
			dataRunReader.Handler.LastOffset()+bufferSize,
		)
	}

	return numberOfBytesRead, nil
}

// newApiFileReader tries to create an io.Reader against a file using Windows APIs. This does not work for certain systems files.
func newApiFileReader(file foundFile) (io.Reader, error) {
	return os.Open(file.fullPath)
}

// newRawFileReader creates an io.Reader against the raw file on disk. This reader bypasses read blocks.
func newRawFileReader(handler volume.Handler, file foundFile) io.Reader {
	return &dataRunsReader{
		Handler:                       handler,
		DataRuns:                      file.dataRuns,
		fileName:                      file.fullPath,
		dataRunTracker:                0,
		dataRunBytesLeftToReadTracker: 0,
		totalFileSize:                 file.size,
		initialized:                   false,
	}
}
