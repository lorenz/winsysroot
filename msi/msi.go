package msi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/richardlehane/mscfb"
)

// Lists source media disks for the installation.
type Media struct {
	DiskID        uint16
	LastSequence1 uint16
	LastSequence2 uint16
	DiskPrompt    string
	Cabinet       string
	VolumeLabel   string
	Source        string
}

type File struct {
	File       string
	Component  string
	FileName   string
	FileSize1  uint16
	FileSize2  uint16
	Version    string
	Language   string
	Attributes uint16
	Sequence1  uint16
	Sequence2  uint16
}

type Component struct {
	Component   string
	ComponentID string
	Directory   string
	Attributes  uint16
	Condition   string
	KeyPath     string
}

type Directory struct {
	Directory       string
	DirectoryParent string
	DefaultDir      string
}

var msiNameAlphabet = []rune("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz._!")

func decodeName(name string) string {
	var decodedName []rune
	// Algorithm thanks to https://stackoverflow.com/questions/9734978/view-msi-strings-in-binary
	for _, r := range name {
		if r >= 0x3800 && r < 0x4800 {
			decodedName = append(decodedName, msiNameAlphabet[(r-0x3800)&0x3F], msiNameAlphabet[((r-0x3800)>>6)&0x3F])
		} else if r >= 0x4800 && r <= 0x4840 {
			decodedName = append(decodedName, msiNameAlphabet[r-0x4800])
		} else {
			decodedName = append(decodedName, r)
		}
	}
	return string(decodedName)
}

func decodeStrings(stringData, stringPool []byte) []string {
	var strs []string
	poolReader := bytes.NewReader(stringPool)
	var offset uint32
	for {
		var occNumber, stringLen uint16
		err := binary.Read(poolReader, binary.LittleEndian, &stringLen)
		if err == io.EOF {
			return strs
		}
		if err != nil {
			panic(err)
		}
		if err := binary.Read(poolReader, binary.LittleEndian, &occNumber); err != nil {
			panic(err)
		}
		if occNumber > 0 {
			var bigStringLen uint32 = uint32(stringLen)
			if stringLen == 0 {
				if err := binary.Read(poolReader, binary.LittleEndian, &bigStringLen); err != nil {
					panic(err)
				}
			}
			strs = append(strs, string(stringData[offset:offset+bigStringLen]))
			offset += bigStringLen
		} else {
			strs = append(strs, "")
		}
	}
}

func parseTable(data []uint16, stringTable []string, target interface{}) {
	targetVal := reflect.ValueOf(target)
	nColumns := targetVal.Type().Elem().Elem().NumField()
	if len(data)%nColumns != 0 {
		panic("invalid table data")
	}
	rowVal := reflect.New(targetVal.Type().Elem().Elem()).Elem()
	nRows := len(data) / nColumns
	for i := 0; i < nRows; i++ {
		for j := 0; j < nColumns; j++ {
			val := data[(nRows*j)+i]
			f := rowVal.Field(j)
			switch f.Type().Kind() {
			case reflect.String:
				if int(val) >= len(stringTable) {
					fmt.Println("bug in case")
					break
				}
				f.SetString(stringTable[val])
			case reflect.Uint16:
				f.SetUint(uint64(val))
			default:
				panic("unimplemented type")
			}
		}
		targetVal.Elem().Set(reflect.Append(targetVal.Elem(), rowVal))
	}
}

func getModernName(name string) string {
	parts := strings.SplitN(name, "|", 2)
	return parts[len(parts)-1]
}

type MSI struct {
	// File name in CAB -> Final path
	FileMap map[string]string
	// List of CAB files used
	CABFiles []string
}

func Parse(reader io.ReaderAt) (*MSI, error) {
	doc, err := mscfb.New(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MS-CFB header (not an MSI file?): %w", err)
	}
	var stringPool, stringData []byte
	rawTableData := make(map[string][]uint16)
	for entry, err := doc.Next(); err == nil; entry, err = doc.Next() {
		name := decodeName(entry.Name)
		if name == "!_StringPool" {
			stringPool, err = io.ReadAll(entry)
		}
		if name == "!_StringData" {
			stringData, err = io.ReadAll(entry)
		}
		if strings.HasPrefix(name, "!") && !strings.HasPrefix(name, "!_") {
			raw := make([]uint16, entry.Size/2)
			binary.Read(doc, binary.LittleEndian, &raw)
			rawTableData[strings.TrimPrefix(name, "!")] = raw
		}
		if err != nil {
			log.Fatal(err)
		}
	}
	stringsList := decodeStrings(stringData, stringPool)

	var dirs []Directory
	parseTable(rawTableData["Directory"], stringsList, &dirs)
	dirMap := make(map[string]Directory)
	dirPathMap := make(map[string]string)
	for _, dir := range dirs {
		if dir.Directory == "TARGETDIR" {
			dir.DefaultDir = "."
		}
		dirMap[dir.Directory] = dir
	}
	for _, dir := range dirs {
		pathParts := []string{getModernName(dir.DefaultDir)}
		nextParent := dir.DirectoryParent
		for {
			if nextParent == "" {
				break
			}
			parent := dirMap[nextParent]
			nextParent = parent.DirectoryParent
			pathParts = append(pathParts, getModernName(parent.DefaultDir))
		}
		// Reverse order
		sort.SliceStable(pathParts, func(i, j int) bool {
			return i > j
		})
		dirPathMap[dir.Directory] = path.Join(pathParts...)
	}

	var components []Component
	componentDirMap := make(map[string]string)
	parseTable(rawTableData["Component"], stringsList, &components)
	for _, cmp := range components {
		componentDirMap[cmp.Component] = dirPathMap[cmp.Directory]
	}

	var medias []Media
	parseTable(rawTableData["Media"], stringsList, &medias)

	var files []File
	parseTable(rawTableData["File"], stringsList, &files)
	fileToPath := make(map[string]string)
	for _, f := range files {
		fileToPath[f.File] = filepath.Join(componentDirMap[f.Component], getModernName(f.FileName))
	}
	var data MSI
	data.FileMap = fileToPath
	for _, m := range medias {
		if m.Cabinet == "" {
			continue
		}
		data.CABFiles = append(data.CABFiles, m.Cabinet)
	}
	return &data, nil
}
