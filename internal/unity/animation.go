package unity

import (
	"encoding/json"
	"fmt"
	"math"
)

// Converting a Live2D AnimationClip to a motion3.json, ported from
// Perfare/UnityLive2DExtractor (MIT): CubismMotion3Converter.cs reads the
// clip's muscle clip (streamed, dense and constant curves) into keyframes per
// binding, Program.cs turns keyframes into motion3 segments, and AssetStudio's
// AnimationClip.cs (StreamedClip.ReadData, FindBinding) decodes the streamed
// frames and maps curves to bindings.

// keyframe is a curve's key: Hermite slopes, as Unity keeps them.
type keyframe struct {
	time, value, inSlope, outSlope float32
}

// track is the keys of one motion3 curve.
type track struct {
	target, id string
	keys       []keyframe
}

// animation collects a clip's tracks in the order they're first seen.
type animation struct {
	tracks []*track
	byID   map[string]*track
}

func (anim *animation) track(target, id string) *track {
	key := target + "/" + id
	if t, ok := anim.byID[key]; ok {
		return t
	}
	t := &track{target: target, id: id}
	anim.byID[key] = t
	anim.tracks = append(anim.tracks, t)
	return t
}

// binding is a clip curve's target, resolved to a motion3 curve.
type binding struct {
	target, id string
}

// streamedKey is a key of a streamed frame: the curve's cubic coefficients
// from this key to the next.
type streamedKey struct {
	index   int
	coeff   [4]float32
	inSlope float32
}

type streamedFrame struct {
	time float32
	keys []streamedKey
}

// motion3 targets (curve Target values of the motion3.json format)
const (
	targetParameter = "Parameter"
	targetPart      = "PartOpacity"
	targetModel     = "Model"
)

// convertClip converts an AnimationClip into a motion3.json. paths maps the
// CRC32 of the prefab's GameObject paths to the paths.
func convertClip(p *prefab, clip map[string]any, paths map[uint32]string) ([]byte, error) {
	muscle, _ := clip["m_MuscleClip"].(map[string]any)
	clipData, _ := muscle["m_Clip"].(map[string]any)
	data, _ := clipData["data"].(map[string]any)
	if data == nil {
		return nil, fmt.Errorf("no muscle clip data")
	}
	stopTime := asFloat32(muscle["m_StopTime"])

	bindings := clipBindings(p, clip, paths)
	anim := &animation{byID: map[string]*track{}}
	add := func(curve int, key keyframe) {
		if curve < 0 || curve >= len(bindings) || bindings[curve].id == "" {
			return
		}
		b := bindings[curve]
		t := anim.track(b.target, b.id)
		t.keys = append(t.keys, key)
	}

	streamed, _ := data["m_StreamedClip"].(map[string]any)
	frames, err := readStreamedFrames(asWords(streamed["data"]))
	if err != nil {
		return nil, err
	}
	// the first and last frames are sentinels (at -inf and +inf)
	for i := 1; i < len(frames)-1; i++ {
		for _, key := range frames[i].keys {
			add(key.index, keyframe{frames[i].time, key.coeff[3], key.inSlope, key.coeff[2]})
		}
	}
	streamCount := int(asInt(streamed["curveCount"]))

	dense, _ := data["m_DenseClip"].(map[string]any)
	denseCount := int(asInt(dense["m_CurveCount"]))
	samples := asFloats(dense["m_SampleArray"])
	frameCount := int(asInt(dense["m_FrameCount"]))
	sampleRate, beginTime := asFloat32(dense["m_SampleRate"]), asFloat32(dense["m_BeginTime"])
	for frame := 0; frame < frameCount && sampleRate > 0; frame++ {
		time := beginTime + float32(frame)/sampleRate
		for curve := 0; curve < denseCount; curve++ {
			// UnityLive2DExtractor reads data[curve] here, ignoring the
			// frame's offset: the sample is at frame*count+curve
			if i := frame*denseCount + curve; i < len(samples) {
				add(streamCount+curve, keyframe{time: time, value: samples[i]})
			}
		}
	}

	constant, _ := data["m_ConstantClip"].(map[string]any)
	values := asFloats(constant["data"])
	for _, time := range []float32{0, stopTime} {
		for curve, value := range values {
			add(streamCount+denseCount+curve, keyframe{time: time, value: value})
		}
	}

	type userData struct {
		Time  float32 `json:"Time"`
		Value string  `json:"Value"`
	}
	var events []userData
	userDataSize := 0
	list, _ := clip["m_Events"].([]any)
	for _, item := range list {
		event, _ := item.(map[string]any)
		value := asString(event["data"])
		events = append(events, userData{asFloat32(event["time"]), value})
		userDataSize += len(value)
	}

	type curve struct {
		Target   string    `json:"Target"`
		ID       string    `json:"Id"`
		Segments []float32 `json:"Segments"`
	}
	curves := make([]curve, 0, len(anim.tracks))
	segmentCount, pointCount := 0, 0
	for _, t := range anim.tracks {
		segments, n := motionSegments(t.keys)
		curves = append(curves, curve{t.target, t.id, segments})
		segmentCount += n
		pointCount += pointsIn(segments)
	}
	for _, c := range curves {
		for _, v := range c.Segments {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return nil, fmt.Errorf("curve %s/%s: value %v", c.Target, c.ID, v)
			}
		}
	}
	type meta struct {
		Duration             float32 `json:"Duration"`
		Fps                  float32 `json:"Fps"`
		Loop                 bool    `json:"Loop"`
		AreBeziersRestricted bool    `json:"AreBeziersRestricted"`
		CurveCount           int     `json:"CurveCount"`
		TotalSegmentCount    int     `json:"TotalSegmentCount"`
		TotalPointCount      int     `json:"TotalPointCount"`
		UserDataCount        int     `json:"UserDataCount"`
		TotalUserDataSize    int     `json:"TotalUserDataSize"`
	}
	return json.MarshalIndent(struct {
		Version  int        `json:"Version"`
		Meta     meta       `json:"Meta"`
		Curves   []curve    `json:"Curves"`
		UserData []userData `json:"UserData,omitempty"`
	}{
		Version: 3,
		Meta: meta{
			Duration: stopTime, Fps: asFloat32(clip["m_SampleRate"]), Loop: true, AreBeziersRestricted: true,
			CurveCount: len(curves), TotalSegmentCount: segmentCount, TotalPointCount: pointCount,
			UserDataCount: len(events), TotalUserDataSize: userDataSize,
		},
		Curves:   curves,
		UserData: events,
	}, "", "  ")
}

