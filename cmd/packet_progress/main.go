// Command packet_progress reports which client commands belfast answers, and
// how. It type-checks the server, finds every packets.Handle registration (the
// handler's request type is the CS packet) and classifies each handler by what
// its call graph reaches:
//
//   - stateful: the database, the handler reads or writes saved state;
//   - static: game config tables loaded at startup and no saved state, or a
//     handler marked //belfast:static, complete without either (a heartbeat,
//     a report the server ignores);
//   - stub: neither, a constant, captured or echoed reply, or nothing at all.
//
// The client's Lua (-lua) gives the commands it can send, the denominator, and
// names the protocol modules after the controller directories sending them.
//
//	go run ./cmd/packet_progress -svg progress.svg -png progress.png
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/tools/go/packages"
)

const (
	modulePath   = "github.com/ggmolly/belfast"
	dbPath       = modulePath + "/internal/db"
	packetsPath  = modulePath + "/internal/packets"
	protobufPath = modulePath + "/internal/protobuf"
)

type status string

const (
	statusStateful status = "stateful"
	statusStatic   status = "static"
	statusStub     status = "stub"
	statusMissing  status = "missing"
)

var statuses = []status{statusStateful, statusStatic, statusStub, statusMissing}

// staticDirective marks a handler complete without saved state or config.
const staticDirective = "//belfast:static"

type packet struct {
	ID      uint32   `json:"id"`
	Status  status   `json:"status"`
	Handler string   `json:"handler,omitempty"`
	Pos     string   `json:"pos,omitempty"`
	Config  []string `json:"config,omitempty"`   // config tables the handler reads
	Literal []string `json:"literals,omitempty"` // hard-coded package tables it reads
	Lua     string   `json:"lua,omitempty"`      // a Lua file sending it
}

type module struct {
	ID     uint32         `json:"id"`
	Label  string         `json:"label,omitempty"`
	Counts map[status]int `json:"counts"`
	Total  int            `json:"total"`
}

type report struct {
	Generated string         `json:"generated"`
	Revision  string         `json:"revision,omitempty"`
	Counts    map[status]int `json:"counts"`
	Total     int            `json:"total"`
	Modules   []*module      `json:"modules"`
	Packets   []*packet      `json:"packets"`
}

func main() {
	luaDir := flag.String("lua", "AzurLaneLuaScripts/EN", "client Lua scripts; without them every CS packet counts")
	jsonOut := flag.String("json", ".cache/packet-progress.json", "JSON report (empty to skip)")
	svgOut := flag.String("svg", ".cache/packet-progress.svg", "SVG card")
	pngOut := flag.String("png", "", "PNG render of the SVG (needs rsvg-convert)")
	pngScale := flag.Float64("png-scale", 2, "PNG zoom")
	flag.Parse()

	pkgs, err := load()
	if err != nil {
		log.Fatal(err)
	}
	handled := analyze(pkgs)
	sent, luaFiles, luaDirs := scanLua(*luaDir)
	rep := build(handled, csPackets(pkgs), sent, luaFiles, luaDirs)

	if *jsonOut != "" {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		writeFile(*jsonOut, append(data, '\n'))
	}
	writeFile(*svgOut, []byte(renderSVG(rep)))
	if *pngOut != "" {
		cmd := exec.Command("rsvg-convert", "-z", strconv.FormatFloat(*pngScale, 'f', -1, 64), "-o", *pngOut, *svgOut)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("rsvg-convert: %v", err)
		}
	}
	fmt.Printf("%d/%d answered: %d stateful, %d static, %d stub, %d missing\n",
		rep.Total-rep.Counts[statusMissing], rep.Total,
		rep.Counts[statusStateful], rep.Counts[statusStatic], rep.Counts[statusStub], rep.Counts[statusMissing])
}

func writeFile(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Fatal(err)
	}
}

// load type-checks the module's packages, except the generated protobuf ones
// (their types come from export data).
func load() ([]*packages.Package, error) {
	listed, err := packages.Load(&packages.Config{Mode: packages.NeedName}, "./...")
	if err != nil {
		return nil, err
	}
	var patterns []string
	for _, pkg := range listed {
		if pkg.PkgPath != protobufPath && !strings.HasPrefix(pkg.PkgPath, modulePath+"/internal/proto") {
			patterns = append(patterns, pkg.PkgPath)
		}
	}
	mode := packages.NeedName | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedImports
	pkgs, err := packages.Load(&packages.Config{Mode: mode}, patterns...)
	if err != nil {
		return nil, err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("type-checking failed")
	}
	return pkgs, nil
}

