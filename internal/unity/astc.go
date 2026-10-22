package unity

// ASTC block decoding, ported from texture2ddecoder's src/astc.rs
// (https://github.com/UniversalGameExtraction/texture2ddecoder, MIT
// licence), itself a port of Perfare's Texture2DDecoder. The structure,
// tables and rounding follow it so the output matches what UnityPy exports.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// decodeASTC decodes ASTC LDR/HDR blocks of blockWidth x blockHeight into
// RGBA bytes (R, G, B, A per pixel, width*height*4), rows in the order the
// texture stores them (no flip).
func decodeASTC(data []byte, width, height, blockWidth, blockHeight int) (out []byte, err error) {
	if width <= 0 || height <= 0 || blockWidth < 2 || blockHeight < 2 {
		return nil, fmt.Errorf("astc: bad size %dx%d, block %dx%d", width, height, blockWidth, blockHeight)
	}
	if blockWidth*blockHeight > 144 {
		return nil, fmt.Errorf("astc: block %dx%d is too big", blockWidth, blockHeight)
	}
	blocksX := (width + blockWidth - 1) / blockWidth
	blocksY := (height + blockHeight - 1) / blockHeight
	if len(data) < blocksX*blocksY*16 {
		return nil, fmt.Errorf("astc: %d bytes for %d blocks", len(data), blocksX*blocksY)
	}
	// a malformed block can index past its tables; report it instead of
	// taking the caller down
	defer func() {
		if recovered := recover(); recovered != nil {
			out, err = nil, fmt.Errorf("astc: corrupt block: %v", recovered)
		}
	}()

	out = make([]byte, width*height*4)
	var block [144][4]uint8
	offset := 0
	for by := range blocksY {
		for bx := range blocksX {
			decodeASTCBlock(data[offset:], blockWidth, blockHeight, &block)
			offset += 16
			x := bx * blockWidth
			copyWidth := min(blockWidth, width-x)
			y0 := by * blockHeight
			copyHeight := min(blockHeight, height-y0)
			for row := range copyHeight {
				dst := out[((y0+row)*width+x)*4:]
				for col := range copyWidth {
					copy(dst[col*4:col*4+4], block[row*blockWidth+col][:])
				}
			}
		}
	}
	return out, nil
}

var errASTCFormat = errors.New("unsupported ASTC format")

var weightPrecTableA = [16]int{0, 0, 0, 3, 0, 5, 3, 0, 0, 0, 5, 3, 0, 5, 3, 0}
var weightPrecTableB = [16]int{0, 0, 1, 0, 2, 0, 1, 3, 0, 0, 1, 2, 4, 2, 3, 5}

var cemTableA = [19]int{0, 3, 5, 0, 3, 5, 0, 3, 5, 0, 3, 5, 0, 3, 5, 0, 3, 0, 0}
var cemTableB = [19]int{8, 6, 5, 7, 5, 4, 6, 4, 3, 5, 3, 2, 4, 2, 1, 3, 1, 2, 1}

// astcBlock is one block's decoded parameters.
type astcBlock struct {
	bw, bh           int // block footprint
	width, height    int // weight grid
	partNum          int
	dualPlane        bool
	planeSelector    int
	weightRange      int
	weightNum        int
	cem              [4]int
	cemRange         int
	endpointValueNum int
	endpoints        [4][8]int
	weights          [144][2]int
	partition        [144]int
}

// intSeq is one value of an integer sequence: its low bits and its trit or
// quint.
type intSeq struct {
	bits, nonbits uint64
}

func decodeASTCBlock(buf []byte, blockWidth, blockHeight int, out *[144][4]uint8) {
	switch {
	case buf[0] == 0xfc && buf[1]&1 == 1:
		// void-extent block: one color
		var c [4]uint8
		if buf[1]&2 != 0 {
			c = [4]uint8{f16ToU8(buf[8:]), f16ToU8(buf[10:]), f16ToU8(buf[12:]), f16ToU8(buf[14:])}
		} else {
			c = [4]uint8{buf[9], buf[11], buf[13], buf[15]}
		}
		for i := range blockWidth * blockHeight {
			out[i] = c
		}
	case (buf[0]&0xc3 == 0xc0 && buf[1]&1 == 1) || buf[0]&0xf == 0:
		// reserved or illegal block mode: the error color
		for i := range blockWidth * blockHeight {
			out[i] = [4]uint8{255, 0, 255, 255}
		}
	default:
		block := astcBlock{bw: blockWidth, bh: blockHeight}
		block.decodeParams(buf)
		block.decodeEndpoints(buf)
		block.decodeWeights(buf)
		if block.partNum > 1 {
			block.selectPartition(buf)
		}
		block.applyColor(out)
	}
}

// getbits reads numBits (at most 25) little-endian bits at bitOffset.
func getbits(buf []byte, bitOffset, numBits int) int {
	start := bitOffset / 8
	end := (bitOffset + numBits + 7) / 8
	var raw [4]byte
	copy(raw[:], buf[start:end])
	return int(int32(binary.LittleEndian.Uint32(raw[:]))>>(bitOffset%8)) & (1<<numBits - 1)
}

// getbits64 reads up to 64 bits at bit, which may be past either end of the
// block's first 64 bits.
func getbits64(buf []byte, bit, length int) uint64 {
	var mask uint64 = 1<<length - 1
	if length == 64 {
		mask = math.MaxUint64
	}
	lo := binary.LittleEndian.Uint64(buf[:8])
	switch {
	case length == 0:
		return 0
	case bit >= 64:
		return binary.LittleEndian.Uint64(buf[8:16]) >> (bit - 64) & mask
	case bit <= 0:
		return lo << -bit & mask
	case bit+length <= 64:
		return lo >> bit & mask
	default:
		return lo>>bit | binary.LittleEndian.Uint64(buf[8:16])<<(64-bit)&mask
	}
}

func bitReverseU8(c uint8, bits int) uint8 {
	return bitReverseTable[c] >> (8 - bits)
}

func bitReverseU64(d uint64, bits int) uint64 {
	ret := uint64(bitReverseTable[d&0xff])<<56 |
		uint64(bitReverseTable[d>>8&0xff])<<48 |
		uint64(bitReverseTable[d>>16&0xff])<<40 |
		uint64(bitReverseTable[d>>24&0xff])<<32 |
		uint64(bitReverseTable[d>>32&0xff])<<24 |
		uint64(bitReverseTable[d>>40&0xff])<<16 |
		uint64(bitReverseTable[d>>48&0xff])<<8 |
		uint64(bitReverseTable[d>>56&0xff])
	return ret >> (64 - bits)
}

