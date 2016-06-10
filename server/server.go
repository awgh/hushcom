package server

import (
	"bencrypt"
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hushcom"
	"log"
	"ratnet"
	"ratnet/modules"
	"time"
)

// Remove item from list
func rmFrmList(list *[]string, item string) {
	var newList []string
	for _, listItem := range *list {
		if listItem != item {
			newList = append(newList, item)
		}
	}
	list = &newList
}

// Scan list for item
func chkList(list *[]string, item string) bool {
	for _, listItem := range *list {
		if listItem == item {
			return true
		}
	}
	return false
}

// HCSrvChan - Server data record
type HCSrvChan struct {
	Key      interface{}
	Password string
	Admins   []string
	Users    []string
}

// Server - Hushcom Server
type Server struct {
	// Globals
	HCSrvChans map[string]*HCSrvChan
	HCSrvUsers map[string]interface{}

	// Settings
	Database func() *sql.DB
}

// Singleton Instance of HushCom Server
var ServerInstance *Server

// Register this module with core
func init() {
	server := new(Server)
	ServerInstance = server
	server.HCSrvChans = make(map[string]*HCSrvChan)
	server.HCSrvUsers = make(map[string]interface{})
	modules.Dispatchers[hushcom.ServerID] = server
}

// GetName - Getter for readable name of the module
func (Server) GetName() string {
	return "HushCom Server Module"
}

