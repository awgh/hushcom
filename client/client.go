package client

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/awgh/bencrypt/bc"
	"github.com/awgh/bencrypt/ecc"
	"github.com/awgh/hushcom"
	"github.com/awgh/ratnet/api"
)

var (
	// HUSHCOM - Destination name for the Hushcom Server
	HUSHCOM = "HushComServer"
	// HUSHCOMPKA - ASCII base64 version of Server PubKey
	HUSHCOMPKA = "Dr66J/nYJ622ElEXXqOBtonVOdlsFJwcehdqRx75bW4="
	// HUSHCOMPK - binary version of Server PubKey
	HUSHCOMPK bc.PubKey
)

// JSONResp - Response Structure to AJAX
type JSONResp struct {
	From    string
	MsgType string
	Channel string
	Data    interface{}
}

// Client - Hushcom Client
type Client struct {
	// Globals
	userKeys map[string]bc.PubKey // map of username to binary public key (not b64)
	Node     api.Node

	CurrentProfileName   string
	CurrentProfilePubKey bc.PubKey

	// Output - Buffered AJAX Output
	Output string
}

// New : Make a new instance of Hushcom Client
func New(node api.Node) *Client {

	client := new(Client)
	client.CurrentProfileName = ""
	client.CurrentProfilePubKey = nil
	client.Node = node

	client.userKeys = make(map[string]bc.PubKey)

	hcpk := new(ecc.PubKey)
	hcpk.FromB64(HUSHCOMPKA)
	HUSHCOMPK = hcpk
	client.userKeys[HUSHCOM] = hcpk

	client.Output = ""
	return client
}

// GetName - Getter for readable name of the module
func (*Client) GetName() string {
	return "HushCom Client Module"
}

// HandleMsg - handler for messages
func (modInst *Client) HandleMsg(msg api.Msg) error {

	// Create a decoder and receive a value.
	dec := gob.NewDecoder(msg.Content)
	var metaData hushcom.Msg
	err := dec.Decode(&metaData)
	if err != nil {
		log.Println("HushCom Client HandleDispatch: Invalid Data")
		return err
	}

	var l func(...interface{})
	if metaData.MsgType == "ListChansResp" {
		l = func(params ...interface{}) {}
	} else {
		l = log.Println
	}

	l("HushCom Client HandleDispatch", metaData.MsgType)

	// Non-Authenticated (not signature-checked) Message Handlers
	switch metaData.MsgType {
	// From Peers
	// - JoinChan: Channel join request
	// - JoinChanResp: Channel join response
	case "JoinChan":
		var msgObj hushcom.JoinChanMsg
		if err := json.Unmarshal(metaData.Data, &msgObj); err != nil {
			return errors.New("Could not unmarshal 'JoinChan' message")
		}
		//todo: check password here, currently only auto-accepts
		k := new(ecc.PubKey)
		if err := k.FromB64(msgObj.ReqPubKey); err != nil {
			return err
		}
		modInst.userKeys[metaData.From] = k
		result, err := modInst.Node.GetChannelPrivKey(msgObj.Channel)
		if err != nil {
			return err
		}
		var resp hushcom.JoinChanRespMsg
		resp.Channel = string(msgObj.Channel)
		resp.ChannelKey = string(result)
		l("Sending JoinChanResp with:")
		l("   From:\t", metaData.From)
		//l("   Signing Key:", modInst.CurrentProfilePubKey)
		l("   DestKey:\t", msgObj.ReqPubKey)

		bx, ex := base64.StdEncoding.DecodeString(string(result))
		l("   ChanKey:\t", bx, ex)

		if err := modInst.HCSend("JoinChanResp", false, metaData.From,
			modInst.CurrentProfilePubKey, k, resp); err != nil {
			return err
		}
		return nil // end of JoinChan case

	case "JoinChanResp":
		l("JOIN CHANNEL RESPONSE RECEIVED")
		var msgObj hushcom.JoinChanRespMsg
		if err := json.Unmarshal(metaData.Data, &msgObj); err != nil {
			return errors.New("Could not unmarshal 'JoinChanResp' message")
		}
		if err := modInst.Node.AddChannel(msgObj.Channel, msgObj.ChannelKey); err != nil {
			return err
		}
		crypt := new(ecc.KeyPair)
		crypt.FromB64(msgObj.ChannelKey)
		pk := crypt.GetPubKey()

		var resp hushcom.ChannelMsg
		resp.Channel = msgObj.Channel
		resp.Text = metaData.From + " has admitted " + modInst.CurrentProfileName + " to channel."

		if err := modInst.HCSend("Channel", true, msgObj.Channel,
			modInst.CurrentProfilePubKey, pk, resp); err != nil {
			return err
		}
		return nil

	case "Channel":
		var msgObj hushcom.ChannelMsg
		if err := json.Unmarshal(metaData.Data, &msgObj); err != nil {
			return errors.New("Could not unmarshal 'Channel' message")
		}
		// todo: check signature and pass an "authenticated" flag to UI
		var resp JSONResp
		resp.MsgType = metaData.MsgType
		resp.From = metaData.From
		resp.Channel = msgObj.Channel
		resp.Data = msgObj.Text
		outb, err := json.Marshal(resp)
		if err != nil {
			log.Println("JSON Marshal failed in Channel")
			return err
		}
		modInst.Output += string(outb) + "\n"
		//log.Println("Handled Unauthenticated Message: ", metaData.MsgType)
		return nil
	}

	// Verify that the msg signature matches the user's key
	if !hushcom.VerifyMsg(HUSHCOMPK, metaData) {
		return errors.New("Failure to authenticate user: " + metaData.From + " with signature " + hex.EncodeToString(metaData.Sig) + ".")
	}
	// At this point, the message is considered authenticated.
	l("Message Passed Auth: ", metaData.MsgType)

	// Authenticated (signature-checked) Message Handlers
	switch metaData.MsgType {
	// Client-Handled Messages:
	// From Server
	// - RegisterResp: Register a new nick/pubkey pair
	// - ListChans: Enumerate public channels
	case "RegisterResp":
		var msgObj hushcom.RegisterRespMsg
		if err := json.Unmarshal(metaData.Data, &msgObj); err != nil {
			return errors.New("Could not unmarshal 'RegisterResp' message")
		}
		var resp JSONResp
		resp.MsgType = metaData.MsgType
		resp.From = metaData.From
		resp.Data = msgObj
		outb, err := json.Marshal(resp)
		if err != nil {
			log.Println("JSON Marshal failed in RegisterResp")
			return err
		}
		modInst.Output += string(outb) + "\n"

	case "ListChansResp":
		var msgObj hushcom.ListChansRespMsg
		if err := json.Unmarshal(metaData.Data, &msgObj); err != nil {
			return errors.New("Could not unmarshal 'ListChansResp' message")
		}
		var resp JSONResp
		resp.MsgType = metaData.MsgType
		resp.From = metaData.From
		resp.Data = msgObj
		outb, err := json.Marshal(resp)
		if err != nil {
			log.Println("JSON Marshal failed in ListChansResp")
			return err
		}
		modInst.Output += string(outb) + "\n"
	}

	return nil
}

