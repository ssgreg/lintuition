package classify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ssgreg/lintuition/sdk"
)

// Identifier is a classifier whose answers can be cached. Identity names everything that changes
// an answer besides the request: backend, endpoint, model revision, inference settings. A backend
// without it (the scripted fake) is never cached. The identity must not contain credentials.
type Identifier interface {
	Identity() string
}

// CacheVersion changes when the cached record or the key changes shape.
const CacheVersion = "1"

// Bundle is what one cached request holds: the validated answers of every sample. With votes > 1
// it is the whole set of samples, so a replay reproduces the same vote; it is never counted as new
// samples.
type Bundle struct {
	Samples [][]sdk.Answer `json:"samples"`
	Usage   sdk.Usage      `json:"usage"`
	Created time.Time      `json:"created"`
}

// Cache stores validated answer bundles by content hash under a directory.
type Cache struct {
	Dir string
	TTL time.Duration
	Now func() time.Time
}

// Key is the content hash of everything that decides an answer: the cache version, the backend
// identity, the linter's version, the number of samples, the approved state and the questions.
func Key(identity, linterVersion string, votes int, req sdk.Request) (string, error) {
	b, err := json.Marshal(struct {
		V         string
		Identity  string
		Linter    string
		Votes     int
		State     map[string]any
		Questions []sdk.Question
	}{CacheVersion, identity, linterVersion, votes, req.State, req.Questions})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (c *Cache) path(key string) string { return filepath.Join(c.Dir, key[:2], key+".json") }

// Lookup returns a fresh bundle that holds exactly votes valid samples for the questions, or false.
// A corrupt, expired, short or invalid record is a miss, so the next request replaces it.
func (c *Cache) Lookup(key string, votes int, qs []sdk.Question) (Bundle, bool) {
	bd, ok := c.Get(key)
	if !ok || len(bd.Samples) != votes {
		return Bundle{}, false
	}
	for _, s := range bd.Samples {
		if _, err := Check(qs, sdk.Response{Answers: s}); err != nil {
			return Bundle{}, false
		}
	}
	return bd, true
}

// Get returns a fresh bundle, or false. A corrupt or expired record is a miss.
func (c *Cache) Get(key string) (Bundle, bool) {
	b, err := os.ReadFile(c.path(key))
	if err != nil {
		return Bundle{}, false
	}
	var bd Bundle
	if err := json.Unmarshal(b, &bd); err != nil || len(bd.Samples) == 0 {
		return Bundle{}, false
	}
	if c.TTL > 0 && c.now().Sub(bd.Created) > c.TTL {
		return Bundle{}, false
	}
	return bd, true
}

// Put stores a bundle atomically: a reader sees the old record or the new one, never half of one.
// Files are private to the user: answers can echo private prose.
func (c *Cache) Put(key string, bd Bundle) error {
	if len(bd.Samples) == 0 {
		return errors.New("cache: empty bundle")
	}
	bd.Created = c.now()
	b, err := json.Marshal(bd)
	if err != nil {
		return err
	}
	dir := filepath.Dir(c.path(key))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return err
	}
	// On a failure the temporary file is removed best-effort; the write's error is the one to report.
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	if err := os.Chmod(f.Name(), 0o600); err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	return os.Rename(f.Name(), c.path(key))
}

var (
	shardRE  = regexp.MustCompile(`^[0-9a-f]{2}$`)
	recordRE = regexp.MustCompile(`^[0-9a-f]{64}\.json$`)
	tempRE   = regexp.MustCompile(`^\.tmp-[0-9]+$`)
)

// Clean removes cache records, and only them: files named like a record (or a temporary record)
// inside two-hex-digit shard directories directly under Dir. Anything else in Dir, a file someone
// else put there or a whole project if Dir was pointed at one, is left alone. With expiredOnly it
// removes only records older than the TTL. Empty shard directories are removed afterwards.
func (c *Cache) Clean(expiredOnly bool) (int, error) {
	if c.Dir == "" {
		return 0, errors.New("cache: no directory")
	}
	shards, err := os.ReadDir(c.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n := 0
	for _, sh := range shards {
		if !sh.IsDir() || !shardRE.MatchString(sh.Name()) {
			continue
		}
		dir := filepath.Join(c.Dir, sh.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			return n, err
		}
		for _, f := range files {
			if f.IsDir() || !recordRE.MatchString(f.Name()) && !tempRE.MatchString(f.Name()) {
				continue
			}
			p := filepath.Join(dir, f.Name())
			if expiredOnly {
				key := strings.TrimSuffix(f.Name(), ".json")
				if _, fresh := c.Get(key); fresh {
					continue
				}
			}
			if err := os.Remove(p); err != nil {
				return n, err
			}
			n++
		}
		_ = os.Remove(dir) // only succeeds when empty
	}
	return n, nil
}

func (c *Cache) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// DefaultCacheDir is the user cache directory for lintuition answers.
func DefaultCacheDir() (string, error) {
	d, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "lintuition", "answers"), nil
}
