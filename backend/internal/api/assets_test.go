package api

import (
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/compile"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

func treeWithAssets(paths ...string) *model.Resume {
	t := &model.Resume{}
	for _, p := range paths {
		t.Assets = append(t.Assets, model.AssetRef{Path: p})
	}
	return t
}

func TestCompileAssetsPlaceholdersForMissingImages(t *testing.T) {
	tree := treeWithAssets("picture", "img/logo.png", "fig.pdf")
	got := compileAssets(tree, []byte(`{}`))
	if got["picture.png"] != compile.PlaceholderKey || got["img/logo.png"] != compile.PlaceholderKey {
		t.Fatalf("missing images should get placeholders: %v", got)
	}
	if _, ok := got["fig.pdf"]; ok {
		t.Fatalf("a pdf reference must not be faked: %v", got)
	}
}

func TestCompileAssetsPrefersUploads(t *testing.T) {
	tree := treeWithAssets("picture")
	got := compileAssets(tree, []byte(`{"picture.jpg":"resume/x/abc.jpg"}`))
	if got["picture.jpg"] != "resume/x/abc.jpg" {
		t.Fatalf("upload lost: %v", got)
	}
	if _, ok := got["picture.png"]; ok {
		t.Fatalf("an uploaded reference must not also get a placeholder: %v", got)
	}
}

func TestMarkUploaded(t *testing.T) {
	tree := treeWithAssets("picture", "a.png")
	markUploaded(tree, []byte(`{"picture.png":"k"}`))
	if !tree.Assets[0].Uploaded || tree.Assets[1].Uploaded {
		t.Fatalf("uploaded flags wrong: %+v", tree.Assets)
	}
}

func TestPlaceholderImageDecodes(t *testing.T) {
	for _, name := range []string{"p.png", "p.jpg", "p"} {
		if b := compile.PlaceholderImage(name); len(b) < 100 {
			t.Fatalf("%s: placeholder too small (%d bytes)", name, len(b))
		}
	}
}