func u16At(buf []byte) int {
	return int(binary.LittleEndian.Uint16(buf))
}

// decodeIntSeq reads count values of the integer sequence encoding (a 3 for
// trits, 5 for quints, 0 for plain bits, b bits each) starting at bit
// offset, or ending there when reverse.
func decodeIntSeq(buf []byte, offset, a, b, count int, reverse bool, out []intSeq) {
	if count == 0 {
		return
	}
	mt := [5]int{0, 2, 4, 5, 7}
	mq := [3]int{0, 3, 5}
	mask := uint64(1)<<b - 1
	n := 0
	p := offset
	switch a {
	case 3:
		blockCount := (count + 4) / 5
		lastBlockCount := (count+4)%5 + 1
		blockSize := 8 + 5*b
		lastBlockSize := (blockSize*lastBlockCount + 4) / 5
		for i := range blockCount {
			size := blockSize
			if i == blockCount-1 {
				size = lastBlockSize
			}
			var d uint64
			if reverse {
				d = bitReverseU64(getbits64(buf, p-size, size), size)
			} else {
				d = getbits64(buf, p, size)
			}
			x := d>>b&3 | d>>(b*2)&0xc | d>>(b*3)&0x10 | d>>(b*4)&0x60 | d>>(b*5)&0x80
			for j := range 5 {
				if n < count {
					out[n] = intSeq{bits: d >> (mt[j] + b*j) & mask, nonbits: uint64(tritsTable[j][x])}
					n++
				}
			}
			if reverse {
				p -= blockSize
			} else {
				p += blockSize
			}
		}
	case 5:
		blockCount := (count + 2) / 3
		lastBlockCount := (count+2)%3 + 1
		blockSize := 7 + 3*b
		lastBlockSize := (blockSize*lastBlockCount + 2) / 3
		for i := range blockCount {
			size := blockSize
			if i == blockCount-1 {
				size = lastBlockSize
			}
			var d uint64
			if reverse {
				d = bitReverseU64(getbits64(buf, p-size, size), size)
			} else {
				d = getbits64(buf, p, size)
			}
			x := d>>b&7 | d>>(b*2)&0x18 | d>>(b*3)&0x60
			for j := range 3 {
				if n < count {
					out[n] = intSeq{bits: d >> (mq[j] + b*j) & mask, nonbits: uint64(quintsTable[j][x])}
					n++
				}
			}
			if reverse {
				p -= blockSize
			} else {
				p += blockSize
			}
		}
	default:
		if reverse {
			p -= b
			for ; n < count; n++ {
				out[n] = intSeq{bits: uint64(bitReverseU8(uint8(getbits(buf, p, b)), b))}
				p -= b
			}
		} else {
			for ; n < count; n++ {
				out[n] = intSeq{bits: uint64(getbits(buf, p, b))}
				p += b
			}
		}
	}
}

func (block *astcBlock) decodeParams(buf []byte) {
	block.dualPlane = buf[1]&4 != 0
	block.weightRange = int(buf[0]>>4&1 | buf[1]<<2&8)

	if buf[0]&3 != 0 {
		block.weightRange |= int(buf[0] << 1 & 6)
		switch buf[0] & 0xc {
		case 0:
			block.width = (u16At(buf) >> 7 & 3) + 4
			block.height = int(buf[0]>>5&3) + 2
		case 4:
			block.width = (u16At(buf) >> 7 & 3) + 8
			block.height = int(buf[0]>>5&3) + 2
		case 8:
			block.width = int(buf[0]>>5&3) + 2
			block.height = (u16At(buf) >> 7 & 3) + 8
		case 12:
			if buf[1]&1 != 0 {
				block.width = int(buf[0]>>7&1) + 2
				block.height = int(buf[0]>>5&3) + 2
			} else {
				block.width = int(buf[0]>>5&3) + 2
				block.height = int(buf[0]>>7&1) + 6
			}
		}
	} else {
		block.weightRange |= int(buf[0] >> 1 & 6)
		switch u16At(buf) & 0x180 {
		case 0:
			block.width = 12
			block.height = int(buf[0]>>5&3) + 2
		case 0x80:
			block.width = int(buf[0]>>5&3) + 2
			block.height = 12
		case 0x100:
			block.width = int(buf[0]>>5&3) + 6
			block.height = int(buf[1]>>1&3) + 6
			block.dualPlane = false
			block.weightRange &= 7
		case 0x180:
			if buf[0]&0x20 != 0 {
				block.width, block.height = 10, 6
			} else {
				block.width, block.height = 6, 10
			}
		}
	}

	block.partNum = int(buf[1]>>3&3) + 1
	block.weightNum = block.width * block.height
	if block.dualPlane {
		block.weightNum *= 2
	}

	var weightBits int
	switch weightPrecTableA[block.weightRange] {
	case 3:
		weightBits = block.weightNum*weightPrecTableB[block.weightRange] + (block.weightNum*8+4)/5
	case 5:
		weightBits = block.weightNum*weightPrecTableB[block.weightRange] + (block.weightNum*7+2)/3
	default:
		weightBits = block.weightNum * weightPrecTableB[block.weightRange]
	}

	var configBits, cemBase int
	if block.partNum == 1 {
		block.cem[0] = u16At(buf[1:]) >> 5 & 0xf
		configBits = 17
	} else {
		cemBase = u16At(buf[2:]) >> 7 & 3
		if cemBase == 0 {
			cem := int(buf[3] >> 1 & 0xf)
			for i := range block.partNum {
				block.cem[i] = cem
			}
			configBits = 29
		} else {
			for i := range block.partNum {
				block.cem[i] = (int(buf[3]>>(i+1)&1) + cemBase - 1) << 2
			}
			switch block.partNum {
			case 2:
				block.cem[0] |= int(buf[3] >> 3 & 3)
				block.cem[1] |= getbits(buf, 126-weightBits, 2)
			case 3:
				block.cem[0] |= int(buf[3] >> 4 & 1)
				block.cem[0] |= getbits(buf, 122-weightBits, 2) & 2
				block.cem[1] |= getbits(buf, 124-weightBits, 2)
				block.cem[2] |= getbits(buf, 126-weightBits, 2)
			case 4:
				for i := range 4 {
					block.cem[i] |= getbits(buf, 120+i*2-weightBits, 2)
				}
			}
			configBits = 25 + block.partNum*3
		}
	}

	if block.dualPlane {
		configBits += 2
		if cemBase != 0 {
			block.planeSelector = getbits(buf, 130-weightBits-block.partNum*3, 2)
		} else {
			block.planeSelector = getbits(buf, 126-weightBits, 2)
		}
	}

	remainBits := 128 - configBits - weightBits

	block.endpointValueNum = 0
	for i := range block.partNum {
		block.endpointValueNum += (block.cem[i] >> 1 & 6) + 2
	}

	// the largest endpoint range that fits in the bits left
	for i := range cemTableA {
		var endpointBits int
		switch cemTableA[i] {
		case 3:
			endpointBits = block.endpointValueNum*cemTableB[i] + (block.endpointValueNum*8+4)/5
		case 5:
			endpointBits = block.endpointValueNum*cemTableB[i] + (block.endpointValueNum*7+2)/3
		default:
			endpointBits = block.endpointValueNum * cemTableB[i]
		}
		if endpointBits <= remainBits {
			block.cemRange = i
			break
		}
	}
}

