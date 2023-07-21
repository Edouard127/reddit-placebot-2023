package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"go.uber.org/zap"
	"golang.org/x/net/websocket"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	*zap.Logger
	Username    string `json:"username"`
	Password    string `json:"password"`
	AccessToken string `json:"access_token"`

	Browser      *Browser               `json:"-"`
	Socket       *websocket.Conn        `json:"-"`
	Page         *rod.Page              `json:"-"`
	Cookies      []*proto.NetworkCookie `json:"cookies"`
	AssignedData *CircularQueue[Pair[Point, Color]]

	packetid int
}

func (cl *Client) Login(board *Board, wg *sync.WaitGroup) error {
	defer wg.Done()

	if cl.AccessToken != "" {
		go cl.connect(board)
		cl.Info("Login successful")
		return nil
	}

	cl.Browser.Request(cl)
	defer cl.Browser.Free()

	if cl.Cookies == nil {
		cl.Page = cl.Browser.MustPage("https://old.reddit.com/login")

		cl.Page.MustElement("#user_login").MustInput(cl.Username)
		cl.Page.MustElement("#passwd_login").MustInput(cl.Password)
		cl.Page.MustElement("#rem_login").MustClick()

		cl.Page.MustElement("#login-form > div.c-clearfix.c-submit-group > button").MustClick()
		cl.Page.MustWaitStable()

		// Get current url
		if cl.Page.MustInfo().URL != "https://old.reddit.com/" {
			cl.Error("Login failed", zap.String("username", cl.Username))
			return fmt.Errorf("login failed")
		}

		cl.Cookies = cl.Page.MustCookies()
	} else {
		cl.Page = cl.Browser.MustPage("https://old.reddit.com/")
		cl.Page.MustSetCookies(toParam(cl.Cookies)...)
		cl.Page.MustReload()
		cl.Page.MustWaitStable()
	}

	cl.getAccessToken()
	go cl.connect(board)

	cl.Info("Login successful")
	return nil
}

func (cl *Client) getAccessToken() {
	var connInit ConnectionInit

	cl.Page = cl.Browser.MustPage("https://www.reddit.com/r/place/")

	wait := cl.Page.EachEvent(func(e *proto.NetworkWebSocketFrameSent) bool {
		json.Unmarshal([]byte(e.Response.PayloadData), &connInit)
		return true
	})

	wait()

	cl.AccessToken = connInit.Payload.Authorization
}

func (cl *Client) connect(board *Board) {
	var err error

	login, _ := json.Marshal(ConnectionInit{
		Type: "connection_init",
		Payload: Authorization{
			Authorization: cl.AccessToken,
		},
	})

	cl.packetid++

	subscribe, _ := json.Marshal(Subscribe{
		Id:   fmt.Sprintf("%d", cl.packetid),
		Type: "start",
		Payload: Var[VarInput[SubscribeConfig]]{
			Variables: VarInput[SubscribeConfig]{
				Input: Input[SubscribeConfig]{
					Channel: SubscribeConfig{
						TeamOwner: "GARLICBREAD",
						Category:  "CONFIG",
					},
				},
			},
			OperationName: "configuration",
			Query:         "subscription configuration($input: SubscribeInput!) {\n  subscribe(input: $input) {\n    id\n    ... on BasicMessage {\n      data {\n        __typename\n        ... on ConfigurationMessageData {\n          colorPalette {\n            colors {\n              hex\n              index\n              __typename\n            }\n            __typename\n          }\n          canvasConfigurations {\n            index\n            dx\n            dy\n            __typename\n          }\n          activeZone {\n            topLeft {\n              x\n              y\n              __typename\n            }\n            bottomRight {\n              x\n              y\n              __typename\n            }\n            __typename\n          }\n          canvasWidth\n          canvasHeight\n          adminConfiguration {\n            maxAllowedCircles\n            maxUsersPerAdminBan\n            __typename\n          }\n          __typename\n        }\n      }\n      __typename\n    }\n    __typename\n  }\n}\n",
		},
	})

	getCanvas := func(tag string) []byte {
		cl.packetid++
		data, _ := json.Marshal(Replace{
			Id:   fmt.Sprintf("%d", cl.packetid),
			Type: "start",
			Payload: Var[VarInput[SubscribeReplace]]{
				Variables: VarInput[SubscribeReplace]{
					Input: Input[SubscribeReplace]{
						Channel: SubscribeReplace{
							TeamOwner: "GARLICBREAD",
							Category:  "CANVAS",
							Tag:       tag,
						},
					},
				},
				OperationName: "replace",
				Query:         "subscription replace($input: SubscribeInput!) {\n  subscribe(input: $input) {\n    id\n    ... on BasicMessage {\n      data {\n        __typename\n        ... on FullFrameMessageData {\n          __typename\n          name\n          timestamp\n        }\n        ... on DiffFrameMessageData {\n          __typename\n          name\n          currentTimestamp\n          previousTimestamp\n        }\n      }\n      __typename\n    }\n    __typename\n  }\n}\n",
			},
		})
		return data
	}

	cl.Socket.Write(login)

	var errorPayload ConnectionUnauthorized
	for {
		var message string
		err = websocket.Message.Receive(cl.Socket, &message)
		if err != nil {
			fmt.Println("Error receiving message from socket", err)
			return
		}

		json.Unmarshal([]byte(message), &errorPayload)

		if errorPayload.Type != "connection_error" {
			break
		} else {
			cl.Error("Connection error", zap.String("username", cl.Username), zap.String("message", errorPayload.Payload.Message))
		}
	}

	cl.Socket.Write(subscribe)

	var data SubscribedData
	for {
		var message string
		err = websocket.Message.Receive(cl.Socket, &message)
		if err != nil {
			fmt.Println("Error receiving message from socket", err)
			return
		}

		json.Unmarshal([]byte(message), &data)
		fmt.Println(message)

		if data.Type == "data" {
			break
		}
	}

	board.SetController(cl) // Do not remove
	board.SetColors(cl, data.Payload.Data.Subscribe.Data.ColorPalette.Colors)
	board.SetRequiredData(cl, ImageColorConvert(LoadBMP(board.Start.X, board.Start.Y)))

	cl.Socket.Write(getCanvas("1"))
	cl.Socket.Write(getCanvas("2"))
	cl.Socket.Write(getCanvas("3"))
	cl.Socket.Write(getCanvas("4"))
	cl.Socket.Write(getCanvas("5"))

	var canvasData CanvasUpdate
	for {
		var message string
		err = websocket.Message.Receive(cl.Socket, &message)
		if err != nil {
			fmt.Println("Error receiving message from socket", err)
			return
		}

		json.Unmarshal([]byte(message), &canvasData)

		board.SetCurrentData(cl, canvasData.Payload.Data.Subscribe.Data.Name)
	}
}

