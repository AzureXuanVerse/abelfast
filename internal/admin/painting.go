package admin

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ggmolly/belfast/internal/gateway"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/unity"
)

// A ship's painting (GET /api/painting/biaoqiang_4.png) is drawn the way the
// client draws it: the painting/<name> prefab lays out MeshImages, each a
// mesh cut out of a texture atlas of one of the painting's _tex bundles
// (painting/<name>_tex, its _rw character layer, _bj background layers,
// ...). The prefab names its meshes by path id only, so the _tex bundles of
// the painting are opened, most likely first, until every mesh is found.
// Variants are paintings of their own: <name>_n without the background,
// <name>_hx censored, <name>_ex the alternate one.

// paintingVariantTokens are the name suffixes of a painting's variants.
var paintingVariantTokens = map[string]bool{"n": true, "hx": true, "ex": true}

// paintingMaxBundles bounds the _tex bundles opened for one painting.
const paintingMaxBundles = 24

// registerPainting adds the painting routes, behind the session check.
func (server *Server) registerPainting(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/painting/{file}", server.authedFile("Painting", server.handlePainting))
	mux.HandleFunc("GET /api/painting/{name}/variants", server.authed(server.handlePaintingVariants))
}

// handlePainting answers GET /api/painting/{name}.png.
func (server *Server) handlePainting(r *http.Request) (servedFile, error) {
	name, ok := strings.CutSuffix(r.PathValue("file"), ".png")
	if !ok || !validPaintingName(name) {
		return servedFile{}, notFound("want <painting>.png")
	}
	key, file, err := server.assets.painting(name)
	return servedFile{etag: key, path: file, contentType: "image/png"}, err
}

// paintingVariant is a variant of a painting, by its own painting name.
type paintingVariant struct {
	Name   string `json:"name"`
	Suffix string `json:"suffix"` // "" for the painting itself, "_n", "_hx", "_n_hx", ...
}

// handlePaintingVariants answers GET /api/painting/{name}/variants: the
// variants of the painting (name or one of its variants) this version has.
func (server *Server) handlePaintingVariants(r *http.Request) (any, error) {
	name := r.PathValue("name")
	if !validPaintingName(name) {
		return nil, badRequest("bad painting name")
	}
	bundles, err := server.assets.list(gateway.AssetsPainting)
	if err != nil {
		return nil, err
	}
	base := paintingBase(bundles, name)
	var variants []paintingVariant
	for bundle := range bundles {
		rest, ok := strings.CutPrefix(bundle, "painting/"+base)
		if !ok || (rest != "" && rest[0] != '_') {
			continue
		}
		suffix := strings.TrimSuffix(rest, "_tex")
		if suffix != rest {
			// a variant drawn by its _tex alone counts when it has no prefab
			if _, ok := bundles["painting/"+base+suffix]; ok {
				continue
			}
		}
		if !onlyVariantTokens(suffix) {
			continue
		}
		variants = append(variants, paintingVariant{Name: base + suffix, Suffix: suffix})
	}
	if len(variants) == 0 {
		return nil, notFound("no painting " + name + " in this version")
	}
	slices.SortFunc(variants, func(a, b paintingVariant) int {
		if len(a.Suffix) != len(b.Suffix) {
			return len(a.Suffix) - len(b.Suffix)
		}
		return strings.Compare(a.Suffix, b.Suffix)
	})
	return map[string]any{"painting": base, "variants": variants}, nil
}

// validPaintingName accepts the client's painting names (letters, digits,
// _ and -), which end up in bundle paths.
func validPaintingName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// onlyVariantTokens says whether a suffix ("", "_n", "_n_hx") is made of
// variant tokens only.
func onlyVariantTokens(suffix string) bool {
	if suffix == "" {
		return true
	}
	for _, token := range strings.Split(suffix[1:], "_") {
		if !paintingVariantTokens[token] {
			return false
		}
	}
	return true
}

// paintingBase strips a painting name's variant suffixes, as long as what
// remains is a painting of the list.
func paintingBase(bundles map[string]string, name string) string {
	for {
		i := strings.LastIndexByte(name, '_')
		if i < 0 || !paintingVariantTokens[name[i+1:]] {
			return name
		}
		_, prefab := bundles["painting/"+name[:i]]
		_, tex := bundles["painting/"+name[:i]+"_tex"]
		if !prefab && !tex {
			return name
		}
		name = name[:i]
	}
}

