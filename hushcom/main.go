package main

import (
	"bencrypt"
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hushcom"
	"hushcom/client"
	"log"
	"net/http"
	"os"
	"ratnet"
	"ratnet/modules"
	"ratnet/transports"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/coocood/jas"
)

// go get github.com/coocood/jas

// usage: ./hushcom -dbfile=ratnet2.ql -p=20003 -ap=21003

var (
	// Settings
	updateInterval       = 500 * time.Millisecond
	secondsToCache int64 = 300

	// Instance Handles
	db func() *sql.DB
	hc *client.Client

	// last poll times
	lastPollLocal, lastPollRemote int64

	// container for wait groups
	wg = &sync.WaitGroup{}
)

func init() {
	hc = (modules.Dispatchers[hushcom.ClientID]).(*client.Client)
	hc.CurrentProfileName = ""
	hc.CurrentProfilePubKey = nil
	lastPollLocal = 0
	lastPollRemote = 0
}

// pollServer will keep trying until either we get a result or the timeout expires
func pollServer(host string, pubsrv string, db func() *sql.DB) (bool, error) {
	//	log.Println("Last Polls Local/Remote:", lastPollLocal, lastPollRemote)

	// Pickup Local
	var a ratnet.ApiCall
	a.Action = "ID"
	a.Args = []string{}
	rpubkey, err := transports.RemoteAPI(host, &a)
	//	log.Println("Remote ID Result:", rpubkey, err)
	if err != nil {
		return false, err
	}

	a.Action = "PickupMail"
	a.Args = []string{string(rpubkey), strconv.FormatInt(lastPollLocal, 10)}
	toRemote, err := ratnet.Api(&a, db, true, "pollServer local pickup: ")
	if err != nil {
		return false, err
	}

	// Pickup Remote
	a.Action = "PickupMail"
	a.Args = []string{string(pubsrv), strconv.FormatInt(lastPollRemote, 10)}
	toLocal, err := transports.RemoteAPI(host, &a)
	//	log.Println("Remote PickupMail Result:", toLocal, err)
	if err != nil {
		return false, err
	}

	// Strip Timestamps
	binary.Read(bytes.NewBuffer(toRemote[:8]), binary.BigEndian, &lastPollLocal)
	binary.Read(bytes.NewBuffer(toLocal[:8]), binary.BigEndian, &lastPollRemote)
	toLocal = toLocal[8:]
	toRemote = toRemote[8:]
	//log.Println("New Polls Local/Remote:", lastPollLocal, lastPollRemote)

	/*if len(toRemote) > 0 {
		log.Println("Local PickupMail Result:", string(toRemote))
	}*/

	// Dropoff Remote
	if len(toRemote) > 0 {
		a.Action = "DeliverMail"
		a.Args = []string{string(toRemote)}
		_, err := transports.RemoteAPI(host, &a)
		//		log.Println("Remote DeliverMail Result:", res, err)
		if err != nil {
			return false, err
		}
	}
	// Dropoff Local
	if len(toLocal) > 0 {
		a.Action = "DeliverMail"
		a.Args = []string{string(toLocal)}
		_, err := ratnet.Api(&a, db, true, "pollServer local deliver: ")
		//		log.Println("Local DeliverMail Result:", res, err)
		if err != nil {
			return false, err
		}
	}
	return true, nil
	//}
	//}
}

func clientLoop(pubsrv string, db func() *sql.DB) {
	counter := 0
	for {
		time.Sleep(updateInterval)

		// Get Server List
		var a ratnet.ApiCall
		a.Action = "GetServer"
		a.Args = []string{}
		result, err := ratnet.Api(&a, db, true)
		if err != nil {
			log.Println("GetServer error in ClientLoop: ", err)
			continue
		}
		var servers []ratnet.ServerConf
		err = json.Unmarshal([]byte(result), &servers)
		if err != nil {
			log.Println("JSON Unmarshal error in ClientLoop: ", err)
			continue
		}
		for _, element := range servers {
			if element.Enabled {
				//t := time.Now()
				_, err := pollServer(element.Uri, pubsrv, db)
				//dur := time.Now().Sub(t)
				//log.Println("pollServer completed in: ", dur)

				if err != nil {
					log.Println("pollServer error: ", err.Error())
				}
			}
		}

		if counter%500 == 0 {
			ratnet.FlushOutbox(db(), secondsToCache)
		}
		counter++
	}
}

