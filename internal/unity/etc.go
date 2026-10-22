package unity

// ETC1 / ETC2 / EAC block decoding (TextureFormat ETC_RGB4, ETC2_RGB,
// ETC2_RGBA8), written from the Khronos ETC2 specification (the OpenGL ES
// 3.0 spec's annex C): 4x4 blocks, an 8-byte colour block (individual,
// differential, and ETC2's T, H and planar modes) preceded for RGBA8 by an
// 8-byte EAC alpha block.

import (
	"encoding/binary"
	"fmt"
)

// etcModifiers are the ETC1 intensity tables: +a, +b, -a, -b by pixel index.
var etcModifiers = [8][4]int{
	{2, 8, -2, -8}, {5, 17, -5, -17}, {9, 29, -9, -29}, {13, 42, -13, -42},
	{18, 60, -18, -60}, {24, 80, -24, -80}, {33, 106, -33, -106}, {47, 183, -47, -183},
}

// etcDistances are the T and H modes' distance table.
var etcDistances = [8]int{3, 6, 11, 16, 23, 32, 41, 64}

// eacModifiers are the EAC alpha tables, by table index and pixel index.
var eacModifiers = [16][8]int{
	{-3, -6, -9, -15, 2, 5, 8, 14}, {-3, -7, -10, -13, 2, 6, 9, 12},
	{-2, -5, -8, -13, 1, 4, 7, 12}, {-2, -4, -6, -13, 1, 3, 5, 12},
	{-3, -6, -8, -12, 2, 5, 7, 11}, {-3, -7, -9, -11, 2, 6, 8, 10},
	{-4, -7, -8, -11, 3, 6, 7, 10}, {-3, -5, -8, -11, 2, 4, 7, 10},
	{-2, -6, -8, -10, 1, 5, 7, 9}, {-2, -5, -8, -10, 1, 4, 7, 9},
	{-2, -4, -8, -10, 1, 3, 7, 9}, {-2, -5, -7, -10, 1, 4, 6, 9},
	{-3, -4, -7, -10, 2, 3, 6, 9}, {-1, -2, -3, -10, 0, 1, 2, 9},
	{-4, -6, -8, -9, 3, 5, 7, 8}, {-3, -5, -7, -9, 2, 4, 6, 8},
}

// decodeETC decodes ETC2 RGB blocks (ETC1 ones when etc1 is set: no T, H or
// planar mode), preceded by EAC alpha blocks when alpha is set, into RGBA
// bytes (width*height*4), rows in the order the texture stores them (no
// flip).
func decodeETC(data []byte, width, height int, etc1, alpha bool) ([]byte, error) {
	blockSize := 8
	if alpha {
		blockSize = 16
	}
	blocksX, blocksY := (width+3)/4, (height+3)/4
	if len(data) < blocksX*blocksY*blockSize {
		return nil, fmt.Errorf("etc: %d bytes for %d blocks", len(data), blocksX*blocksY)
	}
	out := make([]byte, width*height*4)
	var block [16][4]byte
	for by := 0; by < blocksY; by++ {
		for bx := 0; bx < blocksX; bx++ {
			src := data[(by*blocksX+bx)*blockSize:]
			if alpha {
				etcColorBlock(binary.BigEndian.Uint64(src[8:]), etc1, &block)
				eacAlphaBlock(binary.BigEndian.Uint64(src), &block)
			} else {
				etcColorBlock(binary.BigEndian.Uint64(src), etc1, &block)
			}
			for i, pixel := range block {
				// pixels are numbered down each column (index = x*4 + y)
				x, y := bx*4+i/4, by*4+i%4
				if x < width && y < height {
					copy(out[(y*width+x)*4:], pixel[:])
				}
			}
		}
	}
	return out, nil
}

