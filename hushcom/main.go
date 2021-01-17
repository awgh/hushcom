package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/awgh/bencrypt/bc"
	"github.com/awgh/bencrypt/ecc"
	"github.com/awgh/hushcom/client"
	"github.com/awgh/ratnet/api"
	"github.com/awgh/ratnet/nodes/qldb"
	"github.com/awgh/ratnet/policy/poll"
	"github.com/awgh/ratnet/transports/tls"

	"github.com/coocood/jas"
)

// usage: ./hushcom -dbfile=ratnet2.ql -p=20003

func handleCORS(r *http.Request, responseHeader http.Header) bool {
	responseHeader.Add("Access-Control-Allow-Origin", "*")
	responseHeader.Add("Access-Control-Allow-Methods", "GET,POST,PUT")
	return r.Method != "OPTIONS"
}

func serve(transportAdmin api.Transport, node api.Node, listenRest, certfile, keyfile string) {
	node.FlushOutbox(0)
	node.SetPolicy(
		poll.New(transportAdmin, node, 500, 0))
	if err := node.Start(); err != nil {
		log.Fatal(err.Error())
	}
	pubsrv, err := node.ID()
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Println("Public Routing Key: " + pubsrv.ToB64())

	if err := node.AddContact(client.HUSHCOM, client.HUSHCOMPKA); err != nil {
		log.Fatal(err.Error())
	}

	var peer api.Peer
	debug := true // todo: make this a CLI param
	if debug {
		peer = api.Peer{Name: "localhost", URI: "127.0.0.1:20001", Enabled: true}
	} else if runtime.GOOS == "android" {
		peer = api.Peer{Name: "awgh.wtf.im", URI: "104.233.91.188:20001", Enabled: true}
	} else {
		peer = api.Peer{Name: "awgh.wtf.im", URI: "awgh.wtf.im:20001", Enabled: true}
	}
	if err := node.AddPeer(peer.Name, peer.Enabled, peer.URI); err != nil {
		log.Fatal(err.Error())
	}

	hc := client.New(node)
	go func() {
		for {
			msg := <-node.Out()
			if err := hc.HandleMsg(msg); err != nil {
				log.Println("hushcom ratnet bg thread: " + err.Error())
			}
		}
	}()

	// start REST api second, since it does not trigger cert generation
	router := jas.NewRouter(newProfile(hc), newServer(hc), newChannel(hc), newRemote(hc), newPoll(hc))
	router.BasePath = "/v1/"
	router.HandleCORS = handleCORS

	// Serve the static javascript files
	pwd, _ := os.Getwd()
	log.Println("CWD: " + pwd)
	log.Println(listenRest)

	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir("js"))
	mux.Handle("/js/", http.StripPrefix("/js/", fs))

	mux.Handle(router.BasePath, router)
	fmt.Println(router.HandledPaths(true))

	// wg.Add(1)
	log.Fatal(http.ListenAndServeTLS(listenRest, certfile, keyfile, mux))
	// wg.Done()
}

func main() {
	var dbFile string
	var restPort int

	flag.StringVar(&dbFile, "dbfile", "ratnet.ql", "QL Database File")
	flag.IntVar(&restPort, "p", 20011, "HTTPS REST Port (localhost)")

	flag.Parse()
	restString := fmt.Sprintf("localhost:%d", restPort)

	node := qldb.New(new(ecc.KeyPair), new(ecc.KeyPair))
	node.BootstrapDB(dbFile)

	certfile := "cert.pem"
	keyfile := "key.pem"
	bc.InitSSL(certfile, keyfile, true)

	cert, err := ioutil.ReadFile(certfile)
	if err != nil {
		log.Fatal(err)
	}
	key, err := ioutil.ReadFile(keyfile)
	if err != nil {
		log.Fatal(err)
	}

	serve(tls.New(cert, key, node, true), node, restString, certfile, keyfile)
}
