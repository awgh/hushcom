package main

import (
	"errors"
	"log"
	"strconv"

	"github.com/awgh/bencrypt/ecc"
	"github.com/awgh/hushcom"
	"github.com/awgh/hushcom/client"

	"github.com/coocood/jas"
)

//  REST API DEFINITION
//

func jaserr(ctx *jas.Context, err error) {
	if err != nil {
		ctx.Error = jas.NewRequestError(err.Error())
		ctx.Data = nil
	}
}

// Poll - Hacky AJAX endpoint, todo: replace me with web sockets
type Poll struct {
	hc *client.Client
}

func newPoll(hc *client.Client) *Poll {
	p := new(Poll)
	p.hc = hc
	return p
}

// Get poll updates
func (p *Poll) Get(ctx *jas.Context) { // `GET /v1/poll`
	ctx.Data = p.hc.Output
	p.hc.Output = ""
	//log.Println("poll result: ", ctx.Data)
}

// Profile - Rest Calls for Profiles
type Profile struct {
	hc *client.Client
}

func newProfile(hc *client.Client) *Profile {
	p := new(Profile)
	p.hc = hc
	return p
}

// Get profile list
func (p *Profile) Get(ctx *jas.Context) { // `GET /v1/profile`
	result, err := p.hc.Node.GetProfiles()
	ctx.Data = result
	jaserr(ctx, err)
	log.Printf("getprofile result: %+v  err: %+v\n", result, err)
}

// PutLoad - Load a profile
func (p *Profile) PutLoad(ctx *jas.Context) { // `PUT /v1/profile/load`
	/*
		body:  Name=abc
	*/
	name := ctx.RequireString("Name")

	key, err := p.hc.Node.LoadProfile(name)
	p.hc.CurrentProfileName = name
	p.hc.CurrentProfilePubKey = key
	ctx.Data = "OK"
	jaserr(ctx, err)
}

// Put - Add a new profile or update existing profile
func (p *Profile) Put(ctx *jas.Context) { // `PUT /v1/profile`
	/*
		body:  Name=abc&Enabled=true
	*/
	name := ctx.RequireString("Name")
	enabled, err := strconv.ParseBool(ctx.RequireString("Enabled"))
	jaserr(ctx, err)
	if err == nil {
		err = p.hc.Node.AddProfile(name, enabled)
		ctx.Data = "OK"
		jaserr(ctx, err)
	}
}

// PostDelete - Delete a profile
func (p *Profile) PostDelete(ctx *jas.Context) { // `POST /v1/profile/delete`
	/*
		body:  Name=abc
	*/
	name := ctx.RequireString("Name")
	err := p.hc.Node.DeleteProfile(name)
	ctx.Data = "OK"
	jaserr(ctx, err)
}

// Server - Rest Calls for Servers
type Server struct {
	hc *client.Client
}

func newServer(hc *client.Client) *Server {
	p := new(Server)
	p.hc = hc
	return p
}

// Get server list
func (s *Server) Get(ctx *jas.Context) { // `GET /v1/server`
	result, err := s.hc.Node.GetPeers()
	ctx.Data = result
	jaserr(ctx, err)
}

// Put - Add a new server or update existing server
func (s *Server) Put(ctx *jas.Context) { // `PUT /v1/server`
	name := ctx.RequireString("Name")
	uri := ctx.RequireString("URI")
	enabled, err := strconv.ParseBool(ctx.RequireString("Enabled"))
	jaserr(ctx, err)
	if err == nil {
		err := s.hc.Node.AddPeer(name, enabled, uri)
		ctx.Data = "OK"
		jaserr(ctx, err)
	}
}

// PostDelete - Delete a server
func (s *Server) PostDelete(ctx *jas.Context) { // `POST /v1/server/delete`
	name := ctx.RequireString("Name")
	err := s.hc.Node.DeletePeer(name)
	ctx.Data = "OK"
	jaserr(ctx, err)
}

// Channel - Rest Calls for Channels
type Channel struct {
	hc *client.Client
}

func newChannel(hc *client.Client) *Channel {
	p := new(Channel)
	p.hc = hc
	return p
}

