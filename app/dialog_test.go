package main

import (
	"reflect"
	"testing"
)

func TestCleanPaths(t *testing.T) {
	in := []string{"/a/b.png", "", "/a/../a/b.png", "/c d/e.txt", "/c d/e.txt/"}
	want := []string{"/a/b.png", "/c d/e.txt"}
	if got := cleanPaths(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanPaths = %v, want %v", got, want)
	}
	if got := cleanPaths(nil); got == nil || len(got) != 0 {
		t.Fatalf("cleanPaths(nil) harus slice kosong non-nil, dapat %#v", got)
	}
}
