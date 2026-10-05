// Package testname holds twins for test-name-vs-assertion.
//
// Every explanation in the tests is a separate comment, a blank line above the test, so it never
// reaches the classifier: a test's doc comment is sent, and a hint in it would give the answer away.
package testname

import "errors"

// ErrBadQuote is returned for an unbalanced quote.
var ErrBadQuote = errors.New("unbalanced quote")

// Unquote removes the quotes around s.
func Unquote(s string) (string, error) {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", ErrBadQuote
	}
	return s[1 : len(s)-1], nil
}

// Store keeps values.
type Store interface{ Put(key, v string) error }

// Cache keeps copies of values for fast reads.
type Cache interface{ Write(key, v string) error }

// Saver writes values to a store and a cache.
type Saver struct {
	Store Store
	Cache Cache
}

// Save puts v into the store, then writes it to the cache. A failed cache write is ignored: the
// cache only speeds up reads.
func (s *Saver) Save(key, v string) error {
	if err := s.Store.Put(key, v); err != nil {
		return err
	}
	_ = s.Cache.Write(key, v)
	return nil
}

// Wrap prefixes err; a nil err stays nil.
func Wrap(err error, prefix string) error {
	if err == nil {
		return nil
	}
	return errors.New(prefix + ": " + err.Error())
}
