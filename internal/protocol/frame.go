package protocol

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

/*
Frame layout (see packer.lua):
  - 2 bytes (uint16) : size of the rest of the frame
  - 1 byte  (uint8)  : 0x00, or 0x01 when the payload is zlib compressed
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

// AppendFrame appends a framed packet to dst. A payload too large for the
// size field is zlib compressed and flagged (the client inflates it, e.g. the
// SC_12001 dock of a commander with hundreds of ships).
func AppendFrame(dst []byte, id int, index int, payload []byte) []byte {
	flag := byte(0x00)
	if len(payload)+MinFrameSize > math.MaxUint16 {
		var compressed bytes.Buffer
		w := zlib.NewWriter(&compressed)
		w.Write(payload)
		w.Close()
		payload, flag = compressed.Bytes(), 0x01
	}
	dst = binary.BigEndian.AppendUint16(dst, uint16(len(payload)+MinFrameSize))
	dst = append(dst, flag)
	dst = binary.BigEndian.AppendUint16(dst, uint16(id))
	dst = binary.BigEndian.AppendUint16(dst, uint16(index))
	return append(dst, payload...)
}

// FrameSize returns the length of the whole frame starting with header (at
// least its 2 byte size field), an error when the size field is too small.
func FrameSize(header []byte) (int, error) {
	size := int(binary.BigEndian.Uint16(header))
	if size < MinFrameSize {
		return 0, fmt.Errorf("invalid frame size %d", size)
	}
	return 2 + size, nil
}

// ReadFrame reads one complete frame from r.
func ReadFrame(r io.Reader) ([]byte, error) {
	var size [2]byte
	if _, err := io.ReadFull(r, size[:]); err != nil {
		return nil, err
	}
	length, err := FrameSize(size[:])
	if err != nil {
		return nil, err
	}
	frame := make([]byte, length)
	copy(frame, size[:])
	if _, err := io.ReadFull(r, frame[2:]); err != nil {
		return nil, err
	}
	return frame, nil
}
