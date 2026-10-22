package unity

import (
	"encoding/binary"
	"fmt"
	"image"
	"path"
)

// Unity TextureFormat values Image decodes
const (
	formatAlpha8    = 1
	formatRGB24     = 3
	formatRGBA32    = 4
	formatARGB32    = 5
	formatRGB565    = 7
	formatRGBA4444  = 13
	formatBGRA32    = 14
	formatETCRGB4   = 34
	formatETC2RGB   = 45
	formatETC2RGBA8 = 47
)

// astcBlocks are the ASTC formats' block sizes: ASTC_4x4 (48) to 12x12 (53)
// and their HDR variants (66 to 71).
var astcBlocks = map[int][2]int{
	48: {4, 4}, 49: {5, 5}, 50: {6, 6}, 51: {8, 8}, 52: {10, 10}, 53: {12, 12},
	66: {4, 4}, 67: {5, 5}, 68: {6, 6}, 69: {8, 8}, 70: {10, 10}, 71: {12, 12},
}

// Texture is a Texture2D's image, as stored.
type Texture struct {
	Name          string
	Width, Height int
	Format        int // Unity TextureFormat
	Data          []byte
}

// Font is a Font's file.
type Font struct {
	Name string
	Data []byte // TTF/OTF bytes
}

// objects reads the bundle's objects of a class through their type trees.
func (bundle *Bundle) objects(classID int32) ([]map[string]any, error) {
	var out []map[string]any
	for _, name := range bundle.serialized {
		file, err := readSerialized(bundle.Files[name])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for _, obj := range file.objects {
			if obj.classID != classID {
				continue
			}
			fields, err := file.read(obj)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			out = append(out, fields)
		}
	}
	return out, nil
}

// Textures returns the bundle's Texture2Ds, their image data read from the
// resource file when it is streamed.
func (bundle *Bundle) Textures() ([]Texture, error) {
	objects, err := bundle.objects(classTexture2D)
	if err != nil {
		return nil, err
	}
	textures := make([]Texture, 0, len(objects))
	for _, fields := range objects {
		texture := Texture{
			Name:   asString(fields["m_Name"]),
			Width:  int(asInt(fields["m_Width"])),
			Height: int(asInt(fields["m_Height"])),
			Format: int(asInt(fields["m_TextureFormat"])),
		}
		texture.Data, _ = fields["image data"].([]byte)
		if stream, ok := fields["m_StreamData"].(map[string]any); ok && len(texture.Data) == 0 {
			if source := asString(stream["path"]); source != "" {
				// "archive:/CAB-<hash>/CAB-<hash>.resS", a file of this bundle
				resource, ok := bundle.Files[path.Base(source)]
				if !ok {
					return nil, fmt.Errorf("texture %s: streamed from %s, not in the bundle", texture.Name, source)
				}
				offset, size := asInt(stream["offset"]), asInt(stream["size"])
				if offset < 0 || size < 0 || offset+size > int64(len(resource)) {
					return nil, fmt.Errorf("texture %s: %d bytes at %d past %s's %d", texture.Name, size, offset, source, len(resource))
				}
				texture.Data = resource[offset : offset+size]
			}
		}
		textures = append(textures, texture)
	}
	return textures, nil
}

// Fonts returns the bundle's fonts.
func (bundle *Bundle) Fonts() ([]Font, error) {
	objects, err := bundle.objects(classFont)
	if err != nil {
		return nil, err
	}
	fonts := make([]Font, 0, len(objects))
	for _, fields := range objects {
		data, _ := fields["m_FontData"].([]byte)
		fonts = append(fonts, Font{Name: asString(fields["m_Name"]), Data: data})
	}
	return fonts, nil
}

// Image decodes the texture, top row first (Unity stores rows bottom-up).
func (texture Texture) Image() (*image.NRGBA, error) {
	w, h := texture.Width, texture.Height
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("texture %s: size %dx%d", texture.Name, w, h)
	}
	var rgba []byte
	if block, ok := astcBlocks[texture.Format]; ok {
		var err error
		rgba, err = decodeASTC(texture.Data, w, h, block[0], block[1])
		if err != nil {
			return nil, fmt.Errorf("texture %s: %w", texture.Name, err)
		}
	} else if texture.Format == formatETCRGB4 || texture.Format == formatETC2RGB || texture.Format == formatETC2RGBA8 {
		var err error
		rgba, err = decodeETC(texture.Data, w, h, texture.Format == formatETCRGB4, texture.Format == formatETC2RGBA8)
		if err != nil {
			return nil, fmt.Errorf("texture %s: %w", texture.Name, err)
		}
	} else {
		pixel, ok := pixelSizes[texture.Format]
		if !ok {
			return nil, fmt.Errorf("texture %s: format %s is not supported", texture.Name, formatName(texture.Format))
		}
		if len(texture.Data) < w*h*pixel {
			return nil, fmt.Errorf("texture %s: %d bytes for %dx%d %s", texture.Name, len(texture.Data), w, h, formatName(texture.Format))
		}
		rgba = make([]byte, w*h*4)
		for i := 0; i < w*h; i++ {
			unpackPixel(texture.Format, texture.Data[i*pixel:], rgba[i*4:i*4+4])
		}
	}
	if len(rgba) < w*h*4 {
		return nil, fmt.Errorf("texture %s: decoded %d bytes for %dx%d", texture.Name, len(rgba), w, h)
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		copy(img.Pix[y*img.Stride:y*img.Stride+w*4], rgba[(h-1-y)*w*4:(h-y)*w*4])
	}
	return img, nil
}

// pixelSizes are the bytes per pixel of the uncompressed formats.
var pixelSizes = map[int]int{
	formatAlpha8: 1, formatRGB24: 3, formatRGBA32: 4, formatARGB32: 4,
	formatRGB565: 2, formatRGBA4444: 2, formatBGRA32: 4,
}

// unpackPixel writes one pixel of an uncompressed format as RGBA.
func unpackPixel(format int, src, dst []byte) {
	switch format {
	case formatAlpha8:
		dst[0], dst[1], dst[2], dst[3] = 255, 255, 255, src[0]
	case formatRGB24:
		dst[0], dst[1], dst[2], dst[3] = src[0], src[1], src[2], 255
	case formatRGBA32:
		copy(dst, src[:4])
	case formatARGB32:
		dst[0], dst[1], dst[2], dst[3] = src[1], src[2], src[3], src[0]
	case formatBGRA32:
		dst[0], dst[1], dst[2], dst[3] = src[2], src[1], src[0], src[3]
	case formatRGB565:
		v := binary.LittleEndian.Uint16(src)
		r, g, b := v>>11&0x1f, v>>5&0x3f, v&0x1f
		dst[0], dst[1], dst[2], dst[3] = byte(r<<3|r>>2), byte(g<<2|g>>4), byte(b<<3|b>>2), 255
	case formatRGBA4444:
		v := binary.LittleEndian.Uint16(src)
		dst[0], dst[1], dst[2], dst[3] = byte((v>>12&0xf)*0x11), byte((v>>8&0xf)*0x11), byte((v>>4&0xf)*0x11), byte((v&0xf)*0x11)
	}
}

// formatName names a TextureFormat in errors.
func formatName(format int) string {
	if format == 46 {
		return "ETC2_RGBA1 (46)"
	}
	return fmt.Sprintf("TextureFormat %d", format)
}

func asString(value any) string {
	s, _ := value.(string)
	return s
}

func asInt(value any) int64 {
	n, _ := value.(int64)
	return n
}
