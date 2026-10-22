package unity

import (
	"errors"
	"fmt"
	"image"
	"math"
	"strings"
)

// The client's paintings (painting/<name> bundles) are prefabs of
// MeshImage components: a MeshImage draws a mesh cut out of a packed
// texture (a painting/<...>_tex bundle) over its RectTransform, the mesh's
// vertices given in pixels of the original image (mRawSpriteSize) and its
// UVs in the atlas. The prefab's root is usually the whole painting (or the
// background), its children layers drawn over it ("layers"/<name>_rw, the
// character at a higher resolution, <name>_bj1, ...). Reconstructing one
// mesh follows nobbyfix/AzurLane-AssetDownloader (src/azlassets/imgrecon.py:
// each quad's UV area pasted at its vertex position), done here per
// triangle so any mesh works.

// classRectTransform is the RectTransform class id.
const classRectTransform = 224

// PaintingPrefab is how a painting prefab lays its MeshImages out: the
// canvas, in pixels of the root's own image, and the layers drawn on it in
// order.
type PaintingPrefab struct {
	Width, Height int
	Layers        []PaintingLayer
}

// PaintingLayer is one MeshImage of a painting prefab.
type PaintingLayer struct {
	Name string // its GameObject's name
	// Mesh is the path id of its mesh, in one of the painting's _tex bundles
	Mesh int64
	// Width and Height are its image's size (mRawSpriteSize), the space of
	// its mesh's vertices
	Width, Height int
	// a pixel (x, y) of its image (top row first) lands on the canvas at
	// (ScaleX*x + X, ScaleY*y + Y)
	ScaleX, ScaleY, X, Y float64
	// Color tints it (RGBA, 1 is unchanged)
	Color [4]float64
}

// PaintingPart is a mesh of a _tex bundle and the texture it cuts.
type PaintingPart struct {
	Mesh    Mesh
	Texture Texture
}

// affine maps a node's local coordinates to the root's: x' = sx*x + tx,
// y' = sy*y + ty (y up). The prefabs don't rotate their layers.
type affine struct{ sx, sy, tx, ty float64 }

func (a affine) then(b affine) affine {
	// a applied after b: x -> a(b(x))
	return affine{a.sx * b.sx, a.sy * b.sy, a.sx*b.tx + a.tx, a.sy*b.ty + a.ty}
}

// rect is a RectTransform's rectangle in its own local space.
type rect struct{ minX, minY, w, h float64 }

// PaintingPrefab reads the bundle's painting prefab: its root's rectangle
// as the canvas and the active MeshImages under it. The error is
// ErrNoMeshImage when the prefab draws no MeshImage.
func (bundle *Bundle) PaintingPrefab() (*PaintingPrefab, error) {
	for _, name := range bundle.serialized {
		file, err := readSerialized(bundle.Files[name])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		p := &prefab{file: file, objects: map[int64]object{}, fields: map[int64]map[string]any{}}
		for _, obj := range file.objects {
			p.objects[obj.pathID] = obj
		}
		for _, obj := range file.objects {
			if obj.classID != classRectTransform {
				continue
			}
			root := p.get(obj.pathID)
			if root == nil || p.ptr(root["m_Father"]) != nil {
				continue
			}
			return p.paintingPrefab(root)
		}
	}
	return nil, errors.New("no RectTransform root in this bundle")
}

// ErrNoMeshImage says a prefab draws no MeshImage (an older painting is a
// plain texture).
var ErrNoMeshImage = errors.New("the prefab has no MeshImage")

