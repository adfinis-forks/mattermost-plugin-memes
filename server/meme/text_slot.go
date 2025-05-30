package meme

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
	"unicode"

	"github.com/ccoveille/go-safecast"
	"github.com/golang/freetype/truetype"
	"github.com/pkg/errors"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

type HorizontalAlignment int

const (
	Left   HorizontalAlignment = -1
	Center HorizontalAlignment = 0
	Right  HorizontalAlignment = 1
)

type VerticalAlignment int

const (
	Top    VerticalAlignment = -1
	Middle VerticalAlignment = 0
	Bottom VerticalAlignment = 1
)

type TextSlot struct {
	Bounds              image.Rectangle
	Font                *truetype.Font
	MaxFontSize         float64
	HorizontalAlignment HorizontalAlignment
	VerticalAlignment   VerticalAlignment
	TextColor           color.Color
	OutlineColor        color.Color
	AllUppercase        bool
}

func (s *TextSlot) Render(img draw.Image, text string) error {
	if s.AllUppercase {
		text = strings.ToUpper(text)
	}

	layout, err := s.TextLayout(text)
	if layout == nil {
		return err
	}

	textColor := s.TextColor
	if textColor == nil {
		textColor = color.Black
	}

	for i, line := range layout.Lines {
		if s.OutlineColor != nil {
			// it's okay, memes aren't supposed to look good
			offset := layout.Face.Metrics().Height / 16
			for _, delta := range []fixed.Point26_6{
				{X: offset, Y: offset},
				{X: -offset, Y: offset},
				{X: -offset, Y: -offset},
				{X: offset, Y: -offset},
			} {
				drawer := font.Drawer{
					Dst:  img,
					Src:  image.NewUniform(s.OutlineColor),
					Face: layout.Face,
					Dot:  layout.LinePositions[i].Add(delta),
				}
				drawer.DrawString(line)
			}
		}

		drawer := font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(textColor),
			Face: layout.Face,
			Dot:  layout.LinePositions[i],
		}
		drawer.DrawString(line)
	}
	return nil
}

type TextLayout struct {
	Face          font.Face
	Lines         []string
	LinePositions []fixed.Point26_6
}

func (s *TextSlot) TextLayout(text string) (*TextLayout, error) {
	fontSize := s.MaxFontSize
	if fontSize == 0.0 {
		fontSize = 80.0
	}

	safeHlimit, err := safecast.ToInt32(s.Bounds.Dx() * 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert bounds to int32")
	}
	hlimit := fixed.Int26_6(safeHlimit)
	safeVlimit, err := safecast.ToInt32(s.Bounds.Dy() * 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert bounds to int32")
	}
	vlimit := fixed.Int26_6(safeVlimit)

	for fontSize >= 6.0 {
		face := truetype.NewFace(s.Font, &truetype.Options{
			Size: fontSize,
		})
		lineLimit := s.Bounds.Dy() / int(face.Metrics().Height/64)
		lines, widths := lines(face, text, hlimit)
		if len(lines) > lineLimit {
			fontSize -= (fontSize + 9) / 10
			continue
		}

		layout := &TextLayout{
			Face:          face,
			Lines:         lines,
			LinePositions: make([]fixed.Point26_6, len(lines)),
		}

		safeY, err := safecast.ToInt32(s.Bounds.Min.Y * 64)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert bounds to int32")
		}
		y := fixed.Int26_6(safeY)
		safeLenLines, err := safecast.ToInt32((len(lines) * 64))
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert lines to int32")
		}
		totalHeight := face.Metrics().Height.Mul(fixed.Int26_6(safeLenLines))
		//nolint:exhaustive
		switch s.VerticalAlignment {
		case Middle:
			y += (vlimit - totalHeight) / 2
		case Bottom:
			y += (vlimit - totalHeight)
		}

		for i, width := range widths {
			safeX, err := safecast.ToInt32(s.Bounds.Min.X * 64)
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert bounds to int32")
			}
			x := fixed.Int26_6(safeX)
			//nolint:exhaustive
			switch s.HorizontalAlignment {
			case Center:
				x += (hlimit - width) / 2
			case Right:
				x += (hlimit - width)
			}
			y += face.Metrics().Height
			layout.LinePositions[i] = fixed.Point26_6{
				X: x,
				Y: y,
			}
		}
		return layout, nil
	}

	return nil, nil
}

func lines(face font.Face, text string, limit fixed.Int26_6) (lines []string, widths []fixed.Int26_6) {
	for text != "" {
		line, width, remaining := firstLine(face, text, limit)
		if line == "" {
			return nil, nil
		}
		lines = append(lines, line)
		widths = append(widths, width)
		text = remaining
	}
	return
}

func firstLine(face font.Face, text string, limit fixed.Int26_6) (string, fixed.Int26_6, string) {
	text = strings.TrimSpace(text)

	pos := 0
	lastBreak := 0

	var width fixed.Int26_6
	var lastBreakWidth fixed.Int26_6

	var prev rune = -1
	for _, r := range text {
		advance, ok := face.GlyphAdvance(r)
		if !ok {
			continue
		}
		if prev >= 0 {
			advance += face.Kern(prev, r)
		}

		if unicode.IsSpace(r) && !unicode.IsSpace(prev) {
			lastBreak = pos
			lastBreakWidth = width
		}

		if width+advance > limit {
			if lastBreak == 0 {
				return string([]rune(text)[:pos]), width, string([]rune(text)[pos:])
			}
			return string([]rune(text)[:lastBreak]), lastBreakWidth, string([]rune(text)[lastBreak:])
		}

		pos++
		width += advance
		prev = r
	}

	return text, width, ""
}
