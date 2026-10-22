package unity

import (
	"encoding/binary"
	"fmt"
	"math"
	"path"
)

// classMesh is the Mesh class id.
const classMesh = 43

// Mesh is the part of a Mesh the paintings need: its vertices' positions
// and first UV set, and its triangles as vertex indices.
type Mesh struct {
	Name      string
	PathID    int64
	Positions [][3]float32
	UVs       [][2]float32
	Triangles [][3]uint32
}

// vertex channels (ShaderChannel): position is 0, the first UV set 4
const (
	channelPosition = 0
	channelUV0      = 4
)

// vertexFormatSizes are the byte sizes of the VertexFormat values (Unity
// 2019+): Float, Float16, UNorm8, SNorm8, UNorm16, SNorm16, UInt8, SInt8,
// UInt16, SInt16, UInt32, SInt32.
var vertexFormatSizes = [...]int{4, 2, 1, 1, 2, 2, 1, 1, 2, 2, 4, 4}

// Meshes returns the bundle's meshes.
func (bundle *Bundle) Meshes() ([]Mesh, error) {
	var meshes []Mesh
	for _, name := range bundle.serialized {
		file, err := readSerialized(bundle.Files[name])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for _, obj := range file.objects {
			if obj.classID != classMesh {
				continue
			}
			fields, err := file.read(obj)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			mesh, err := bundle.readMesh(fields)
			if err != nil {
				return nil, fmt.Errorf("mesh %s: %w", asString(fields["m_Name"]), err)
			}
			mesh.PathID = obj.pathID
			meshes = append(meshes, mesh)
		}
	}
	return meshes, nil
}

// readMesh decodes a Mesh's fields: its vertex data (from the object or the
// resource file it is streamed to) or its compressed mesh, and the
// triangles of its submeshes.
func (bundle *Bundle) readMesh(fields map[string]any) (Mesh, error) {
	mesh := Mesh{Name: asString(fields["m_Name"])}
	vertexData, _ := fields["m_VertexData"].(map[string]any)
	count := int(asInt(vertexData["m_VertexCount"]))
	data, _ := vertexData["m_DataSize"].([]byte)
	if stream, ok := fields["m_StreamData"].(map[string]any); ok && len(data) == 0 {
		if source := asString(stream["path"]); source != "" {
			resource, ok := bundle.Files[path.Base(source)]
			if !ok {
				return mesh, fmt.Errorf("streamed from %s, not in the bundle", source)
			}
			offset, size := asInt(stream["offset"]), asInt(stream["size"])
			if offset < 0 || size < 0 || offset+size > int64(len(resource)) {
				return mesh, fmt.Errorf("%d bytes at %d past %s's %d", size, offset, source, len(resource))
			}
			data = resource[offset : offset+size]
		}
	}

	if count > 0 && len(data) > 0 {
		channels, _ := vertexData["m_Channels"].([]any)
		positions, err := readChannel(data, channels, count, channelPosition)
		if err != nil {
			return mesh, fmt.Errorf("positions: %w", err)
		}
		uvs, err := readChannel(data, channels, count, channelUV0)
		if err != nil {
			return mesh, fmt.Errorf("uv: %w", err)
		}
		if positions == nil || uvs == nil {
			return mesh, fmt.Errorf("no positions or no uv")
		}
		mesh.Positions = make([][3]float32, count)
		mesh.UVs = make([][2]float32, count)
		for i := range count {
			copy(mesh.Positions[i][:], positions[i])
			copy(mesh.UVs[i][:], uvs[i])
		}
	} else if compressed, ok := fields["m_CompressedMesh"].(map[string]any); ok {
		if err := mesh.readCompressed(compressed); err != nil {
			return mesh, err
		}
	}
	if len(mesh.Positions) == 0 {
		return mesh, fmt.Errorf("no vertices")
	}

	indices, err := readIndices(fields, len(mesh.Positions))
	if err != nil {
		return mesh, err
	}
	mesh.Triangles = indices
	return mesh, nil
}

