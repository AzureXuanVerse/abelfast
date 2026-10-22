// Package unity reads the client's Unity asset bundles (UnityFS) far enough
// to take the textures and fonts out of them: the archive, its serialized
// file read through the type trees it carries, and the texture formats the
// client's art uses.
package unity

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/pierrec/lz4/v4"
)

// archive flags (ArchiveFlags in Unity's AssetBundle format)
const (
	compressionMask        = 0x3f
	blocksInfoAtEnd        = 0x80
	blocksInfoPaddingStart = 0x200
)

// compression kinds of the block info and the blocks
const (
	compressionNone  = 0
	compressionLZMA  = 1
	compressionLZ4   = 2
	compressionLZ4HC = 3
)

// nodeSerializedFile marks a node holding a serialized file (the CAB-...
// file the objects are in), the others are resources it points into.
const nodeSerializedFile = 0x4

// Bundle is a UnityFS asset bundle's files by path (the serialized file
// CAB-..., its .resS, ...).
type Bundle struct {
	Files map[string][]byte

	// serialized are the paths of the serialized files among Files
	serialized []string
}

// ReadBundle unpacks a UnityFS asset bundle.
func ReadBundle(data []byte) (*Bundle, error) {
	r := &reader{data: data, bigEndian: true}
	if signature := r.cstring(); signature != "UnityFS" {
		return nil, fmt.Errorf("not a UnityFS bundle (signature %q)", signature)
	}
	format := r.u32()
	r.cstring() // unity version, "5.x.x" in the client's bundles
	r.cstring() // unity revision
	r.i64()     // bundle size
	compressedInfo := int(r.u32())
	uncompressedInfo := int(r.u32())
	flags := r.u32()
	if format >= 7 {
		r.align(16)
	}
	if r.err != nil {
		return nil, fmt.Errorf("bundle header: %w", r.err)
	}

	var packedInfo []byte
	if flags&blocksInfoAtEnd != 0 {
		packedInfo = (&reader{data: data, pos: len(data) - compressedInfo}).bytes(compressedInfo)
	} else {
		packedInfo = r.bytes(compressedInfo)
	}
	info, err := decompress(packedInfo, uncompressedInfo, flags&compressionMask)
	if err != nil {
		return nil, fmt.Errorf("block info: %w", err)
	}
	if flags&blocksInfoPaddingStart != 0 {
		r.align(16)
	}

	ir := &reader{data: info, bigEndian: true}
	ir.bytes(16) // uncompressed data hash
	type block struct {
		uncompressed, compressed int
		flags                    uint16
	}
	blocks := make([]block, ir.i32())
	for i := range blocks {
		blocks[i] = block{int(ir.u32()), int(ir.u32()), ir.u16()}
	}
	type node struct {
		offset, size int64
		flags        uint32
		path         string
	}
	nodes := make([]node, ir.i32())
	for i := range nodes {
		nodes[i] = node{ir.i64(), ir.i64(), ir.u32(), ir.cstring()}
	}
	if ir.err != nil {
		return nil, fmt.Errorf("block info: %w", ir.err)
	}

	var stream bytes.Buffer
	for i, b := range blocks {
		raw := r.bytes(b.compressed)
		if r.err != nil {
			return nil, fmt.Errorf("block %d: %w", i, r.err)
		}
		unpacked, err := decompress(raw, b.uncompressed, uint32(b.flags)&compressionMask)
		if err != nil {
			return nil, fmt.Errorf("block %d: %w", i, err)
		}
		stream.Write(unpacked)
	}

	bundle := &Bundle{Files: make(map[string][]byte, len(nodes))}
	all := stream.Bytes()
	for _, n := range nodes {
		if n.offset < 0 || n.size < 0 || n.offset+n.size > int64(len(all)) {
			return nil, fmt.Errorf("node %s: %d bytes at %d past the %d bytes of data", n.path, n.size, n.offset, len(all))
		}
		bundle.Files[n.path] = all[n.offset : n.offset+n.size]
		if n.flags&nodeSerializedFile != 0 {
			bundle.serialized = append(bundle.serialized, n.path)
		}
	}
	return bundle, nil
}

// decompress unpacks a block of the given compression kind.
func decompress(data []byte, size int, kind uint32) ([]byte, error) {
	switch kind {
	case compressionNone:
		return data, nil
	case compressionLZ4, compressionLZ4HC:
		out := make([]byte, size)
		n, err := lz4.UncompressBlock(data, out)
		if err != nil {
			return nil, fmt.Errorf("lz4: %w", err)
		}
		if n != size {
			return nil, fmt.Errorf("lz4: got %d bytes, want %d", n, size)
		}
		return out, nil
	case compressionLZMA:
		return nil, errors.New("LZMA compressed bundles are not supported")
	}
	return nil, fmt.Errorf("unknown compression %d", kind)
}
