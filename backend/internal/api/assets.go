package api

import (
	"encoding/json"
	"path"
	"strings"

	"github.com/harsh3dev/multi-resume/backend/internal/compile"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

// graphicsExts are the extensions LaTeX tries when an image reference has none.
var graphicsExts = []string{".png", ".jpg", ".jpeg"}

// assetMapOf decodes drafts.asset_map: { "<tex path>": "<object key>" }.
func assetMapOf(raw []byte) map[string]string {
	m := map[string]string{}
	_ = json.Unmarshal(raw, &m)
	return m
}

// uploadedPath returns the file in m that satisfies the reference ref, or "".
// A reference without an extension (\photo{picture}) is satisfied by any
// supported extension, because LaTeX resolves the extension itself.
func uploadedPath(m map[string]string, ref string) string {
	if _, ok := m[ref]; ok {
		return ref
	}
	if path.Ext(ref) == "" {
		for _, ext := range graphicsExts {
			if _, ok := m[ref+ext]; ok {
				return ref + ext
			}
		}
	}
	return ""
}

// markUploaded sets Uploaded on every image reference from the asset map. The
// flag is derived on read, never stored, so it can't go stale.
func markUploaded(tree *model.Resume, raw []byte) {
	if tree == nil {
		return
	}
	m := assetMapOf(raw)
	for i := range tree.Assets {
		tree.Assets[i].Uploaded = uploadedPath(m, tree.Assets[i].Path) != ""
	}
}

// compileAssets builds the assets argument for Compile: every uploaded file,
// plus a generated placeholder for each referenced image that has none yet, so
// a template with a photo still compiles before the user uploads theirs.
func compileAssets(tree *model.Resume, raw []byte) map[string]string {
	out := assetMapOf(raw)
	if tree == nil {
		return out
	}
	for _, a := range tree.Assets {
		if uploadedPath(out, a.Path) != "" {
			continue
		}
		target := a.Path
		switch strings.ToLower(path.Ext(target)) {
		case "":
			target += ".png"
		case ".png", ".jpg", ".jpeg":
		default:
			continue // not a raster image we can stand in for (pdf, eps, svg…)
		}
		if compile.ValidateAssetPath(target) != nil {
			continue
		}
		out[target] = compile.PlaceholderKey
	}
	return out
}
