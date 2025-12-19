//go:build js && wasm

package assets

import "embed"

// TexturesFS embeds the default watercolor texture PNGs for js/wasm builds.
//
// NOTE: go:embed patterns must not use ".." and must be relative to this file.
// Keeping the embed source here (repo-root assets/) allows us to embed assets
// without duplicating files.
//
//go:embed textures/*.png
var TexturesFS embed.FS
