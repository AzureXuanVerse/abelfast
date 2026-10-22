package unity

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"image/png"
	"sort"
	"strings"
)

// The client's Live2D skins (live2d/<painting> bundles) are prefabs of Live2D's
// Cubism SDK for Unity: the model's .moc3 kept as a CubismMoc MonoBehaviour,
// its textures, its physics3.json as a TextAsset and its motions converted to
// Unity AnimationClips driving the CubismParameter / CubismPart components
// of the prefab's GameObjects. Live2D turns them back into the files of a
// standard Cubism 3/4 model (model3.json, moc3, textures, physics3.json,
// motion3.json) as the Cubism web runtime loads them. The approach follows
// Perfare/UnityLive2DExtractor (MIT, Program.cs and CubismMotion3Converter.cs).

// class ids of the objects the prefab is made of
const (
	classGameObject    = 1
	classTransform     = 4
	classTextAsset     = 49
	classAnimationClip = 74
	classMonoScript    = 115
)

// Live2DModel is a Live2D model's files, by path relative to its model3.json.
type Live2DModel struct {
	// Model3 is the model3.json's file name, <name>.model3.json
	Model3 string
	Files  map[string][]byte
	// Motions are the motion files' group names in the model3.json, the idle
	// motion ("Idle") first
	Motions []string
}

// prefab is a serialized file's objects by path id, read on demand.
type prefab struct {
	file    *serializedFile
	objects map[int64]object
	fields  map[int64]map[string]any
}

func (p *prefab) get(pathID int64) map[string]any {
	if fields, ok := p.fields[pathID]; ok {
		return fields
	}
	obj, ok := p.objects[pathID]
	if !ok {
		return nil
	}
	fields, err := p.file.read(obj)
	if err != nil {
		fields = nil
	}
	p.fields[pathID] = fields
	return fields
}

// ptr follows a PPtr field to an object of the same file.
func (p *prefab) ptr(value any) map[string]any {
	ref, _ := value.(map[string]any)
	if ref == nil || asInt(ref["m_FileID"]) != 0 || asInt(ref["m_PathID"]) == 0 {
		return nil
	}
	return p.get(asInt(ref["m_PathID"]))
}

// scriptName is the class name of a MonoBehaviour's script.
func (p *prefab) scriptName(mono map[string]any) string {
	if script := p.ptr(mono["m_Script"]); script != nil {
		return asString(script["m_ClassName"])
	}
	return ""
}

// Live2D converts the bundle's Live2D prefab into a Cubism model named name
// (the painting: the model3.json, moc3 and physics3.json are named after it).
func (bundle *Bundle) Live2D(name string) (*Live2DModel, error) {
	var p *prefab
	var moc []byte
	for _, path := range bundle.serialized {
		file, err := readSerialized(bundle.Files[path])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		candidate := &prefab{file: file, objects: map[int64]object{}, fields: map[int64]map[string]any{}}
		for _, obj := range file.objects {
			candidate.objects[obj.pathID] = obj
		}
		for _, obj := range file.objects {
			if obj.classID != classMono {
				continue
			}
			mono := candidate.get(obj.pathID)
			if mono != nil && candidate.scriptName(mono) == "CubismMoc" {
				moc, _ = mono["_bytes"].([]byte)
				break
			}
		}
		if moc != nil {
			p = candidate
			break
		}
	}
	if p == nil || len(moc) == 0 {
		return nil, errors.New("no CubismMoc in this bundle")
	}

	model := &Live2DModel{Model3: name + ".model3.json", Files: map[string][]byte{}}
	model.Files[name+".moc3"] = moc

	// textures: the moc's texture indices follow the names (texture_00, ...)
	textures, err := bundle.Textures()
	if err != nil {
		return nil, err
	}
	sort.Slice(textures, func(i, j int) bool { return textures[i].Name < textures[j].Name })
	var texturePaths []string
	for _, texture := range textures {
		img, err := texture.Image()
		if err != nil {
			return nil, err
		}
		var out bytes.Buffer
		if err := png.Encode(&out, img); err != nil {
			return nil, err
		}
		file := uniqueName(model.Files, "textures/"+safeName(texture.Name), ".png")
		model.Files[file] = out.Bytes()
		texturePaths = append(texturePaths, file)
	}

	var physics string
	var clips []map[string]any
	var eyeBlink, lipSync []string
	paths := map[uint32]string{}
	for _, obj := range p.file.objects {
		switch obj.classID {
		case classTextAsset:
			fields := p.get(obj.pathID)
			if fields != nil && physics == "" && strings.HasSuffix(strings.ToLower(asString(fields["m_Name"])), ".physics3") {
				physics = name + ".physics3.json"
				model.Files[physics] = []byte(asString(fields["m_Script"]))
			}
		case classAnimationClip:
			if fields := p.get(obj.pathID); fields != nil {
				clips = append(clips, fields)
			}
		case classTransform:
			fields := p.get(obj.pathID)
			if fields != nil && p.ptr(fields["m_Father"]) == nil {
				p.hashPaths(fields, "", paths)
			}
		case classMono:
			fields := p.get(obj.pathID)
			if fields == nil {
				continue
			}
			switch p.scriptName(fields) {
			case "CubismEyeBlinkParameter":
				if object := p.ptr(fields["m_GameObject"]); object != nil {
					eyeBlink = append(eyeBlink, asString(object["m_Name"]))
				}
			case "CubismMouthParameter":
				if object := p.ptr(fields["m_GameObject"]); object != nil {
					lipSync = append(lipSync, asString(object["m_Name"]))
				}
			}
		}
	}

	// the idle motion first, the others by name
	sort.SliceStable(clips, func(i, j int) bool {
		a, b := asString(clips[i]["m_Name"]), asString(clips[j]["m_Name"])
		if (a == "idle") != (b == "idle") {
			return a == "idle"
		}
		return a < b
	})
	var motionPaths []string
	for _, clip := range clips {
		clipName := asString(clip["m_Name"])
		motion, err := convertClip(p, clip, paths)
		if err != nil {
			return nil, fmt.Errorf("animation %s: %w", clipName, err)
		}
		file := uniqueName(model.Files, "motions/"+safeName(clipName), ".motion3.json")
		model.Files[file] = motion
		group := clipName
		if clipName == "idle" {
			group = "Idle"
		}
		model.Motions = append(model.Motions, group)
		motionPaths = append(motionPaths, file)
	}

	model3, err := model3JSON(name+".moc3", texturePaths, physics, model.Motions, motionPaths, eyeBlink, lipSync)
	if err != nil {
		return nil, err
	}
	model.Files[model.Model3] = model3
	return model, nil
}

