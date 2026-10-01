package report

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"sync"
)

// Nolint finds //nolint directives, golangci-lint style, with lintuition's stricter rule: a directive
// suppresses only when it names the linter and explains why.
//
//	x := f() //nolint:premature-success // the call cannot fail here: f is a pure lookup
//
// Scope, as in golangci-lint: a directive at the end of a line covers that line; a directive in a
// comment group that ends right above a declaration or statement, starting in its column, covers
// that whole node; one in the group attached to the package clause covers the file. A bare //nolint, //nolint:all, or a directive without an explanation does
// not suppress. Names of linters lintuition does not have are allowed: they belong to other tools.
type Nolint struct {
	mu    sync.Mutex
	files map[string]*fileDirectives
}

type directive struct {
	linters  map[string]bool
	from, to int // lines covered, inclusive
}

type fileDirectives struct {
	ds []directive
}

// NewNolint returns an empty index; files are parsed on first use.
func NewNolint() *Nolint { return &Nolint{files: map[string]*fileDirectives{}} }

var nolintRE = regexp.MustCompile(`^//\s*nolint:([a-z0-9][a-z0-9,-]*)\s*(?://\s*(.*))?$`)

// Covers reports whether a directive suppresses the linter at path:line. path is read from disk.
func (n *Nolint) Covers(path, linter string, line int) bool {
	n.mu.Lock()
	fd, ok := n.files[path]
	if !ok {
		fd = parseDirectives(path)
		n.files[path] = fd
	}
	n.mu.Unlock()
	for _, d := range fd.ds {
		if d.linters[linter] && line >= d.from && line <= d.to {
			return true
		}
	}
	return false
}

func parseDirectives(path string) *fileDirectives {
	src, err := os.ReadFile(path)
	if err != nil {
		return &fileDirectives{}
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return &fileDirectives{}
	}
	fd := &fileDirectives{}
	// The first line of each node, for directives that stand on their own line above one.
	starts := map[int]ast.Node{}
	ast.Inspect(f, func(nd ast.Node) bool {
		switch nd.(type) {
		case ast.Decl, ast.Stmt, *ast.ValueSpec, *ast.TypeSpec, *ast.Field, *ast.KeyValueExpr:
			l := fset.Position(nd.Pos()).Line
			if _, ok := starts[l]; !ok {
				starts[l] = nd
			}
		}
		return true
	})
	pkgLine := fset.Position(f.Package).Line
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			m := nolintRE.FindStringSubmatch(c.Text)
			if m == nil || strings.TrimSpace(m[2]) == "" {
				continue
			}
			ls := map[string]bool{}
			for _, l := range strings.Split(m[1], ",") {
				if l != "" && l != "all" {
					ls[l] = true
				}
			}
			if len(ls) == 0 {
				continue
			}
			pos := fset.Position(c.Pos())
			d := directive{linters: ls, from: pos.Line, to: pos.Line}
			end := fset.Position(cg.End()).Line
			switch {
			case pos.Line < pkgLine:
				// File scope only for the comment group attached to the package clause; a detached
				// group above it covers its own line.
				if end+1 == pkgLine {
					d.from, d.to = 1, fset.File(f.Pos()).LineCount()
				}
			case ownLine(src, pos):
				// As golangci-lint: the group must end right above the node and start in its column.
				if nd, ok := starts[end+1]; ok && fset.Position(nd.Pos()).Column == fset.Position(cg.Pos()).Column {
					d.from = pos.Line
					d.to = fset.Position(nd.End()).Line
				}
			}
			fd.ds = append(fd.ds, d)
		}
	}
	return fd
}

// ownLine reports whether only blanks precede the comment on its line.
func ownLine(src []byte, pos token.Position) bool {
	start := pos.Offset - (pos.Column - 1)
	if start < 0 || pos.Offset > len(src) {
		return false
	}
	return strings.TrimSpace(string(src[start:pos.Offset])) == ""
}