func (p *prefab) paintingPrefab(root map[string]any) (*PaintingPrefab, error) {
	out := &PaintingPrefab{}
	size := vec2(root["m_SizeDelta"])
	pivot := vec2(root["m_Pivot"])
	rootRect := rect{-pivot[0] * size[0], -pivot[1] * size[1], size[0], size[1]}
	if rootRect.w <= 0 || rootRect.h <= 0 {
		return nil, fmt.Errorf("root rectangle %gx%g", rootRect.w, rootRect.h)
	}

	// the canvas is the root's rectangle at the resolution of the root's
	// own image when it has one, so it is copied pixel for pixel
	kx, ky := 1.0, 1.0
	if mi := p.meshImage(root); mi != nil {
		raw := vec2(mi["mRawSpriteSize"])
		if raw[0] > 0 && raw[1] > 0 {
			kx, ky = raw[0]/rootRect.w, raw[1]/rootRect.h
		}
	} else if long := max(rootRect.w, rootRect.h); long > 2048 {
		kx = 2048 / long
		ky = kx
	}
	out.Width = int(math.Round(rootRect.w * kx))
	out.Height = int(math.Round(rootRect.h * ky))
	// root local (y up) -> canvas pixels (y down)
	canvas := affine{kx, -ky, -rootRect.minX * kx, (rootRect.minY + rootRect.h) * ky}

	var walk func(node map[string]any, local affine, r rect, depth int)
	walk = func(node map[string]any, local affine, r rect, depth int) {
		if depth > 64 {
			return
		}
		gameObject := p.ptr(node["m_GameObject"])
		if gameObject == nil {
			return
		}
		if active, ok := gameObject["m_IsActive"].(bool); ok && !active {
			return
		}
		if mi := p.meshImage(node); mi != nil {
			if layer, ok := paintingLayer(mi, r, canvas.then(local)); ok {
				layer.Name = asString(gameObject["m_Name"])
				out.Layers = append(out.Layers, layer)
			}
		}
		children, _ := node["m_Children"].([]any)
		for _, ref := range children {
			child := p.ptr(ref)
			if child == nil {
				continue
			}
			placed, childRect := placeChild(child, r)
			walk(child, local.then(placed), childRect, depth+1)
		}
	}
	walk(root, affine{1, 1, 0, 0}, rootRect, 0)
	if len(out.Layers) == 0 {
		return out, ErrNoMeshImage
	}
	return out, nil
}

// placeChild returns how a child's local space sits in its parent's, and
// its rectangle. A RectTransform's pivot is at its anchors' reference point
// in the parent's rectangle plus m_AnchoredPosition; a plain Transform has
// no rectangle and sits at its m_LocalPosition.
func placeChild(child map[string]any, parent rect) (affine, rect) {
	scale := vec3(child["m_LocalScale"])
	if _, ok := child["m_AnchorMin"]; !ok {
		position := vec3(child["m_LocalPosition"])
		return affine{scale[0], scale[1], position[0], position[1]}, rect{}
	}
	anchorMin, anchorMax := vec2(child["m_AnchorMin"]), vec2(child["m_AnchorMax"])
	pivot, size := vec2(child["m_Pivot"]), vec2(child["m_SizeDelta"])
	anchored := vec2(child["m_AnchoredPosition"])
	minX := parent.minX + parent.w*anchorMin[0]
	maxX := parent.minX + parent.w*anchorMax[0]
	minY := parent.minY + parent.h*anchorMin[1]
	maxY := parent.minY + parent.h*anchorMax[1]
	w, h := maxX-minX+size[0], maxY-minY+size[1]
	x := minX + (maxX-minX)*pivot[0] + anchored[0]
	y := minY + (maxY-minY)*pivot[1] + anchored[1]
	return affine{scale[0], scale[1], x, y}, rect{-pivot[0] * w, -pivot[1] * h, w, h}
}

// meshImage returns a transform's enabled MeshImage component (a
// MonoBehaviour with mMesh and mRawSpriteSize).
func (p *prefab) meshImage(node map[string]any) map[string]any {
	gameObject := p.ptr(node["m_GameObject"])
	if gameObject == nil {
		return nil
	}
	components, _ := gameObject["m_Component"].([]any)
	for _, entry := range components {
		ref, _ := entry.(map[string]any)
		ref, _ = ref["component"].(map[string]any)
		obj, ok := p.objects[asInt(ref["m_PathID"])]
		if !ok || asInt(ref["m_FileID"]) != 0 || obj.classID != classMono {
			continue
		}
		mono := p.get(asInt(ref["m_PathID"]))
		if _, ok := mono["mRawSpriteSize"]; !ok {
			continue
		}
		if enabled := asInt(mono["m_Enabled"]); enabled == 0 {
			return nil
		}
		return mono
	}
	return nil
}

