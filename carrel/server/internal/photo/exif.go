package photo

import (
	"bytes"
	"encoding/binary"
)

// orientation reads EXIF tag 0x0112 from the first APP1 segment of a JPEG
// and nothing else: no EXIF library (architecture.md). Anything it cannot
// read is orientation 1.
func orientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		switch {
		case marker == 0xFF: // fill byte
			i++
			continue
		case marker == 0xD9 || marker == 0xDA: // end of image, start of scan
			return 1
		case marker == 0x01 || marker >= 0xD0 && marker <= 0xD7: // no length
			i += 2
			continue
		}
		size := int(binary.BigEndian.Uint16(data[i+2:]))
		if size < 2 || i+2+size > len(data) {
			return 1
		}
		if seg := data[i+4 : i+2+size]; marker == 0xE1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")) {
			return tiffOrientation(seg[6:])
		}
		i += 2 + size
	}
	return 1
}

// tiffOrientation finds 0x0112 in IFD0 of a TIFF header, in either byte order.
func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(t[2:]) != 42 {
		return 1
	}
	ifd := int64(order.Uint32(t[4:]))
	if ifd < 8 || ifd+2 > int64(len(t)) {
		return 1
	}
	n := int64(order.Uint16(t[ifd:]))
	for k := int64(0); k < n; k++ {
		e := ifd + 2 + 12*k
		if e+12 > int64(len(t)) {
			return 1
		}
		if order.Uint16(t[e:]) != 0x0112 {
			continue
		}
		// SHORT, one value, kept in the first two bytes of the value field.
		if order.Uint16(t[e+2:]) != 3 || order.Uint32(t[e+4:]) != 1 {
			return 1
		}
		if v := int(order.Uint16(t[e+8:])); v >= 1 && v <= 8 {
			return v
		}
		return 1
	}
	return 1
}
