package memelibrary

import (
	"bytes"
	"image"
	"io/fs"
	"path/filepath"
	"strings"

	// registering decoder functions.
	_ "image/jpeg"
	_ "image/png"

	"github.com/golang/freetype/truetype"

	"github.com/adfinis-forks/mattermost-plugin-memes/server/meme"
)

var (
	fonts     = make(map[string]*truetype.Font)
	images    = make(map[string]image.Image)
	metadata  = make(map[string]*Metadata)
	templates = make(map[string]*meme.Template)
)

func isImageAsset(assetName string) bool {
	ext := strings.ToLower(filepath.Ext(assetName))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png"
}

func mustLoadImage(fsys fs.FS, assetName string) image.Image {
	img, _, err := image.Decode(bytes.NewReader(MustAsset(fsys, assetName)))
	if err != nil {
		panic(err)
	}
	return img
}

func init() {
	fontAssets, _ := AssetDir("assets/fonts")
	err := fs.WalkDir(fontAssets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".ttf" {
			return nil
		}

		fontName := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		font, err := truetype.Parse(MustAsset(fontAssets, path))
		if err != nil {
			panic(err)
		}
		fonts[fontName] = font
		return nil
	})
	if err != nil {
		panic(err)
	}

	imageAssets, _ := AssetDir("assets/images")
	err = fs.WalkDir(imageAssets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if !isImageAsset(d.Name()) {
			return nil
		}

		templateName := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		images[templateName] = mustLoadImage(imageAssets, path)

		return nil
	})
	if err != nil {
		panic(err)
	}

	metadataAssets, _ := AssetDir("assets/metadata")
	err = fs.WalkDir(metadataAssets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(d.Name()) != ".yaml" {
			return nil
		}
		templateName := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		if _, ok := images[templateName]; !ok {
			return nil
		}
		if _, ok := metadata[templateName]; ok {
			return nil
		}
		m, err := ParseMetadata(MustAsset(metadataAssets, path))
		if err != nil {
			return err
		}
		metadata[templateName] = m
		return nil
	})
	if err != nil {
		panic(err)
	}

	for templateName, metadata := range metadata {
		img := images[templateName]

		template := &meme.Template{
			Name:      templateName,
			Image:     img,
			TextSlots: metadata.TextSlots(img.Bounds()),
		}
		templates[templateName] = template
		for _, alias := range metadata.Aliases {
			templates[alias] = template
		}
	}
}

func Memes() map[string]*Metadata {
	return metadata
}

func Template(name string) *meme.Template {
	return templates[name]
}

func PatternMatch(input string) (*meme.Template, []string) {
	for templateName, metadata := range metadata {
		if text := metadata.PatternMatch(input); text != nil {
			return templates[templateName], text
		}
	}
	return nil, nil
}
