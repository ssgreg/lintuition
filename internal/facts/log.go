package facts

import (
	"go/ast"
	"go/types"
	"strings"
)

// Levels a log call can have. Unleveled is log.Print and friends.
const (
	LevelDebug     = "debug"
	LevelInfo      = "info"
	LevelWarn      = "warn"
	LevelError     = "error"
	LevelFatal     = "fatal"
	LevelPanic     = "panic"
	LevelUnleveled = "unleveled"
)

// LogCall is a recognised call to a logger.
type LogCall struct {
	Call  *ast.CallExpr
	Func  *types.Func
	Level string
	// Message is the constant message or format; MessageKnown is false when it is built at run time.
	Message      string
	MessageKnown bool
	// MessageArg is the index of the message argument, -1 when there is none.
	MessageArg int
}

// logPackages are the logger packages recognised by path. Any other package whose last path element
// is one of logNames, or ends with "logger" or "logging", is treated as a logger package too, when
// the called name is a level and the call returns nothing. This is a heuristic: logic, login and
// catalog are not loggers, and a project logger under another name is not recognised.
var logPackages = map[string]bool{
	"log":                        true,
	"log/slog":                   true,
	"go.uber.org/zap":            true,
	"github.com/sirupsen/logrus": true,
	"github.com/ssgreg/logf":     true,
	"github.com/rs/zerolog":      true,
	"github.com/rs/zerolog/log":  true,
}

// level maps a method or function name to its level: Infof, Infow, InfoContext, Infox -> info.
func level(name string) (string, bool) {
	n := name
	for _, suf := range []string{"Context", "Ctx", "ln", "f", "w", "x"} {
		if strings.HasSuffix(n, suf) && len(n) > len(suf) {
			trimmed := strings.TrimSuffix(n, suf)
			if _, ok := levels[trimmed]; ok {
				n = trimmed
				break
			}
		}
	}
	l, ok := levels[n]
	return l, ok
}

var levels = map[string]string{
	"Trace": LevelDebug, "Debug": LevelDebug, "Info": LevelInfo, "Warn": LevelWarn, "Warning": LevelWarn,
	"Error": LevelError, "Fatal": LevelFatal, "Panic": LevelPanic, "DPanic": LevelPanic, "Print": LevelUnleveled,
}

var logNames = map[string]bool{"log": true, "logs": true, "logr": true, "logf": true, "logx": true, "logutil": true, "klog": true, "glog": true}

func isLogPackage(pkg *types.Package) bool {
	if pkg == nil {
		return false
	}
	if logPackages[pkg.Path()] {
		return true
	}
	last := strings.ToLower(pkg.Path()[strings.LastIndex(pkg.Path(), "/")+1:])
	return logNames[last] || strings.HasSuffix(last, "logger") || strings.HasSuffix(last, "logging")
}

// AsLogCall recognises a call to a logger: a level-named function or method of a logger package
// (log.Printf, slog.Info, (*zap.Logger).Warn, (*logf.Logger).Info), or zerolog's
// log.Info().Msg("..."). The message is the first string parameter.
//
// A logging call returns nothing. That rule keeps field constructors that share a level name, such
// as logf.Error(err) or zap.Error(err), from being read as log lines.
func AsLogCall(info *types.Info, call *ast.CallExpr) (LogCall, bool) {
	fn := Callee(info, call)
	if fn == nil || !isLogPackage(fn.Pkg()) || fn.Type().(*types.Signature).Results().Len() > 0 {
		return LogCall{}, false
	}
	lc := LogCall{Call: call, Func: fn, MessageArg: -1}
	if fn.Name() == "Msg" || fn.Name() == "Msgf" {
		// zerolog: the level is the call the event came from.
		inner, ok := chainRoot(call)
		if !ok {
			return LogCall{}, false
		}
		innerFn := Callee(info, inner)
		if innerFn == nil {
			return LogCall{}, false
		}
		l, ok := level(innerFn.Name())
		if !ok {
			return LogCall{}, false
		}
		lc.Level = l
	} else {
		l, ok := level(fn.Name())
		if !ok {
			return LogCall{}, false
		}
		lc.Level = l
	}
	sig := fn.Type().(*types.Signature)
	for i := 0; i < sig.Params().Len() && i < len(call.Args); i++ {
		t := sig.Params().At(i).Type()
		if sig.Variadic() && i == sig.Params().Len()-1 {
			// Print(v ...any): the first argument is the message when it is a constant string.
			lc.Message, lc.MessageKnown = ConstString(info, call.Args[i])
			lc.MessageArg = i
			break
		}
		if b, ok := t.Underlying().(*types.Basic); ok && b.Kind() == types.String {
			lc.Message, lc.MessageKnown = ConstString(info, call.Args[i])
			lc.MessageArg = i
			break
		}
	}
	return lc, true
}

// chainRoot returns the call a method is called on: for log.Info().Str("k", v).Msg("m") it is
// log.Info().
func chainRoot(call *ast.CallExpr) (*ast.CallExpr, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	x, ok := ast.Unparen(sel.X).(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	for {
		s, ok := ast.Unparen(x.Fun).(*ast.SelectorExpr)
		if !ok {
			return x, true
		}
		if _, isLevel := level(s.Sel.Name); isLevel {
			return x, true
		}
		next, ok := ast.Unparen(s.X).(*ast.CallExpr)
		if !ok {
			return x, true
		}
		x = next
	}
}
