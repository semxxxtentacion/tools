package mirea

import (
	"encoding/binary"
	"fmt"
	"math"
	"regexp"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type protoField struct {
	wireType uint64
	varint   uint64
	bytes    []byte
	bytes64  []byte
}

// appendVarint encodes a uint64 as a protobuf varint and appends it to buf
func appendVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}

// readVarint reads a varint from data at offset, returning (value, newOffset, error)
func readVarint(data []byte, offset int) (uint64, int, error) {
	var result uint64
	var shift uint
	for offset < len(data) {
		b := data[offset]
		offset++
		result |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, offset, nil
		}
		shift += 7
		if shift >= 64 {
			return 0, 0, fmt.Errorf("varint overflow")
		}
	}
	return 0, 0, fmt.Errorf("unexpected end of data")
}

// getFields extracts all fields with the given field number from protobuf-encoded data
func getFields(data []byte, fieldNum uint64) []protoField {
	var result []protoField
	offset := 0
	for offset < len(data) {
		tag, newOffset, err := readVarint(data, offset)
		if err != nil {
			break
		}
		offset = newOffset
		wireType := tag & 0x07
		fNum := tag >> 3

		switch wireType {
		case 0: // varint
			val, newOff, err := readVarint(data, offset)
			if err != nil {
				return result
			}
			if fNum == fieldNum {
				result = append(result, protoField{wireType: 0, varint: val})
			}
			offset = newOff
		case 1: // 64-bit
			if offset+8 > len(data) {
				return result
			}
			if fNum == fieldNum {
				b := make([]byte, 8)
				copy(b, data[offset:offset+8])
				result = append(result, protoField{wireType: 1, bytes64: b})
			}
			offset += 8
		case 2: // length-delimited
			length, newOff, err := readVarint(data, offset)
			if err != nil {
				return result
			}
			offset = newOff
			if offset+int(length) > len(data) {
				return result
			}
			if fNum == fieldNum {
				b := make([]byte, length)
				copy(b, data[offset:offset+int(length)])
				result = append(result, protoField{wireType: 2, bytes: b})
			}
			offset += int(length)
		case 5: // 32-bit
			offset += 4
		default:
			return result
		}
	}
	return result
}

// fixed64AsFloat64 interprets 8 bytes as an IEEE 754 little-endian double
func fixed64AsFloat64(b []byte) float64 {
	bits := binary.LittleEndian.Uint64(b)
	return math.Float64frombits(bits)
}

// binaryToUUID converts 16 bytes to a UUID string (big-endian)
func binaryToUUID(b []byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// extractBinaryUUIDs recursively scans for 16-byte length-delimited fields and returns them as UUID strings
func extractBinaryUUIDs(data []byte) []string {
	seen := make(map[string]bool)
	var result []string
	var scan func([]byte)
	scan = func(d []byte) {
		offset := 0
		for offset < len(d) {
			tag, newOffset, err := readVarint(d, offset)
			if err != nil {
				break
			}
			offset = newOffset
			wireType := tag & 0x07
			switch wireType {
			case 0:
				_, newOff, err := readVarint(d, offset)
				if err != nil {
					return
				}
				offset = newOff
			case 1:
				offset += 8
			case 2:
				length, newOff, err := readVarint(d, offset)
				if err != nil {
					return
				}
				offset = newOff
				if offset+int(length) > len(d) {
					return
				}
				chunk := d[offset : offset+int(length)]
				if length == 16 {
					u := binaryToUUID(chunk)
					if !seen[u] {
						seen[u] = true
						result = append(result, u)
					}
				}
				scan(chunk)
				offset += int(length)
			case 5:
				offset += 4
			default:
				return
			}
		}
	}
	scan(data)
	return result
}

// extractUUIDs recursively scans all length-delimited fields and returns UUID strings found
func extractUUIDs(data []byte) []string {
	seen := make(map[string]bool)
	var result []string

	var scan func([]byte)
	scan = func(d []byte) {
		offset := 0
		for offset < len(d) {
			tag, newOffset, err := readVarint(d, offset)
			if err != nil {
				break
			}
			offset = newOffset
			wireType := tag & 0x07

			switch wireType {
			case 0:
				_, newOff, err := readVarint(d, offset)
				if err != nil {
					return
				}
				offset = newOff
			case 1:
				offset += 8
			case 2:
				length, newOff, err := readVarint(d, offset)
				if err != nil {
					return
				}
				offset = newOff
				if offset+int(length) > len(d) {
					return
				}
				chunk := d[offset : offset+int(length)]
				s := string(chunk)
				if uuidRegex.MatchString(s) && !seen[s] {
					seen[s] = true
					result = append(result, s)
				}
				// recurse into nested messages
				scan(chunk)
				offset += int(length)
			case 5:
				offset += 4
			default:
				return
			}
		}
	}

	scan(data)
	return result
}
