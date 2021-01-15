package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/awgh/bencrypt/bc"
	"github.com/awgh/bencrypt/ecc"
	"github.com/awgh/hushcom/server"
	"github.com/awgh/ratnet/api"
	"github.com/awgh/ratnet/nodes/qldb"
	"github.com/awgh/ratnet/policy"
	"github.com/awgh/ratnet/transports/tls"
)

// usage: ./hushcomd -dbfile=ratnet2.ql -p=20003 -ap=21003

func serve(transportPublic api.Transport, node api.Node, listenPublic string) {

	node.SetPolicy(
		policy.NewServer(transportPublic, listenPublic, false))

	log.Println("Public Hushcom Server starting: ", listenPublic)
	if err := node.Start(); err != nil {
		log.Fatal(err)
	}
}

var db func() *sql.DB

func main() {

	var dbFile string
	var publicPort int

	flag.StringVar(&dbFile, "dbfile", "ratnet.ql", "QL Database File")
	flag.IntVar(&publicPort, "p", 20001, "HTTPS Public Port (*)")
	flag.Parse()
	publicString := fmt.Sprintf(":%d", publicPort)

	node := qldb.New(new(ecc.KeyPair), new(ecc.KeyPair))
	db = node.BootstrapDB(dbFile)

	serverInst := server.New(node)
	go func() {
		for {
			msg := <-node.Out()
			if err := serverInst.HandleMsg(msg); err != nil {
				log.Println("hushcomd ratnet bg thread: " + err.Error())
				log.Println(msg)
			}
		}
	}()

	// print public content key
	pubsrv, err := node.CID()
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Println("Public Content Key: ", pubsrv.ToB64())

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
	serve(tls.New(cert, key, node, true), node, publicString)

	for {
		time.Sleep(time.Second * 3600)
	}
}
