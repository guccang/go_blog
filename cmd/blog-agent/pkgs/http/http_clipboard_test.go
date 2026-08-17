package http

import (
	"persistence"
	"reflect"
	"testing"
)

func TestUniqueClipboardImageIDs(t *testing.T) {
	got := uniqueClipboardImageIDs([]string{" a ", "a", "", "bad/path", "b"})
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("uniqueClipboardImageIDs() = %#v, want %#v", got, want)
	}
}

func TestClipboardResponseBuildsAccountProtectedMediaURLs(t *testing.T) {
	got := clipboardResponse(persistence.ClipboardItem{ID: "item", ImageIDs: []string{"one", "two"}})
	want := []string{"/media/one", "/media/two"}
	if !reflect.DeepEqual(got.Images, want) {
		t.Fatalf("clipboardResponse().Images = %#v, want %#v", got.Images, want)
	}
}