// csPackets is every CS_<id> message type of the protobuf package.
func csPackets(pkgs []*packages.Package) []uint32 {
	var ids []uint32
	for _, pkg := range pkgs {
		for _, imported := range pkg.Types.Imports() {
			if imported.Path() != protobufPath {
				continue
			}
			for _, name := range imported.Scope().Names() {
				if id, ok := csID(name); ok {
					ids = append(ids, id)
				}
			}
			return ids
		}
	}
	log.Fatal("no package imports " + protobufPath)
	return nil
}

func csID(name string) (uint32, bool) {
	digits, ok := strings.CutPrefix(name, "CS_")
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseUint(digits, 10, 32)
	return uint32(id), err == nil
}

type funcDecl struct {
	decl *ast.FuncDecl
	pkg  *packages.Package
}

type analyzer struct {
	fset  *token.FileSet
	decls map[*types.Func]funcDecl
	// config are the package variables startup assigns: tables loaded from the
	// game data. literals are the ones initialised with a composite literal.
	config, literals map[*types.Var]bool
	// owners are the packages registering handlers, the only ones whose
	// variables count as game data (not, say, the logger's level)
	owners map[*types.Package]bool
}

// analyze returns the classified handler of every registered CS packet.
func analyze(pkgs []*packages.Package) map[uint32]*packet {
	a := &analyzer{decls: map[*types.Func]funcDecl{}, config: map[*types.Var]bool{}, literals: map[*types.Var]bool{}, owners: map[*types.Package]bool{}}
	for _, pkg := range pkgs {
		a.fset = pkg.Fset
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				switch decl := decl.(type) {
				case *ast.FuncDecl:
					if fn, ok := pkg.TypesInfo.Defs[decl.Name].(*types.Func); ok && decl.Body != nil {
						a.decls[fn] = funcDecl{decl, pkg}
					}
				case *ast.GenDecl:
					a.collectLiterals(pkg, decl)
				}
			}
		}
	}

	handled := map[uint32]*packet{}
	var startup []*types.Func
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) != 1 {
					return true
				}
				callee := calledFunc(pkg.TypesInfo, call.Fun)
				if callee == nil || callee.Pkg() == nil || callee.Pkg().Path() != packetsPath || !strings.HasPrefix(callee.Name(), "Handle") {
					return true
				}
				handler := calledFunc(pkg.TypesInfo, call.Args[0])
				id, ok := requestID(pkg.TypesInfo.TypeOf(call.Args[0]))
				if handler == nil || !ok {
					log.Printf("%s: unrecognised registration", pkg.Fset.Position(call.Pos()))
					return true
				}
				if previous := handled[id]; previous != nil {
					log.Printf("CS_%d registered twice: %s and %s", id, previous.Handler, handler.Name())
				}
				handled[id] = &packet{ID: id, Handler: handler.Name(), Pos: a.position(handler)}
				if enclosing := enclosingFunc(pkg, file, call); enclosing != nil {
					startup = append(startup, enclosing)
					a.owners[enclosing.Pkg()] = true
				}
				return true
			})
		}
	}
	a.collectConfig(startup)

	for id, p := range handled {
		handler := a.handlerFunc(p.Handler, id)
		reach := a.reach(handler)
		p.Config, p.Literal = names(reach.config), names(reach.literals)
		switch {
		case reach.state:
			p.Status = statusStateful
		case len(reach.config) > 0 || a.marked(handler, staticDirective):
			p.Status = statusStatic
		default:
			p.Status = statusStub
		}
	}
	return handled
}

func (a *analyzer) position(fn *types.Func) string {
	decl, ok := a.decls[fn]
	if !ok {
		return ""
	}
	pos := decl.pkg.Fset.Position(decl.decl.Pos())
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, pos.Filename); err == nil {
			pos.Filename = rel
		}
	}
	return fmt.Sprintf("%s:%d", pos.Filename, pos.Line)
}

func (a *analyzer) marked(fn *types.Func, directive string) bool {
	decl, ok := a.decls[fn]
	if !ok || decl.decl.Doc == nil {
		return false
	}
	return slices.ContainsFunc(decl.decl.Doc.List, func(c *ast.Comment) bool { return c.Text == directive })
}