// paintingLayer places a MeshImage's image on the canvas: its vertices span
// its rectangle, mRawSpriteSize to the rectangle's size.
func paintingLayer(mi map[string]any, r rect, toCanvas affine) (PaintingLayer, bool) {
	mesh, _ := mi["mMesh"].(map[string]any)
	pathID := asInt(mesh["m_PathID"])
	raw := vec2(mi["mRawSpriteSize"])
	if pathID == 0 || raw[0] <= 0 || raw[1] <= 0 || r.w <= 0 || r.h <= 0 {
		return PaintingLayer{}, false
	}
	width, height := int(math.Ceil(raw[0])), int(math.Ceil(raw[1]))
	// image pixel (x, y down) -> mesh space (x, height - y) -> local
	// rectangle -> root -> canvas
	toLocal := affine{r.w / raw[0], -r.h / raw[1], r.minX, r.minY + float64(height)*r.h/raw[1]}
	placed := toCanvas.then(toLocal)
	layer := PaintingLayer{
		Mesh: pathID, Width: width, Height: height,
		ScaleX: placed.sx, ScaleY: placed.sy, X: placed.tx, Y: placed.ty,
		Color: [4]float64{1, 1, 1, 1},
	}
	if color, ok := mi["m_Color"].(map[string]any); ok {
		for i, key := range []string{"r", "g", "b", "a"} {
			if v, ok := color[key].(float64); ok {
				layer.Color[i] = v
			}
		}
	}
	if layer.ScaleX <= 0 || layer.ScaleY <= 0 {
		return PaintingLayer{}, false
	}
	return layer, true
}

// PaintingParts returns the bundle's meshes, each with the texture it cuts
// (the texture named like the mesh without its "-mesh", else the bundle's
// only one).
func (bundle *Bundle) PaintingParts() ([]PaintingPart, error) {
	meshes, err := bundle.Meshes()
	if err != nil {
		return nil, err
	}
	if len(meshes) == 0 {
		return nil, nil
	}
	textures, err := bundle.Textures()
	if err != nil {
		return nil, err
	}
	parts := make([]PaintingPart, 0, len(meshes))
	for _, mesh := range meshes {
		want := strings.TrimSuffix(mesh.Name, "-mesh")
		found := -1
		for i, texture := range textures {
			if strings.EqualFold(texture.Name, want) {
				found = i
				break
			}
		}
		if found < 0 && len(textures) == 1 {
			found = 0
		}
		if found < 0 {
			return nil, fmt.Errorf("mesh %s: no texture %s among the bundle's %d", mesh.Name, want, len(textures))
		}
		parts = append(parts, PaintingPart{Mesh: mesh, Texture: textures[found]})
	}
	return parts, nil
}

