package main

import (
	"flag"
	"fmt"
	"github.com/awgh/hushcom/server"
	"log"
	"github.com/awgh/ratnet"
	"github.com/awgh/ratnet/transports"
	"time"
)

// usage: ./hushcomd -dbfile=ratnet2.ql -p=20003 -ap=21003

var serverInst *server.Server

func serve(database string, listenPublic string,
	certfile string, keyfile string) {

	transports.NewServer("https", listenPublic, certfile, keyfile, database, false)
	log.Println("Public Server started: ", listenPublic)
}

func main() {

	var dbFile string
	var publicPort int

	flag.StringVar(&dbFile, "dbfile", "ratnet.ql", "QL Database File")
	flag.IntVar(&publicPort, "p", 20001, "HTTPS Public Port (*)")

	flag.Parse()
	publicString := fmt.Sprintf(":%d", publicPort)

	serverInst := server.ServerInstance
	db := ratnet.BootstrapDB(dbFile)
	serverInst.Database = db

	serve(dbFile, publicString, "cert.pem", "key.pem")

	// print public content key
	var id ratnet.ApiCall
	id.Action = "CID"
	pubsrv, err := ratnet.Api(&id, db, true)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Println("Public Content Key: ", string(pubsrv))

	for {
		time.Sleep(time.Second * 3600)
	}
}
