package enumcomment

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// word is an identifier-like word of a comment (Go letters, digits and underscores) or one
// punctuation rune; spaces are dropped.
type word struct {
	text  string
	end   int // byte offset right after the word in the comment
	ident bool
}

func isIdentRune(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

func tokenize(s string) []word {
	var out []word
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case isIdentRune(r):
			j := i + n
			for j < len(s) {
				r2, n2 := utf8.DecodeRuneInString(s[j:])
				if !isIdentRune(r2) {
					break
				}
				j += n2
			}
			out = append(out, word{s[i:j], j, true})
			i = j
		case unicode.IsSpace(r):
			i += n
		default:
			out = append(out, word{s[i : i+n], i + n, false})
			i += n
		}
	}
	return out
}

// isName reports whether an identifier token is one of the block's constants. A one-letter name
// never counts: it would match the article "A".
func isName(t word, names map[string]bool) bool {
	return t.ident && names[t.text] && utf8.RuneCountInString(t.text) >= 2
}

// subject returns the constant a comment opens with, as Go doc comments do ("StateIdle is ..."),
// and whether that opening names several constants ("ReadIdle and WriteIdle ...",
// "ReadIdle, WriteIdle ..."). It is empty when the comment does not open with a constant's name.
func subject(toks []word, names map[string]bool) (string, bool) {
	if len(toks) == 0 || !isName(toks[0], names) {
		return "", false
	}
	j := 1
	if j < len(toks) && toks[j].text == "," {
		j++
	}
	if j < len(toks) && toks[j].ident && (strings.EqualFold(toks[j].text, "and") || strings.EqualFold(toks[j].text, "or")) {
		j++
	}
	return toks[0].text, j > 1 && j < len(toks) && isName(toks[j], names)
}

// constWord stands for any constant name of the block in a template.
const constWord = "\x00"

// template returns the comment's identifier words, lower-cased, with every constant name of the
// block replaced by one placeholder.
func template(toks []word, names map[string]bool) []string {
	var out []string
	for _, t := range toks {
		switch {
		case !t.ident:
		case isName(t, names):
			out = append(out, constWord)
		default:
			out = append(out, strings.ToLower(t.text))
		}
	}
	return out
}

// differing returns the one position where two templates of equal length differ: -1 when they are
// equal, -2 when they differ in more than one position or in length.
func differing(a, b []string) int {
	if len(a) != len(b) {
		return -2
	}
	at := -1
	for k := range a {
		if a[k] != b[k] {
			if at >= 0 {
				return -2
			}
			at = k
		}
	}
	return at
}

// nameParts maps each lower-cased word of the constants' names ("kind", "for", "done" for
// KindForDone; snake_case is split too) to the constants that have it.
func nameParts(names []string) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, n := range names {
		for _, p := range splitName(n) {
			if out[p] == nil {
				out[p] = map[string]bool{}
			}
			out[p][n] = true
		}
	}
	return out
}

// splitName splits an identifier into lower-cased words at underscores, at a lower-to-upper case
// change, and before the last capital of an upper-case run that a lower-case letter follows
// (URLLen is URL, Len).
func splitName(n string) []string {
	rs := []rune(n)
	var out []string
	start := 0
	cut := func(i int) {
		if i > start {
			out = append(out, strings.ToLower(string(rs[start:i])))
		}
		start = i
	}
	for i := 0; i < len(rs); i++ {
		switch {
		case rs[i] == '_':
			cut(i)
			start = i + 1
		case i > start && unicode.IsUpper(rs[i]) && unicode.IsLower(rs[i-1]):
			cut(i)
		case i > start && unicode.IsUpper(rs[i]) && unicode.IsUpper(rs[i-1]) && i+1 < len(rs) && unicode.IsLower(rs[i+1]):
			cut(i)
		}
	}
	cut(len(rs))
	return out
}

// singles returns the one constant whose name has a word matching w, or "" when none or several
// do. A word of the name that equals w matches; only when none does, a word that w extends by at
// most two letters ("reads" for Read) matches.
func singles(w string, parts map[string]map[string]bool) string {
	cs := parts[w]
	if len(cs) == 0 {
		found := map[string]bool{}
		for p, pcs := range parts {
			if utf8.RuneCountInString(p) >= 3 && strings.HasPrefix(w, p) && utf8.RuneCountInString(w)-utf8.RuneCountInString(p) <= 2 {
				for c := range pcs {
					found[c] = true
				}
			}
		}
		cs = found
	}
	if len(cs) != 1 {
		return ""
	}
	for c := range cs {
		return c
	}
	return ""
}
