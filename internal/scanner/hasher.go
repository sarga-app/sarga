package scanner

import (
	"io"
	"os"

	"github.com/zeebo/xxh3"
)

func HashFile(path string) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // path comes from scanner's validated walk
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return hashReader(f)
}

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
