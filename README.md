### Reddit place bot 2023

- [x] 👍 API-less authentication
- [x] 👍 Worker system
- [x] 👍 Place pixels

## Requirements
- Go 1.20+ (https://go.dev/dl/)
- The Tor Expert Bundle (https://www.torproject.org/download/tor/)
- A connection to the internet

## How to use
You need to download the Tor Expert Bundle from https://www.torproject.org/download/tor/

FOR WINDOWS USERS: You need to add the tor.exe inside the tor folder to your PATH, you can do that by following this tutorial: https://www.architectryan.com/2018/03/17/add-to-the-path-on-windows-10/

If you wish to build, the project, click [here to get the build tutorial](#how-to-build)

Then you need to either [build the project](#how-to-build), or download the latest build from the actions tab, click on the first action and go to the artifacts.

Once you have to program, you must add users in the file data/users.json.rename and then rename it to data/users.json

After that, you have to put an image in the BMP format in the images folder, named image.bmp

Then, you can run the program with `./redditplacebot.exe -minX=64 -minY=64` to start the program, the `minX` and `minY` flags represent the top left of your image in the r/place canvas.

## How to build
Download and install Golang 1.20+ from https://golang.org/dl/

Open a terminal and run `go build .` inside the scripts folder

A .exe file called `redditplacebot` will appear after the process (go build .) ends

As simple as that

## How does it work
When you put a new user, it will log in using a headless browser by allocating a browser access to that specific client because rod doesn't support multithreaded browser, and then navigate to the r/place reddit.

It will then intercept the websocket to extract the user's token, free the allocation and give it to the next one.

When the process it finished, it will save all the VALID users to the users.json file.

When you run the program again, it will load the users from the file without going through the login process.

The worker system is pretty straightforward, when a new user joins, it will be added to the queue.

At every second, there is a ticker that will invoke a queue checking, it will try to use all the clients to place a pixel.

Each client has an assigned pair of point to color, which represents the pixel that must be exchanged for the right one, of your image.

## How to avoid getting banned
Use a rotating Tor configuration

Put this in a file named `torrc` on Windows at %APPDATA%\tor\
```
CircuitBuildTimeout 30
LearnCircuitBuildTimeout 0 
MaxCircuitDirtiness 30
```
This will assure a new circuit every 30 seconds

By the way I am not paid for any of this code, the more I'm getting paid, the more I will work on this.
If you'd like to support you can create an issue and will give you my crypto addresses.
