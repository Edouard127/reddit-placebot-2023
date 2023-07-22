package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"go.uber.org/zap"
	"golang.org/x/net/proxy"
	"golang.org/x/net/websocket"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"sync"
	"time"
)

type Client struct {
	*zap.Logger `json:"-"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	AccessToken string `json:"access_token"`

	HTTP          *http.Client                       `json:"-"`
	Browser       *Browser                           `json:"-"`
	Socket        *websocket.Conn                    `json:"-"`
	Page          *rod.Page                          `json:"-"`
	Cookies       []*proto.NetworkCookie             `json:"cookies"`
	AssignedData  *CircularQueue[Pair[Point, Color]] `json:"-"`
	ProxyRotation *CircularQueue[string]             `json:"-"`

	packetid int
}

func (cl *Client) Login(board *Board, wg *sync.WaitGroup) error {
	defer wg.Done()
	defer cl.Setup()

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
		Payload: Var[VarInput[Input[SubscribeConfig]]]{
			Variables: VarInput[Input[SubscribeConfig]]{
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
			Payload: Var[VarInput[Input[SubscribeReplace]]]{
				Variables: VarInput[Input[SubscribeReplace]]{
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

func (cl *Client) Setup() {
	dialer, err := proxy.SOCKS5("tcp", "127.0.0.1:9050", nil, proxy.Direct)
	if err != nil {
		panic(err)
	}

	jar, _ := cookiejar.New(nil)

	cl.HTTP = &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		},
		Jar: jar,
	}

	cookies := make([]*http.Cookie, len(cl.Cookies))
	for i, cookie := range cl.Cookies {
		cookies[i] = &http.Cookie{
			Name:  cookie.Name,
			Value: cookie.Value,
		}
	}

	cl.HTTP.Jar.SetCookies(&url.URL{
		Scheme: "https",
		Host:   ".reddit.com",
		Path:   "/",
	}, cookies)
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
	fmt.Println("Placing pixel", data.First, data.Second, board.GetCanvasIndex(data.First), data.First.toPlacePoint(board.GetCanvasIndex(data.First)))

	place, _ := json.Marshal(Place{
		OperationName: "setPixel",
		Query:         "mutation setPixel($input: ActInput!) {\n  act(input: $input) {\n    data {\n      ... on BasicMessage {\n        id\n        data {\n          ... on GetUserCooldownResponseMessageData {\n            nextAvailablePixelTimestamp\n            __typename\n          }\n          ... on SetPixelResponseMessageData {\n            timestamp\n            __typename\n          }\n          __typename\n        }\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}\n",
		Variables: PlacePixel{
			Input: PlaceInput[PlaceData]{
				ActionName: "r/replace:set_pixel",
				PixelMessageData: PlaceData{
					CanvasIndex: board.GetCanvasIndex(data.First),
					ColorIndex:  GetColorIndex(data.Second),
					Coordinate:  data.First.toPlacePoint(board.GetCanvasIndex(data.First)),
				},
			},
		},
	})

	req, err := http.NewRequest("POST", "https://gql-realtime-2.reddit.com/query", bytes.NewReader(place))
	if err != nil {
		cl.Error("Error creating request", zap.Error(err))
		if errors.Is(err, os.ErrDeadlineExceeded) {
			cl.Setup()
			return cl.Place(board)
		}
		return time.Now()
	}

	req.Header.Set("Authorization", cl.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://hot-potato.reddit.com")
	req.Header.Set("Referer", "https://hot-potato.reddit.com/")

	resp, err := cl.HTTP.Do(req)
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
		if cl.GetPlaceHistory(data.First, board.GetCanvasIndex(data.First)).Data.Act.Data[0].Data.UserInfo.Username != cl.Username {
			cl.Info("There was an error placing pixel", zap.String("message", "Pixel was not placed, or was placed somewhere else"))
		}
		return time.Now().Add(5 * time.Minute).Add(time.Duration(rand.Intn(60)) * time.Second)
	} else {
		cl.Info("Error placing pixel", zap.String("message", response.Errors[0].Message))

		if response.Errors[0].Message == "Ratelimited" {
			if int64(response.Errors[0].Extensions.NextAvailablePixelTimestamp.(float64))/1000 == math.MaxInt32 {
				cl.Info("Account has been banned from r/place")
			}

			return time.Unix(int64(response.Errors[0].Extensions.NextAvailablePixelTimestamp.(float64))/1000, int64(response.Errors[0].Extensions.NextAvailablePixelTimestamp.(float64))).Add(time.Duration(rand.Intn(60)) * time.Second)
		}

		if response.Errors[0].Message == "unable to verify user" {
			cl.Info("Account does not have access to r/place due to not being email verified")
		}

		return time.Now().Add(math.MaxInt32 * time.Second)
	}
}

func (cl *Client) GetPlaceHistory(at Point, canvas int) HistoryResponse {
	history, _ := json.Marshal(History{
		OperationName: "pixelHistory",
		Query:         "mutation pixelHistory($input: ActInput!) {\n  act(input: $input) {\n    data {\n      ... on BasicMessage {\n        id\n        data {\n          ... on GetTileHistoryResponseMessageData {\n            lastModifiedTimestamp\n            userInfo {\n              userID\n              username\n              __typename\n            }\n            __typename\n          }\n          __typename\n        }\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}\n",
		Variables: VarInput[PlaceInput[PlaceData]]{
			Input: PlaceInput[PlaceData]{
				ActionName: "r/replace:get_tile_history",
				PixelMessageData: PlaceData{
					CanvasIndex: canvas,
					Coordinate:  at.toPlacePoint(canvas),
				},
			},
		},
	})

	req, err := http.NewRequest("POST", "https://gql-realtime-2.reddit.com/query", bytes.NewReader(history))
	if err != nil {
		cl.Error("Error creating request", zap.Error(err))
	}

	req.Header.Set("Authorization", cl.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://hot-potato.reddit.com")
	req.Header.Set("Referer", "https://hot-potato.reddit.com/")

	resp, err := cl.HTTP.Do(req)
	if err != nil {
		cl.Error("Error sending request", zap.Error(err))
	}

	defer resp.Body.Close()

	var response HistoryResponse

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		cl.Error("Error decoding response", zap.Error(err))
	}

	return response
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