// readChannel reads a vertex channel as floats, nil when the mesh has no
// such channel. Each stream holds the channels that name it, interleaved
// per vertex; the streams follow each other, each starting on 16 bytes.
func readChannel(data []byte, channels []any, count, channel int) ([][]float32, error) {
	if channel >= len(channels) {
		return nil, nil
	}
	type info struct{ stream, offset, format, dimension int }
	infos := make([]info, len(channels))
	streams := 0
	for i, raw := range channels {
		fields, _ := raw.(map[string]any)
		infos[i] = info{
			stream:    int(asInt(fields["stream"])),
			offset:    int(asInt(fields["offset"])),
			format:    int(asInt(fields["format"])),
			dimension: int(asInt(fields["dimension"])) & 0xf,
		}
		if infos[i].dimension > 0 {
			if infos[i].format >= len(vertexFormatSizes) {
				return nil, fmt.Errorf("channel %d: vertex format %d", i, infos[i].format)
			}
			streams = max(streams, infos[i].stream+1)
		}
	}
	want := infos[channel]
	if want.dimension == 0 {
		return nil, nil
	}
	start := 0
	stride := 0
	for stream := 0; stream <= want.stream; stream++ {
		stride = 0
		for _, other := range infos {
			if other.stream == stream && other.dimension > 0 {
				stride += other.dimension * vertexFormatSizes[other.format]
			}
		}
		if stream < want.stream {
			start += count * stride
			start = (start + 15) &^ 15
		}
	}
	size := vertexFormatSizes[want.format]
	end := start + (count-1)*stride + want.offset + want.dimension*size
	if end > len(data) {
		return nil, fmt.Errorf("%d vertices of %d bytes at %d past the %d bytes of vertex data", count, stride, start, len(data))
	}
	out := make([][]float32, count)
	for i := range count {
		values := make([]float32, want.dimension)
		at := start + i*stride + want.offset
		for d := range values {
			values[d] = vertexValue(data[at+d*size:], want.format)
		}
		out[i] = values
	}
	return out, nil
}

// vertexValue reads one component of a vertex format as a float.
func vertexValue(b []byte, format int) float32 {
	le := binary.LittleEndian
	switch format {
	case 0:
		return math.Float32frombits(le.Uint32(b))
	case 1:
		return halfToFloat(le.Uint16(b))
	case 2:
		return float32(b[0]) / 255
	case 3:
		return max(float32(int8(b[0]))/127, -1)
	case 4:
		return float32(le.Uint16(b)) / 65535
	case 5:
		return max(float32(int16(le.Uint16(b)))/32767, -1)
	case 6:
		return float32(b[0])
	case 7:
		return float32(int8(b[0]))
	case 8:
		return float32(le.Uint16(b))
	case 9:
		return float32(int16(le.Uint16(b)))
	case 10:
		return float32(le.Uint32(b))
	default:
		return float32(int32(le.Uint32(b)))
	}
}

// halfToFloat widens an IEEE 754 half.
func halfToFloat(h uint16) float32 {
	sign := uint32(h>>15) << 31
	exp := uint32(h>>10) & 0x1f
	frac := uint32(h) & 0x3ff
	switch {
	case exp == 0 && frac == 0:
		return math.Float32frombits(sign)
	case exp == 0:
		// subnormal
		f := float32(frac) / 1024 / 16384
		if sign != 0 {
			f = -f
		}
		return f
	case exp == 0x1f:
		return math.Float32frombits(sign | 0xff<<23 | frac<<13)
	}
	return math.Float32frombits(sign | (exp+112)<<23 | frac<<13)
}

