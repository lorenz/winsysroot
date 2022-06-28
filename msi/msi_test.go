package msi

import (
	"reflect"
	"testing"
)

func Test_decodeStringVector(t *testing.T) {
	type args struct {
		stringData []byte
		stringPool []byte
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{"testcase", args{stringData: []byte("NameTableTypeColumn"), stringPool: []byte{0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x0A, 0x00, 0x05, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x06, 0x00, 0x06, 0x00, 0x02, 0x00}}, []string{"", "Name", "Table", "", "Type", "Column"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeStrings(tt.args.stringData, tt.args.stringPool); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decodeStringVector() = %v, want %v", got, tt.want)
			}
		})
	}
}
