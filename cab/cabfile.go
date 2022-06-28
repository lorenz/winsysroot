// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package cabfile provides a bare minimum implementation of a parser for the
// Microsoft Cabinet file format. Its goal is to support the feature set of
// Cabinet files produced by gcab for the LVFS project.
//
// Normative references for this implementation are [MS-CAB] for the Cabinet
// file format and [MS-MCI] for the Microsoft ZIP Compression and Decompression
// Data Structure.
//
// [MS-CAB]: http://download.microsoft.com/download/4/d/a/4da14f27-b4ef-4170-a6e6-5b1ef85b1baa/[ms-cab].pdf
// [MS-MCI]: http://interoperability.blob.core.windows.net/files/MS-MCI/[MS-MCI].pdf
package cab

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"time"
)

// Cabinet provides read-only access to Microsoft Cabinet files.
type Cabinet struct {
	r     io.ReadSeeker
	hdr   *cfHeader
	fldrs []*cfFolder
	files []*file

	fileIdx    int
	fileReader io.Reader

	folderIdx uint16
	folderBuf []byte
}

type cfHeader struct {
	Signature    [4]byte
	Reserved1    uint32
	CBCabinet    uint32 // size of this cabinet file in bytes
	Reserved2    uint32
	COFFFiles    uint32 // offset of the first CFFILE entry
	Reserved3    uint32 // reserved
	VersionMinor uint8  // cabinet file format version, minor
	VersionMajor uint8  // cabinet file format version, major
	CFolders     uint16 // number of CFFOLDER entries in this cabinet
	CFiles       uint16 // number of CFFILE entries in this cabinet
	Flags        uint16 // cabinet file option indicators
	SetID        uint16 // must be the same for all cabinets in a set
	ICabinet     uint16 // number of this cabinet file in a set
}

// cfHeaderReserve is present right after cfHeader if the RESERVE_PRESENT flag is set
type cfHeaderReserve struct {
	// Indicates the size, in bytes, of the abReserve field in this CFHEADER structure.
	CBCFHeader uint16
	// Indicates the size, in bytes, of the abReserve field in each CFFOLDER field entry.
	CBCFFolder uint8
	// The cbCFDATA field indicates the size, in bytes, of the abReserve field in each CFDATA field entry.
	CBCFData uint8
}

const (
	hdrPrevCabinet uint16 = 1 << iota
	hdrNextCabinet
	hdrReservePresent
)

type cfFolder struct {
	COFFCabStart uint32 // offset of the first CFDATA block in this folder
	CCFData      uint16 // number of CFDATA blocks in this folder
	TypeCompress uint16 // compression type indicator
}

const (
	compMask    uint16 = 0xf
	compNone           = 0x0
	compMSZIP          = 0x1
	compQuantum        = 0x2
	compLZX            = 0x3
)

type cfFile struct {
	CBFile          uint32 // uncompressed size of this file in bytes
	UOffFolderStart uint32 // uncompressed offset of this file in the folder
	IFolder         uint16 // index into the CFFOLDER area
	Date            uint16 // date stamp for this file
	Time            uint16 // time stamp for this file
	Attribs         uint16 // attribute flags for this file
}

const (
	attribReadOnly = 1 << iota // file is read-only
	attribHidden               // file is hidden
	attribSystem               // file is a system file
	_
	_
	attribArchive   // file modified since last backup
	attribExec      // run after extraction
	attribNameIsUTF // filename is UTF-encoded
)

type file struct {
	*cfFile
	name string
}

type Header struct {
	// Name of the file including path
	Name string
	// Time the file was created
	CreateTime time.Time
	// The file size in bytes
	Size uint32
}

type cfData struct {
	Checksum uint32 // checksum of this CFDATA entry
	CBData   uint16 // number of compressed bytes in this block
	CBUncomp uint16 // number of uncompressed bytes in this block
}

