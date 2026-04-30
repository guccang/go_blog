package blog

import (
	"module"
	"reflect"
	"testing"
)

func TestCollectPublicBlogTitles(t *testing.T) {
	blogs := map[string]*module.Blog{
		"a":                  {Title: "a", AuthType: module.EAuthType_public},
		"b":                  {Title: "b", AuthType: module.EAuthType_private},
		"c":                  {Title: "c", AuthType: module.EAuthType_public | module.EAuthType_diary},
		publicStateBlogTitle: {Title: publicStateBlogTitle, AuthType: module.EAuthType_private},
	}

	got := collectPublicBlogTitles(blogs)
	want := []string{"a", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectPublicBlogTitles()=%v want=%v", got, want)
	}
}

func TestApplyPublicBlogTitles(t *testing.T) {
	blogs := map[string]*module.Blog{
		"a": {Title: "a", AuthType: module.EAuthType_private},
		"b": {Title: "b", AuthType: module.EAuthType_encrypt},
	}

	applyPublicBlogTitles(blogs, []string{"a", "b", "missing"})

	if (blogs["a"].AuthType & module.EAuthType_public) == 0 {
		t.Fatalf("expected blog a to become public, auth=%d", blogs["a"].AuthType)
	}
	if (blogs["b"].AuthType & module.EAuthType_public) == 0 {
		t.Fatalf("expected blog b to preserve existing flags and add public, auth=%d", blogs["b"].AuthType)
	}
	if (blogs["b"].AuthType & module.EAuthType_encrypt) == 0 {
		t.Fatalf("expected blog b encrypt flag to be preserved, auth=%d", blogs["b"].AuthType)
	}
}