func handleCORS(r *http.Request, responseHeader http.Header) bool {
	responseHeader.Add("Access-Control-Allow-Origin", "*")
	responseHeader.Add("Access-Control-Allow-Methods", "GET,POST,PUT")
	if r.Method == "OPTIONS" {
		return false
	}
	return true
}

func serve(database string, listenRest string, certfile string, keyfile string) {

	db = ratnet.BootstrapDB(database)
	hc.Database = db
	ratnet.FlushOutbox(db(), 0)

	transports.InitSSL(certfile, keyfile)

	// start REST api second, since it does not trigger cert generation
	router := jas.NewRouter(new(Profile), new(Server), new(Channel), new(Remote), new(Poll))
	router.BasePath = "/v1/"
	router.HandleCORS = handleCORS

	// Serve the static javascript files
	pwd, _ := os.Getwd()
	log.Println("CWD: " + pwd)
	fs := http.FileServer(http.Dir("js"))
	http.Handle("/js/", http.StripPrefix("/js/", fs))

	fmt.Println(router.HandledPaths(true))
	http.Handle(router.BasePath, router)

	var id ratnet.ApiCall
	id.Action = "ID"
	pubsrv, err := ratnet.Api(&id, db, true)
	if err != nil {
		log.Fatal(err.Error())
	}

	var dst ratnet.ApiCall
	dst.Action = "AddDest"
	dst.Args = []string{client.HUSHCOM, client.HUSHCOMPKA}
	_, err = ratnet.Api(&dst, db, true)
	if err != nil {
		log.Fatal(err.Error())
	}

	var a ratnet.ApiCall
	a.Action = "UpdateServer"
	debug := false //todo: make this a CLI param
	if debug {
		a.Args = []string{"localhost", "https://127.0.0.1:20001", "1"}
	} else if runtime.GOOS == "android" {
		a.Args = []string{"awgh.wtf.im", "https://104.233.91.188:20001", "1"}
	} else {
		a.Args = []string{"awgh.wtf.im", "https://awgh.wtf.im:20001", "1"}
	}

	_, err = ratnet.Api(&a, db, true)
	if err != nil {
		log.Fatal(err.Error())
	}

	wg.Add(1)
	go func() {
		err = http.ListenAndServeTLS(listenRest, certfile, keyfile, nil)
		log.Println("TLS server crash: ", err)
		wg.Done()
	}()

	clientLoop(string(pubsrv), db)
}

func main() {

	var dbFile string
	var restPort int

	flag.StringVar(&dbFile, "dbfile", "ratnet.ql", "QL Database File")
	flag.IntVar(&restPort, "p", 20011, "HTTPS REST Port (localhost)")

	flag.Parse()
	restString := fmt.Sprintf("localhost:%d", restPort)

	serve(dbFile, restString, "cert.pem", "key.pem")
}

//
//  REST API
//

func jaserr(ctx *jas.Context, err error) {
	if err != nil {
		ctx.Error = jas.NewRequestError(err.Error())
	}
}

// Poll - Hacky AJAX endpoint, todo: replace me with web sockets
type Poll struct{}

// Get poll updates
func (*Poll) Get(ctx *jas.Context) { // `GET /v1/poll`
	ctx.Data = client.Output
	client.Output = ""
	//log.Println("poll result: ", ctx.Data)
}

// Profile - Rest Calls for Profiles
type Profile struct{}

