package library

import (
	"io"
	"os"

	"github.com/zeebo/xxh3"
)

// HashFile computes the xxh3-128 content hash of the file at path and returns
// it as a 16-byte slice suitable for storage in the track_files.content_hash
// BLOB column (CHECK length = 16).
func HashFile(path string) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // path comes from scanner's validated walk
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return hashReader(f)
}

// HashBytes computes the xxh3-128 hash of b and returns it as a 16-byte slice.
// Used for cover art deduplication.
func HashBytes(b []byte) []byte {
	u := xxh3.Hash128(b)
	arr := u.Bytes()
	return arr[:]
}

func hashReader(r io.Reader) ([]byte, error) {
	h := xxh3.New128()
	if _, err := io.Copy(h, r); err != nil {
		return nil, err
	}
	arr := h.Sum128().Bytes()
	return arr[:], nil
}