// paintingCandidates lists the _tex bundles that may hold a painting's
// meshes, most likely first: the painting's family (its base name and the
// layers and variants after it, not the other skins named after it), the
// ones carrying all of the painting's variant tokens first, then the ones
// with the fewest other variants and the fewest layers.
func paintingCandidates(bundles map[string]string, name string) []string {
	base := paintingBase(bundles, name)
	want := map[string]bool{}
	if rest := strings.TrimPrefix(name, base); rest != "" {
		for _, token := range strings.Split(rest[1:], "_") {
			want[token] = true
		}
	}
	type candidate struct {
		bundle                string
		missing, extra, layer int
	}
	var candidates []candidate
	for bundle := range bundles {
		rest, ok := strings.CutPrefix(bundle, "painting/"+base)
		if !ok || !strings.HasSuffix(rest, "_tex") || (rest[0] != '_') {
			continue
		}
		var tokens []string
		if rest != "_tex" {
			tokens = strings.Split(strings.TrimSuffix(rest, "_tex")[1:], "_")
		}
		// another skin: a prefix of its tokens is a painting that isn't a
		// variant of this one (beierfasite_2 for beierfasite)
		other := false
		for k := 1; k <= len(tokens) && !other; k++ {
			prefix := base + "_" + strings.Join(tokens[:k], "_")
			if _, ok := bundles["painting/"+prefix]; ok && !onlyVariantTokens("_"+strings.Join(tokens[:k], "_")) {
				other = true
			}
		}
		if other {
			continue
		}
		c := candidate{bundle: bundle}
		have := map[string]bool{}
		for _, token := range tokens {
			have[token] = true
			switch {
			case want[token]:
			case paintingVariantTokens[token]:
				c.extra++
			default:
				c.layer++
			}
		}
		for token := range want {
			if !have[token] {
				c.missing++
			}
		}
		candidates = append(candidates, c)
	}
	slices.SortFunc(candidates, func(a, b candidate) int {
		switch {
		case a.missing != b.missing:
			return a.missing - b.missing
		case a.extra != b.extra:
			return a.extra - b.extra
		case a.layer != b.layer:
			return a.layer - b.layer
		}
		return strings.Compare(a.bundle, b.bundle)
	})
	out := make([]string, len(candidates))
	for i, c := range candidates {
		out[i] = c.bundle
	}
	return out
}

// painting returns the cache key and file of a painting's PNG, drawing it
// first when needed. The key is made of the md5s of the prefab and of the
// _tex bundles it may draw from, so a changed bundle draws it again.
func (assets *assets) painting(name string) (string, string, error) {
	bundles, err := assets.list(gateway.AssetsPainting)
	if err != nil {
		return "", "", err
	}
	prefabMD5, hasPrefab := bundles["painting/"+name]
	texMD5, hasTex := bundles["painting/"+name+"_tex"]
	if !hasPrefab && !hasTex {
		return "", "", notFound("no painting " + name + " in this version")
	}
	candidates := paintingCandidates(bundles, name)
	sum := md5.New()
	for _, bundle := range candidates {
		sum.Write([]byte(bundles[bundle]))
	}
	first := prefabMD5
	if !hasPrefab {
		first = texMD5
	}
	key := first + "-" + hex.EncodeToString(sum.Sum(nil))[:16] + "-painting.png"
	file, err := assets.cached(key, func() ([]byte, error) {
		start := time.Now()
		img, opened, err := assets.drawPainting(bundles, name, candidates)
		if err != nil {
			return nil, fmt.Errorf("painting %s: %w", name, err)
		}
		var out bytes.Buffer
		if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&out, img); err != nil {
			return nil, err
		}
		logger.LogEvent("Admin", "Painting", fmt.Sprintf("%s: %dx%d from %d bundles in %s", name, img.Rect.Dx(), img.Rect.Dy(), opened, time.Since(start).Round(time.Millisecond)), logger.LOG_LEVEL_INFO)
		return out.Bytes(), nil
	})
	if err != nil {
		return "", "", err
	}
	return strings.TrimSuffix(key, ".png"), file, nil
}

