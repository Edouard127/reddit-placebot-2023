package main

import (
	"fmt"
	"image/png"
	"math"
	"net/http"
)

type Board struct {
	Start, End   Point
	RequiredData *BMPImage // The image to draw on the canvas
	CurrentData  *BMPImage // Only the canvas data between Start and End, so we don't flood the memory and the cpu
	controller   *Client   // Only one client will control the information to the board, so we don't flood the memory and the cpu
}

func NewBoard(start Point) *Board {
	return &Board{Start: start}
}

func (b *Board) GetCanvasIndex(at Point) int {
	at.X += 1500

	if at.X >= 0 && at.X < 3000 {
		if at.Y >= 0 {
			return int(at.X)/1000 + 3
		} else {
			return int(at.X) / 1000
		}
	}

	panic(fmt.Sprintf("Point %v is not in the canvas", at))
}

func (b *Board) GetDifferentData() map[Point]Color {
	differentData := make(map[Point]Color, 0)

	for point, color := range b.RequiredData.Colors {
		if b.CurrentData.Colors[point] != color {
			differentData[point] = color
		}
	}

	return differentData
}

func (b *Board) SetController(controller *Client) {
	if b.controller == nil {
		b.controller = controller
		b.controller.Logger.Info("Controller changed")
	}
}

func (b *Board) checkForController(c *Client) bool {
	return b.controller == c && b.controller != nil
}

func (b *Board) SetColors(c *Client, colors []SubscribeColor) {
	if !b.checkForController(c) {
		return
	}

	SetActiveColors(colors)
}

// SetRequiredData should be called after we're connected to the websocket and received the SubscribedData
func (b *Board) SetRequiredData(c *Client, data *BMPImage) {
	if !b.checkForController(c) {
		return
	}

	b.End = Point{X: data.Width, Y: data.Height}
	b.RequiredData = data
}

func (b *Board) SetCurrentData(c *Client, url string) {
	if !b.checkForController(c) {
		return
	}

	b.downloadImage(url)
}

func (b *Board) WaitForData() {
	for b.RequiredData == nil || b.CurrentData == nil {
	}
}

var Colors = map[int]Color{
	0:  hexToRGB("#6D001A"), // Darkest Red
	1:  hexToRGB("#BE0039"), // Dark Red
	2:  hexToRGB("#FF4500"), // Red
	3:  hexToRGB("#FFA800"), // Orange
	4:  hexToRGB("#FFD635"), // Yellow
	5:  hexToRGB("#FFF8B8"), // Light Yellow
	6:  hexToRGB("#00A368"), // Dark Green
	7:  hexToRGB("#00CC78"), // Green
	8:  hexToRGB("#7EED56"), // Light Green
	9:  hexToRGB("#00756F"), // Dark Teal
	10: hexToRGB("#009EAA"), // Teal
	11: hexToRGB("#00CCC0"), // Light Teal
	12: hexToRGB("#2450A4"), // Dark Blue
	13: hexToRGB("#3690EA"), // Blue
	14: hexToRGB("#51E9F4"), // Light Blue
	15: hexToRGB("#493AC1"), // Indigo
	16: hexToRGB("#6A5CFF"), // Periwinkle
	17: hexToRGB("#94B3FF"), // Lavender
	18: hexToRGB("#811E9F"), // Dark Purple
	19: hexToRGB("#B44AC0"), // Purple
	20: hexToRGB("#E4ABFF"), // Light Purple
	21: hexToRGB("#DE107F"), // Magenta
	22: hexToRGB("#FF3881"), // Pink
	23: hexToRGB("#FF99AA"), // Light Pink
	24: hexToRGB("#6D482F"), // Dark Brown
	25: hexToRGB("#9C6926"), // Brown
	26: hexToRGB("#FFB470"), // Beige
	27: hexToRGB("#000000"), // Black
	28: hexToRGB("#515252"), // Dark Gray
	29: hexToRGB("#898D90"), // Gray
	30: hexToRGB("#D4D7D9"), // Light Gray
	31: hexToRGB("#FFFFFF"), // White
}

var ActiveColors = make(map[int]Color, 0)

func SetActiveColors(colors []SubscribeColor) {
	for _, color := range colors {
		ActiveColors[color.Index] = Colors[color.Index]
	}
}

func GetColorIndex(color Color) int {
	for index, c := range ActiveColors {
		if c == color {
			return index
		}
	}
	return -1
}

func ImageColorConvert(image *BMPImage) *BMPImage {
	for point, color := range image.Colors {
		image.Colors[point] = *closestColor(color)
	}
	return image
}

func closestColor(color Color) *Color {
	var closestColor *Color
	var closestDistance = math.MaxFloat64
	for _, c := range Colors {
		distance := euclideanDistance(color, c)
		if closestDistance == 0 || distance < closestDistance {
			closestDistance = distance
			closestColor = &c
		}
	}
	return closestColor
}

func hexToRGB(hexColor string) Color {
	if len(hexColor) > 0 && hexColor[0] == '#' {
		hexColor = hexColor[1:]
	}

	var r, g, b uint8
	n, err := fmt.Sscanf(hexColor, "%02x%02x%02x", &r, &g, &b)
	if err != nil || n != 3 {
		panic(err)
	}

	return Color{r, g, b}
}

func (b *Board) downloadImage(url string) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}

	image, err := png.Decode(resp.Body)
	if err != nil {
		panic(err)
	}

	bmpImage := &BMPImage{
		Width:  image.Bounds().Dx(),
		Height: image.Bounds().Dy(),
		Colors: make(map[Point]Color, image.Bounds().Dx()),
	}

	for x := b.Start.X; x < b.End.X; x++ {
		for y := b.Start.Y; y < b.End.Y; y++ {
			r, g, b, _ := image.At(x, y).RGBA()
			bmpImage.Colors[Point{x, y}] = Color{uint8(r), uint8(g), uint8(b)}
		}
	}

	b.CurrentData = bmpImage
}
