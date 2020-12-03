package yttracker

import (
	"bytes"
	"encoding/binary"
	"reflect"
)

// EqualSorted check if two arrays are equal
func EqualSorted(listA, listB interface{}) (ok bool) {
	if listA == nil || listB == nil {
		return listA == listB
	}
	aKind := reflect.TypeOf(listA).Kind()
	bKind := reflect.TypeOf(listB).Kind()
	if aKind != reflect.Array && aKind != reflect.Slice {
		return false
	}
	if bKind != reflect.Array && bKind != reflect.Slice {
		return false
	}
	aValue := reflect.ValueOf(listA)
	bValue := reflect.ValueOf(listB)
	if aValue.Len() != bValue.Len() {
		return false
	}
	// Mark indexes in bValue that we already used
	visited := make([]bool, bValue.Len())
	for i := 0; i < aValue.Len(); i++ {
		element := aValue.Index(i).Interface()
		found := false
		for j := 0; j < bValue.Len(); j++ {
			if visited[j] {
				continue
			}

			if ObjectsAreEqual(bValue.Index(j).Interface(), element) {
				visited[j] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// ObjectsAreEqual check if two objects are equal
func ObjectsAreEqual(expected, actual interface{}) bool {
	if expected == nil || actual == nil {
		return expected == actual
	}
	if exp, ok := expected.([]byte); ok {
		act, ok := actual.([]byte)
		if !ok {
			return false
		} else if exp == nil || act == nil {
			return exp == nil && act == nil
		}
		return bytes.Equal(exp, act)
	}
	return reflect.DeepEqual(expected, actual)

}

// Max select maximum value
func Max(num ...int64) int64 {
	max := num[0]
	for _, v := range num {
		if v > max {
			max = v
		}
	}
	return max
}

// Min select minimum value
func Min(num ...int64) int64 {
	min := num[0]
	for _, v := range num {
		if v < min {
			min = v
		}
	}
	return min
}

//Int64ToBytes convert int64 to byte slice
func Int64ToBytes(data int64) []byte {
	bytebuf := bytes.NewBuffer([]byte{})
	binary.Write(bytebuf, binary.BigEndian, data)
	return bytebuf.Bytes()
}

//Int32ToBytes convert int32 to byte slice
func Int32ToBytes(data int32) []byte {
	bytebuf := bytes.NewBuffer([]byte{})
	binary.Write(bytebuf, binary.BigEndian, data)
	return bytebuf.Bytes()
}

//Uint32ToBytes convert uint32 to byte slice
func Uint32ToBytes(data uint32) []byte {
	bytebuf := bytes.NewBuffer([]byte{})
	binary.Write(bytebuf, binary.BigEndian, data)
	return bytebuf.Bytes()
}

//BytesToInt32 convert byte slice to int32
func BytesToInt32(bys []byte) int32 {
	bytebuff := bytes.NewBuffer(bys)
	var data int32
	binary.Read(bytebuff, binary.BigEndian, &data)
	return data
}

//BytesToInt64 convet byte slice to int64
func BytesToInt64(bys []byte) int64 {
	bytebuff := bytes.NewBuffer(bys)
	var data int64
	binary.Read(bytebuff, binary.BigEndian, &data)
	return data
}

//Uint16ToBytes convert uint16 to byte slice
func Uint16ToBytes(data uint16) []byte {
	bytebuf := bytes.NewBuffer([]byte{})
	binary.Write(bytebuf, binary.BigEndian, data)
	return bytebuf.Bytes()
}

//BytesToUint16 convet byte slice to uint16
func BytesToUint16(bys []byte) uint16 {
	bytebuff := bytes.NewBuffer(bys)
	var data uint16
	binary.Read(bytebuff, binary.BigEndian, &data)
	return data
}
