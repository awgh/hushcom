package client

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

var (
	// HUSHCOM - Destination name for the Hushcom Server
	HUSHCOM = "HushComServer"
	// HUSHCOMPKA - ASCII base64 version of Server PubKey
	HUSHCOMPKA = "BN0Z1v4ejW8fqq7uUtVrl9sZBKoz15BUknZ2ZSEPUT4="
	// HUSHCOMPK - binary version of Server PubKey
	HUSHCOMPK []byte
	// Output - Buffered AJAX Output
	Output string
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
	userKeys map[string][]byte // map of username to binary public key (not b64)
	Database func() *sql.DB

	CurrentProfileName   string
	CurrentProfilePubKey []byte
}

// Register this module with core
func init() {
	var client *Client
	client = new(Client)
	client.userKeys = make(map[string][]byte)
	HUSHCOMPK, _ = base64.StdEncoding.DecodeString(HUSHCOMPKA)
	client.userKeys[HUSHCOM] = HUSHCOMPK
	modules.Dispatchers[hushcom.ClientID] = client

	Output = ""
}

// GetName - Getter for readable name of the module
func (Client) GetName() string {
	return "HushCom Client Module"
}

// HandleDispatch - handler for messages that match the Dispatcher code
func (modInst Client) HandleDispatch(msg []byte) error {

	// Create a decoder and receive a value.
	dec := gob.NewDecoder(bytes.NewBuffer(msg))
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
		modInst.userKeys[metaData.From] = msgObj.ReqPubKey
		var a ratnet.ApiCall
		a.Action = "GetChannelPrivKey"
		a.Args = []string{msgObj.Channel}
		result, err := ratnet.Api(&a, modInst.Database, true)
		if err != nil {
			log.Println("PRIVATE CHANNEL KEY RESULT: ", result)
			x, _ := base64.StdEncoding.DecodeString(string(result))
			log.Println("PRIVATE CHANNEL KEY DECODE: ", x)
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

		b, err := modInst.Send("JoinChanResp", false, metaData.From, hushcom.ClientID,
			modInst.CurrentProfilePubKey, msgObj.ReqPubKey, resp)
		_, err = ratnet.Api(b, modInst.Database, true, "JoinChanResp: ")
		if err != nil {
			return err
		}
		return nil // end of JoinChan case

	case "JoinChanResp":
		l("JOIN CHANNEL RESPONSE RECEIVED")
		var msgObj hushcom.JoinChanRespMsg
		if err := json.Unmarshal(metaData.Data, &msgObj); err != nil {
			return errors.New("Could not unmarshal 'JoinChanResp' message")
		}
		var a ratnet.ApiCall
		a.Action = "AddChannel"
		a.Args = []string{msgObj.Channel, msgObj.ChannelKey}
		_, err := ratnet.Api(&a, modInst.Database, true)
		if err != nil {
			return err
		}

		crypt := new(bencrypt.ECC)
		err = crypt.B64toPrivateKey(msgObj.ChannelKey)
		if err != nil {
			return err
		}
		pk, ok := crypt.GetPubKey().([]byte)
		if !ok {
			return errors.New("Invalid Channel Key in JoinChanResp handler")
		}
		var resp hushcom.ChannelMsg
		resp.Channel = msgObj.Channel
		resp.Text = metaData.From + " has admitted " + modInst.CurrentProfileName + " to channel."

		b, err := modInst.Send("Channel", true, msgObj.Channel, hushcom.ClientID,
			modInst.CurrentProfilePubKey, pk, resp)
		_, err = ratnet.Api(b, modInst.Database, true)
		if err != nil {
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
		Output += string(outb) + "\n"
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
		Output += string(outb) + "\n"

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
		Output += string(outb) + "\n"
	}

	return nil
}

// Server-Handled Messages:
// - Register: Register a new nick/pubkey pair
// - Unregister: Remove a nick/pubkey pair
// - ListChans: Enumerate public channels
// - NewChan: Create a new channel

// NewRegisterMsg - Create a "register a user" message for the Hushcom server
func (modInst Client) NewRegisterMsg() (*ratnet.ApiCall, error) {
	var reg hushcom.RegisterMsg
	reg.Key = modInst.CurrentProfilePubKey
	return modInst.Send("Register", false, HUSHCOM, hushcom.ServerID, modInst.CurrentProfilePubKey, HUSHCOMPK, reg)
}

// NewUnregisterMsg - Create an "Unregister a user" message for the Hushcom server
func (modInst Client) NewUnregisterMsg() (*ratnet.ApiCall, error) {
	return modInst.Send("Unregister", false, HUSHCOM, hushcom.ServerID, modInst.CurrentProfilePubKey, HUSHCOMPK, nil)
}

// NewListChansMsg - Create a "List Public Channels" message for the Hushcom server
func (modInst Client) NewListChansMsg() (*ratnet.ApiCall, error) {
	return modInst.Send("ListChans", false, HUSHCOM, hushcom.ServerID, modInst.CurrentProfilePubKey,
		HUSHCOMPK, nil)
}

// NewNewChanMsg - Create a "register a new channel" message for the Hushcom server
func (modInst Client) NewNewChanMsg(chanName string, chanPubKey []byte) (*ratnet.ApiCall, error) {
	var reg hushcom.NewChanMsg
	reg.ChanName = chanName
	reg.ChanPubKey = chanPubKey

	log.Println("NewNewChanMsg: ", chanName, chanPubKey)

	return modInst.Send("NewChan", false, HUSHCOM, hushcom.ServerID, modInst.CurrentProfilePubKey, HUSHCOMPK, reg)
}

// Client-Handled Messages:
// - NewJoinChanMsg: Create a join channel request
// - NewJoinChanRespMsg: Create a join channel response

// NewJoinChanMsg - Create a join channel request
func (modInst Client) NewJoinChanMsg(channelName string, channelPubKey []byte,
	password string) (*ratnet.ApiCall, error) {

	var reg hushcom.JoinChanMsg
	reg.Channel = channelName
	reg.ReqPubKey = modInst.CurrentProfilePubKey
	reg.Password = password

	log.Println("JoinChan:", reg)
	log.Println("   DestKey:", channelPubKey)

	return modInst.Send("JoinChan", true, channelName, hushcom.ClientID,
		modInst.CurrentProfilePubKey, channelPubKey, reg)
}

// NewJoinChanRespMsg - Create a join channel response
func (modInst Client) NewJoinChanRespMsg(channelName string, channelPrivKeyB64 string,
	userName string, destKey []byte) (*ratnet.ApiCall, error) {

	var reg hushcom.JoinChanRespMsg
	reg.ChannelKey = channelPrivKeyB64
	return modInst.Send("JoinChanResp", false, userName, hushcom.ClientID,
		modInst.CurrentProfilePubKey, destKey, reg)
}

// Send - Send message via this client instance
func (modInst Client) Send(
	msgType string, channel bool, to string, handler uint16,
	signKey []byte, destKey []byte, hcmsg interface{}) (*ratnet.ApiCall, error) {

	var msg hushcom.Msg
	var err error
	msg.From = modInst.CurrentProfileName
	msg.MsgType = msgType
	msg.Timestamp = time.Now().UTC().UnixNano()

	if hcmsg != nil {
		jsonb, err := json.Marshal(hcmsg)
		msg.Data = jsonb
		if err != nil {
			return nil, err
		}
	}
	msg.Sig, err = hushcom.SignMsg(signKey, msg)
	if err != nil {
		return nil, err
	}
	// Create an encoder and send a value.
	var output bytes.Buffer
	enc := gob.NewEncoder(&output)
	err = enc.Encode(msg)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, handler)
	buf = append(buf, output.Bytes()...)

	var b ratnet.ApiCall
	if channel {
		b.Action = "SendChannel"
	} else {
		b.Action = "Send"
	}
	if destKey == nil {
		b.Args = []string{to, base64.StdEncoding.EncodeToString(buf)}
	} else {
		b.Args = []string{to, base64.StdEncoding.EncodeToString(buf), base64.StdEncoding.EncodeToString(destKey)}
	}
	return &b, nil
}
