# What is this?

The Hushcom project is a sample distributed IRC-like chat program, which demonstrates how the [ratnet project](https://github.com/awgh/ratnet) can be used to develop a full application.

The project includes both a client and server program.

# Documentation

API Docs are available here: https://godoc.org/github.com/awgh/hushcom

# Hushcom Client

The main executable is in the hushcom/ folder, which also includes a REST-ful web interface that serves the actual UI to the browser.

The client/ folder implements the message handling/passing logic of the client.

# Hushcom Server

The main executable is in the hushcomd/ folder.

The server/ folder implements the message handling/passing logic of the server.

# Hushcom Points of Interest

Hushcom is interesting as an example for several reasons:

- Implements signed and authenticated messages over ratnet, which does not normally provide them. This is meant to demonstrate how easy this is to do using existing components, as well as providing tested sample code to accomplish this.

- Implements a browser-based UI using a REST interface and internal web server.

- Cross-compiles to Android, where it has been deployed as an APK (native hushcomd plus simple UI app that just opens a browser widget to localhost)

- Distributed IRC scheme. The Hushcom server only knows user and channel names and PUBLIC keys. The server cannot read the contents of any channel or private messages.

#Demo

There is an include webix-based demo which can be launched with the "go.bat" or "go.sh" command from the "wwwtest" directory. This will launch two client sessions which can be connected to via:

- https://localhost:20011/js/hc.html 
- https://localhost:20021/js/hc.html

SSL Certs are self-signed, so you will likely have to click through some warnings.

From this point, you can create user profiles, add and join channels, and talk to yourself from one browser session to the other.

# Related Projects

- [Bencrypt, crypto abstraction layer & utils](https://github.com/awgh/bencrypt)
- [Ratnet, onion-routed messaging system with pluggable transports](https://github.com/awgh/ratnet)
- [HushCom, simple IRC-like client & server](https://github.com/awgh/hushcom)

#Authors and Contributors

awgh@awgh.org (@awgh)