func setEndpoint(e *[8]int, r1, g1, b1, a1, r2, g2, b2, a2 int) {
	*e = [8]int{r1, g1, b1, a1, r2, g2, b2, a2}
}

func setEndpointClamp(e *[8]int, r1, g1, b1, a1, r2, g2, b2, a2 int) {
	*e = [8]int{
		clampInt(r1, 0, 255), clampInt(g1, 0, 255), clampInt(b1, 0, 255), clampInt(a1, 0, 255),
		clampInt(r2, 0, 255), clampInt(g2, 0, 255), clampInt(b2, 0, 255), clampInt(a2, 0, 255),
	}
}

// setEndpointBlue applies blue contraction.
func setEndpointBlue(e *[8]int, r1, g1, b1, a1, r2, g2, b2, a2 int) {
	*e = [8]int{(r1 + b1) >> 1, (g1 + b1) >> 1, b1, a1, (r2 + b2) >> 1, (g2 + b2) >> 1, b2, a2}
}

func setEndpointBlueClamp(e *[8]int, r1, g1, b1, a1, r2, g2, b2, a2 int) {
	*e = [8]int{
		clampInt((r1+b1)>>1, 0, 255), clampInt((g1+b1)>>1, 0, 255), clampInt(b1, 0, 255), clampInt(a1, 0, 255),
		clampInt((r2+b2)>>1, 0, 255), clampInt((g2+b2)>>1, 0, 255), clampInt(b2, 0, 255), clampInt(a2, 0, 255),
	}
}

func setEndpointHDRClamp(e *[8]int, r1, g1, b1, a1, r2, g2, b2, a2 int) {
	*e = [8]int{
		clampInt(r1, 0, 0xfff), clampInt(g1, 0, 0xfff), clampInt(b1, 0, 0xfff), clampInt(a1, 0, 0xfff),
		clampInt(r2, 0, 0xfff), clampInt(g2, 0, 0xfff), clampInt(b2, 0, 0xfff), clampInt(a2, 0, 0xfff),
	}
}

func clampInt(v, lo, hi int) int {
	return min(max(v, lo), hi)
}

func bitTransferSigned(v []int, a, b int) {
	v[b] = v[b]>>1 | v[a]&0x80
	v[a] = v[a] >> 1 & 0x3f
	if v[a]&0x20 != 0 {
		v[a] -= 0x40
	}
}

// decodeEndpointsHDR7 is color endpoint mode 7, HDR RGB base+scale.
func decodeEndpointsHDR7(e *[8]int, v []int) {
	modeval := v[2]>>4&0x8 | v[1]>>5&0x4 | v[0]>>6
	var majorComponent, mode int
	switch {
	case modeval&0xc != 0xc:
		majorComponent, mode = modeval>>2, modeval&3
	case modeval != 0xf:
		majorComponent, mode = modeval&3, 4
	default:
		majorComponent, mode = 0, 5
	}
	c := [4]int{v[0] & 0x3f, v[1] & 0x1f, v[2] & 0x1f, v[3] & 0x1f}

	shift := 0
	switch mode {
	case 0:
		c[3] |= v[3] & 0x60
		c[0] |= v[3] >> 1 & 0x40
		c[0] |= v[2] << 1 & 0x80
		c[0] |= v[1] << 3 & 0x300
		c[0] |= v[2] << 5 & 0x400
		shift = 1
	case 1:
		c[1] |= v[1] & 0x20
		c[2] |= v[2] & 0x20
		c[0] |= v[3] >> 1 & 0x40
		c[0] |= v[2] << 1 & 0x80
		c[0] |= v[1] << 2 & 0x100
		c[0] |= v[3] << 4 & 0x600
		shift = 1
	case 2:
		c[3] |= v[3] & 0xe0
		c[0] |= v[2] << 1 & 0xc0
		c[0] |= v[1] << 3 & 0x300
		shift = 2
	case 3:
		c[1] |= v[1] & 0x20
		c[2] |= v[2] & 0x20
		c[3] |= v[3] & 0x60
		c[0] |= v[3] >> 1 & 0x40
		c[0] |= v[2] << 1 & 0x80
		c[0] |= v[1] << 2 & 0x100
		shift = 3
	case 4:
		c[1] |= v[1] & 0x60
		c[2] |= v[2] & 0x60
		c[3] |= v[3] & 0x20
		c[0] |= v[3] >> 1 & 0x40
		c[0] |= v[3] << 1 & 0x80
		shift = 4
	case 5:
		c[1] |= v[1] & 0x60
		c[2] |= v[2] & 0x60
		c[3] |= v[3] & 0x60
		c[0] |= v[3] >> 1 & 0x40
		shift = 5
	}
	for i := range c {
		c[i] <<= shift
	}
	if mode != 5 {
		c[1] = c[0] - c[1]
		c[2] = c[0] - c[2]
	}
	switch majorComponent {
	case 1:
		setEndpointHDRClamp(e, c[1]-c[3], c[0]-c[3], c[2]-c[3], 0x780, c[1], c[0], c[2], 0x780)
	case 2:
		setEndpointHDRClamp(e, c[2]-c[3], c[1]-c[3], c[0]-c[3], 0x780, c[2], c[1], c[0], 0x780)
	default:
		setEndpointHDRClamp(e, c[0]-c[3], c[1]-c[3], c[2]-c[3], 0x780, c[0], c[1], c[2], 0x780)
	}
}