// New returns a new Cabinet with the header structures parsed and sanity checked.
func New(r io.ReadSeeker) (*Cabinet, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("could not seek to the beginning: %v", err)
	}

	// CFHEADER
	var hdr cfHeader
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, fmt.Errorf("could not deserialize header: %v", err)
	}
	if !bytes.Equal(hdr.Signature[:], []byte("MSCF")) {
		return nil, fmt.Errorf("invalid Cabinet file signature: %v", hdr.Signature)
	}
	if hdr.Reserved1 != 0 || hdr.Reserved2 != 0 || hdr.Reserved3 != 0 {
		return nil, fmt.Errorf("reserved files must be zero: %v, %v, %v", hdr.Reserved1, hdr.Reserved2, hdr.Reserved3)
	}
	if hdr.VersionMajor != 1 || hdr.VersionMinor != 3 {
		return nil, fmt.Errorf("Cabinet file version has unsupported version %d.%d", hdr.VersionMajor, hdr.VersionMinor)
	}
	if (hdr.Flags&hdrPrevCabinet) != 0 || (hdr.Flags&hdrNextCabinet) != 0 {
		return nil, errors.New("multi-part Cabinet files are unsupported")
	}
	if (hdr.Flags & hdrReservePresent) != 0 {
		var reserveHdr cfHeaderReserve
		if err := binary.Read(r, binary.LittleEndian, &reserveHdr); err != nil {
			return nil, fmt.Errorf("coult not deserialize reserved header: %w", err)
		}
		if reserveHdr.CBCFData != 0 || reserveHdr.CBCFFolder != 0 {
			return nil, errors.New("cabinet file with reserved folder and data sections unsupported")
		}
		appSpecificHdr := make([]byte, reserveHdr.CBCFHeader)
		if _, err := io.ReadFull(r, appSpecificHdr); err != nil {
			return nil, fmt.Errorf("failed to read app-specific header: %w", err)
		}
	}

	// CFFOLDER
	var fldrs []*cfFolder
	for i := uint16(0); i < hdr.CFolders; i++ {
		var fldr cfFolder
		if err := binary.Read(r, binary.LittleEndian, &fldr); err != nil {
			return nil, fmt.Errorf("could not deserialize folder %d: %v", i, err)
		}
		switch fldr.TypeCompress & compMask {
		case compNone:
		case compMSZIP:
		default:
			return nil, fmt.Errorf("folder compressed with unsupported algorithm %d", fldr.TypeCompress)
		}
		fldrs = append(fldrs, &fldr)
	}

	// CFFILE
	if _, err := r.Seek(int64(hdr.COFFFiles), io.SeekStart); err != nil {
		return nil, fmt.Errorf("could not seek to start of CFFILE section: %v", err)
	}
	var files []*file
	for i := uint16(0); i < hdr.CFiles; i++ {
		var f cfFile
		if err := binary.Read(r, binary.LittleEndian, &f); err != nil {
			return nil, fmt.Errorf("could not deserialize file %d: %v", i, err)
		}
		off, err := r.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("could not preserve current offset: %v", err)
		}
		fn, err := bufio.NewReader(r).ReadBytes('\x00')
		if err != nil {
			return nil, fmt.Errorf("could not read filename for file %d: %v", i, err)
		}
		if _, err := r.Seek(off+int64(len(fn)), io.SeekStart); err != nil {
			return nil, fmt.Errorf("could not seek to the end of file entry %d: %v", i, err)
		}
		files = append(files, &file{&f, string(fn[:len(fn)-1])})
	}
	sort.Slice(files, func(i, j int) bool {
		// Sort by folder first, then by offset
		return (uint64(files[i].IFolder)<<32)+uint64(files[i].UOffFolderStart) < (uint64(files[j].IFolder)<<32)+uint64(files[j].UOffFolderStart)
	})

	return &Cabinet{r: r, hdr: &hdr, fldrs: fldrs, files: files, folderIdx: math.MaxUint16}, nil
}

// FileList returns the list of filenames in the Cabinet file.
func (c *Cabinet) FileList() []string {
	var names []string
	for _, f := range c.files {
		names = append(names, f.name)
	}
	return names
}

type folderDataReader struct {
	r    io.Reader
	fldr *cfFolder

	// MS-ZIP requires that the history buffer is preserved across block boundaries
	lastBlock bytes.Buffer

	// Index of the next block to be read
	blockIdx int

	// Reader of the current block
	blockReader io.Reader

	rawBlockReader io.ReadCloser
}

