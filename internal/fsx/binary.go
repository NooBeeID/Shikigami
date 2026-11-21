package fsx

import (
	"bytes"
	"unicode/utf8"
)

// binarySampleSize adalah jumlah byte awal yang diperiksa untuk deteksi biner.
const binarySampleSize = 8 * 1024

// IsBinary melaporkan apakah sampel (awal file) tampak biner: mengandung NUL
// atau bukan UTF-8 valid dalam 8 KiB pertama.
func IsBinary(sample []byte) bool {
	truncated := len(sample) > binarySampleSize
	if truncated {
		sample = sample[:binarySampleSize]
	}
	if len(sample) == 0 {
		return false
	}
	if bytes.IndexByte(sample, 0) >= 0 {
		return true
	}
	if truncated {
		// Buang potongan rune tidak lengkap di ekor akibat pemotongan.
		for back := 1; back <= utf8.UTFMax-1 && back <= len(sample); back++ {
			i := len(sample) - back
			if utf8.RuneStart(sample[i]) {
				if !utf8.FullRune(sample[i:]) {
					sample = sample[:i]
				}
				break
			}
		}
	}
	return !utf8.Valid(sample)
}
