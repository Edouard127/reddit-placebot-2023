### Reddit place bot 2023

- [x] 👍 API-less authentication
- [x] 👍 Worker system
- [x] 👍 Place pixels

## Requirements
- Go 1.20+
- The tor expert bundle (https://www.torproject.org/download/tor/)
- A connection to the internet

## How to use
1. Put the image you would like to draw in the `images` folder in the following format: `image.bmp`
2. Run the tor expert bundle
3. Run `go build .` to build the program
4. Run `./redditplacebot -minX=64 -minY=64` to run the program. The `minX` and `minY` flags are optional and default to 0.

## How does it work
When you put a new user, it will log in using a headless browser by allocating a browser access to that specific client because rod doesn't support multithreaded browser, and then navigate to the r/place reddit.

It will then intercept the websocket to extract the user's token, free the allocation and give it to the next one.

When the process it finished, it will save all the VALID users to the users.json file.

When you run the program again, it will load the users from the file without going through the login process.

The worker system is pretty straightforward, when a new user joins, it will be added to the queue.

At every second, there is a ticker that will invoke a queue checking, it will try to use all the clients to place a pixel.

Each client has an assigned pair of point to color, which represents the pixel that must be exchanged for the right one, of your image.
