// Package webfs embeds the built Vite frontend (web/dist) into the binary.
// The dist directory is produced by `pnpm build` in web/ and is committed
// only as a build artifact; in development the server can also fall back
// to proxying to the Vite dev server.
package webfs

import "embed"

//go:embed all:dist
var FS embed.FS