// HandleDispatch - handler for messages that match the Dispatcher code
func (modInst Server) HandleDispatch(msg []byte) error {

	//	log.Println("HushCom Server HandleDispatch")

	// Create a decoder and receive a value.
	dec := gob.NewDecoder(bytes.NewBuffer(msg))
	var metaData hushcom.Msg
	err := dec.Decode(&metaData)
	if err != nil {
		log.Fatal("gob decode:", err)
	}
	var l func(...interface{})
	if metaData.MsgType == "ListChans" {
		l = func(params ...interface{}) {}
	} else {
		l = log.Println
	}
	l("Message Type: ", metaData.MsgType)

	userKey := modInst.HCSrvUsers[metaData.From]
	var newUser = true
	// is this a register message
	if metaData.MsgType == "Register" {
		log.Println("HushCom Server Register Message Received")
		if userKey != nil {
			newUser = false
			log.Println("User " + metaData.From + " is already registered")
		}
		// unmarshal msg into msgObj
		var msgObj hushcom.RegisterMsg
		if err := json.Unmarshal(metaData.Data, &msgObj); err != nil {
			return errors.New("Could not unmarshal 'Register' message")
		}
		userKey = msgObj.Key
	}
	// todo : sanity check key here via bencrypt
	if userKey == nil {
		// this is not a register message and it isn't signed, so ignore
		return nil //todo: return security error of some kind?
	}
	//	log.Println("user key: ", userKey)

	// Verify that the msg signature matches the user's key (or new key for Register)
	key, ok := userKey.([]byte) //todo - fix for RSA?
	if !ok {
		log.Fatal("userKey type assertion failed - your code is broken.")
	}
	if !hushcom.VerifyMsg(key, metaData) {
		return errors.New("Failure to authenticate user: " + metaData.From + " with signature " + hex.EncodeToString(metaData.Sig) + ".")
	}
	// At this point, the message is considered authenticated.

	l("... passed auth: ", metaData.MsgType)

	// Message Type Handlers
	switch metaData.MsgType {

	// Server-Handled Messages:
	// - Register: Register a new nick/pubkey pair
	// - Unregister: Remove a nick/pubkey pair
	// - ListChans: Enumerate public channels
	// - NewChan: Create a new channel

	case "Register":
		if newUser {
			// yay! New user!
			modInst.HCSrvUsers[metaData.From] = userKey
			uk, ok := userKey.([]byte) //todo: revisit this for RSA support
			if !ok {
				return errors.New("Register: Tried to use an RSA key?")
			}
			var a ratnet.ApiCall
			a.Action = "AddDest"
			a.Args = []string{metaData.From, base64.StdEncoding.EncodeToString(uk)}
			_, err := ratnet.Api(&a, modInst.Database, true)
			if err != nil {
				return err
			}
		}

		// send registration response
		var msg hushcom.Msg
		msg.From = modInst.GetName()
		msg.MsgType = "RegisterResp"
		msg.Timestamp = time.Now().UTC().UnixNano()
		var reg hushcom.RegisterRespMsg
		reg.Success = true
		jsonb, err := json.Marshal(reg)
		if err != nil {
			return err
		}
		msg.Data = jsonb
		return modInst.sendToClient(msg, metaData.From)

	case "UnRegister":
		// remove user from all chans
		for _, channel := range modInst.HCSrvChans {
			for _, user := range channel.Users {
				if user == metaData.From {
					rmFrmList(&channel.Admins, metaData.From)
					rmFrmList(&channel.Users, metaData.From)
				}
			}
		}
		// remove user's key from master key list
		delete(modInst.HCSrvUsers, metaData.From)

	case "ListChans":
		// get list of public chans
		var chans []hushcom.Channel
		for channel := range modInst.HCSrvChans {
			srvChan := modInst.HCSrvChans[channel]
			if srvChan.Password == "" {
				var c hushcom.Channel
				c.Name = channel
				c.PubKey = srvChan.Key
				chans = append(chans, c)
			}
		}
		var msg hushcom.Msg
		msg.From = modInst.GetName()
		msg.MsgType = "ListChansResp"
		msg.Timestamp = time.Now().UTC().UnixNano()

		var reg hushcom.ListChansRespMsg
		reg.Channels = chans
		jsonb, err := json.Marshal(reg)
		if err != nil {
			return err
		}
		msg.Data = jsonb
		return modInst.sendToClient(msg, metaData.From)

	case "NewChan":
		// unmarshal msg into msgObj
		var msgObj hushcom.NewChanMsg
		if err := json.Unmarshal(metaData.Data, &msgObj); err != nil {
			return errors.New("Could not unmarshal 'NewChan' message")
		}
		// does this channel exist?
		if modInst.HCSrvChans[msgObj.ChanName] != nil {
			return errors.New("Error creating channel")
		}
		// if channel doesn't exist, create channel
		modInst.HCSrvChans[msgObj.ChanName] = new(HCSrvChan)               // make chan object
		modInst.HCSrvChans[msgObj.ChanName].Key = msgObj.ChanPubKey        // add chan key
		modInst.HCSrvChans[msgObj.ChanName].Password = msgObj.ChanPassword // add password (optional)
		modInst.HCSrvChans[msgObj.ChanName].Admins = append(               // make user a chan admin
			modInst.HCSrvChans[msgObj.ChanName].Admins,
			metaData.From,
		)
		l("New Channel Registered with pubkey: ", msgObj)

	default:
		return errors.New("Unknown message type from user " + metaData.From + ".")
	}

	return nil
}

func (modInst Server) sendToClient(msg hushcom.Msg, destName string) error {

	var cid ratnet.ApiCall
	cid.Action = "CID"
	cpubsrv, err := ratnet.Api(&cid, modInst.Database, true)
	if err != nil {
		return err
	}
	crypt := new(bencrypt.ECC)
	cp, _ := crypt.B64toPublicKey(string(cpubsrv))
	uk, ok := cp.([]byte) //todo: revisit this for RSA support
	if !ok {
		return errors.New("Tried to use an RSA key?")
	}

	msg.Sig, err = hushcom.SignMsg(uk, msg)
	if err != nil {
		return err
	}
	// Create an encoder and send a value.
	var output bytes.Buffer
	enc := gob.NewEncoder(&output)
	err = enc.Encode(msg)
	if err != nil {
		return err
	}
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, hushcom.ClientID)
	buf = append(buf, output.Bytes()...)

	var b ratnet.ApiCall
	b.Action = "Send"
	b.Args = []string{destName, base64.StdEncoding.EncodeToString(buf)}
	_, err = ratnet.Api(&b, modInst.Database, true)
	if err != nil {
		return err
	}
	return nil
}
