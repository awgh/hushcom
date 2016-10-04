package hushcom

import (
	"bytes"
	"encoding/binary"

	"github.com/awgh/bencrypt/bc"
)

// Msg - Core message struct for HC messages
type Msg struct {
	From      string // nick of sender, signed by sender
	Timestamp int64  // timestamp set and signed by sender
	MsgType   string // verb, signed by sender
	Data      []byte // inner data, typically JSON
	Sig       []byte // signature of (From + Timestamp + MsgType + msg)
}

// SignMe - convert a message to a byte array for signing purposes only
func (inst Msg) SignMe() []byte {

	output := []byte(inst.From)
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(inst.Timestamp))
	output = append(output, b...)
	output = append(output, []byte(inst.MsgType)...)
	output = append(output, inst.Data...)
	return output
}

// Messages

// RegisterMsg - Register a new user/pubkey pair
type RegisterMsg struct {
	Key string // b64 pubkey
}

// RegisterRespMsg - Registration response message
type RegisterRespMsg struct {
	Success bool
}

// NewChanMsg - Create a new channel
type NewChanMsg struct {
	ChanName     string
	ChanPubKey   string // b64 pubkey
	ChanPassword string
}

// JoinChanMsg - Join channel request
type JoinChanMsg struct {
	Channel   string
	ReqPubKey string // b64 pubkey
	Password  string
}

// JoinChanRespMsg - Join channel response
type JoinChanRespMsg struct {
	Channel    string
	ChannelKey string // this should be base64 encoded
}

// ChannelMsg - Message in a channel
type ChannelMsg struct {
	Channel string
	Text    string
}

// Channel - Common Representation of a Channel
type Channel struct {
	Name   string
	PubKey string // this should be base64 encoded
}

// ListChansRespMsg - List channels response
type ListChansRespMsg struct {
	Channels []Channel
}

// SignMsg - Sign a message (key is not base64 here)
func SignMsg(key bc.PubKey, msg Msg) ([]byte, error) {
	//log.Println("SignMsg, key:", key)
	return bc.DestHash(key, msg.SignMe())
}

// VerifyMsg - Verify a message signature
func VerifyMsg(key bc.PubKey, msg Msg) bool {
	sig, err := SignMsg(key, msg)
	if err != nil {
		return false
	}
	return (bytes.Compare(sig, msg.Sig) == 0)
}