// motionSegments turns a track's keys into motion3 segments (the first
// point, then a type and its points per segment) and counts the segments,
// as UnityLive2DExtractor's Program.cs does: a key 0.01s after the previous
// one followed by a key of the same value is an inverse stepped segment, an
// infinite in slope a stepped one, flat slopes a linear one, the rest
// Bézier curves with their control points a third of the way along the
// Hermite tangents.
func motionSegments(keys []keyframe) ([]float32, int) {
	segments := []float32{keys[0].time, keys[0].value}
	count := 0
	for j := 1; j < len(keys); j++ {
		key, prev := keys[j], keys[j-1]
		count++
		if abs32(key.time-prev.time-0.01) < 0.0001 && j+1 < len(keys) && keys[j+1].value == key.value {
			next := keys[j+1]
			segments = append(segments, 3, next.time, next.value)
			j++
			continue
		}
		switch {
		case math.IsInf(float64(key.inSlope), 1):
			segments = append(segments, 2, key.time, key.value)
		case prev.outSlope == 0 && abs32(key.inSlope) < 0.0001:
			segments = append(segments, 0, key.time, key.value)
		default:
			tangent := (key.time - prev.time) / 3
			segments = append(segments, 1,
				prev.time+tangent, prev.outSlope*tangent+prev.value,
				key.time-tangent, key.value-key.inSlope*tangent,
				key.time, key.value)
		}
	}
	return segments, count
}

// pointsIn counts the points of a curve's segments: its first point and
// each segment's (three for a Bézier, one otherwise).
func pointsIn(segments []float32) int {
	points := 1
	for i := 2; i < len(segments); {
		if segments[i] == 1 {
			points += 3
			i += 7
		} else {
			points++
			i += 3
		}
	}
	return points
}

