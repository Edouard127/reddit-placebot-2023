package main

import (
	"errors"
	"github.com/sergeymakinen/go-bmp"
	"os"
)

type BMPImage struct {
	Width  int
	Height int
	Colors map[Point]Color
}

type Color struct {
	R uint8
	G uint8
	B uint8
}

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

var canvasConfiguration = map[int]Pair[int, int]{
	0: {0, 0},
	1: {1000, 0},
	2: {2000, 0},
	3: {0, 1000},
	4: {1000, 1000},
	5: {2000, 1000},
}

func (p Point) toPlacePoint(canvas int) Point {
	return Point{
		X: p.X + 1500 - canvasConfiguration[canvas].First,
		Y: p.Y + 1000 - canvasConfiguration[canvas].Second,
	}
}

func LoadBMP(offsetX, offsetY int) *BMPImage {
	f, err := os.Open("images/image.bmp")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			panic("Please add an image to the images/image.bmp file")
		}
		panic(err)
	}

	image, err := bmp.Decode(f)
	if err != nil {
		panic(err)
	}

	bmpImage := &BMPImage{
		Width:  image.Bounds().Dx(),
		Height: image.Bounds().Dy(),
		Colors: make(map[Point]Color, image.Bounds().Dx()),
	}

	for x := 0; x < image.Bounds().Dx(); x++ {
		for y := 0; y < image.Bounds().Dy(); y++ {
			r, g, b, _ := image.At(x, y).RGBA()
			bmpImage.Colors[Point{x + offsetX, y + offsetY}] = Color{
				R: uint8(r),
				G: uint8(g),
				B: uint8(b),
			}
		}
	}

	return bmpImage
}