func (f *folderDataReader) nextBlock() error {
	if uint16(f.blockIdx) >= f.fldr.CCFData {
		return io.EOF
	}
	if f.rawBlockReader != nil {
		// Close old raw block reader to make sure everthing was read
		f.rawBlockReader.Close()
	}
	var d cfData
	if err := binary.Read(f.r, binary.LittleEndian, &d); err != nil {
		return fmt.Errorf("could not deserialize data structure %d: %v", f.blockIdx, err)
	}
	f.rawBlockReader = ExactReader(f.r, int64(d.CBData))

	// TODO: Checksum the block
	switch f.fldr.TypeCompress {
	case compNone:
		if d.CBData != d.CBUncomp {
			return fmt.Errorf("compressed bytes %d of data section %d do not equal uncompressed bytes %d when no compression was specified", d.CBData, f.blockIdx, d.CBUncomp)
		}
		f.blockReader = f.rawBlockReader
	case compMSZIP:
		sig := make([]byte, 2)
		if _, err := io.ReadFull(f.rawBlockReader, sig); err != nil {
			return fmt.Errorf("failed to read MS-ZIP signature: %w", err)
		}
		if !bytes.Equal(sig, []byte("CK")) {
			return fmt.Errorf("invalid MS-ZIP signature %q in data block %d", sig, f.blockIdx)
		}
		var r io.Reader
		if f.lastBlock.Len() == 0 {
			r = ExactReader(flate.NewReader(f.rawBlockReader), int64(d.CBUncomp))
		} else {
			r = flate.NewReaderDict(f.rawBlockReader, f.lastBlock.Next(f.lastBlock.Len()))
		}
		f.blockReader = io.TeeReader(ExactReader(r, int64(d.CBUncomp)), &f.lastBlock)
	default:
		return errors.New("unsupported compression")
	}
	f.blockIdx++
	return nil
}

func (f *folderDataReader) Read(p []byte) (n int, err error) {
	n, err = f.blockReader.Read(p)
	if err == io.EOF {
		return n, f.nextBlock()
	}
	return
}

func (c *Cabinet) folderData(idx uint16) (*folderDataReader, error) {
	if int(idx) >= len(c.fldrs) {
		return nil, errors.New("folder number out of range")
	}
	fldr := c.fldrs[idx]
	if _, err := c.r.Seek(int64(fldr.COFFCabStart), io.SeekStart); err != nil {
		return nil, fmt.Errorf("could not seek to start of data section: %v", err)
	}
	r := &folderDataReader{
		r:    c.r,
		fldr: fldr,
	}
	r.nextBlock()
	return r, nil
}

func (c *Cabinet) Read(p []byte) (n int, err error) {
	return c.fileReader.Read(p)
}

func (c *Cabinet) Next() (*Header, error) {
	if c.fileIdx >= len(c.files) {
		return nil, io.EOF
	}
	f := c.files[c.fileIdx]
	if f.IFolder != c.folderIdx {
		var err error
		c.folderIdx = f.IFolder
		var r io.Reader
		r, err = c.folderData(c.folderIdx)
		if err != nil {
			return nil, fmt.Errorf("failed to read new folder data stream: %w", err)
		}
		// Necessary as CAB allows overlapping files
		c.folderBuf, err = io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("failed to read folder data stream: %w", err)
		}
	}
	if len(c.folderBuf) < int(f.UOffFolderStart)+int(f.CBFile) {
		return nil, fmt.Errorf("file segment out of range")
	}
	c.fileReader = bytes.NewReader(c.folderBuf[f.UOffFolderStart : f.UOffFolderStart+f.CBFile])
	c.fileIdx++
	return &Header{
		Name:       f.name,
		CreateTime: msDosTimeToTime(f.Date, f.Time),
		Size:       f.CBFile,
	}, nil
}

// Content returns the content of the file specified by its filename as an
// io.Reader. Note that the entire folder which contains the file in question
// is decompressed for every file request.
func (c *Cabinet) Content(name string) (io.Reader, error) {
	for _, f := range c.files {
		if f.name != name {
			continue
		}
		data, err := c.folderData(f.IFolder)
		if err != nil {
			return nil, fmt.Errorf("could not acquire uncompressed data for folder %d: %v", f.IFolder, err)
		}

		if _, err := io.CopyN(io.Discard, data, int64(f.UOffFolderStart)); err != nil {
			return nil, fmt.Errorf("could not seek to start of data: %v", err)
		}
		return ExactReader(data, int64(f.CBFile)), nil
	}
	return nil, fmt.Errorf("file %q not found in Cabinet", name)
}