// drawPainting draws a painting from its prefab's MeshImages, or from its
// _tex bundle alone when it has no prefab or the prefab no MeshImage (the
// mesh at its own bounds, or the texture itself without a mesh). It
// returns how many bundles were opened.
func (assets *assets) drawPainting(bundles map[string]string, name string, candidates []string) (*image.NRGBA, int, error) {
	opened := 0
	if md5, ok := bundles["painting/"+name]; ok {
		data, err := assets.fetch("resource/" + md5)
		if err != nil {
			return nil, 0, err
		}
		opened++
		bundle, err := unity.ReadBundle(data)
		if err != nil {
			return nil, opened, fmt.Errorf("painting/%s: %w", name, err)
		}
		prefab, err := bundle.PaintingPrefab()
		switch {
		case err == nil:
			images, n, err := assets.paintingLayers(bundles, prefab, candidates)
			opened += n
			if err != nil {
				return nil, opened, err
			}
			return prefab.Compose(images), opened, nil
		case !errors.Is(err, unity.ErrNoMeshImage):
			return nil, opened, fmt.Errorf("painting/%s: %w", name, err)
		}
	}
	md5, ok := bundles["painting/"+name+"_tex"]
	if !ok {
		return nil, opened, notFound("painting " + name + " draws no mesh and has no _tex bundle")
	}
	data, err := assets.fetch("resource/" + md5)
	if err != nil {
		return nil, opened, err
	}
	opened++
	bundle, err := unity.ReadBundle(data)
	if err != nil {
		return nil, opened, fmt.Errorf("painting/%s_tex: %w", name, err)
	}
	parts, err := bundle.PaintingParts()
	if err != nil {
		return nil, opened, fmt.Errorf("painting/%s_tex: %w", name, err)
	}
	if len(parts) > 0 {
		img, err := parts[0].Render(0, 0)
		return img, opened, err
	}
	textures, err := bundle.Textures()
	if err != nil {
		return nil, opened, err
	}
	if len(textures) == 0 {
		return nil, opened, notFound("painting/" + name + "_tex has no texture")
	}
	found := textures[0]
	for _, texture := range textures {
		if strings.EqualFold(texture.Name, name) {
			found = texture
		}
	}
	img, err := found.Image()
	return img, opened, err
}

// paintingLayers renders the prefab's layers, by mesh path id, from the
// candidate _tex bundles: the most likely ones are opened together, the
// others one by one until every layer is found.
func (assets *assets) paintingLayers(bundles map[string]string, prefab *unity.PaintingPrefab, candidates []string) (map[int64]*image.NRGBA, int, error) {
	wanted := map[int64]unity.PaintingLayer{}
	for _, layer := range prefab.Layers {
		wanted[layer.Mesh] = layer
	}
	images := map[int64]*image.NRGBA{}
	var mu sync.Mutex
	open := func(bundle string) error {
		data, err := assets.fetch("resource/" + bundles[bundle])
		if err != nil {
			return err
		}
		read, err := unity.ReadBundle(data)
		if err != nil {
			return fmt.Errorf("%s: %w", bundle, err)
		}
		parts, err := read.PaintingParts()
		if err != nil {
			return fmt.Errorf("%s: %w", bundle, err)
		}
		for _, part := range parts {
			layer, ok := wanted[part.Mesh.PathID]
			if !ok {
				continue
			}
			img, err := part.Render(layer.Width, layer.Height)
			if err != nil {
				return fmt.Errorf("%s: %w", bundle, err)
			}
			mu.Lock()
			images[part.Mesh.PathID] = img
			mu.Unlock()
		}
		return nil
	}
	missing := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(images) < len(wanted)
	}

	candidates = candidates[:min(len(candidates), paintingMaxBundles)]
	// the bundles of the painting's own variant come first: as many as it
	// has layers, opened together
	together := candidates[:min(len(candidates), len(wanted))]
	errs := make([]error, len(together))
	var wg sync.WaitGroup
	for i, bundle := range together {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = open(bundle)
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, len(together), err
	}
	opened := len(together)
	for _, bundle := range candidates[len(together):] {
		if !missing() {
			break
		}
		if err := open(bundle); err != nil {
			return nil, opened, err
		}
		opened++
	}
	if len(images) == 0 {
		return nil, opened, fmt.Errorf("none of the %d meshes in %d _tex bundles", len(wanted), opened)
	}
	if missing() {
		var names []string
		for id, layer := range wanted {
			if images[id] == nil {
				names = append(names, layer.Name)
			}
		}
		logger.LogEvent("Admin", "Painting", fmt.Sprintf("layers %s not found in %d _tex bundles, drawn without them", strings.Join(names, ", "), opened), logger.LOG_LEVEL_WARN)
	}
	return images, opened, nil
}