func (cl *Client) Assign(data map[Point]Color) {
	for point, color := range data {
		cl.AssignedData.Enqueue(Pair[Point, Color]{point, color})
	}
}

// Place places a pixel at the given point, does not require a browser allocation
// Fix: It doesn't return any errors but doesn't place a pixel either
func (cl *Client) Place(board *Board) time.Time {
	data := cl.AssignedData.Dequeue()
	fmt.Println("Placing pixel", data.First, data.Second)

	place, _ := json.Marshal(Place{
		OperationName: "setPixel",
		Query:         "mutation setPixel($input: ActInput!) {\n  act(input: $input) {\n    data {\n      ... on BasicMessage {\n        id\n        data {\n          ... on GetUserCooldownResponseMessageData {\n            nextAvailablePixelTimestamp\n            __typename\n          }\n          ... on SetPixelResponseMessageData {\n            timestamp\n            __typename\n          }\n          __typename\n        }\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}\n",
		Variables: PlacePixel{
			Input: PlaceInput{
				ActionName: "r/replace:set_pixel",
				PixelMessageData: PlaceData{
					CanvasIndex: board.GetCanvasIndex(Point{-499, -495}),
					ColorIndex:  GetColorIndex(data.Second),
					Coordinate:  Point{-494, 495}.toPlacePoint(),
				},
			},
		},
	})

	req, err := http.NewRequest("POST", "https://gql-realtime-2.reddit.com/query", bytes.NewReader(place))
	if err != nil {
		cl.Error("Error creating request", zap.Error(err))
		return time.Now()
	}

	req.Header.Set("Authorization", cl.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://hot-potato.reddit.com")
	req.Header.Set("Referer", "https://hot-potato.reddit.com/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cl.Error("Error sending request", zap.Error(err))
		return time.Now()
	}

	defer resp.Body.Close()

	var response Error

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		cl.Error("Error decoding response", zap.Error(err))
		return time.Now()
	}

	if len(response.Errors) == 0 {
		return time.Now().Add(5 * time.Minute)
	}

	return time.Unix(int64(response.Errors[0].Extensions.NextAvailablePixelTimestamp.(float64))/1000, int64(response.Errors[0].Extensions.NextAvailablePixelTimestamp.(float64)))
}

func (cl *Client) GetCookie(fn func(*proto.NetworkCookie) bool) *proto.NetworkCookie {
	for _, cookie := range cl.Cookies {
		if fn(cookie) {
			return cookie
		}
	}

	return nil
}

func toParam(cookies []*proto.NetworkCookie) []*proto.NetworkCookieParam {
	var cookiesParam []*proto.NetworkCookieParam

	for _, cookie := range cookies {
		cookiesParam = append(cookiesParam, &proto.NetworkCookieParam{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Secure:   cookie.Secure,
			HTTPOnly: cookie.HTTPOnly,
			SameSite: cookie.SameSite,
			Expires:  cookie.Expires,
		})
	}

	return cookiesParam
}