func (a *analyzer) handlerFunc(name string, id uint32) *types.Func {
	for fn := range a.decls {
		if fn.Name() != name {
			continue
		}
		if got, ok := requestID(fn.Type()); ok && got == id {
			return fn
		}
	}
	return nil
}

// collectLiterals records the package variables initialised with a
// composite literal (hard-coded tables).
func (a *analyzer) collectLiterals(pkg *packages.Package, decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		spec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, name := range spec.Names {
			if i >= len(spec.Values) {
				break
			}
			value := ast.Unparen(spec.Values[i])
			if unary, ok := value.(*ast.UnaryExpr); ok && unary.Op == token.AND {
				value = unary.X
			}
			if _, ok := value.(*ast.CompositeLit); !ok {
				continue
			}
			if v, ok := pkg.TypesInfo.Defs[name].(*types.Var); ok && !types.Implements(v.Type(), errorType) {
				a.literals[v] = true
			}
		}
	}
}

// collectConfig records the package variables assigned by what the functions
// registering handlers call: the game data loaded at startup.
func (a *analyzer) collectConfig(startup []*types.Func) {
	seen := map[*types.Func]bool{}
	var visit func(fn *types.Func)
	visit = func(fn *types.Func) {
		decl, ok := a.decls[fn]
		if !ok || seen[fn] {
			return
		}
		seen[fn] = true
		info := decl.pkg.TypesInfo
		ast.Inspect(decl.decl.Body, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					a.markConfig(info, lhs)
				}
			case *ast.UnaryExpr:
				if node.Op == token.AND {
					a.markConfig(info, node.X)
				}
			case *ast.Ident:
				if callee, ok := info.Uses[node].(*types.Func); ok {
					visit(callee.Origin())
				}
			}
			return true
		})
	}
	for _, fn := range startup {
		visit(fn)
	}
}

func (a *analyzer) markConfig(info *types.Info, expr ast.Expr) {
	for {
		switch e := ast.Unparen(expr).(type) {
		case *ast.IndexExpr:
			expr = e.X
			continue
		case *ast.SelectorExpr:
			expr = e.X
			if v, ok := info.Uses[e.Sel].(*types.Var); ok && isPackageVar(v) {
				a.config[v] = a.owners[v.Pkg()]
				return
			}
			continue
		case *ast.Ident:
			if v, ok := info.Uses[e].(*types.Var); ok && isPackageVar(v) {
				a.config[v] = a.owners[v.Pkg()]
			}
		}
		return
	}
}

var errorType = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

func isPackageVar(v *types.Var) bool {
	return v.Pkg() != nil && v.Parent() == v.Pkg().Scope()
}

type reach struct {
	state            bool
	config, literals map[string]bool
}

// reach walks everything fn can call within the module.
func (a *analyzer) reach(fn *types.Func) reach {
	r := reach{config: map[string]bool{}, literals: map[string]bool{}}
	seen := map[*types.Func]bool{}
	var visit func(fn *types.Func)
	visit = func(fn *types.Func) {
		decl, ok := a.decls[fn]
		if !ok || seen[fn] {
			return
		}
		seen[fn] = true
		info := decl.pkg.TypesInfo
		ast.Inspect(decl.decl.Body, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			switch obj := info.Uses[ident].(type) {
			case *types.Func:
				if obj.Pkg() != nil && obj.Pkg().Path() == dbPath {
					r.state = true
				}
				visit(obj.Origin())
			case *types.Var:
				switch {
				case a.config[obj]:
					r.config[obj.Name()] = true
				case a.literals[obj]:
					r.literals[obj.Name()] = true
				}
			}
			return true
		})
	}
	visit(fn)
	return r
}

func names(set map[string]bool) []string {
	list := make([]string, 0, len(set))
	for name := range set {
		list = append(list, name)
	}
	sort.Strings(list)
	return list
}

// calledFunc resolves a function or method expression (f, pkg.F, x.M).
func calledFunc(info *types.Info, expr ast.Expr) *types.Func {
	switch e := ast.Unparen(expr).(type) {
	case *ast.Ident:
		fn, _ := info.Uses[e].(*types.Func)
		return originOf(fn)
	case *ast.SelectorExpr:
		fn, _ := info.Uses[e.Sel].(*types.Func)
		return originOf(fn)
	case *ast.IndexExpr:
		return calledFunc(info, e.X)
	case *ast.IndexListExpr:
		return calledFunc(info, e.X)
	}
	return nil
}