// decodeEndpointsHDR11 is color endpoint modes 11, 14 and 15, HDR RGB.
func decodeEndpointsHDR11(e *[8]int, v []int, alpha1, alpha2 int) {
	majorComponent := v[4]>>7 | v[5]>>6&2
	if majorComponent == 3 {
		setEndpoint(e, v[0]<<4, v[2]<<4, v[4]<<5&0xfe0, alpha1, v[1]<<4, v[3]<<4, v[5]<<5&0xfe0, alpha2)
		return
	}
	mode := v[1]>>7 | v[2]>>6&2 | v[3]>>5&4
	va := v[0] | v[1]<<2&0x100
	vb0 := v[2] & 0x3f
	vb1 := v[3] & 0x3f
	vc := v[1] & 0x3f
	var vd0, vd1 int

	switch mode {
	case 0, 2:
		vd0 = v[4] & 0x7f
		if vd0&0x40 != 0 {
			vd0 |= 0xff80
		}
		vd1 = v[5] & 0x7f
		if vd1&0x40 != 0 {
			vd1 |= 0xff80
		}
	case 1, 3, 5, 7:
		vd0 = v[4] & 0x3f
		if vd0&0x20 != 0 {
			vd0 |= 0xffc0
		}
		vd1 = v[5] & 0x3f
		if vd1&0x20 != 0 {
			vd1 |= 0xffc0
		}
	default:
		vd0 = v[4] & 0x1f
		if vd0&0x10 != 0 {
			vd0 |= 0xffe0
		}
		vd1 = v[5] & 0x1f
		if vd1&0x10 != 0 {
			vd1 |= 0xffe0
		}
	}

	switch mode {
	case 0:
		vb0 |= v[2] & 0x40
		vb1 |= v[3] & 0x40
	case 1:
		vb0 |= v[2] & 0x40
		vb1 |= v[3] & 0x40
		vb0 |= v[4] << 1 & 0x80
		vb1 |= v[5] << 1 & 0x80
	case 2:
		va |= v[2] << 3 & 0x200
		vc |= v[3] & 0x40
	case 3:
		va |= v[4] << 3 & 0x200
		vc |= v[5] & 0x40
		vb0 |= v[2] & 0x40
		vb1 |= v[3] & 0x40
	case 4:
		va |= v[4] << 4 & 0x200
		va |= v[5] << 5 & 0x400
		vb0 |= v[2] & 0x40
		vb1 |= v[3] & 0x40
		vb0 |= v[4] << 1 & 0x80
		vb1 |= v[5] << 1 & 0x80
	case 5:
		va |= v[2] << 3 & 0x200
		va |= v[3] << 4 & 0x400
		vc |= v[5] & 0x40
		vc |= v[4] << 1 & 0x80
	case 6:
		va |= v[4] << 4 & 0x200
		va |= v[5] << 5 & 0x400
		va |= v[4] << 5 & 0x800
		vc |= v[5] & 0x40
		vb0 |= v[2] & 0x40
		vb1 |= v[3] & 0x40
	case 7:
		va |= v[2] << 3 & 0x200
		va |= v[3] << 4 & 0x400
		va |= v[4] << 5 & 0x800
		vc |= v[5] & 0x40
	}

	shamt := mode>>1 ^ 3
	va <<= shamt
	vb0 <<= shamt
	vb1 <<= shamt
	vc <<= shamt
	mult := 1 << shamt
	vd0 *= mult
	vd1 *= mult

	switch majorComponent {
	case 1:
		setEndpointHDRClamp(e, va-vb0-vc-vd0, va-vc, va-vb1-vc-vd1, alpha1, va-vb0, va, va-vb1, alpha2)
	case 2:
		setEndpointHDRClamp(e, va-vb1-vc-vd1, va-vb0-vc-vd0, va-vc, alpha1, va-vb1, va-vb0, va, alpha2)
	default:
		setEndpointHDRClamp(e, va-vc, va-vb0-vc-vd0, va-vb1-vc-vd1, alpha1, va, va-vb0, va-vb1, alpha2)
	}
}

