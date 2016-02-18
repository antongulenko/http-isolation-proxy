package services

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"math"
	"net"
	"syscall"
)

var (
	ConfiguredOpenFilesLimit uint64
)

func init() {
	flag.Uint64Var(&ConfiguredOpenFilesLimit, "ofl", ConfiguredOpenFilesLimit,
		"Set to >0 for configuring the open files limit (only possible as root)")
}

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

func ConfigureOpenFilesLimit() {
	if ConfiguredOpenFilesLimit > 0 {
		if err := SetOpenFilesLimit(ConfiguredOpenFilesLimit); err != nil {
			L.Warnf("Failed to set open files limit to %v: %v", ConfiguredOpenFilesLimit, err)
		} else {
			L.Logf("Successfully set open files limit to %v", ConfiguredOpenFilesLimit)
		}
	}
}

func SetOpenFilesLimit(ulimit uint64) error {
	rLimit := syscall.Rlimit{
		Max: ulimit,
		Cur: ulimit,
	}
	return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
}

func FirstIpAddress() (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			// Loopback and disabled interfaces are not interesting
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				return v.IP, nil
			case *net.IPAddr:
				return v.IP, nil
			}
		}
	}
	return nil, errors.New("No valid network interfaces found")
}
