package memelibrary

import (
	"embed"
	"io/fs"
)

//go:embed assets/**
var assets embed.FS

func AssetDir(name string) (fs.FS, error) {
	return fs.Sub(assets, name) //nolint:wrapcheck
}

func MustAsset(fsys fs.FS, name string) []byte {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		panic(err)
	}
	return data
}
