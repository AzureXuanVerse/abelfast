package admin

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/ggmolly/belfast/internal/gateway"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/region"
	"github.com/ggmolly/belfast/internal/unity"
)

// The game's art isn't shipped with belfast: the back-office asks for a
// client bundle path (GET /api/assets/props/20001.png), and the bundle is
// fetched from the region's official CDN the first time, decoded and kept
// under the asset dir. Paths are resolved through the bundle list of the
// clients' current $azhash$ version, so a game update brings the new art by
// itself: a changed bundle has a new md5, the cache's key.

// assetTimeout bounds one download from the CDN; hash lists are a few MB.
const assetTimeout = 60 * time.Second

// assetMaxAge is how long a browser reuses an asset before revalidating it
// against its ETag, the bundle md5.
const assetMaxAge = 24 * time.Hour

// assets serves the decoded bundles of the clients' current version.
type assets struct {
	// dir keeps the hash lists (hash/<version>) and the decoded bundles
	// (<md5>.png, <md5>.ttf)
	dir     string
	gateway *gateway.Gateway
	client  http.Client

	mu sync.Mutex
	// lists are the current bundle lists by kind (gateway.Assets*)
	lists  map[string]bundleList
	flight singleflight.Group
}

// bundleList is a version's bundle list: bundle path -> md5.
type bundleList struct {
	version string
	bundles map[string]string
}

// assetKinds are the files a bundle can be served as, by extension.
var assetKinds = map[string]string{
	".png": "image/png",
	".ttf": "font/ttf",
}

// handleAsset answers GET /api/assets/{path...}: path is a bundle path with
// the extension of what to serve from it, e.g. squareicon/beierfasite.png or
// font/zhunyuan_bold.ttf. A bundle holding several textures (an atlas
// bundle) needs the texture named after it: ui/commonui_atlas/ellipse_c.png.
func (server *Server) handleAsset(r *http.Request) (servedFile, error) {
	name := r.PathValue("path")
	ext := path.Ext(name)
	contentType, ok := assetKinds[ext]
	if !ok {
		return servedFile{}, notFound("want a .png or .ttf asset")
	}
	md5, file, err := server.assets.get(strings.TrimSuffix(name, ext), ext)
	return servedFile{etag: md5, path: file, contentType: contentType}, err
}

// servedFile is a file to answer with, its ETag and content type.
type servedFile struct {
	etag, path, contentType string
}