// Render reconstructs the image the part's mesh draws on a width x height
// canvas, its mesh space's origin at the bottom left. A zero size takes the
// mesh's bounds.
//
// The meshes are made of axis-aligned quads, each cutting a rectangle of
// the atlas. Their vertices are pixel indices (from 0 to the size - 1, the
// edge quads clamped there), so like imgrecon a quad's texels are pasted
// as they are, the atlas rectangle's top left at the quad's top left
// vertex: the edge quads keep the row or column the clamp took off and the
// original image comes back pixel for pixel. Any other triangle is drawn
// through its UVs.
func (part PaintingPart) Render(width, height int) (*image.NRGBA, error) {
	texture, err := part.Texture.Image()
	if err != nil {
		return nil, err
	}
	mesh := part.Mesh
	if width <= 0 || height <= 0 {
		var maxX, maxY float32
		for _, p := range mesh.Positions {
			maxX, maxY = max(maxX, p[0]), max(maxY, p[1])
		}
		width, height = int(math.Ceil(float64(maxX)))+1, int(math.Ceil(float64(maxY)))+1
	}
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 {
		return nil, fmt.Errorf("mesh %s: canvas %dx%d", mesh.Name, width, height)
	}
	out := image.NewNRGBA(image.Rect(0, 0, width, height))
	tw, th := float64(texture.Rect.Dx()), float64(texture.Rect.Dy())
	// canvas row of a mesh y: its top row is the vertex index height - 1
	top := float64(height - 1)
	for i := 0; i < len(mesh.Triangles); i++ {
		if i+1 < len(mesh.Triangles) {
			if quad, ok := meshQuad(mesh, mesh.Triangles[i], mesh.Triangles[i+1]); ok {
				l, r := math.Round(quad.u0*tw), math.Round(quad.u1*tw)
				t, b := math.Round((1-quad.v1)*th), math.Round((1-quad.v0)*th)
				pasteRect(out, texture, int(l), int(t), int(r-l), int(b-t), int(quad.x0), int(top-quad.y1))
				i++
				continue
			}
		}
		var xs, ys, us, vs [3]float64
		for k, v := range mesh.Triangles[i] {
			xs[k] = float64(mesh.Positions[v][0])
			ys[k] = top - float64(mesh.Positions[v][1])
			us[k] = float64(mesh.UVs[v][0]) * tw
			vs[k] = (1 - float64(mesh.UVs[v][1])) * th
		}
		drawTriangle(out, texture, xs, ys, us, vs)
	}
	return out, nil
}

// quad is an axis-aligned quad of a mesh: its positions' and UVs' ranges,
// UVs growing with the positions.
type quad struct{ x0, y0, x1, y1, u0, v0, u1, v1 float64 }

// meshQuad says whether two triangles make an axis-aligned quad on whole
// pixels whose UVs map its corners in the same orientation.
func meshQuad(mesh Mesh, a, b [3]uint32) (quad, bool) {
	vertices := map[uint32]bool{}
	for _, v := range a {
		vertices[v] = true
	}
	for _, v := range b {
		vertices[v] = true
	}
	if len(vertices) != 4 {
		return quad{}, false
	}
	q := quad{x0: math.Inf(1), y0: math.Inf(1), x1: math.Inf(-1), y1: math.Inf(-1), u0: math.Inf(1), v0: math.Inf(1), u1: math.Inf(-1), v1: math.Inf(-1)}
	for v := range vertices {
		p, uv := mesh.Positions[v], mesh.UVs[v]
		q.x0, q.x1 = min(q.x0, float64(p[0])), max(q.x1, float64(p[0]))
		q.y0, q.y1 = min(q.y0, float64(p[1])), max(q.y1, float64(p[1]))
		q.u0, q.u1 = min(q.u0, float64(uv[0])), max(q.u1, float64(uv[0]))
		q.v0, q.v1 = min(q.v0, float64(uv[1])), max(q.v1, float64(uv[1]))
	}
	if q.x0 != math.Trunc(q.x0) || q.y1 != math.Trunc(q.y1) || q.x1 <= q.x0 || q.y1 <= q.y0 {
		return quad{}, false
	}
	// every corner: x at x0 or x1 with u at u0 or u1 alike, same for y and v
	corners := map[[2]bool]bool{}
	for v := range vertices {
		p, uv := mesh.Positions[v], mesh.UVs[v]
		right, up := float64(p[0]) == q.x1, float64(p[1]) == q.y1
		if !right && float64(p[0]) != q.x0 || !up && float64(p[1]) != q.y0 {
			return quad{}, false
		}
		if (float64(uv[0]) == q.u1) != right || (float64(uv[1]) == q.v1) != up {
			return quad{}, false
		}
		corners[[2]bool{right, up}] = true
	}
	return q, len(corners) == 4
}

// pasteRect copies a w x h rectangle of src at (sx, sy) to dst at (dx, dy),
// clipped to both.
func pasteRect(dst, src *image.NRGBA, sx, sy, w, h, dx, dy int) {
	for y := 0; y < h; y++ {
		if sy+y < 0 || sy+y >= src.Rect.Dy() || dy+y < 0 || dy+y >= dst.Rect.Dy() {
			continue
		}
		x0 := max(0, -sx, -dx)
		x1 := min(w, src.Rect.Dx()-sx, dst.Rect.Dx()-dx)
		if x1 <= x0 {
			continue
		}
		s := src.PixOffset(src.Rect.Min.X+sx+x0, src.Rect.Min.Y+sy+y)
		d := dst.PixOffset(dst.Rect.Min.X+dx+x0, dst.Rect.Min.Y+dy+y)
		copy(dst.Pix[d:d+(x1-x0)*4], src.Pix[s:s+(x1-x0)*4])
	}
}

