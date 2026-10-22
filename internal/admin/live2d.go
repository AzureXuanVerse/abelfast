package admin

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ggmolly/belfast/internal/gateway"
	"github.com/ggmolly/belfast/internal/unity"
)

// A skin's Live2D model (live2d/<painting> in the l2d bundle list) is
// served as the files of a standard Cubism model, converted from the
// client's Unity prefab the first time and kept under the asset dir by the
// bundle's md5: GET /api/live2d/<painting>/<painting>.model3.json and the
// files it names. The Cubism Core runtime the web viewer needs isn't part of
// belfast: the page loads it from Live2D.

// registerLive2D adds the Live2D routes, behind the same session check as
// the other assets.
func (server *Server) registerLive2D(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/live2d", server.authed(server.handleLive2DList))
	mux.HandleFunc("GET /api/live2d/{name}/{file...}", server.authedFile("Live2D", server.handleLive2DFile))
}

// handleLive2DList answers GET /api/live2d: the paintings with a Live2D
// model in the clients' current version, by name.
func (server *Server) handleLive2DList(*http.Request) (any, error) {
	bundles, err := server.assets.list(gateway.AssetsLive2D)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(bundles))
	for bundle := range bundles {
		if name, ok := strings.CutPrefix(bundle, "live2d/"); ok && !strings.Contains(name, "/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return map[string][]string{"paintings": names}, nil
}

// live2dTypes are the content types of a model's files.
var live2dTypes = map[string]string{
	".json": "application/json",
	".moc3": "application/octet-stream",
	".png":  "image/png",
}

// handleLive2DFile answers GET /api/live2d/{name}/{file...}.
func (server *Server) handleLive2DFile(r *http.Request) (servedFile, error) {
	name, file := r.PathValue("name"), r.PathValue("file")
	contentType, ok := live2dTypes[path.Ext(file)]
	if !ok || file != path.Clean(file) || strings.HasPrefix(file, "../") || strings.HasPrefix(file, "/") {
		return servedFile{}, notFound("no such file")
	}
	md5, dir, err := server.assets.live2d(name)
	if err != nil {
		return servedFile{}, err
	}
	served := servedFile{etag: md5, path: filepath.Join(dir, filepath.FromSlash(file)), contentType: contentType}
	if _, err := os.Stat(served.path); err != nil {
		return servedFile{}, notFound("no " + file + " in this model")
	}
	return served, nil
}

// live2d returns the md5 of a painting's Live2D bundle and the directory its
// converted model is in, downloading and converting it first when needed.
// The model3.json is written last: when it's there, so are the others.
func (assets *assets) live2d(name string) (string, string, error) {
	if name == "" || name != path.Base(name) || strings.HasPrefix(name, ".") {
		return "", "", notFound("no Live2D model " + name)
	}
	md5, err := assets.bundle(gateway.AssetsLive2D, "live2d/"+name)
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(assets.dir, md5+"-live2d")
	_, err = assets.cached(md5+"-live2d/"+name+".model3.json", func() ([]byte, error) {
		data, err := assets.fetch("resource/" + md5)
		if err != nil {
			return nil, err
		}
		bundle, err := unity.ReadBundle(data)
		if err != nil {
			return nil, fmt.Errorf("live2d/%s (%s): %w", name, md5, err)
		}
		model, err := bundle.Live2D(name)
		if err != nil {
			return nil, fmt.Errorf("live2d/%s (%s): %w", name, md5, err)
		}
		for file, data := range model.Files {
			if file == model.Model3 {
				continue
			}
			if err := writeFileAtomic(filepath.Join(dir, filepath.FromSlash(file)), data); err != nil {
				return nil, err
			}
		}
		return model.Files[model.Model3], nil
	})
	if err != nil {
		return "", "", err
	}
	return md5, dir, nil
}
