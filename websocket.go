package main

type Payload[T any] struct {
	Id      string `json:"id"`
	Type    string `json:"type"`
	Payload T      `json:"payload"`
}

type Authorization struct {
	Authorization string `json:"Authorization"`
}

type Var[Chan any] struct {
	Variables     Chan   `json:"variables"`
	OperationName string `json:"operationName"`
	Query         string `json:"query"`
}

type VarInput[Chan any] struct {
	Input Input[Chan] `json:"input"`
}

type Input[Chan any] struct {
	Channel Chan `json:"channel"`
}

type SubscribeConfig struct {
	TeamOwner string `json:"teamOwner"`
	Category  string `json:"category"`
}

type SubscribeReplace struct {
	TeamOwner string `json:"teamOwner"`
	Category  string `json:"category"`
	Tag       string `json:"tag"`
}

type Message struct {
	Message string `json:"message"`
}

type SubscribeResponse struct {
	Data DataIndexer[BoardData] `json:"data"`
}

type DataIndexer[Indexer any] struct {
	Subscribe SubscribeData[Indexer] `json:"subscribe"`
}

type SubscribeData[Data any] struct {
	Id   string `json:"id"`
	Data Data   `json:"data"`
}

type BoardData struct {
	ColorPalette ColorPalette           `json:"colorPalette"`
	Canvas       []CanvasConfigurations `json:"canvasConfigurations"`
	Active       ActiveZone             `json:"activeZone"`
	CanvasWidth  int                    `json:"canvasWidth"`
	CanvasHeight int                    `json:"canvasHeight"`
}

type ColorPalette struct {
	Colors []SubscribeColor `json:"colors"`
}

type SubscribeColor struct {
	Hex   string `json:"hex"`
	Index int    `json:"index"`
}

type CanvasConfigurations struct {
	Index int `json:"index"`
	Dx    int `json:"dx"`
	Dy    int `json:"dy"`
}

type ActiveZone struct {
	TopLeft     Point `json:"topLeft"`
	BottomRight Point `json:"bottomRight"`
}

type CanvasUpdateData struct {
	Data DataIndexer[CanvasInfo] `json:"data"`
}

type CanvasInfo struct {
	CurrentTimestamp  int64  `json:"currentTimestamp"`
	Name              string `json:"name"`
	PreviousTimestamp int64  `json:"previousTimestamp"`
}

type PlacePixel struct {
	Input PlaceInput `json:"input"`
}

type PlaceInput struct {
	ActionName       string    `json:"actionName"`
	PixelMessageData PlaceData `json:"PixelMessageData"`
}

type PlaceData struct {
	CanvasIndex int   `json:"canvasIndex"`
	ColorIndex  int   `json:"colorIndex"`
	Coordinate  Point `json:"coordinate"`
}

type ConnectionInit Payload[Authorization]
type Subscribe Payload[Var[VarInput[SubscribeConfig]]]
type Replace Payload[Var[VarInput[SubscribeReplace]]]
type Place Var[PlacePixel]

type ConnectionUnauthorized Payload[Message]
type SubscribedData Payload[SubscribeResponse]
type CanvasUpdate Payload[CanvasUpdateData]