// drawTriangle copies the texels a triangle covers in the texture (us, vs
// in texels, top row first) to the pixels whose centre it covers in dst
// (xs, ys in pixels, y down). Pixels on a shared edge are drawn by both
// triangles, from the same texels, so the seams leave no gap.
func drawTriangle(dst, src *image.NRGBA, xs, ys, us, vs [3]float64) {
	area := (xs[1]-xs[0])*(ys[2]-ys[0]) - (xs[2]-xs[0])*(ys[1]-ys[0])
	if math.Abs(area) < 1e-9 {
		return
	}
	bounds := dst.Rect
	x0 := max(int(math.Floor(min(xs[0], xs[1], xs[2]))), bounds.Min.X)
	x1 := min(int(math.Ceil(max(xs[0], xs[1], xs[2]))), bounds.Max.X)
	y0 := max(int(math.Floor(min(ys[0], ys[1], ys[2]))), bounds.Min.Y)
	y1 := min(int(math.Ceil(max(ys[0], ys[1], ys[2]))), bounds.Max.Y)
	const eps = 1e-7
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	for y := y0; y < y1; y++ {
		py := float64(y) + 0.5
		for x := x0; x < x1; x++ {
			px := float64(x) + 0.5
			// barycentric weights of the pixel centre
			w0 := ((xs[1]-px)*(ys[2]-py) - (xs[2]-px)*(ys[1]-py)) / area
			w1 := ((xs[2]-px)*(ys[0]-py) - (xs[0]-px)*(ys[2]-py)) / area
			w2 := 1 - w0 - w1
			if w0 < -eps || w1 < -eps || w2 < -eps {
				continue
			}
			u := w0*us[0] + w1*us[1] + w2*us[2]
			v := w0*vs[0] + w1*vs[1] + w2*vs[2]
			tx := min(max(int(math.Floor(u)), 0), sw-1)
			ty := min(max(int(math.Floor(v)), 0), sh-1)
			s := src.PixOffset(tx+src.Rect.Min.X, ty+src.Rect.Min.Y)
			d := dst.PixOffset(x, y)
			copy(dst.Pix[d:d+4], src.Pix[s:s+4])
		}
	}
}

// Compose draws the layers' images (by mesh path id, each the size of its
// layer) on the canvas in order, alpha blended. A layer without an image
// is skipped. Layers scaled down are averaged over their footprint,
// layers scaled up interpolated.
func (painting *PaintingPrefab) Compose(images map[int64]*image.NRGBA) *image.NRGBA {
	w, h := painting.Width, painting.Height
	// premultiplied, 0..1
	acc := make([]float32, w*h*4)
	for _, layer := range painting.Layers {
		img := images[layer.Mesh]
		if img == nil {
			continue
		}
		composeLayer(acc, w, h, img, layer)
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		a := acc[i*4+3]
		if a <= 0 {
			continue
		}
		for c := range 3 {
			out.Pix[i*4+c] = clamp8(acc[i*4+c] / a)
		}
		out.Pix[i*4+3] = clamp8(a)
	}
	return out
}

