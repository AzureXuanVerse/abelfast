package unity

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// reader reads the bundle's and serialized files' fields in order. The first
// error sticks: the reads after it return zero values and err tells.
type reader struct {
	data      []byte
	pos       int
	bigEndian bool
	err       error
}

var errShort = errors.New("unexpected end of data")

func (r *reader) bytes(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || r.pos+n > len(r.data) {
		r.err = errShort
		return nil
	}
	out := r.data[r.pos : r.pos+n]
	r.pos += n
	return out
}

func (r *reader) order() binary.ByteOrder {
	if r.bigEndian {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

func (r *reader) u8() uint8 {
	if b := r.bytes(1); b != nil {
		return b[0]
	}
	return 0
}

func (r *reader) u16() uint16 {
	if b := r.bytes(2); b != nil {
		return r.order().Uint16(b)
	}
	return 0
}

func (r *reader) u32() uint32 {
	if b := r.bytes(4); b != nil {
		return r.order().Uint32(b)
	}
	return 0
}

func (r *reader) u64() uint64 {
	if b := r.bytes(8); b != nil {
		return r.order().Uint64(b)
	}
	return 0
}

func (r *reader) i32() int32 { return int32(r.u32()) }

func (r *reader) i64() int64 { return int64(r.u64()) }

func (r *reader) cstring() string {
	if r.err != nil {
		return ""
	}
	end := bytes.IndexByte(r.data[r.pos:], 0)
	if end < 0 {
		r.err = errShort
		return ""
	}
	s := string(r.data[r.pos : r.pos+end])
	r.pos += end + 1
	return s
}

// align skips to the next multiple of n from the start of the data.
func (r *reader) align(n int) {
	if pad := (n - r.pos%n) % n; pad > 0 {
		r.bytes(pad)
	}
}

// commonStrings is Unity's shared type tree string buffer: node type and
// field names with the high bit of their offset set point into it.
const commonStrings = "AABB\x00AnimationClip\x00AnimationCurve\x00AnimationState\x00Array\x00Base\x00BitField\x00bitset\x00bool\x00char\x00ColorRGBA\x00Component\x00data\x00deque\x00double\x00dynamic_array\x00FastPropertyName\x00first\x00float\x00Font\x00GameObject\x00Generic Mono\x00GradientNEW\x00GUID\x00GUIStyle\x00int\x00list\x00long long\x00map\x00Matrix4x4f\x00MdFour\x00MonoBehaviour\x00MonoScript\x00m_ByteSize\x00m_Curve\x00m_EditorClassIdentifier\x00m_EditorHideFlags\x00m_Enabled\x00m_ExtensionPtr\x00m_GameObject\x00m_Index\x00m_IsArray\x00m_IsStatic\x00m_MetaFlag\x00m_Name\x00m_ObjectHideFlags\x00m_PrefabInternal\x00m_PrefabParentObject\x00m_Script\x00m_StaticEditorFlags\x00m_Type\x00m_Version\x00Object\x00pair\x00PPtr<Component>\x00PPtr<GameObject>\x00PPtr<Material>\x00PPtr<MonoBehaviour>\x00PPtr<MonoScript>\x00PPtr<Object>\x00PPtr<Prefab>\x00PPtr<Sprite>\x00PPtr<TextAsset>\x00PPtr<Texture>\x00PPtr<Texture2D>\x00PPtr<Transform>\x00Prefab\x00Quaternionf\x00Rectf\x00RectInt\x00RectOffset\x00second\x00set\x00short\x00size\x00SInt16\x00SInt32\x00SInt64\x00SInt8\x00staticvector\x00string\x00TextAsset\x00TextMesh\x00Texture\x00Texture2D\x00Transform\x00TypelessData\x00UInt16\x00UInt32\x00UInt64\x00UInt8\x00unsigned int\x00unsigned long long\x00unsigned short\x00vector\x00Vector2f\x00Vector3f\x00Vector4f\x00m_ScriptingClassIdentifier\x00Gradient\x00Type*\x00int2_storage\x00int3_storage\x00BoundsInt\x00m_CorrespondingSourceObject\x00m_PrefabInstance\x00m_PrefabAsset\x00FileSize\x00Hash128\x00RenderingLayerMask\x00fixed_array\x00EntityId\x00LoadableObjectId\x00LoadableSceneId\x00"

// typeNode is a field of a type tree: its type name, field name, children
// and whether it is aligned to 4 bytes after being read.
type typeNode struct {
	typ, name string
	byteSize  int32
	metaFlag  int32
	children  []*typeNode
}

// alignAfter is the kAlignBytes meta flag.
func (node *typeNode) alignAfter() bool { return node.metaFlag&0x4000 != 0 }

// object is a serialized object: its class and where its fields are.
type object struct {
	classID int32
	tree    *typeNode
	data    []byte
	pathID  int64
}

// serializedFile is the part of a serialized file that's needed to read its
// objects.
type serializedFile struct {
	objects   []object
	bigEndian bool
}

// class ids of the objects read
const (
	classTexture2D = 28
	classFont      = 128
	classMono      = 114
)

// readSerialized parses a serialized file's header and metadata.
func readSerialized(data []byte) (*serializedFile, error) {
	r := &reader{data: data, bigEndian: true}
	r.u32() // metadata size
	r.u32() // file size
	version := r.u32()
	dataOffset := int64(r.u32())
	if version < 19 {
		return nil, fmt.Errorf("serialized file version %d is older than the 2019+ layout this reads", version)
	}
	bigEndian := r.u8() != 0
	r.bytes(3)
	if version >= 22 {
		r.u32() // metadata size
		r.i64() // file size
		dataOffset = r.i64()
		r.i64()
	}
	r.bigEndian = bigEndian

	r.cstring() // unity version
	r.i32()     // target platform
	typeTrees := r.u8() != 0
	if !typeTrees {
		return nil, errors.New("the serialized file has no type trees")
	}

	type serializedType struct {
		classID int32
		tree    *typeNode
	}
	types := make([]serializedType, r.i32())
	for i := range types {
		classID := r.i32()
		r.u8()  // stripped
		r.u16() // script type index
		if classID == classMono {
			r.bytes(16) // script id
		}
		r.bytes(16) // type hash
		tree, err := readTypeTree(r, version)
		if err != nil {
			return nil, fmt.Errorf("type %d (class %d): %w", i, classID, err)
		}
		if version >= 21 {
			r.bytes(4 * int(r.i32())) // type dependencies
		}
		types[i] = serializedType{classID, tree}
	}

	file := &serializedFile{bigEndian: bigEndian}
	count := int(r.i32())
	for i := 0; i < count && r.err == nil; i++ {
		r.align(4)
		pathID := r.i64()
		var start int64
		if version >= 22 {
			start = r.i64()
		} else {
			start = int64(r.u32())
		}
		size := int64(r.u32())
		typeID := int(r.i32())
		if r.err != nil {
			break
		}
		if typeID < 0 || typeID >= len(types) {
			return nil, fmt.Errorf("object %d: type %d of %d", i, typeID, len(types))
		}
		begin := dataOffset + start
		if begin < 0 || size < 0 || begin+size > int64(len(data)) {
			return nil, fmt.Errorf("object %d: %d bytes at %d past the file's %d", i, size, begin, len(data))
		}
		file.objects = append(file.objects, object{types[typeID].classID, types[typeID].tree, data[begin : begin+size], pathID})
	}
	if r.err != nil {
		return nil, fmt.Errorf("serialized file metadata: %w", r.err)
	}
	return file, nil
}

// readTypeTree reads a type tree blob (the flat node list, version 19+
// layout, and its string buffer) into a tree.
func readTypeTree(r *reader, version uint32) (*typeNode, error) {
	nodeCount := int(r.i32())
	stringSize := int(r.i32())
	type flat struct {
		level              uint8
		typeOff, nameOff   uint32
		byteSize, metaFlag int32
	}
	flats := make([]flat, nodeCount)
	for i := range flats {
		r.u16() // version
		level := r.u8()
		r.u8() // type flags
		typeOff, nameOff := r.u32(), r.u32()
		byteSize := r.i32()
		r.i32() // index
		metaFlag := r.i32()
		if version >= 19 {
			r.u64() // ref type hash
		}
		flats[i] = flat{level, typeOff, nameOff, byteSize, metaFlag}
	}
	local := r.bytes(stringSize)
	if r.err != nil {
		return nil, r.err
	}
	str := func(off uint32) string {
		buf := local
		if off&0x80000000 != 0 {
			buf, off = []byte(commonStrings), off&0x7fffffff
		}
		if int(off) >= len(buf) {
			return fmt.Sprint(off)
		}
		end := bytes.IndexByte(buf[off:], 0)
		if end < 0 {
			return string(buf[off:])
		}
		return string(buf[off : int(off)+end])
	}
	if nodeCount == 0 {
		return nil, errors.New("empty type tree")
	}
	// the nodes are listed depth first, each child one level below its parent
	var stack []*typeNode
	var root *typeNode
	for _, f := range flats {
		node := &typeNode{typ: str(f.typeOff), name: str(f.nameOff), byteSize: f.byteSize, metaFlag: f.metaFlag}
		level := int(f.level)
		if level == 0 {
			root = node
			stack = []*typeNode{node}
			continue
		}
		if level > len(stack) || root == nil {
			return nil, fmt.Errorf("node %s at level %d under %d levels", node.name, level, len(stack))
		}
		stack = stack[:level]
		parent := stack[level-1]
		parent.children = append(parent.children, node)
		stack = append(stack, node)
	}
	return root, nil
}

// read reads an object's fields through its type tree.
func (file *serializedFile) read(obj object) (map[string]any, error) {
	r := &reader{data: obj.data, bigEndian: file.bigEndian}
	fields := map[string]any{}
	for _, child := range obj.tree.children {
		fields[child.name] = readValue(r, child)
	}
	if r.err != nil {
		return nil, fmt.Errorf("class %d object: %w", obj.classID, r.err)
	}
	return fields, nil
}

// readValue reads one field: a primitive, a string, a byte blob, an array, a
// map or a structure of named fields.
func readValue(r *reader, node *typeNode) any {
	align := node.alignAfter()
	var value any
	switch node.typ {
	case "SInt8":
		value = int64(int8(r.u8()))
	case "UInt8", "char":
		value = int64(r.u8())
	case "bool":
		value = r.u8() != 0
	case "SInt16", "short":
		value = int64(int16(r.u16()))
	case "UInt16", "unsigned short":
		value = int64(r.u16())
	case "SInt32", "int":
		value = int64(r.i32())
	case "UInt32", "unsigned int", "Type*":
		value = int64(r.u32())
	case "SInt64", "long long":
		value = r.i64()
	case "UInt64", "unsigned long long", "FileSize":
		value = int64(r.u64())
	case "float":
		value = float64(math.Float32frombits(r.u32()))
	case "double":
		value = math.Float64frombits(r.u64())
	case "string":
		value = string(r.bytes(int(r.i32())))
		if len(node.children) > 0 && node.children[0].alignAfter() {
			align = true
		}
	case "TypelessData":
		value = r.bytes(int(r.i32()))
	case "map":
		array := node.children[0]
		align = align || array.alignAfter()
		size := int(r.i32())
		pair := array.children[1]
		entries := make([][2]any, 0, min(size, 1<<16))
		for i := 0; i < size && r.err == nil; i++ {
			entries = append(entries, [2]any{readValue(r, pair.children[0]), readValue(r, pair.children[1])})
		}
		value = entries
	default:
		if len(node.children) > 0 && node.children[0].typ == "Array" {
			array := node.children[0]
			align = align || array.alignAfter()
			size := int(r.i32())
			element := array.children[1]
			if len(element.children) == 0 && element.byteSize == 1 {
				// byte arrays (vector<char> font data, ...) stay one slice
				value = r.bytes(size)
				if element.alignAfter() {
					align = true
				}
				break
			}
			items := make([]any, 0, min(size, 1<<16))
			for i := 0; i < size && r.err == nil; i++ {
				items = append(items, readValue(r, element))
			}
			value = items
			break
		}
		fields := make(map[string]any, len(node.children))
		for _, child := range node.children {
			fields[child.name] = readValue(r, child)
		}
		value = fields
	}
	if align {
		r.align(4)
	}
	return value
}
