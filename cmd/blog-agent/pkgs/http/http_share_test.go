package http

import "testing"

func TestPublicBlogShareURLIncludesAccount(t *testing.T) {
	got := publicBlogShareURL("http", "blog.guccang.cn:8881", "Generate Game", "ztt")
	want := "http://blog.guccang.cn:8881/get?blogname=Generate+Game&account=ztt"
	if got != want {
		t.Fatalf("publicBlogShareURL() = %q, want %q", got, want)
	}
}