// composeLayer blends one layer over the premultiplied canvas.
func composeLayer(acc []float32, w, h int, img *image.NRGBA, layer PaintingLayer) {
	sw, sh := img.Rect.Dx(), img.Rect.Dy()
	x0 := max(int(math.Floor(layer.X)), 0)
	y0 := max(int(math.Floor(layer.Y)), 0)
	x1 := min(int(math.Ceil(layer.X+layer.ScaleX*float64(sw))), w)
	y1 := min(int(math.Ceil(layer.Y+layer.ScaleY*float64(sh))), h)
	// exact copies when the layer is at its own resolution on whole pixels
	exact := math.Abs(layer.ScaleX-1) < 1e-4 && math.Abs(layer.ScaleY-1) < 1e-4 &&
		math.Abs(layer.X-math.Round(layer.X)) < 1e-3 && math.Abs(layer.Y-math.Round(layer.Y)) < 1e-3
	// samples per pixel and axis when scaled down
	nx := max(int(math.Ceil(1/layer.ScaleX)), 1)
	ny := max(int(math.Ceil(1/layer.ScaleY)), 1)
	tint := [4]float32{float32(layer.Color[0]), float32(layer.Color[1]), float32(layer.Color[2]), float32(layer.Color[3])}
	texel := func(x, y int) [4]float32 {
		x = min(max(x, 0), sw-1)
		y = min(max(y, 0), sh-1)
		o := img.PixOffset(x+img.Rect.Min.X, y+img.Rect.Min.Y)
		a := float32(img.Pix[o+3]) / 255 * tint[3]
		return [4]float32{
			float32(img.Pix[o]) / 255 * tint[0] * a,
			float32(img.Pix[o+1]) / 255 * tint[1] * a,
			float32(img.Pix[o+2]) / 255 * tint[2] * a,
			a,
		}
	}
	// bilinear sample at a point in texels (premultiplied), transparent
	// outside the image
	bilinear := func(sx, sy float64) [4]float32 {
		fx, fy := sx-0.5, sy-0.5
		ix, iy := int(math.Floor(fx)), int(math.Floor(fy))
		ax, ay := float32(fx-float64(ix)), float32(fy-float64(iy))
		var out [4]float32
		for dy := range 2 {
			for dx := range 2 {
				tx, ty := ix+dx, iy+dy
				if tx < 0 || ty < 0 || tx >= sw || ty >= sh {
					continue
				}
				wx, wy := ax, ay
				if dx == 0 {
					wx = 1 - ax
				}
				if dy == 0 {
					wy = 1 - ay
				}
				t := texel(tx, ty)
				for c := range 4 {
					out[c] += t[c] * wx * wy
				}
			}
		}
		return out
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			var px [4]float32
			switch {
			case exact:
				tx, ty := x-int(math.Round(layer.X)), y-int(math.Round(layer.Y))
				if tx < 0 || ty < 0 || tx >= sw || ty >= sh {
					continue
				}
				px = texel(tx, ty)
			case nx == 1 && ny == 1:
				px = bilinear((float64(x)+0.5-layer.X)/layer.ScaleX, (float64(y)+0.5-layer.Y)/layer.ScaleY)
			default:
				n := float32(nx * ny)
				for j := range ny {
					for i := range nx {
						sx := (float64(x) + (float64(i)+0.5)/float64(nx) - layer.X) / layer.ScaleX
						sy := (float64(y) + (float64(j)+0.5)/float64(ny) - layer.Y) / layer.ScaleY
						if sx < 0 || sy < 0 || sx >= float64(sw) || sy >= float64(sh) {
							continue
						}
						t := texel(int(sx), int(sy))
						for c := range 4 {
							px[c] += t[c] / n
						}
					}
				}
			}
			if px[3] <= 0 {
				continue
			}
			o := (y*w + x) * 4
			keep := 1 - px[3]
			for c := range 4 {
				acc[o+c] = px[c] + acc[o+c]*keep
			}
		}
	}
}

func clamp8(v float32) uint8 {
	return uint8(min(max(v*255+0.5, 0), 255))
}

// vec2 reads a Vector2f field.
func vec2(value any) [2]float64 {
	fields, _ := value.(map[string]any)
	x, _ := fields["x"].(float64)
	y, _ := fields["y"].(float64)
	return [2]float64{x, y}
}

// vec3 reads a Vector3f field.
func vec3(value any) [3]float64 {
	fields, _ := value.(map[string]any)
	x, _ := fields["x"].(float64)
	y, _ := fields["y"].(float64)
	z, _ := fields["z"].(float64)
	return [3]float64{x, y, z}
}