func (block *astcBlock) decodeEndpoints(buf []byte) {
	tritsScale := [7]uint64{0, 204, 93, 44, 22, 11, 5}
	quintsScale := [6]uint64{0, 113, 54, 26, 13, 6}
	var seq [32]intSeq
	var ev [32]int
	offset := 29
	if block.partNum == 1 {
		offset = 17
	}
	a, b := cemTableA[block.cemRange], cemTableB[block.cemRange]
	decodeIntSeq(buf, offset, a, b, block.endpointValueNum, false, seq[:])

	// unquantize to 0..255
	switch a {
	case 3:
		c := tritsScale[b]
		for i := range block.endpointValueNum {
			mask := (seq[i].bits & 1) * 0x1ff
			x := seq[i].bits >> 1
			var d uint64
			switch b {
			case 2:
				d = 0b100010110 * x
			case 3:
				d = x<<7 | x<<2 | x
			case 4:
				d = x<<6 | x
			case 5:
				d = x<<5 | x>>2
			case 6:
				d = x<<4 | x>>4
			}
			ev[i] = int(mask&0x80 | ((seq[i].nonbits*c+d)^mask)>>2)
		}
	case 5:
		c := quintsScale[b]
		for i := range block.endpointValueNum {
			mask := (seq[i].bits & 1) * 0x1ff
			x := seq[i].bits >> 1
			var d uint64
			switch b {
			case 2:
				d = 0b100001100 * x
			case 3:
				d = x<<7 | x<<1 | x>>1
			case 4:
				d = x<<6 | x>>1
			case 5:
				d = x<<5 | x>>3
			}
			ev[i] = int(mask&0x80 | ((seq[i].nonbits*c+d)^mask)>>2)
		}
	default:
		for i := range block.endpointValueNum {
			bits := seq[i].bits
			switch b {
			case 1:
				ev[i] = int(bits * 0xff)
			case 2:
				ev[i] = int(bits * 0x55)
			case 3:
				ev[i] = int(bits<<5 | bits<<2 | bits>>1)
			case 4:
				ev[i] = int(bits<<4 | bits)
			case 5:
				ev[i] = int(bits<<3 | bits>>2)
			case 6:
				ev[i] = int(bits<<2 | bits>>4)
			case 7:
				ev[i] = int(bits<<1 | bits>>6)
			case 8:
				ev[i] = int(bits)
			}
		}
	}

	v := ev[:]
	for cem := range block.partNum {
		e := &block.endpoints[cem]
		switch block.cem[cem] {
		case 0:
			setEndpoint(e, v[0], v[0], v[0], 255, v[1], v[1], v[1], 255)
		case 1:
			l0 := v[0]>>2 | v[1]&0xc0
			l1 := clampInt(l0+v[1]&0x3f, 0, 255)
			setEndpoint(e, l0, l0, l0, 255, l1, l1, l1, 255)
		case 2:
			var y0, y1 int
			if v[0] <= v[1] {
				y0, y1 = v[0]<<4, v[1]<<4
			} else {
				y0, y1 = v[1]<<4+8, v[0]<<4-8
			}
			setEndpoint(e, y0, y0, y0, 0x780, y1, y1, y1, 0x780)
		case 3:
			var y0, d int
			if v[0]&0x80 != 0 {
				y0 = (v[1]&0xe0)<<4 | (v[0]&0x7f)<<2
				d = (v[1] & 0x1f) << 2
			} else {
				y0 = (v[1]&0xf0)<<4 | (v[0]&0x7f)<<1
				d = (v[1] & 0x0f) << 1
			}
			y1 := clampInt(y0+d, 0, 0xfff)
			setEndpoint(e, y0, y0, y0, 0x780, y1, y1, y1, 0x780)
		case 4:
			setEndpoint(e, v[0], v[0], v[0], v[2], v[1], v[1], v[1], v[3])
		case 5:
			bitTransferSigned(v, 1, 0)
			bitTransferSigned(v, 3, 2)
			v[1] += v[0]
			setEndpointClamp(e, v[0], v[0], v[0], v[2], v[1], v[1], v[1], v[2]+v[3])
		case 6:
			setEndpoint(e, v[0]*v[3]>>8, v[1]*v[3]>>8, v[2]*v[3]>>8, 255, v[0], v[1], v[2], 255)
		case 7:
			decodeEndpointsHDR7(e, v)
		case 8:
			if v[0]+v[2]+v[4] <= v[1]+v[3]+v[5] {
				setEndpoint(e, v[0], v[2], v[4], 255, v[1], v[3], v[5], 255)
			} else {
				setEndpointBlue(e, v[1], v[3], v[5], 255, v[0], v[2], v[4], 255)
			}
		case 9:
			bitTransferSigned(v, 1, 0)
			bitTransferSigned(v, 3, 2)
			bitTransferSigned(v, 5, 4)
			if v[1]+v[3]+v[5] >= 0 {
				setEndpointClamp(e, v[0], v[2], v[4], 255, v[0]+v[1], v[2]+v[3], v[4]+v[5], 255)
			} else {
				setEndpointBlueClamp(e, v[0]+v[1], v[2]+v[3], v[4]+v[5], 255, v[0], v[2], v[4], 255)
			}
		case 10:
			setEndpoint(e, v[0]*v[3]>>8, v[1]*v[3]>>8, v[2]*v[3]>>8, v[4], v[0], v[1], v[2], v[5])
		case 11:
			decodeEndpointsHDR11(e, v, 0x780, 0x780)
		case 12:
			if v[0]+v[2]+v[4] <= v[1]+v[3]+v[5] {
				setEndpoint(e, v[0], v[2], v[4], v[6], v[1], v[3], v[5], v[7])
			} else {
				setEndpointBlue(e, v[1], v[3], v[5], v[7], v[0], v[2], v[4], v[6])
			}
		case 13:
			bitTransferSigned(v, 1, 0)
			bitTransferSigned(v, 3, 2)
			bitTransferSigned(v, 5, 4)
			bitTransferSigned(v, 7, 6)
			if v[1]+v[3]+v[5] >= 0 {
				setEndpointClamp(e, v[0], v[2], v[4], v[6], v[0]+v[1], v[2]+v[3], v[4]+v[5], v[6]+v[7])
			} else {
				setEndpointBlueClamp(e, v[0]+v[1], v[2]+v[3], v[4]+v[5], v[6]+v[7], v[0], v[2], v[4], v[6])
			}
		case 14:
			decodeEndpointsHDR11(e, v, v[6], v[7])
		case 15:
			mode := v[6]>>7&1 | v[7]>>6&2
			v[6] &= 0x7f
			v[7] &= 0x7f
			if mode == 3 {
				decodeEndpointsHDR11(e, v, v[6]<<5, v[7]<<5)
			} else {
				v[6] |= v[7] << (mode + 1) & 0x780
				v[7] = (v[7] & (0x3f >> mode)) ^ (0x20 >> mode) - 0x20>>mode
				v[6] <<= 4 - mode
				v[7] <<= 4 - mode
				decodeEndpointsHDR11(e, v, v[6], clampInt(v[6]+v[7], 0, 0xfff))
			}
		default:
			panic(errASTCFormat)
		}
		v = v[(block.cem[cem]/4+1)*2:]
	}
}

