package live

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"go.etcd.io/bbolt"
	"pkg.jsn.cam/jsn/internal/fsdiff/snapshot"
)

var (
	bucketMeta     = []byte("meta")
	bucketBaseline = []byte("baseline")
	bucketChanges  = []byte("changes")
	bucketContent  = []byte("content")
)

// Store manages persistent storage for a recording session
type Store struct {
	db  *bbolt.DB
	seq uint64 // sequence number for unique keys
}

// Meta contains session metadata
type Meta struct {
	StartTime time.Time `json:"start"`
	RootPath  string    `json:"root"`
	Interval  int       `json:"interval"` // seconds
}

// OpenStore opens or creates a bbolt database for the session
func OpenStore(path string) (*Store, error) {
	db, err := bbolt.Open(path, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Create buckets if they don't exist
	err = db.Update(func(tx *bbolt.Tx) error {
		for _, bucket := range [][]byte{bucketMeta, bucketBaseline, bucketChanges, bucketContent} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create buckets: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database
func (s *Store) Close() error {
	return s.db.Close()
}

// SaveMeta saves session metadata
func (s *Store) SaveMeta(meta *Meta) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		data, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		return b.Put([]byte("meta"), data)
	})
}

// LoadMeta loads session metadata
func (s *Store) LoadMeta() (*Meta, error) {
	var meta Meta
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		data := b.Get([]byte("meta"))
		if data == nil {
			return fmt.Errorf("no meta found")
		}
		return json.Unmarshal(data, &meta)
	})
	return &meta, err
}

// HasBaseline checks if a baseline snapshot exists
func (s *Store) HasBaseline() bool {
	var exists bool
	s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketBaseline)
		exists = b.Get([]byte("snapshot")) != nil
		return nil
	})
	return exists
}

// SaveBaseline saves the initial snapshot (compressed gob)
func (s *Store) SaveBaseline(snap *snapshot.Snapshot) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketBaseline)

		// Compress with gzip
		var buf bytes.Buffer
		gzw := gzip.NewWriter(&buf)
		enc := gob.NewEncoder(gzw)
		if err := enc.Encode(snap); err != nil {
			gzw.Close()
			return fmt.Errorf("encode baseline: %w", err)
		}
		if err := gzw.Close(); err != nil {
			return fmt.Errorf("close gzip: %w", err)
		}

		return b.Put([]byte("snapshot"), buf.Bytes())
	})
}

// LoadBaseline loads the initial snapshot
func (s *Store) LoadBaseline() (*snapshot.Snapshot, error) {
	var snap snapshot.Snapshot
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketBaseline)
		data := b.Get([]byte("snapshot"))
		if data == nil {
			return fmt.Errorf("no baseline found")
		}

		// Decompress
		gzr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("open gzip: %w", err)
		}
		defer gzr.Close()

		dec := gob.NewDecoder(gzr)
		if err := dec.Decode(&snap); err != nil {
			return fmt.Errorf("decode baseline: %w", err)
		}
		return nil
	})
	return &snap, err
}

// AppendChange appends a change record to the changes bucket
func (s *Store) AppendChange(change *Change) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChanges)

		// Use timestamp + sequence for unique, ordered keys
		seq := atomic.AddUint64(&s.seq, 1)
		key := []byte(fmt.Sprintf("%020d_%010d", change.Timestamp.UnixNano(), seq))

		data, err := json.Marshal(change)
		if err != nil {
			return err
		}
		return b.Put(key, data)
	})
}

// AppendChanges appends multiple changes in a single transaction
func (s *Store) AppendChanges(changes []*Change) error {
	if len(changes) == 0 {
		return nil
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChanges)
		for _, change := range changes {
			seq := atomic.AddUint64(&s.seq, 1)
			key := []byte(fmt.Sprintf("%020d_%010d", change.Timestamp.UnixNano(), seq))
			data, err := json.Marshal(change)
			if err != nil {
				return err
			}
			if err := b.Put(key, data); err != nil {
				return err
			}
		}
		return nil
	})
}

// LoadChanges loads all changes from the database
func (s *Store) LoadChanges() ([]*Change, error) {
	var changes []*Change
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChanges)
		return b.ForEach(func(k, v []byte) error {
			var change Change
			if err := json.Unmarshal(v, &change); err != nil {
				return err
			}
			changes = append(changes, &change)
			return nil
		})
	})
	return changes, err
}

// ChangeCount returns the number of changes recorded
func (s *Store) ChangeCount() int {
	var count int
	s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChanges)
		count = b.Stats().KeyN
		return nil
	})
	return count
}

// SaveContent saves file content (deduplicated by hash)
func (s *Store) SaveContent(hash string, content []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketContent)
		// Only save if not already present (deduplication)
		if b.Get([]byte(hash)) != nil {
			return nil
		}
		return b.Put([]byte(hash), content)
	})
}

// LoadContent loads file content by hash
func (s *Store) LoadContent(hash string) ([]byte, error) {
	var content []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketContent)
		data := b.Get([]byte(hash))
		if data == nil {
			return fmt.Errorf("content not found: %s", hash)
		}
		content = make([]byte, len(data))
		copy(content, data)
		return nil
	})
	return content, err
}

// ContentExists checks if content with the given hash exists
func (s *Store) ContentExists(hash string) bool {
	var exists bool
	s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketContent)
		exists = b.Get([]byte(hash)) != nil
		return nil
	})
	return exists
}

// ExportJSON exports the entire session as JSON
func (s *Store) ExportJSON(w io.Writer) error {
	type Export struct {
		Meta     *Meta              `json:"meta"`
		Baseline *snapshot.Snapshot `json:"baseline"`
		Changes  []*Change          `json:"changes"`
		Content  map[string]string  `json:"content"` // hash -> base64 content
	}

	meta, err := s.LoadMeta()
	if err != nil {
		return fmt.Errorf("load meta: %w", err)
	}

	baseline, err := s.LoadBaseline()
	if err != nil {
		return fmt.Errorf("load baseline: %w", err)
	}

	changes, err := s.LoadChanges()
	if err != nil {
		return fmt.Errorf("load changes: %w", err)
	}

	// Collect content
	content := make(map[string]string)
	s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketContent)
		return b.ForEach(func(k, v []byte) error {
			// Store as string (assumes text content)
			content[string(k)] = string(v)
			return nil
		})
	})

	export := Export{
		Meta:     meta,
		Baseline: baseline,
		Changes:  changes,
		Content:  content,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(export)
}
