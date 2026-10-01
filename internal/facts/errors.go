package facts

import (
	"go/ast"
	"go/types"
	"strings"
)

// ErrorCall is a call that makes an error from a message: errors.New, fmt.Errorf, and the
// github.com/pkg/errors and golang.org/x/xerrors equivalents.
type ErrorCall struct {
	Call *ast.CallExpr
	// Message is the constant message or format; MessageKnown is false when it is built at run time.
	Message      string
	MessageKnown bool
	// Wraps is true when the error wraps another (%w, errors.Wrap): the caller can already branch
	// on the wrapped one.
	Wraps bool
}

var errorFuncs = map[string]map[string]bool{
	"errors":                        {"New": true},
	"fmt":                           {"Errorf": true},
	"github.com/pkg/errors":         {"New": true, "Errorf": true, "Wrap": true, "Wrapf": true},
	"golang.org/x/xerrors":          {"New": true, "Errorf": true},
	"github.com/cockroachdb/errors": {"New": true, "Newf": true, "Errorf": true, "Wrap": true, "Wrapf": true},
}

// AsErrorCall recognises a call that makes an error from a message.
func AsErrorCall(info *types.Info, call *ast.CallExpr) (ErrorCall, bool) {
	fn := Callee(info, call)
	if fn == nil || fn.Pkg() == nil || !errorFuncs[fn.Pkg().Path()][fn.Name()] {
		return ErrorCall{}, false
	}
	ec := ErrorCall{Call: call}
	msgArg := 0
	if strings.HasPrefix(fn.Name(), "Wrap") {
		ec.Wraps, msgArg = true, 1
	}
	if msgArg < len(call.Args) {
		ec.Message, ec.MessageKnown = ConstString(info, call.Args[msgArg])
	}
	if ec.MessageKnown && strings.Contains(ec.Message, "%w") {
		ec.Wraps = true
	}
	return ec, true
}