// readStreamedFrames decodes a streamed clip's frames (time, key count, then
// per key the curve index and its four coefficients, all 32-bit words) and
// derives each key's in slope from the previous key of its curve
// (AssetStudio's StreamedClip.ReadData and CalculateNextInSlope).
func readStreamedFrames(words []uint32) ([]streamedFrame, error) {
	var frames []streamedFrame
	for i := 0; i < len(words); {
		if i+2 > len(words) {
			return nil, fmt.Errorf("streamed clip: frame header past the end")
		}
		frame := streamedFrame{time: math.Float32frombits(words[i])}
		count := int(int32(words[i+1]))
		i += 2
		if count < 0 || i+count*5 > len(words) {
			return nil, fmt.Errorf("streamed clip: %d keys past the end", count)
		}
		frame.keys = make([]streamedKey, count)
		for k := range frame.keys {
			key := &frame.keys[k]
			key.index = int(int32(words[i]))
			for c := range key.coeff {
				key.coeff[c] = math.Float32frombits(words[i+1+c])
			}
			i += 5
		}
		frames = append(frames, frame)
	}
	for f := 2; f < len(frames)-1; f++ {
		frame := &frames[f]
		for k := range frame.keys {
			key := &frame.keys[k]
		search:
			for g := f - 1; g >= 0; g-- {
				for _, prev := range frames[g].keys {
					if prev.index == key.index {
						key.inSlope = nextInSlope(prev, frame.time-frames[g].time, key.coeff[3])
						break search
					}
				}
			}
		}
	}
	return frames, nil
}

// nextInSlope is the slope at which prev's cubic reaches the next key's
// value dx later; stepped (infinite) when prev's curve is flat.
func nextInSlope(prev streamedKey, dx, value float32) float32 {
	if prev.coeff[0] == 0 && prev.coeff[1] == 0 && prev.coeff[2] == 0 {
		return float32(math.Inf(1))
	}
	dx = max(dx, 0.0001)
	dy := value - prev.coeff[3]
	length := 1 / (dx * dx)
	d1 := prev.coeff[2] * dx
	d2 := dy + dy + dy - d1 - d1 - prev.coeff[1]/length
	return d2 / dx
}

// clipBindings maps each curve index of a clip to its motion3 target: the
// clip's generic bindings in order, a Transform binding taking three or
// four curves (AssetStudio's FindBinding), resolved through the GameObject
// path hashes, Parameters/<id> and Parts/<id> becoming parameter and part
// opacity curves, and the model controllers' own values the Model ones.
func clipBindings(p *prefab, clip map[string]any, paths map[uint32]string) []binding {
	constant, _ := clip["m_ClipBindingConstant"].(map[string]any)
	list, _ := constant["genericBindings"].([]any)
	var out []binding
	for _, item := range list {
		fields, _ := item.(map[string]any)
		curves := 1
		if asInt(fields["typeID"]) == classTransform {
			switch asInt(fields["attribute"]) {
			case 1, 3, 4: // position, scale, euler
				curves = 3
			case 2: // rotation
				curves = 4
			}
		}
		b := resolveBinding(p, fields, paths)
		for range curves {
			out = append(out, b)
		}
	}
	return out
}

func resolveBinding(p *prefab, fields map[string]any, paths map[uint32]string) binding {
	if hash := uint32(asInt(fields["path"])); hash != 0 {
		if path, ok := paths[hash]; ok {
			for i := len(path) - 1; i >= 0; i-- {
				if path[i] != '/' {
					continue
				}
				switch path[:i] {
				case "Parameters":
					return binding{targetParameter, path[i+1:]}
				case "Parts":
					return binding{targetPart, path[i+1:]}
				}
				// other GameObjects (drawables, ...) aren't motion3 curves
				return binding{}
			}
			return binding{}
		}
	}
	if script := p.ptr(fields["script"]); script != nil {
		switch asString(script["m_ClassName"]) {
		case "CubismRenderController":
			return binding{targetModel, "Opacity"}
		case "CubismEyeBlinkController":
			return binding{targetModel, "EyeBlink"}
		case "CubismMouthController":
			return binding{targetModel, "LipSync"}
		}
	}
	return binding{}
}

func asFloat32(value any) float32 {
	f, _ := value.(float64)
	return float32(f)
}

// asFloats reads an array of floats.
func asFloats(value any) []float32 {
	list, _ := value.([]any)
	out := make([]float32, len(list))
	for i, item := range list {
		out[i] = asFloat32(item)
	}
	return out
}

// asWords reads an array of 32-bit unsigned integers.
func asWords(value any) []uint32 {
	list, _ := value.([]any)
	out := make([]uint32, len(list))
	for i, item := range list {
		out[i] = uint32(asInt(item))
	}
	return out
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