func originOf(fn *types.Func) *types.Func {
	if fn == nil {
		return nil
	}
	return fn.Origin()
}

// requestID is the CS id of a handler's request, its last parameter.
func requestID(t types.Type) (uint32, bool) {
	sig, ok := t.(*types.Signature)
	if !ok || sig.Params().Len() == 0 {
		return 0, false
	}
	req := sig.Params().At(sig.Params().Len() - 1).Type()
	if ptr, ok := req.(*types.Pointer); ok {
		req = ptr.Elem()
	}
	named, ok := req.(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != protobufPath {
		return 0, false
	}
	return csID(named.Obj().Name())
}

func enclosingFunc(pkg *packages.Package, file *ast.File, node ast.Node) *types.Func {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Pos() <= node.Pos() && node.End() <= fn.End() {
			f, _ := pkg.TypesInfo.Defs[fn.Name].(*types.Func)
			return f
		}
	}
	return nil
}

var (
	luaSend = regexp.MustCompile(`:Send\(([^,\n]*)`)
	luaID   = regexp.MustCompile(`\b\d{5}\b`)
)

// scanLua returns the CS ids the client sends, a Lua file sending each one,
// and the controller/command directories sending them. Nil when there are no
// scripts.
func scanLua(dir string) (sent map[uint32]bool, files map[uint32]string, dirs map[uint32][]string) {
	if _, err := os.Stat(dir); err != nil {
		log.Printf("no client Lua at %s, counting every CS packet", dir)
		return nil, nil, nil
	}
	sent, files, dirs = map[uint32]bool{}, map[uint32]string{}, map[uint32][]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".lua") {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range luaSend.FindAllSubmatch(data, -1) {
			for _, digits := range luaID.FindAll(match[1], -1) {
				id64, _ := strconv.ParseUint(string(digits), 10, 32)
				id := uint32(id64)
				sent[id] = true
				if files[id] == "" || commandDir(rel) != "" {
					files[id] = rel
				}
				if dir := commandDir(rel); dir != "" {
					dirs[id] = append(dirs[id], dir)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return sent, files, dirs
}

// commandDir names the system a command script belongs to: its directory
// under controller/command/, or the one holding the other command trees
// (mod/island/command/...).
func commandDir(rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	i := slices.Index(parts, "command")
	switch {
	case i == 1 && parts[0] == "controller" && len(parts) > 3:
		return parts[2]
	case i > 0:
		return parts[i-1]
	}
	return ""
}

// build counts every CS packet the client sends (all of them without the Lua)
// plus any belfast handles, grouped by protocol module (id / 1000).
func build(handled map[uint32]*packet, all []uint32, sent map[uint32]bool, luaFiles map[uint32]string, luaDirs map[uint32][]string) *report {
	rep := &report{Generated: time.Now().UTC().Format("2006-01-02"), Revision: revision(), Counts: map[status]int{}}
	modules := map[uint32]*module{}
	moduleDirs := map[uint32]map[string]int{}
	for _, id := range all {
		p := handled[id]
		if p == nil {
			if sent != nil && !sent[id] {
				continue
			}
			p = &packet{ID: id, Status: statusMissing}
		}
		p.Lua = luaFiles[id]
		rep.Packets = append(rep.Packets, p)
		rep.Counts[p.Status]++
		rep.Total++

		m := modules[id/1000]
		if m == nil {
			m = &module{ID: id / 1000, Counts: map[status]int{}}
			modules[m.ID] = m
			moduleDirs[m.ID] = map[string]int{}
		}
		m.Counts[p.Status]++
		m.Total++
		for _, dir := range luaDirs[id] {
			moduleDirs[m.ID][dir]++
		}
	}
	for id, m := range modules {
		m.Label = topKey(moduleDirs[id])
		rep.Modules = append(rep.Modules, m)
	}
	slices.SortFunc(rep.Packets, func(a, b *packet) int { return int(a.ID) - int(b.ID) })
	slices.SortFunc(rep.Modules, func(a, b *module) int { return int(a.ID) - int(b.ID) })
	return rep
}

func topKey(counts map[string]int) string {
	best := ""
	for key, n := range counts {
		if n > counts[best] || (n == counts[best] && key < best) {
			best = key
		}
	}
	return best
}

func revision() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