func (block *astcBlock) decodeWeights(buf []byte) {
	// sized past the 64 weights (x2 planes) a legal block has: the bilinear
	// infill below reads one row and column past the grid
	var seq [256]intSeq
	var wv [256]int
	a, b := weightPrecTableA[block.weightRange], weightPrecTableB[block.weightRange]
	decodeIntSeq(buf, 128, a, b, block.weightNum, true, seq[:])

	// unquantize to 0..64
	switch {
	case a == 0:
		for i := range block.weightNum {
			bits := seq[i].bits
			switch b {
			case 1:
				if bits != 0 {
					wv[i] = 63
				}
			case 2:
				wv[i] = int(bits<<4 | bits<<2 | bits)
			case 3:
				wv[i] = int(bits<<3 | bits)
			case 4:
				wv[i] = int(bits<<2 | bits>>2)
			case 5:
				wv[i] = int(bits<<1 | bits>>4)
			default:
				panic(errASTCFormat)
			}
			if wv[i] > 32 {
				wv[i]++
			}
		}
	case b == 0:
		s := uint64(16)
		if a == 3 {
			s = 32
		}
		for i := range block.weightNum {
			wv[i] = int(seq[i].nonbits * s)
		}
	default:
		for i := range block.weightNum {
			switch {
			case a == 3 && b == 1:
				wv[i] = int(seq[i].nonbits * 50)
			case a == 3 && b == 2:
				wv[i] = int(seq[i].nonbits * 23)
				if seq[i].bits&2 != 0 {
					wv[i] += 0b1000101
				}
			case a == 3 && b == 3:
				wv[i] = int(seq[i].nonbits*11 + (seq[i].bits<<4|seq[i].bits>>1)&0b1100011)
			case a == 5 && b == 1:
				wv[i] = int(seq[i].nonbits * 28)
			case a == 5 && b == 2:
				wv[i] = int(seq[i].nonbits * 13)
				if seq[i].bits&2 != 0 {
					wv[i] += 0b1000010
				}
			default:
				panic(errASTCFormat)
			}
			mask := (seq[i].bits & 1) * 0x7f
			wv[i] = int(mask&0x20 | (uint64(wv[i])^mask)>>2)
			if wv[i] > 32 {
				wv[i]++
			}
		}
	}

	// infill the weight grid over the block's texels
	ds := (1024 + block.bw/2) / (block.bw - 1)
	dt := (1024 + block.bh/2) / (block.bh - 1)
	pn := 1
	if block.dualPlane {
		pn = 2
	}
	i := 0
	for t := range block.bh {
		for s := range block.bw {
			gs := (ds*s*(block.width-1) + 32) >> 6
			gt := (dt*t*(block.height-1) + 32) >> 6
			fs := gs & 0xf
			ft := gt & 0xf
			v := gs>>4 + (gt>>4)*block.width
			w11 := (fs*ft + 8) >> 4
			w10 := ft - w11
			w01 := fs - w11
			w00 := 16 - fs - ft + w11
			for p := range pn {
				p00 := wv[v*pn+p]
				p01 := wv[(v+1)*pn+p]
				p10 := wv[(v+block.width)*pn+p]
				p11 := wv[(v+block.width+1)*pn+p]
				block.weights[i][p] = (p00*w00 + p01*w01 + p10*w10 + p11*w11 + 8) >> 4
			}
			i++
		}
	}
}

// selectPartition assigns each texel its partition, from the partition
// index hashed with the texel's position.
func (block *astcBlock) selectPartition(buf []byte) {
	smallBlock := block.bw*block.bh < 31
	seed := int(int32(binary.LittleEndian.Uint32(buf))>>13&0x3ff) | (block.partNum-1)<<10

	rnum := uint32(seed)
	rnum ^= rnum >> 15
	rnum -= rnum << 17
	rnum += rnum << 7
	rnum += rnum << 4
	rnum ^= rnum >> 5
	rnum += rnum << 16
	rnum ^= rnum >> 7
	rnum ^= rnum >> 3
	rnum ^= rnum << 6
	rnum ^= rnum >> 17

	var seeds [8]int
	for i := range seeds {
		v := int(rnum >> (i * 4) & 0xf)
		seeds[i] = v * v
	}
	sh := [2]int{5, 5}
	if seed&2 != 0 {
		sh[0] = 4
	}
	if block.partNum == 3 {
		sh[1] = 6
	}
	for i := range seeds {
		if seed&1 != 0 {
			seeds[i] >>= sh[i%2]
		} else {
			seeds[i] >>= sh[1-i%2]
		}
	}

	i := 0
	for t := range block.bh {
		for s := range block.bw {
			x, y := s, t
			if smallBlock {
				x, y = s<<1, t<<1
			}
			a := (seeds[0]*x + seeds[1]*y + int(rnum>>14)) & 0x3f
			b := (seeds[2]*x + seeds[3]*y + int(rnum>>10)) & 0x3f
			c, d := 0, 0
			if block.partNum >= 3 {
				c = (seeds[4]*x + seeds[5]*y + int(rnum>>6)) & 0x3f
			}
			if block.partNum >= 4 {
				d = (seeds[6]*x + seeds[7]*y + int(rnum>>2)) & 0x3f
			}
			switch {
			case a >= b && a >= c && a >= d:
				block.partition[i] = 0
			case b >= c && b >= d:
				block.partition[i] = 1
			case c >= d:
				block.partition[i] = 2
			default:
				block.partition[i] = 3
			}
			i++
		}
	}
}

// selectColor interpolates two LDR endpoint values by a 0..64 weight.
func selectColor(v0, v1, weight int) uint8 {
	return uint8(((((v0<<8|v0)*(64-weight)+(v1<<8|v1)*weight+32)>>6)*255 + 32768) / 65536)
}

// selectColorHDR interpolates two HDR endpoint values (12-bit LNS) and
// clamps the result to 0..255.
func selectColorHDR(v0, v1, weight int) uint8 {
	c := uint16(((v0<<4)*(64-weight) + (v1<<4)*weight + 32) >> 6)
	m := c & 0x7ff
	switch {
	case m < 512:
		m *= 3
	case m < 1536:
		m = 4*m - 512
	default:
		m = 5*m - 2048
	}
	f := f16ToF32(c>>1&0x7c00 | m>>3)
	if math.IsInf(float64(f), 0) || math.IsNaN(float64(f)) {
		return 255
	}
	return uint8(clampInt(int(floorF32(f*255)), 0, 255))
}

// colorFunc picks each color endpoint mode's interpolation, for RGB and for
// alpha (mode 14 has LDR alpha).
var colorFunc = [16]func(int, int, int) uint8{
	selectColor, selectColor, selectColorHDR, selectColorHDR,
	selectColor, selectColor, selectColor, selectColorHDR,
	selectColor, selectColor, selectColor, selectColorHDR,
	selectColor, selectColor, selectColorHDR, selectColorHDR,
}

var alphaFunc = [16]func(int, int, int) uint8{
	selectColor, selectColor, selectColorHDR, selectColorHDR,
	selectColor, selectColor, selectColor, selectColorHDR,
	selectColor, selectColor, selectColor, selectColorHDR,
	selectColor, selectColor, selectColor, selectColorHDR,
}

