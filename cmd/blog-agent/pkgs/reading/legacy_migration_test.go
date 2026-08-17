package reading

import "testing"

func TestIsLegacyReadingFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "reading_book_曾国藩传.md", want: true},
		{name: "reading_book_资治通鉴.MD", want: true},
		{name: "reading_book_资治通鉴.json", want: false},
		{name: "普通博客.md", want: false},
	}
	for _, test := range tests {
		if got := isLegacyReadingFile(test.name); got != test.want {
			t.Fatalf("isLegacyReadingFile(%q) = %t, want %t", test.name, got, test.want)
		}
	}
}

func TestImportLegacyReadingBookRejectsInvalidJSONBeforeStorage(t *testing.T) {
	if _, err := importLegacyReadingBook("ztt", "reading_book_坏数据.md", []byte(`{"book":null}`)); err == nil {
		t.Fatal("expected invalid reading JSON error")
	}
}
