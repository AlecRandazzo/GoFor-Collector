/*
 * Copyright (c) 2019 Alec Randazzo
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 *
 */

package windowscollector

import (
	"errors"
	"fmt"
	vbr "github.com/AlecRandazzo/GoFor-VBR-Parser"
	log "github.com/sirupsen/logrus"
	syscall "golang.org/x/sys/windows"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode"
)

type VolumeHandler struct {
	Handle       syscall.Handle
	VolumeLetter string
	Vbr          vbr.VolumeBootRecord
	mftReader    io.Reader
}

func getHandle(volumeLetter string) (handle syscall.Handle, err error) {
	dwDesiredAccess := uint32(0x80000000) //0x80 FILE_READ_ATTRIBUTES
	dwShareMode := uint32(0x02 | 0x01)
	dwCreationDisposition := uint32(0x03)
	dwFlagsAndAttributes := uint32(0x00)

	volumePath, _ := syscall.UTF16PtrFromString(fmt.Sprintf("\\\\.\\%s:", volumeLetter))
	handle, err = syscall.CreateFile(volumePath, dwDesiredAccess, dwShareMode, nil, dwCreationDisposition, dwFlagsAndAttributes, 0)
	if err != nil {
		err = fmt.Errorf("getHandle() failed to get handle to volume %s: %w", volumeLetter, err)
		return
	}
	return
}

// GetVolumeHandler gets a file handle to the specified volume and parses its volume boot record.
func GetVolumeHandler(volumeLetter string) (volume VolumeHandler, err error) {
	const volumeBootRecordSize = 512
	volume.VolumeLetter = volumeLetter
	volume.Handle, err = getHandle(volumeLetter)
	if err != nil {
		err = fmt.Errorf("GetVolumeHandler() failed to get handle to volume %s: %w", volumeLetter, err)
		return
	}

	// Parse the VBR to get details we need about the volume.
	volumeBootRecord := make([]byte, volumeBootRecordSize)
	_, err = syscall.Read(volume.Handle, volumeBootRecord)
	if err != nil {
		err = fmt.Errorf("GetVolumeHandler() failed to read the volume boot record on volume %v: %w", volumeLetter, err)
		return
	}
	volume.Vbr, err = vbr.RawVolumeBootRecord(volumeBootRecord).Parse()
	if err != nil {
		err = fmt.Errorf("GetVolumeHandler() failed to parse vbr from volume letter %s: %w", volumeLetter, err)
		return
	}
	log.Debugf("Successfully got a file handle to volume %v and read its volume boot record.", volumeLetter)
	return
}

func isLetter(s string) (result bool, err error) {
	// Sanity checking
	if s == "" {
		err = errors.New("isLetter() received a null string")
		return
	} else if len(s) > 1 {
		err = fmt.Errorf("isLetter() received the string %s which is too many letters, function expected a single letter", s)
		return
	}

	for _, r := range s {
		if unicode.IsLetter(r) {
			result = true
			return
		}
	}

	result = false
	return
}

func identifyVolumesOfInterest(exportList *ListOfFilesToExport) (volumesOfInterest []string, err error) {
	volumesOfInterest = make([]string, 0)
	re := regexp.MustCompile(`[^:]+`)
	for index, fileToExport := range *exportList {
		volume := re.FindString(strings.ToLower(fileToExport.FullPath))
		if volume == "%systemdrive%" {
			systemDrive := os.Getenv("SYSTEMDRIVE")
			volume = re.FindString(systemDrive)
			(*exportList)[index].FullPath = strings.Replace(strings.ToLower(fileToExport.FullPath), "%systemdrive%", volume, -1)
		} else {
			var result bool
			result, err = isLetter(volume)
			if err != nil {
				err = fmt.Errorf("isLetter() returned an error: %w", err)
				return
			} else if result == false {
				err = fmt.Errorf("isLetter() indicated that the full path string %s does not start with a letter", fileToExport.FullPath)
				return
			}
		}

		isTracked := false
		for _, trackedVolumes := range volumesOfInterest {
			if trackedVolumes == volume {
				isTracked = true
				break
			}
		}

		if isTracked == true {
			continue
		} else {
			volumesOfInterest = append(volumesOfInterest, volume)
		}
	}

	return
}