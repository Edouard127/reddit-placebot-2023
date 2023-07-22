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

func (p Point) toPlacePoint(canvas int) Point {
	// canvas 0 -> x+1500 y+1000
	// canvas 1 -> x+500 y+1000
	// canvas 2 -> x+1500 y+0
	// canvas 3 -> x+500 y+0
	// canvas 4 -> x+500 y+0
	// canvas 5 -> x+500 y+0
	var x, y int
	switch canvas {
	case 0:
		x = p.X + 1500
		y = p.Y + 1000
	case 1:
		x = p.X + 500
		y = p.Y + 1000
	case 2:
		x = p.X + 1500
		y = p.Y
	default:
		x = p.X + 500
		y = p.Y
	}

	return Point{x, y}
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
