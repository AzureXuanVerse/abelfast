package protocol

import "encoding/binary"

/*
Frame layout (see packer.lua):
  - 2 bytes (uint16) : size of the rest of the frame
  - 1 byte  (uint8)  : 0x00
  - 2 bytes (uint16) : packet id
  - 2 bytes (uint16) : packet index, echoed back in replies
  - rest             : protobuf payload
*/
const HeaderSize = 7

// MinFrameSize is the smallest value of the size field: a frame with an empty payload.
const MinFrameSize = HeaderSize - 2

type Header struct {
	ID    int
	Index int
}

// ParseHeader reads the header of a complete frame (at least HeaderSize bytes).
func ParseHeader(frame []byte) Header {
	return Header{
		ID:    int(binary.BigEndian.Uint16(frame[3:5])),
		Index: int(binary.BigEndian.Uint16(frame[5:7])),
	}
}

// AppendFrame appends a framed packet to dst.
func AppendFrame(dst []byte, id int, index int, payload []byte) []byte {
	dst = binary.BigEndian.AppendUint16(dst, uint16(len(payload)+MinFrameSize))
	dst = append(dst, 0x00)
	dst = binary.BigEndian.AppendUint16(dst, uint16(id))
	dst = binary.BigEndian.AppendUint16(dst, uint16(index))
	return append(dst, payload...)
}