func (block *astcBlock) applyColor(out *[144][4]uint8) {
	// the weight plane each channel uses: plane 0, or 1 for the dual plane's
	// selected channel
	var ps [4]int
	if block.dualPlane {
		ps[block.planeSelector] = 1
	}
	for i := range block.bw * block.bh {
		p := 0
		if block.partNum > 1 {
			p = block.partition[i]
		}
		e := &block.endpoints[p]
		cf, af := colorFunc[block.cem[p]], alphaFunc[block.cem[p]]
		out[i] = [4]uint8{
			cf(e[0], e[4], block.weights[i][ps[0]]),
			cf(e[1], e[5], block.weights[i][ps[1]]),
			cf(e[2], e[6], block.weights[i][ps[2]]),
			af(e[3], e[7], block.weights[i][ps[3]]),
		}
	}
}

// floorF32 floors through a saturating float to int32 conversion, like the
// decoder this is ported from.
func floorF32(x float32) float32 {
	i := f32ToI32(x)
	if x < 0 && x != float32(i) {
		i--
	}
	return float32(i)
}

// f32ToI32 converts like Rust's `as i32`: saturating, NaN to 0.
func f32ToI32(x float32) int32 {
	switch {
	case x != x:
		return 0
	case x >= math.MaxInt32:
		return math.MaxInt32
	case x <= math.MinInt32:
		return math.MinInt32
	}
	return int32(x)
}

// f16ToU8 reads a little-endian half float color channel as 0..255.
func f16ToU8(buf []byte) uint8 {
	f := floorF32(f16ToF32(binary.LittleEndian.Uint16(buf)) * 255)
	if f != f {
		return 0
	}
	return uint8(min(max(f, 0), 255))
}

// f16ToF32 converts an IEEE half float.
func f16ToF32(h uint16) float32 {
	sign := uint32(h>>15) << 31
	exp := uint32(h>>10) & 0x1f
	mant := uint32(h) & 0x3ff
	switch {
	case exp == 0x1f:
		return math.Float32frombits(sign | 0x7f800000 | mant<<13)
	case exp == 0 && mant == 0:
		return math.Float32frombits(sign)
	case exp == 0:
		// subnormal: normalize the mantissa
		e := uint32(127 - 15 + 1)
		for mant&0x400 == 0 {
			mant <<= 1
			e--
		}
		return math.Float32frombits(sign | e<<23 | (mant&0x3ff)<<13)
	}
	return math.Float32frombits(sign | (exp+127-15)<<23 | mant<<13)
}

var bitReverseTable = [256]uint8{
	0x00, 0x80, 0x40, 0xC0, 0x20, 0xA0, 0x60, 0xE0, 0x10, 0x90, 0x50, 0xD0, 0x30, 0xB0, 0x70, 0xF0,
	0x08, 0x88, 0x48, 0xC8, 0x28, 0xA8, 0x68, 0xE8, 0x18, 0x98, 0x58, 0xD8, 0x38, 0xB8, 0x78, 0xF8,
	0x04, 0x84, 0x44, 0xC4, 0x24, 0xA4, 0x64, 0xE4, 0x14, 0x94, 0x54, 0xD4, 0x34, 0xB4, 0x74, 0xF4,
	0x0C, 0x8C, 0x4C, 0xCC, 0x2C, 0xAC, 0x6C, 0xEC, 0x1C, 0x9C, 0x5C, 0xDC, 0x3C, 0xBC, 0x7C, 0xFC,
	0x02, 0x82, 0x42, 0xC2, 0x22, 0xA2, 0x62, 0xE2, 0x12, 0x92, 0x52, 0xD2, 0x32, 0xB2, 0x72, 0xF2,
	0x0A, 0x8A, 0x4A, 0xCA, 0x2A, 0xAA, 0x6A, 0xEA, 0x1A, 0x9A, 0x5A, 0xDA, 0x3A, 0xBA, 0x7A, 0xFA,
	0x06, 0x86, 0x46, 0xC6, 0x26, 0xA6, 0x66, 0xE6, 0x16, 0x96, 0x56, 0xD6, 0x36, 0xB6, 0x76, 0xF6,
	0x0E, 0x8E, 0x4E, 0xCE, 0x2E, 0xAE, 0x6E, 0xEE, 0x1E, 0x9E, 0x5E, 0xDE, 0x3E, 0xBE, 0x7E, 0xFE,
	0x01, 0x81, 0x41, 0xC1, 0x21, 0xA1, 0x61, 0xE1, 0x11, 0x91, 0x51, 0xD1, 0x31, 0xB1, 0x71, 0xF1,
	0x09, 0x89, 0x49, 0xC9, 0x29, 0xA9, 0x69, 0xE9, 0x19, 0x99, 0x59, 0xD9, 0x39, 0xB9, 0x79, 0xF9,
	0x05, 0x85, 0x45, 0xC5, 0x25, 0xA5, 0x65, 0xE5, 0x15, 0x95, 0x55, 0xD5, 0x35, 0xB5, 0x75, 0xF5,
	0x0D, 0x8D, 0x4D, 0xCD, 0x2D, 0xAD, 0x6D, 0xED, 0x1D, 0x9D, 0x5D, 0xDD, 0x3D, 0xBD, 0x7D, 0xFD,
	0x03, 0x83, 0x43, 0xC3, 0x23, 0xA3, 0x63, 0xE3, 0x13, 0x93, 0x53, 0xD3, 0x33, 0xB3, 0x73, 0xF3,
	0x0B, 0x8B, 0x4B, 0xCB, 0x2B, 0xAB, 0x6B, 0xEB, 0x1B, 0x9B, 0x5B, 0xDB, 0x3B, 0xBB, 0x7B, 0xFB,
	0x07, 0x87, 0x47, 0xC7, 0x27, 0xA7, 0x67, 0xE7, 0x17, 0x97, 0x57, 0xD7, 0x37, 0xB7, 0x77, 0xF7,
	0x0F, 0x8F, 0x4F, 0xCF, 0x2F, 0xAF, 0x6F, 0xEF, 0x1F, 0x9F, 0x5F, 0xDF, 0x3F, 0xBF, 0x7F, 0xFF,
}

