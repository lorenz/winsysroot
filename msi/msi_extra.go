package msi

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/richardlehane/mscfb"
)

func decodeTables(tablesReader io.Reader) []uint16 {
	var tableIndices []uint16
	for {
		var stringIdx uint16
		err := binary.Read(tablesReader, binary.LittleEndian, &stringIdx)
		if err == io.EOF {
			return tableIndices
		}
		tableIndices = append(tableIndices, stringIdx)
	}
}

func decodeColumnMeta(columnsReader io.Reader, size int64) []ColumnRaw {
	nColumns := size / int64(binary.Size(ColumnRaw{}))
	columns := make([]ColumnRaw, nColumns)
	for i := int64(0); i < nColumns; i++ {
		if err := binary.Read(columnsReader, binary.LittleEndian, &columns[i].TableNameIdx); err != nil {
			panic(err)
		}
	}
	for i := int64(0); i < nColumns; i++ {
		if err := binary.Read(columnsReader, binary.LittleEndian, &columns[i].ColumnIdx); err != nil {
			panic(err)
		}
	}
	for i := int64(0); i < nColumns; i++ {
		if err := binary.Read(columnsReader, binary.LittleEndian, &columns[i].ColumnNameIdx); err != nil {
			panic(err)
		}
	}
	for i := int64(0); i < nColumns; i++ {
		if err := binary.Read(columnsReader, binary.LittleEndian, &columns[i].ColumnType); err != nil {
			panic(err)
		}
	}
	return columns
}

type ColumnRaw struct {
	TableNameIdx  uint16
	ColumnIdx     uint16
	ColumnNameIdx uint16
	ColumnType    uint16
}

func properDecodingStuff() {
	doc, _ := mscfb.New(strings.NewReader(""))
	var stringPool, stringData []byte
	var tableStringIndices []uint16
	var columnsRaw []ColumnRaw
	rawTableData := make(map[string][]uint16)
	for entry, err := doc.Next(); err == nil; entry, err = doc.Next() {
		name := decodeName(entry.Name)
		if name == "!_StringPool" {
			stringPool, err = io.ReadAll(entry)
		}
		if name == "!_StringData" {
			stringData, err = io.ReadAll(entry)
		}
		if name == "!_Tables" {
			tableStringIndices = decodeTables(entry)
		}
		if name == "!_Columns" {
			columnsRaw = decodeColumnMeta(entry, entry.Size)
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
	for _, idx := range tableStringIndices {
		fmt.Println(stringsList[idx])
	}
	for _, col := range columnsRaw {
		if int(col.TableNameIdx) >= len(stringsList) {
			continue
		}
		fmt.Printf("%v.%v %v %v\n", stringsList[col.TableNameIdx], stringsList[col.ColumnNameIdx], col.ColumnIdx, col.ColumnType)
	}

}