// Get channel list
func (c *Channel) Get(ctx *jas.Context) { // `GET /v1/channel`
	result, err := c.hc.Node.GetChannels()
	ctx.Data = result
	jaserr(ctx, err)
}

// PostDelete - Delete a channel
func (c *Channel) PostDelete(ctx *jas.Context) { // `POST /v1/channel/delete`
	/*
		body:  Name=abc
	*/
	name := ctx.RequireString("Name")
	err := c.hc.Node.DeleteChannel(name)
	ctx.Data = "OK"
	jaserr(ctx, err)
}

// Post - Send message to a channel
func (c *Channel) Post(ctx *jas.Context) { // `POST /v1/channel`
	/*
		body:  Name=abc&Data=message_data
	*/
	name := ctx.RequireString("Name")
	msg := ctx.RequireString("Data")

	var req hushcom.ChannelMsg
	req.Channel = name
	req.Text = msg

	err := c.hc.HCSend("Channel", true, name,
		c.hc.CurrentProfilePubKey, nil, req)
	ctx.Data = "OK"
	jaserr(ctx, err)
}

// Remote - Rest Calls to interact with Hushcom Server
type Remote struct {
	hc *client.Client
}

func newRemote(hc *client.Client) *Remote {
	p := new(Remote)
	p.hc = hc
	return p
}

// PutRegister -  Register loaded user name/key with Hushcom Server
func (r *Remote) PutRegister(ctx *jas.Context) { // `PUT /v1/remote/register`
	if r.hc.CurrentProfileName == "" {
		ctx.Error = jas.NewRequestError("No Profile Loaded")
		return
	}
	log.Println("Creating Register Profile with: ", r.hc.CurrentProfileName, client.HUSHCOM, r.hc.CurrentProfilePubKey)
	err := r.hc.NewRegisterMsg()
	ctx.Data = "OK"
	jaserr(ctx, err)
}

// PutUnregister -  Unregister loaded user name/key with Hushcom Server
func (r *Remote) PutUnregister(ctx *jas.Context) { // `PUT /v1/remote/unregister`
	err := r.hc.NewUnregisterMsg()
	ctx.Data = "OK"
	jaserr(ctx, err)
}

// GetChannel -  Get a list of public channels from Hushcom Server
func (r *Remote) GetChannel(ctx *jas.Context) { // `GET /v1/remote/channel`
	if r.hc.CurrentProfileName == "" {
		err := errors.New("No Profile Loaded")
		jaserr(ctx, err)
		return
	}
	err := r.hc.NewListChansMsg()
	ctx.Data = "OK"
	jaserr(ctx, err)
}

// PutChannel -  Register a new channel on Hushcom Server
func (r *Remote) PutChannel(ctx *jas.Context) { // `PUT /v1/remote/channel`
	name := ctx.RequireString("Name") // Name of channel to create
	// Generate a new channel keypair
	chanCrypt := new(ecc.KeyPair)
	chanCrypt.GenerateKey()
	pubkey := chanCrypt.GetPubKey().ToB64()
	// Save this channel to local DB. todo: remove this if NewChan fails
	err := r.hc.Node.AddChannel(name, chanCrypt.ToB64())
	jaserr(ctx, err)
	if err == nil {
		// Create and send new channel message with new keypair

		log.Println("NewNewChanMsg with pubkey: ", pubkey)

		err := r.hc.NewNewChanMsg(name, pubkey)
		ctx.Data = "OK"
		jaserr(ctx, err)
	}
}

// PostChannelJoin - Send a join request to channel
func (r *Remote) PostChannelJoin(ctx *jas.Context) { // `POST /v1/remote/channel_join`
	/*
		body:  Name=abc&Password=pwd&Key=b64pubkey
	*/
	var password string
	name := ctx.RequireString("Name")
	password, _ = ctx.FindString("Password")
	keyA := ctx.RequireString("Key") // we require pubkey here because it's not in our keyring yet

	key := new(ecc.PubKey)
	err := key.FromB64(keyA)
	jaserr(ctx, err)
	if err == nil {
		err = r.hc.NewJoinChanMsg(name, key, password)
		ctx.Data = "OK"
		jaserr(ctx, err)
	}
}

//
// End of REST API
//