// authedFile answers a GET with the file locate gives, once the session is
// checked: locate's httpError as is, another error as the CDN's (502). The
// file is cached for assetMaxAge against its ETag, a page that has it gets
// a 304 without the file being read.
func (server *Server) authedFile(scope string, locate func(*http.Request) (servedFile, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !server.loggedIn(w, r) {
			return
		}
		served, err := locate(r)
		if err != nil {
			var known httpError
			if errors.As(err, &known) {
				writeJSON(w, known.status, map[string]string{"error": known.msg})
				return
			}
			logger.LogEvent("Admin", scope, r.URL.Path+": "+err.Error(), logger.LOG_LEVEL_ERROR)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		file, err := os.Open(served.path)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", served.contentType)
		w.Header().Set("ETag", `"`+served.etag+`"`)
		w.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", int(assetMaxAge.Seconds())))
		http.ServeContent(w, r, "", time.Time{}, file)
	}
}

// get returns the md5 of the bundle name points to and the cached file
// decoded from it as ext, downloading and decoding it first when needed.
// name is a bundle path, or a bundle path and the texture to take from it.
func (assets *assets) get(name, ext string) (string, string, error) {
	bundles, err := assets.list(gateway.AssetsMain)
	if err != nil {
		return "", "", err
	}
	bundle, texture := name, ""
	md5, ok := bundles[bundle]
	if !ok && ext == ".png" {
		bundle, texture = path.Split(name)
		bundle = strings.TrimSuffix(bundle, "/")
		md5, ok = bundles[bundle]
	}
	if !ok {
		return "", "", notFound("no bundle " + name + " in this version")
	}
	key := md5 + ext
	if texture != "" {
		// texture names come from the client (letters, digits, _ and -),
		// escaped all the same since they end up in a file name
		key = md5 + "-" + url.PathEscape(texture) + ext
	}
	file, err := assets.cached(key, func() ([]byte, error) {
		data, err := assets.fetch("resource/" + md5)
		if err != nil {
			return nil, err
		}
		decoded, err := decodeBundle(data, path.Base(bundle), texture, ext)
		if err != nil {
			return nil, fmt.Errorf("%s (%s): %w", bundle, md5, err)
		}
		return decoded, nil
	})
	if err != nil {
		return "", "", err
	}
	return md5, file, nil
}

// list returns a kind of bundle list (gateway.Assets*) of the clients'
// current version, read from the asset dir or downloaded when the version
// changed.
func (assets *assets) list(kind string) (map[string]string, error) {
	version, err := assets.gateway.AssetHash(kind)
	if err != nil {
		return nil, err
	}
	assets.mu.Lock()
	if current, ok := assets.lists[kind]; ok && current.version == version {
		defer assets.mu.Unlock()
		return current.bundles, nil
	}
	assets.mu.Unlock()

	bundles, err, _ := assets.flight.Do("hash/"+version, func() (any, error) {
		file := filepath.Join(assets.dir, "hash", url.PathEscape(version))
		data, err := os.ReadFile(file)
		if err != nil {
			if data, err = assets.fetch("hash/" + version); err != nil {
				return nil, err
			}
			if err := writeFileAtomic(file, data); err != nil {
				return nil, err
			}
		}
		bundles, err := parseHashList(data)
		if err != nil {
			return nil, fmt.Errorf("hash list %s: %w", version, err)
		}
		assets.mu.Lock()
		if assets.lists == nil {
			assets.lists = map[string]bundleList{}
		}
		assets.lists[kind] = bundleList{version: version, bundles: bundles}
		assets.mu.Unlock()
		logger.LogEvent("Admin", "Asset", fmt.Sprintf("bundle list %s: %d bundles", version, len(bundles)), logger.LOG_LEVEL_INFO)
		return bundles, nil
	})
	if err != nil {
		return nil, err
	}
	return bundles.(map[string]string), nil
}

// bundle returns the md5 of a bundle of a kind of list, 404 when this
// version has none by that path.
func (assets *assets) bundle(kind, name string) (string, error) {
	bundles, err := assets.list(kind)
	if err != nil {
		return "", err
	}
	md5, ok := bundles[name]
	if !ok {
		return "", notFound("no bundle " + name + " in this version")
	}
	return md5, nil
}

// cached returns the file key under the asset dir, making it with make the
// first time (one maker at a time per key). Keys start with the md5 of
// what they are made from, so they never go stale.
func (assets *assets) cached(key string, make func() ([]byte, error)) (string, error) {
	file := filepath.Join(assets.dir, key)
	if _, err := os.Stat(file); err == nil {
		return file, nil
	}
	_, err, _ := assets.flight.Do(key, func() (any, error) {
		if _, err := os.Stat(file); err == nil {
			return nil, nil
		}
		data, err := make()
		if err != nil {
			return nil, err
		}
		return nil, writeFileAtomic(file, data)
	})
	return file, err
}

// fetch downloads <cdn>/android/<file> from the region's asset CDN.
func (assets *assets) fetch(file string) ([]byte, error) {
	cdn, ok := gateway.AssetCDN[region.Current()]
	if !ok {
		return nil, fmt.Errorf("no asset CDN for region %s", region.Current())
	}
	resp, err := assets.client.Get(cdn + "/android/" + file)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s/android/%s: %s", cdn, file, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// parseHashList reads a bundle list: "path,size,md5" lines (CRLF).
func parseHashList(data []byte) (map[string]string, error) {
	bundles := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) != 3 || len(fields[2]) != 32 {
			return nil, fmt.Errorf("bad line %q", line)
		}
		bundles[fields[0]] = fields[2]
	}
	if len(bundles) == 0 {
		return nil, errors.New("empty")
	}
	return bundles, scanner.Err()
}

// decodeBundle turns a bundle into what ext asks for: a texture as a PNG,
// or its font's TTF bytes. The texture is the one named texture, else the
// one named like the bundle (base), else the bundle's only texture.
func decodeBundle(data []byte, base, texture, ext string) ([]byte, error) {
	bundle, err := unity.ReadBundle(data)
	if err != nil {
		return nil, err
	}
	if ext == ".ttf" {
		fonts, err := bundle.Fonts()
		if err != nil {
			return nil, err
		}
		if len(fonts) == 0 {
			return nil, notFound("no font in this bundle")
		}
		return fonts[0].Data, nil
	}
	textures, err := bundle.Textures()
	if err != nil {
		return nil, err
	}
	if len(textures) == 0 {
		return nil, notFound("no texture in this bundle")
	}
	want := texture
	if want == "" {
		want = base
	}
	var found *unity.Texture
	for i := range textures {
		if strings.EqualFold(textures[i].Name, want) {
			found = &textures[i]
			break
		}
	}
	if found == nil && texture == "" && len(textures) == 1 {
		found = &textures[0]
	}
	if found == nil {
		names := make([]string, len(textures))
		for i, other := range textures {
			names[i] = other.Name
		}
		return nil, notFound(fmt.Sprintf("no texture %q, this bundle has: %s", want, strings.Join(names, ", ")))
	}
	img, err := found.Image()
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// writeFileAtomic writes data to file through a temporary file, so a
// concurrent reader never sees half of it.
func writeFileAtomic(file string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), file)
}