// Get profile list
func (*Profile) Get(ctx *jas.Context) { // `GET /v1/profile`
	var a ratnet.ApiCall
	a.Action = "GetProfiles"
	a.Args = []string{}
	result, err := ratnet.Api(&a, db, true)
	jaserr(ctx, err)
	ctx.Data = result
	//	log.Println("getprofile result:", result)
}

// PutLoad - Load a profile
func (*Profile) PutLoad(ctx *jas.Context) { // `PUT /v1/profile/load`
	/*
		body:  Name=abc
	*/
	name := ctx.RequireString("Name")
	var a ratnet.ApiCall
	a.Action = "LoadProfile"
	a.Args = []string{name}
	result, err := ratnet.Api(&a, db, true)
	jaserr(ctx, err)
	key, err := base64.StdEncoding.DecodeString(string(result))
	jaserr(ctx, err)
	hc.CurrentProfileName = name
	hc.CurrentProfilePubKey = key

	ctx.Data = "OK"
}

// Put - Add a new profile or update existing profile
func (*Profile) Put(ctx *jas.Context) { // `PUT /v1/profile`
	/*
		body:  Name=abc&Enabled=true
	*/
	name := ctx.RequireString("Name")
	enabled := ctx.RequireString("Enabled")
	var a ratnet.ApiCall
	a.Action = "UpdateProfile"
	a.Args = []string{name, enabled}
	result, err := ratnet.Api(&a, db, true)
	jaserr(ctx, err)
	ctx.Data = result
}

// PostDelete - Delete a profile
func (*Profile) PostDelete(ctx *jas.Context) { // `POST /v1/profile/delete`
	/*
		body:  Name=abc
	*/
	name := ctx.RequireString("Name")
	var a ratnet.ApiCall
	a.Action = "DeleteProfile"
	a.Args = []string{name}
	result, err := ratnet.Api(&a, db, true)
	jaserr(ctx, err)
	ctx.Data = result
}

// Server - Rest Calls for Servers
type Server struct{}

// Get server list
func (*Server) Get(ctx *jas.Context) { // `GET /v1/server`
	var a ratnet.ApiCall
	a.Action = "GetServer"
	a.Args = []string{}
	result, err := ratnet.Api(&a, db, true)
	jaserr(ctx, err)
	ctx.Data = result
}

// Put - Add a new server or update existing server
func (*Server) Put(ctx *jas.Context) { // `PUT /v1/server`
	name := ctx.RequireString("Name")
	uri := ctx.RequireString("Uri")
	enabled := ctx.RequireString("Enabled")
	var a ratnet.ApiCall
	a.Action = "UpdateServer"
	a.Args = []string{name, uri, enabled}
	result, err := ratnet.Api(&a, db, true)
	jaserr(ctx, err)
	ctx.Data = result
}

// PostDelete - Delete a server
func (*Server) PostDelete(ctx *jas.Context) { // `POST /v1/server/delete`
	name := ctx.RequireString("Name")
	var a ratnet.ApiCall
	a.Action = "DeleteServer"
	a.Args = []string{name}
	result, err := ratnet.Api(&a, db, true)
	jaserr(ctx, err)
	ctx.Data = result
}

// Channel - Rest Calls for Channels
type Channel struct{}

// Get channel list
func (*Channel) Get(ctx *jas.Context) { // `GET /v1/channel`
	var a ratnet.ApiCall
	a.Action = "GetChannels"
	a.Args = []string{}
	result, err := ratnet.Api(&a, db, true)
	jaserr(ctx, err)
	ctx.Data = result
}

// PostDelete - Delete a channel
func (*Channel) PostDelete(ctx *jas.Context) { // `POST /v1/channel/delete`
	/*
		body:  Name=abc
	*/
	name := ctx.RequireString("Name")
	var a ratnet.ApiCall
	a.Action = "DeleteChannel"
	a.Args = []string{name}
	//log.Println(name)
	result, err := ratnet.Api(&a, db, true)
	jaserr(ctx, err)
	ctx.Data = result
}