// readIndices reads the triangles of the mesh's submeshes from its index
// buffer (16 or 32-bit indices), offset by each submesh's base vertex.
func readIndices(fields map[string]any, vertices int) ([][3]uint32, error) {
	buffer, _ := fields["m_IndexBuffer"].([]byte)
	if len(buffer) == 0 {
		// a compressed mesh keeps its indices packed
		if compressed, ok := fields["m_CompressedMesh"].(map[string]any); ok {
			packed, _ := compressed["m_Triangles"].(map[string]any)
			all := unpackBits(packed)
			buffer = make([]byte, 4*len(all))
			for i, v := range all {
				binary.LittleEndian.PutUint32(buffer[4*i:], v)
			}
			fields = map[string]any{"m_IndexFormat": int64(1), "m_SubMeshes": fields["m_SubMeshes"]}
		}
	}
	width := 2
	if asInt(fields["m_IndexFormat"]) == 1 {
		width = 4
	}
	index := func(i int) uint32 {
		if width == 4 {
			return binary.LittleEndian.Uint32(buffer[i*4:])
		}
		return uint32(binary.LittleEndian.Uint16(buffer[i*2:]))
	}
	submeshes, _ := fields["m_SubMeshes"].([]any)
	var triangles [][3]uint32
	for n, raw := range submeshes {
		sub, _ := raw.(map[string]any)
		if topology := asInt(sub["topology"]); topology != 0 {
			return nil, fmt.Errorf("submesh %d: topology %d, not triangles", n, topology)
		}
		first := int(asInt(sub["firstByte"])) / width
		count := int(asInt(sub["indexCount"]))
		base := uint32(asInt(sub["baseVertex"]))
		if first < 0 || count < 0 || (first+count)*width > len(buffer) {
			return nil, fmt.Errorf("submesh %d: %d indices at %d past the %d bytes of indices", n, count, first, len(buffer))
		}
		for i := first; i+2 < first+count; i += 3 {
			triangle := [3]uint32{index(i) + base, index(i+1) + base, index(i+2) + base}
			for _, v := range triangle {
				if int(v) >= vertices {
					return nil, fmt.Errorf("submesh %d: vertex %d of %d", n, v, vertices)
				}
			}
			triangles = append(triangles, triangle)
		}
	}
	return triangles, nil
}

// readCompressed decodes a compressed mesh's positions and first UV set
// (PackedBitVectors; the UV info's 4 bits per set say whether it is there
// and its dimension).
func (mesh *Mesh) readCompressed(compressed map[string]any) error {
	vertices, _ := compressed["m_Vertices"].(map[string]any)
	positions := unpackFloats(vertices)
	count := len(positions) / 3
	if count == 0 {
		return fmt.Errorf("no vertices")
	}
	packedUV, _ := compressed["m_UV"].(map[string]any)
	uvs := unpackFloats(packedUV)
	info := uint32(asInt(compressed["m_UVInfo"]))
	start, dimension := 0, 2
	if info != 0 {
		// the first set present, UV0 when its bit is set
		if info&4 == 0 {
			return fmt.Errorf("no uv")
		}
		dimension = 1 + int(info&3)
	}
	if start+dimension*count > len(uvs) || dimension < 2 {
		return fmt.Errorf("%d uv values for %d vertices", len(uvs), count)
	}
	mesh.Positions = make([][3]float32, count)
	mesh.UVs = make([][2]float32, count)
	for i := range count {
		copy(mesh.Positions[i][:], positions[i*3:i*3+3])
		mesh.UVs[i] = [2]float32{uvs[start+i*dimension], uvs[start+i*dimension+1]}
	}
	return nil
}

// unpackBits reads a PackedBitVector's items: m_NumItems values of
// m_BitSize bits each, least significant bits first.
func unpackBits(packed map[string]any) []uint32 {
	count := int(asInt(packed["m_NumItems"]))
	bits := int(asInt(packed["m_BitSize"]))
	data, _ := packed["m_Data"].([]byte)
	if count <= 0 || bits <= 0 || bits > 32 || count*bits > len(data)*8 {
		return nil
	}
	out := make([]uint32, count)
	pos := 0
	for i := range out {
		var value uint64
		for read := 0; read < bits; {
			byteAt, bitAt := pos/8, pos%8
			take := min(bits-read, 8-bitAt)
			value |= uint64(data[byteAt]>>bitAt&(1<<take-1)) << read
			read += take
			pos += take
		}
		out[i] = uint32(value)
	}
	return out
}

// unpackFloats reads a PackedBitVector of floats: each item scales
// m_Range over its bits, from m_Start.
func unpackFloats(packed map[string]any) []float32 {
	raw := unpackBits(packed)
	bits := int(asInt(packed["m_BitSize"]))
	rangeF, _ := packed["m_Range"].(float64)
	start, _ := packed["m_Start"].(float64)
	scale := rangeF / float64(uint64(1)<<bits-1)
	out := make([]float32, len(raw))
	for i, v := range raw {
		out[i] = float32(float64(v)*scale + start)
	}
	return out
}
