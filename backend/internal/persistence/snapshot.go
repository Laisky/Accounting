package persistence

// SnapshotSink loads and stores a single JSON-serializable snapshot document for
// one domain. File-backed domain stores persist their whole in-memory snapshot
// through a sink.
type SnapshotSink interface {
	// Load decodes the current snapshot into dst, leaving dst untouched when no
	// snapshot has been stored yet (mirrors LoadJSON on a missing file).
	Load(dst any) error
	// Save persists value as the current snapshot.
	Save(value any) error
}

// FileSink persists a snapshot as an atomic JSON file.
type FileSink struct {
	path string
}

// NewFileSink returns a SnapshotSink backed by an atomic JSON file at path.
func NewFileSink(path string) *FileSink {
	return &FileSink{path: path}
}

// Load decodes the JSON file into dst, or leaves dst unchanged when it is absent.
func (s *FileSink) Load(dst any) error {
	return LoadJSON(s.path, dst)
}

// Save writes value to the JSON file with an atomic rename.
func (s *FileSink) Save(value any) error {
	return SaveJSONAtomic(s.path, value)
}
