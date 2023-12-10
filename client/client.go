package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Edouard127/redditplacebot/board"
	"github.com/Edouard127/redditplacebot/util"
	"github.com/Edouard127/redditplacebot/web"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"go.uber.org/zap"
	"golang.org/x/net/proxy"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"nhooyr.io/websocket/wsjson"
	"os"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

type Client struct {
	*zap.Logger `json:"-"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	AccessToken string `json:"access_token"`

	HTTP         *http.Client                                             `json:"-"`
	Browser      *Browser                                                 `json:"-"`
	WSconfig     *websocket.DialOptions                                   `json:"-"`
	Socket       *websocket.Conn                                          `json:"-"`
	Page         *rod.Page                                                `json:"-"`
	Cookies      []*proto.NetworkCookie                                   `json:"cookies"`
	AssignedData *util.CircularQueue[util.Pair[board.Point, board.Color]] `json:"-"`

	packetid int
}

func (cl *Client) Login(wg *sync.WaitGroup) error {
	defer wg.Done()
	defer cl.Setup()

	if cl.AccessToken != "" {
		go cl.connect()
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
	var connInit web.ConnectionInit

	cl.Page = cl.Browser.MustPage("https://www.reddit.com/r/place/")

	wait := cl.Page.EachEvent(func(e *proto.NetworkWebSocketFrameSent) bool {
		json.Unmarshal([]byte(e.Response.PayloadData), &connInit)
		return true
	})

	wait()

	cl.AccessToken = connInit.Payload.Authorization
}

func (cl *Client) connect() {
	var err error
	cl.Socket, _, err = websocket.Dial(context.Background(), "wss://gql-realtime-2.reddit.com/query", cl.WSconfig)
	if err != nil {
		cl.Error("Failed to connect to websocket", zap.Error(err))
		return
	}
	defer cl.Socket.Close(websocket.StatusNormalClosure, "user closed connection")

	login := web.ConnectionInit{
		Type: "connection_init",
		Payload: web.Authorization{
			Authorization: cl.AccessToken,
		},
	}

	cl.packetid++

	subscribe := web.Subscribe{
		Id:   fmt.Sprintf("%d", cl.packetid),
		Type: "start",
		Payload: web.Var[web.VarInput[web.Input[web.SubscribeConfig]]]{
			Variables: web.VarInput[web.Input[web.SubscribeConfig]]{
				Input: web.Input[web.SubscribeConfig]{
					Channel: web.SubscribeConfig{
						TeamOwner: "GARLICBREAD",
						Category:  "CONFIG",
					},
				},
			},
			OperationName: "configuration",
			Query:         "subscription configuration($input: SubscribeInput!) {\n  subscribe(input: $input) {\n    id\n    ... on BasicMessage {\n      data {\n        __typename\n        ... on ConfigurationMessageData {\n          colorPalette {\n            colors {\n              hex\n              index\n              __typename\n            }\n            __typename\n          }\n          canvasConfigurations {\n            index\n            dx\n            dy\n            __typename\n          }\n          activeZone {\n            topLeft {\n              x\n              y\n              __typename\n            }\n            bottomRight {\n              x\n              y\n              __typename\n            }\n            __typename\n          }\n          canvasWidth\n          canvasHeight\n          adminConfiguration {\n            maxAllowedCircles\n            maxUsersPerAdminBan\n            __typename\n          }\n          __typename\n        }\n      }\n      __typename\n    }\n    __typename\n  }\n}\n",
		},
	}

	getCanvas := func(tag string) web.Replace {
		cl.packetid++
		return web.Replace{
			Id:   fmt.Sprintf("%d", cl.packetid),
			Type: "start",
			Payload: web.Var[web.VarInput[web.Input[web.SubscribeReplace]]]{
				Variables: web.VarInput[web.Input[web.SubscribeReplace]]{
					Input: web.Input[web.SubscribeReplace]{
						Channel: web.SubscribeReplace{
							TeamOwner: "GARLICBREAD",
							Category:  "CANVAS",
							Tag:       tag,
						},
					},
				},
				OperationName: "replace",
				Query:         "subscription replace($input: SubscribeInput!) {\n  subscribe(input: $input) {\n    id\n    ... on BasicMessage {\n      data {\n        __typename\n        ... on FullFrameMessageData {\n          __typename\n          name\n          timestamp\n        }\n        ... on DiffFrameMessageData {\n          __typename\n          name\n          currentTimestamp\n          previousTimestamp\n        }\n      }\n      __typename\n    }\n    __typename\n  }\n}\n",
			},
		}
	}

	err = wsjson.Write(context.Background(), cl.Socket, login)

	var errorPayload web.ConnectionUnauthorized
	for {
		err = wsjson.Read(context.Background(), cl.Socket, &errorPayload)
		if err != nil {
			fmt.Println("Error receiving message from socket", err)
			return
		}

		if errorPayload.Type != "connection_error" {
			break
		} else {
			cl.Error("Connection error", zap.String("username", cl.Username), zap.String("message", errorPayload.Payload.Message))
		}
	}

	err = wsjson.Write(context.Background(), cl.Socket, subscribe)

	var data web.SubscribedData
	for {
		err = wsjson.Read(context.Background(), cl.Socket, &data)
		if err != nil {
			fmt.Println("Error receiving message from socket", err)
			return
		}

		if data.Type == "data" {
			break
		}
	}

	board.SetController(cl) // Do not remove
	board.SetColors(cl, data.Payload.Data.Subscribe.Data.ColorPalette.Colors)

	err = wsjson.Write(context.Background(), cl.Socket, getCanvas("0"))
	err = wsjson.Write(context.Background(), cl.Socket, getCanvas("1"))
	err = wsjson.Write(context.Background(), cl.Socket, getCanvas("2"))
	err = wsjson.Write(context.Background(), cl.Socket, getCanvas("3"))
	err = wsjson.Write(context.Background(), cl.Socket, getCanvas("4"))
	err = wsjson.Write(context.Background(), cl.Socket, getCanvas("5"))

	var canvasData web.CanvasUpdate
	for {
		err = wsjson.Read(context.Background(), cl.Socket, &canvasData)
		if err != nil {
			fmt.Println("Error receiving message from socket", err)
			return
		}

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

	go func() {
		for {
			select {
			case <-time.NewTicker(time.Second * 60).C:
				cl.Setup()
			default:

			}
		}
	}()
}

func (cl *Client) Assign(data map[board.Point]board.Color) {
	for point, color := range data {
		cl.AssignedData.Enqueue(util.Pair[board.Point, board.Color]{First: point, Second: color})
	}
}

// Place places a pixel at the given point, does not require a browser allocation
// Fix: It doesn't return any errors but doesn't place a pixel either
func (cl *Client) Place(color, canvas int) time.Time {
	data := cl.AssignedData.Dequeue()

	place, _ := json.Marshal(web.Place{
		OperationName: "setPixel",
		Query:         "mutation setPixel($input: ActInput!) {\n  act(input: $input) {\n    data {\n      ... on BasicMessage {\n        id\n        data {\n          ... on GetUserCooldownResponseMessageData {\n            nextAvailablePixelTimestamp\n            __typename\n          }\n          ... on SetPixelResponseMessageData {\n            timestamp\n            __typename\n          }\n          __typename\n        }\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}\n",
		Variables: web.PlacePixel{
			Input: web.PlaceInput[web.PlaceData]{
				ActionName: "r/replace:set_pixel",
				PixelMessageData: web.PlaceData{
					CanvasIndex: canvas,
					ColorIndex:  color,
					Coordinate:  data.First.ToPlacePoint(canvas),
				},
			},
		},
	})

	req, err := http.NewRequest("POST", "https://gql-realtime-2.reddit.com/query", bytes.NewReader(place))
	if err != nil {
		cl.Error("Error creating request", zap.Error(err))
		if errors.Is(err, os.ErrDeadlineExceeded) {
			cl.Setup()
			return cl.Place(color, canvas)
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

	var response web.Error

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		cl.Error("Error decoding response", zap.Error(err))
		return time.Now()
	}

	if len(response.Errors) == 0 {
		if cl.GetPlaceHistory(data.First, canvas).Data.Act.Data[0].Data.UserInfo.Username != cl.Username {
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

func (cl *Client) GetPlaceHistory(at board.Point, canvas int) web.HistoryResponse {
	history, _ := json.Marshal(web.History{
		OperationName: "pixelHistory",
		Query:         "mutation pixelHistory($input: ActInput!) {\n  act(input: $input) {\n    data {\n      ... on BasicMessage {\n        id\n        data {\n          ... on GetTileHistoryResponseMessageData {\n            lastModifiedTimestamp\n            userInfo {\n              userID\n              username\n              __typename\n            }\n            __typename\n          }\n          __typename\n        }\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}\n",
		Variables: web.VarInput[web.PlaceInput[web.PlaceData]]{
			Input: web.PlaceInput[web.PlaceData]{
				ActionName: "r/replace:get_tile_history",
				PixelMessageData: web.PlaceData{
					CanvasIndex: canvas,
					Coordinate:  at.ToPlacePoint(canvas),
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

	var response web.HistoryResponse

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
