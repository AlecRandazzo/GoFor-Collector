// Copyright (c) 2020 Alec Randazzo

package packrat

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"testing"

	"github.com/AlecRandazzo/Packrat/pkg/parsers/windows/vbr"
)

type dummyHandler struct {
	handle       *os.File
	volumeLetter string
	vbr          vbr.VolumeBootRecord
	reader       io.Reader
	lastOffset   int64
	filePath     string
	err          error
}

func (dummy *dummyHandler) GetHandle() error {
	if dummy.err != nil {
		return dummy.err
	}
	var err error
	dummy.handle, err = os.Open(dummy.filePath)
	// Parse the VBR to get details we need about the volume.
	volumeBootRecord := make([]byte, 512)
	_, err = dummy.handle.Read(volumeBootRecord)
	if err != nil {
		return fmt.Errorf("GetHandle() failed to read the volume boot record on volume %v: %w", dummy.volumeLetter, err)
	}
	dummy.vbr, err = vbr.Parse(volumeBootRecord)
	if err != nil {
		return fmt.Errorf("NewOldVolumeHandler() failed to parse vbr from volume letter %s: %w", dummy.volumeLetter, err)
	}
	log.Debugf("Successfully got a file handle to volume %v and read its volume boot record.", dummy.volumeLetter)

	return err
}

func (dummy dummyHandler) VolumeLetter() string {
	return dummy.volumeLetter
}

func (dummy dummyHandler) Handle() *os.File {
	return dummy.handle
}

func (dummy *dummyHandler) UpdateReader(newReader io.Reader) {
	dummy.reader = newReader
	return
}

func (dummy dummyHandler) Vbr() vbr.VolumeBootRecord {
	return dummy.vbr
}

func (dummy dummyHandler) Reader() io.Reader {
	return dummy.reader
}

func (dummy dummyHandler) LastOffset() int64 {
	return dummy.lastOffset
}

func (dummy *dummyHandler) UpdateLastOffset(newOffset int64) {
	dummy.lastOffset = newOffset
	return
}

func Test_GetHandle(t *testing.T) {
	type args struct {
		volumeLetter string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "no error",
			args:    args{volumeLetter: "C"},
			wantErr: false,
		},
		{
			name:    "nil string input",
			args:    args{volumeLetter: ""},
			wantErr: true,
		},
		{
			name:    "bad input",
			args:    args{volumeLetter: "CD"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle := NewVolumeHandler(tt.args.volumeLetter)
			err := handle.GetHandle()
			if (err != nil) != tt.wantErr {
				t.Errorf("getHandle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			handle.Handle().Close()
		})
	}
}