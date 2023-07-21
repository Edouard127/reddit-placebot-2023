package main

import (
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

func (p Point) toPlacePoint() Point {
	return Point{pointAbsolute(p.X, 500), p.Y} // idk
}

func pointAbsolute(point, size int) int {
	if point < 0 {
		return size + point
	}
	return point
}

func LoadBMP(offsetX, offsetY int) *BMPImage {
	f, err := os.Open("images/image.bmp")
	if err != nil {
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