// Post - Send message to a channel
func (*Channel) Post(ctx *jas.Context) { // `POST /v1/channel`
	/*
		body:  Name=abc&Data=message_data
	*/
	name := ctx.RequireString("Name")
	msg := ctx.RequireString("Data")

	var req hushcom.ChannelMsg
	req.Channel = name
	req.Text = msg

	b, err := hc.Send("Channel", true, name, hushcom.ClientID,
		hc.CurrentProfilePubKey, nil, req)

	result, err := ratnet.Api(b, db, true)
	jaserr(ctx, err)
	ctx.Data = result
}

// Remote - Rest Calls to interact with Hushcom Server
type Remote struct{}

// PutRegister -  Register loaded user name/key with Hushcom Server
func (*Remote) PutRegister(ctx *jas.Context) { // `PUT /v1/remote/register`
	if hc.CurrentProfileName == "" {
		ctx.Error = jas.NewRequestError("No Profile Loaded")
		return
	}

	log.Println("Creating Register Profile with: ", hc.CurrentProfileName, client.HUSHCOM, hc.CurrentProfilePubKey)

	b, err := hc.NewRegisterMsg()
	jaserr(ctx, err)
	result, err := ratnet.Api(b, db, true)
	jaserr(ctx, err)
	ctx.Data = result
}

// PutUnregister -  Unregister loaded user name/key with Hushcom Server
func (*Remote) PutUnregister(ctx *jas.Context) { // `PUT /v1/remote/unregister`
	b, err := hc.NewUnregisterMsg()
	jaserr(ctx, err)
	result, err := ratnet.Api(b, db, true)
	jaserr(ctx, err)
	ctx.Data = result
}

// GetChannel -  Get a list of public channels from Hushcom Server
func (*Remote) GetChannel(ctx *jas.Context) { // `GET /v1/remote/channel`
	if len(hc.CurrentProfileName) < 1 || len(hc.CurrentProfilePubKey) < 1 {
		jaserr(ctx, errors.New("No Profile Loaded"))
	}
	b, err := hc.NewListChansMsg()
	jaserr(ctx, err)
	if err == nil {
		result, err := ratnet.Api(b, db, true)
		jaserr(ctx, err)
		ctx.Data = result
	}
}

// PutChannel -  Register a new channel on Hushcom Server
func (*Remote) PutChannel(ctx *jas.Context) { // `PUT /v1/remote/channel`
	name := ctx.RequireString("Name") // Name of channel to create
	// Generate a new channel keypair
	chanCrypt := new(bencrypt.ECC)
	chanCrypt.GenerateKey()
	pubkey := chanCrypt.GetPubKey()
	pk, ok := pubkey.([]byte)
	if !ok {
		jaserr(ctx, errors.New("PutChannel failed on RSA key"))
	}
	// Save this channel to local DB. todo: remove this if NewChan fails
	var a ratnet.ApiCall
	a.Action = "AddChannel"
	a.Args = []string{name, chanCrypt.B64fromPrivateKey()}
	_, err := ratnet.Api(&a, db, true)
	jaserr(ctx, err)
	// Create and send new channel message with new keypair
	b, err := hc.NewNewChanMsg(name, pk)
	jaserr(ctx, err)
	result, err := ratnet.Api(b, db, true)
	jaserr(ctx, err)
	ctx.Data = result
}

// PostChannelJoin - Send a join request to channel
func (*Remote) PostChannelJoin(ctx *jas.Context) { // `POST /v1/remote/channel_join`
	/*
		body:  Name=abc&Password=pwd&Key=b64pubkey
	*/
	password := ""
	name := ctx.RequireString("Name")
	password, _ = ctx.FindString("Password")
	keyA := ctx.RequireString("Key") // we require pubkey here because it's not in our keyring yet

	key, _ := base64.StdEncoding.DecodeString(keyA)
	b, err := hc.NewJoinChanMsg(name, key, password)
	jaserr(ctx, err)
	result, err := ratnet.Api(b, db, true)
	jaserr(ctx, err)

	ctx.Data = result
}

//
// End of REST API
//
