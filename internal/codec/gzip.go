// Package codec provides shared encoding/decoding utilities used across
// the relay, agent, and MITM layers.
package codec

import (
	"bytes"
	"compress/gzip"
	"io"
)

// Gunzip decompresses a gzip-compressed byte slice. Returns the uncompressed
// data or an error if the input is not valid gzip.
func Gunzip(b []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return out, nil
}

// IsGzipMagic reports whether b starts with the gzip magic bytes (0x1f 0x8b).
func IsGzipMagic(b []byte) bool {
	return len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b
}