//go:build !embed_frontend

package web

import "embed"

func Assets() embed.FS {
	return embed.FS{}
}