var tritsTable = [5][256]uint8{
	{
		0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 2, 0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 0,
		0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 2, 0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 1,
		0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 2, 0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 2,
		0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 2, 0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 2,
		0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 2, 0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 0,
		0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 2, 0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 1,
		0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 2, 0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 2,
		0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 2, 0, 1, 2, 0, 0, 1, 2, 1, 0, 1, 2, 2, 0, 1, 2, 2,
	},
	{
		0, 0, 0, 0, 1, 1, 1, 0, 2, 2, 2, 0, 2, 2, 2, 0, 0, 0, 0, 1, 1, 1, 1, 1, 2, 2, 2, 1, 0, 0, 0, 0,
		0, 0, 0, 0, 1, 1, 1, 0, 2, 2, 2, 0, 2, 2, 2, 0, 0, 0, 0, 1, 1, 1, 1, 1, 2, 2, 2, 1, 1, 1, 1, 0,
		0, 0, 0, 0, 1, 1, 1, 0, 2, 2, 2, 0, 2, 2, 2, 0, 0, 0, 0, 1, 1, 1, 1, 1, 2, 2, 2, 1, 2, 2, 2, 0,
		0, 0, 0, 0, 1, 1, 1, 0, 2, 2, 2, 0, 2, 2, 2, 0, 0, 0, 0, 1, 1, 1, 1, 1, 2, 2, 2, 1, 2, 2, 2, 0,
		0, 0, 0, 0, 1, 1, 1, 0, 2, 2, 2, 0, 2, 2, 2, 0, 0, 0, 0, 1, 1, 1, 1, 1, 2, 2, 2, 1, 0, 0, 0, 1,
		0, 0, 0, 0, 1, 1, 1, 0, 2, 2, 2, 0, 2, 2, 2, 0, 0, 0, 0, 1, 1, 1, 1, 1, 2, 2, 2, 1, 1, 1, 1, 1,
		0, 0, 0, 0, 1, 1, 1, 0, 2, 2, 2, 0, 2, 2, 2, 0, 0, 0, 0, 1, 1, 1, 1, 1, 2, 2, 2, 1, 2, 2, 2, 1,
		0, 0, 0, 0, 1, 1, 1, 0, 2, 2, 2, 0, 2, 2, 2, 0, 0, 0, 0, 1, 1, 1, 1, 1, 2, 2, 2, 1, 2, 2, 2, 1,
	},
	{
		0, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 2, 2, 2, 2, 2, 1, 1, 1, 2, 1, 1, 1, 2, 1, 1, 1, 2, 0, 0, 0, 2,
		0, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 2, 2, 2, 2, 2, 1, 1, 1, 2, 1, 1, 1, 2, 1, 1, 1, 2, 0, 0, 0, 2,
		0, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 2, 2, 2, 2, 2, 1, 1, 1, 2, 1, 1, 1, 2, 1, 1, 1, 2, 0, 0, 0, 2,
		0, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 2, 2, 2, 2, 2, 1, 1, 1, 2, 1, 1, 1, 2, 1, 1, 1, 2, 2, 2, 2, 2,
		0, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 2, 2, 2, 2, 2, 1, 1, 1, 2, 1, 1, 1, 2, 1, 1, 1, 2, 1, 1, 1, 2,
		0, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 2, 2, 2, 2, 2, 1, 1, 1, 2, 1, 1, 1, 2, 1, 1, 1, 2, 1, 1, 1, 2,
		0, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 2, 2, 2, 2, 2, 1, 1, 1, 2, 1, 1, 1, 2, 1, 1, 1, 2, 1, 1, 1, 2,
		0, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 2, 2, 2, 2, 2, 1, 1, 1, 2, 1, 1, 1, 2, 1, 1, 1, 2, 2, 2, 2, 2,
	},
	{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2,
	},
	{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
	},
}

var quintsTable = [3][128]uint8{
	{
		0, 1, 2, 3, 4, 0, 4, 4, 0, 1, 2, 3, 4, 1, 4, 4, 0, 1, 2, 3, 4, 2, 4, 4, 0, 1, 2, 3, 4, 3, 4, 4,
		0, 1, 2, 3, 4, 0, 4, 0, 0, 1, 2, 3, 4, 1, 4, 1, 0, 1, 2, 3, 4, 2, 4, 2, 0, 1, 2, 3, 4, 3, 4, 3,
		0, 1, 2, 3, 4, 0, 2, 3, 0, 1, 2, 3, 4, 1, 2, 3, 0, 1, 2, 3, 4, 2, 2, 3, 0, 1, 2, 3, 4, 3, 2, 3,
		0, 1, 2, 3, 4, 0, 0, 1, 0, 1, 2, 3, 4, 1, 0, 1, 0, 1, 2, 3, 4, 2, 0, 1, 0, 1, 2, 3, 4, 3, 0, 1,
	},
	{
		0, 0, 0, 0, 0, 4, 4, 4, 1, 1, 1, 1, 1, 4, 4, 4, 2, 2, 2, 2, 2, 4, 4, 4, 3, 3, 3, 3, 3, 4, 4, 4,
		0, 0, 0, 0, 0, 4, 0, 4, 1, 1, 1, 1, 1, 4, 1, 4, 2, 2, 2, 2, 2, 4, 2, 4, 3, 3, 3, 3, 3, 4, 3, 4,
		0, 0, 0, 0, 0, 4, 0, 0, 1, 1, 1, 1, 1, 4, 1, 1, 2, 2, 2, 2, 2, 4, 2, 2, 3, 3, 3, 3, 3, 4, 3, 3,
		0, 0, 0, 0, 0, 4, 0, 0, 1, 1, 1, 1, 1, 4, 1, 1, 2, 2, 2, 2, 2, 4, 2, 2, 3, 3, 3, 3, 3, 4, 3, 3,
	},
	{
		0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 0, 0, 0, 0, 1, 4, 0, 0, 0, 0, 0, 0, 2, 4, 0, 0, 0, 0, 0, 0, 3, 4,
		1, 1, 1, 1, 1, 1, 4, 4, 1, 1, 1, 1, 1, 1, 4, 4, 1, 1, 1, 1, 1, 1, 4, 4, 1, 1, 1, 1, 1, 1, 4, 4,
		2, 2, 2, 2, 2, 2, 4, 4, 2, 2, 2, 2, 2, 2, 4, 4, 2, 2, 2, 2, 2, 2, 4, 4, 2, 2, 2, 2, 2, 2, 4, 4,
		3, 3, 3, 3, 3, 3, 4, 4, 3, 3, 3, 3, 3, 3, 4, 4, 3, 3, 3, 3, 3, 3, 4, 4, 3, 3, 3, 3, 3, 3, 4, 4,
	},
}
