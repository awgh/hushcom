package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/awgh/bencrypt/ecc"
	"github.com/awgh/hushcom/server"
	"github.com/awgh/ratnet/api"
	"github.com/awgh/ratnet/nodes/qldb"
	"github.com/awgh/ratnet/policy"
	"github.com/awgh/ratnet/transports/https"
)

// usage: ./hushcomd -dbfile=ratnet2.ql -p=20003 -ap=21003

var serverInst *server.Server

func serve(transportPublic api.Transport, node api.Node, listenPublic string) {

	node.SetPolicy(
		policy.NewServer(transportPublic, listenPublic, false))

	log.Println("Public Hushcom Server starting: ", listenPublic)
	node.Start()
}

func main() {

	var dbFile string
	var publicPort int

	flag.StringVar(&dbFile, "dbfile", "ratnet.ql", "QL Database File")
	flag.IntVar(&publicPort, "p", 20001, "HTTPS Public Port (*)")
	flag.Parse()
	publicString := fmt.Sprintf(":%d", publicPort)

	node := qldb.New(new(ecc.KeyPair), new(ecc.KeyPair))
	node.BootstrapDB(dbFile)

	serverInst := server.New(node)
	go func() {
		for {
			msg := <-node.Out()
			if err := serverInst.HandleMsg(msg); err != nil {
				log.Println("hushcomd ratnet bg thread: " + err.Error())
			}
		}
	}()

	// print public content key
	pubsrv, err := node.CID()
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Println("Public Content Key: ", string(pubsrv.ToB64()))

	serve(https.New("cert.pem", "key.pem", node, true), node, publicString)

	for {
		time.Sleep(time.Second * 3600)
	}
}
