package services

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
)

func UintBytes(value uint64) []byte {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, value)
	return bytes
}

func FloatBytes(value float64) []byte {
	bits := math.Float64bits(value)
	return UintBytes(bits)
}

func BoolBytes(value bool) []byte {
	if value {
		return []byte{1}
	} else {
		return []byte{0}
	}
}

func MakeHash(data ...interface{}) string {
	h := sha1.New()
	for _, value := range data {
		// Try to convert to []byte
		var bytes []byte
		switch val := value.(type) {
		case []byte:
			bytes = val
		case string:
			bytes = ([]byte)(val)
		case bool:
			bytes = BoolBytes(val)
		case float64:
			bytes = FloatBytes(val)
		case uint64:
			bytes = UintBytes(val)
		default:
			L.Warnf("Cannot convert %T to []byte: %v\n", value, value)
			bytes = ([]byte)(fmt.Sprintf("%v", value))
		}
		h.Write(bytes)
	}
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}
