package memelibrary

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsImageAsset(t *testing.T) {
	assert.True(t, isImageAsset("anakin-right.jpg"))
	assert.False(t, isImageAsset("anakin-right.json"))
}

func TestMustLoadImage(t *testing.T) {
	assert.NotPanics(t, func() {
		img := mustLoadImage(assets, "assets/images/anakin-right.jpg")
		assert.NotNil(t, img)
	})

	assert.Panics(t, func() {
		mustLoadImage(assets, "this-asset-does-not-exist.jpg")
	})

	assert.Panics(t, func() {
		mustLoadImage(assets, "assets/metadata/anakin-right.yaml")
	})
}

func TestTemplate(t *testing.T) {
	assert.Nil(t, Template("not-a-template"))

	template := Template("anakin-right")
	require.NotNil(t, template)
	assert.NotNil(t, template.Image)
}

func TestPatternMatch(t *testing.T) {
	for name, metadata := range Memes() {
		for _, pattern := range metadata.Patterns {
			template, text := PatternMatch(pattern.Example)
			assert.Equal(t, Template(name), template)
			assert.NotNil(t, text)
		}
	}
}