// hashPaths maps the CRC32 of a transform's path, and of every path suffix
// after a "/", to the path, then does the same for its children: the
// animation clips' bindings name their GameObject by such a hash of its path
// from the animated root (CubismMotion3Converter.CreateBonePathHash).
func (p *prefab) hashPaths(transform map[string]any, parent string, paths map[uint32]string) {
	object := p.ptr(transform["m_GameObject"])
	if object == nil {
		return
	}
	path := asString(object["m_Name"])
	if parent != "" {
		path = parent + "/" + path
	}
	for suffix := path; ; {
		paths[crc32.ChecksumIEEE([]byte(suffix))] = suffix
		i := strings.IndexByte(suffix, '/')
		if i < 0 {
			break
		}
		suffix = suffix[i+1:]
	}
	children, _ := transform["m_Children"].([]any)
	for _, child := range children {
		if fields := p.ptr(child); fields != nil {
			p.hashPaths(fields, path, paths)
		}
	}
}

// model3JSON writes the model3.json, its motion groups in order.
func model3JSON(moc string, textures []string, physics string, groups, motions []string, eyeBlink, lipSync []string) ([]byte, error) {
	type group struct {
		Target string   `json:"Target"`
		Name   string   `json:"Name"`
		IDs    []string `json:"Ids"`
	}
	var motionGroups bytes.Buffer
	motionGroups.WriteByte('{')
	for i, name := range groups {
		if i > 0 {
			motionGroups.WriteByte(',')
		}
		key, _ := json.Marshal(name)
		entry, _ := json.Marshal([]map[string]any{{"File": motions[i]}})
		motionGroups.Write(key)
		motionGroups.WriteByte(':')
		motionGroups.Write(entry)
	}
	motionGroups.WriteByte('}')
	references := struct {
		Moc      string          `json:"Moc"`
		Textures []string        `json:"Textures"`
		Physics  string          `json:"Physics,omitempty"`
		Motions  json.RawMessage `json:"Motions"`
	}{moc, textures, physics, motionGroups.Bytes()}
	var parameterGroups []group
	if len(eyeBlink) > 0 {
		parameterGroups = append(parameterGroups, group{"Parameter", "EyeBlink", eyeBlink})
	}
	if len(lipSync) > 0 {
		parameterGroups = append(parameterGroups, group{"Parameter", "LipSync", lipSync})
	}
	return json.MarshalIndent(struct {
		Version        int     `json:"Version"`
		FileReferences any     `json:"FileReferences"`
		Groups         []group `json:"Groups,omitempty"`
	}{3, references, parameterGroups}, "", "  ")
}

// safeName keeps a client asset name usable in a URL path segment.
func safeName(name string) string {
	clean := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' {
			return r
		}
		return '_'
	}, name)
	if strings.Trim(clean, ".") == "" {
		clean = "_" + clean
	}
	return clean
}

// uniqueName is base+ext, numbered when files already has it.
func uniqueName(files map[string][]byte, base, ext string) string {
	name := base + ext
	for i := 2; ; i++ {
		if _, taken := files[name]; !taken {
			return name
		}
		name = fmt.Sprintf("%s_%d%s", base, i, ext)
	}
}