// etcColorBlock decodes a colour block, opaque, into block (by pixel index).
func etcColorBlock(bits uint64, etc1 bool, block *[16][4]byte) {
	field := func(hi, n uint) int { return int(bits >> (hi - n + 1) & (1<<n - 1)) }
	// the pixel index bits: most significant bit plane, then least
	index := func(i int) int { return int(bits>>(16+i)&1)<<1 | int(bits>>i&1) }
	set := func(i, r, g, b int) { block[i] = [4]byte{etcClamp(r), etcClamp(g), etcClamp(b), 255} }

	var base [2][3]int
	if field(33, 1) == 0 {
		// individual: two 4-bit colours
		for c := range 3 {
			base[0][c] = extend4(field(63-uint(c)*8, 4))
			base[1][c] = extend4(field(59-uint(c)*8, 4))
		}
	} else {
		// differential: a 5-bit colour and a signed 3-bit delta, an
		// overflowing sum selecting ETC2's T, H or planar mode
		var five, sum [3]int
		for c := range 3 {
			five[c] = field(63-uint(c)*8, 5)
			delta := field(58-uint(c)*8, 3)
			if delta >= 4 {
				delta -= 8
			}
			sum[c] = five[c] + delta
		}
		switch {
		case etc1:
			// ETC1 has no other mode: the 5-bit sum wraps
			for c := range 3 {
				sum[c] &= 31
			}
		case sum[0] < 0 || sum[0] > 31:
			etcT(field, index, set)
			return
		case sum[1] < 0 || sum[1] > 31:
			etcH(field, index, set)
			return
		case sum[2] < 0 || sum[2] > 31:
			etcPlanar(field, set)
			return
		}
		for c := range 3 {
			base[0][c], base[1][c] = extend5(five[c]), extend5(sum[c])
		}
	}
	tables := [2]int{field(39, 3), field(36, 3)}
	flip := field(32, 1) == 1
	for i := range 16 {
		x, y := i/4, i%4
		sub := x / 2
		if flip {
			sub = y / 2
		}
		m := etcModifiers[tables[sub]][index(i)]
		set(i, base[sub][0]+m, base[sub][1]+m, base[sub][2]+m)
	}
}

// etcT is ETC2's T mode: one colour, and three around a second.
func etcT(field func(uint, uint) int, index func(int) int, set func(int, int, int, int)) {
	c1 := [3]int{extend4(field(60, 2)<<2 | field(57, 2)), extend4(field(55, 4)), extend4(field(51, 4))}
	c2 := [3]int{extend4(field(47, 4)), extend4(field(43, 4)), extend4(field(39, 4))}
	d := etcDistances[field(35, 2)<<1|field(32, 1)]
	paint := [4][3]int{c1, {c2[0] + d, c2[1] + d, c2[2] + d}, c2, {c2[0] - d, c2[1] - d, c2[2] - d}}
	for i := range 16 {
		p := paint[index(i)]
		set(i, p[0], p[1], p[2])
	}
}

// etcH is ETC2's H mode: two colours, each spread by a distance.
func etcH(field func(uint, uint) int, index func(int) int, set func(int, int, int, int)) {
	r1, g1, b1 := field(62, 4), field(58, 3)<<1|field(52, 1), field(51, 1)<<3|field(49, 3)
	r2, g2, b2 := field(46, 4), field(42, 4), field(38, 4)
	order := 0
	if r1<<8|g1<<4|b1 >= r2<<8|g2<<4|b2 {
		order = 1
	}
	d := etcDistances[field(34, 1)<<2|field(32, 1)<<1|order]
	c1 := [3]int{extend4(r1), extend4(g1), extend4(b1)}
	c2 := [3]int{extend4(r2), extend4(g2), extend4(b2)}
	paint := [4][3]int{
		{c1[0] + d, c1[1] + d, c1[2] + d}, {c1[0] - d, c1[1] - d, c1[2] - d},
		{c2[0] + d, c2[1] + d, c2[2] + d}, {c2[0] - d, c2[1] - d, c2[2] - d},
	}
	for i := range 16 {
		p := paint[index(i)]
		set(i, p[0], p[1], p[2])
	}
}

// etcPlanar is ETC2's planar mode: a gradient from the origin colour
// towards the horizontal and vertical ones.
func etcPlanar(field func(uint, uint) int, set func(int, int, int, int)) {
	o := [3]int{extend6(field(62, 6)), extend7(field(56, 1)<<6 | field(54, 6)), extend6(field(48, 1)<<5 | field(44, 2)<<3 | field(41, 3))}
	h := [3]int{extend6(field(38, 5)<<1 | field(32, 1)), extend7(field(31, 7)), extend6(field(24, 6))}
	v := [3]int{extend6(field(18, 6)), extend7(field(12, 7)), extend6(field(5, 6))}
	for i := range 16 {
		x, y := i/4, i%4
		var c [3]int
		for k := range 3 {
			c[k] = (x*(h[k]-o[k]) + y*(v[k]-o[k]) + 4*o[k] + 2) >> 2
		}
		set(i, c[0], c[1], c[2])
	}
}

// eacAlphaBlock decodes an EAC alpha block into block's alpha.
func eacAlphaBlock(bits uint64, block *[16][4]byte) {
	base := int(bits >> 56)
	multiplier := int(bits >> 52 & 0xf)
	table := &eacModifiers[bits>>48&0xf]
	for i := range 16 {
		index := bits >> (45 - 3*i) & 7
		block[i][3] = etcClamp(base + table[index]*multiplier)
	}
}

func extend4(v int) int { return v<<4 | v }
func extend5(v int) int { return v<<3 | v>>2 }
func extend6(v int) int { return v<<2 | v>>4 }
func extend7(v int) int { return v<<1 | v>>6 }

func etcClamp(v int) byte { return byte(min(max(v, 0), 255)) }