// Server-Handled Messages:
// - Register: Register a new nick/pubkey pair
// - Unregister: Remove a nick/pubkey pair
// - ListChans: Enumerate public channels
// - NewChan: Create a new channel

// NewRegisterMsg - Create a "register a user" message for the Hushcom server
func (modInst *Client) NewRegisterMsg() error {
	var reg hushcom.RegisterMsg
	reg.Key = modInst.CurrentProfilePubKey.ToB64()
	return modInst.HCSend("Register", false, HUSHCOM, modInst.CurrentProfilePubKey, HUSHCOMPK, reg)
}

// NewUnregisterMsg - Create an "Unregister a user" message for the Hushcom server
func (modInst *Client) NewUnregisterMsg() error {
	return modInst.HCSend("Unregister", false, HUSHCOM, modInst.CurrentProfilePubKey, HUSHCOMPK, nil)
}

// NewListChansMsg - Create a "List Public Channels" message for the Hushcom server
func (modInst *Client) NewListChansMsg() error {
	return modInst.HCSend("ListChans", false, HUSHCOM, modInst.CurrentProfilePubKey,
		HUSHCOMPK, nil)
}

// NewNewChanMsg - Create a "register a new channel" message for the Hushcom server
func (modInst *Client) NewNewChanMsg(chanName string, chanPubKey string) error {
	var reg hushcom.NewChanMsg
	reg.ChanName = chanName
	reg.ChanPubKey = chanPubKey
	return modInst.HCSend("NewChan", false, HUSHCOM, modInst.CurrentProfilePubKey, HUSHCOMPK, reg)
}

// Client-Handled Messages:
// - NewJoinChanMsg: Create a join channel request
// - NewJoinChanRespMsg: Create a join channel response

// NewJoinChanMsg - Create a join channel request
func (modInst *Client) NewJoinChanMsg(channelName string, channelPubKey bc.PubKey,
	password string) error {

	var reg hushcom.JoinChanMsg
	reg.Channel = channelName
	reg.ReqPubKey = modInst.CurrentProfilePubKey.ToB64()
	reg.Password = password

	return modInst.HCSend("JoinChan", true, channelName,
		modInst.CurrentProfilePubKey, channelPubKey, reg)
}

// NewJoinChanRespMsg - Create a join channel response
func (modInst *Client) NewJoinChanRespMsg(channelName string, channelPrivKeyB64 string,
	userName string, destKey bc.PubKey) error {

	var reg hushcom.JoinChanRespMsg
	reg.ChannelKey = channelPrivKeyB64
	return modInst.HCSend("JoinChanResp", false, userName,
		modInst.CurrentProfilePubKey, destKey, reg)
}

// HCSend - Send message via this client instance
func (modInst *Client) HCSend(
	msgType string, channel bool, to string,
	signKey bc.PubKey, destKey bc.PubKey, hcmsg interface{}) error {

	var err error
	var msg hushcom.Msg
	msg.From = modInst.CurrentProfileName
	msg.MsgType = msgType
	msg.Timestamp = time.Now().UTC().UnixNano()

	if hcmsg != nil {
		jsonb, err := json.Marshal(hcmsg)
		if err != nil {
			return err
		}
		msg.Data = jsonb
	}
	msg.Sig, err = hushcom.SignMsg(signKey, msg)
	if err != nil {
		return err
	}

	// Create a GOB encoder and encode the message
	var output bytes.Buffer
	enc := gob.NewEncoder(&output)
	err = enc.Encode(msg)
	if err != nil {
		return err
	}
	if channel {
		if destKey == nil {
			modInst.Node.SendChannel(to, output.Bytes())
		} else {
			modInst.Node.SendChannel(to, output.Bytes(), destKey)
		}
	} else if destKey == nil {
		modInst.Node.Send(to, output.Bytes())
	} else {
		modInst.Node.Send(to, output.Bytes(), destKey)
	}
	return err
}
