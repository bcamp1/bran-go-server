package main

import (
	"fmt"
	"math/rand"
	"time"

	socketio "github.com/googollee/go-socket.io"
	k "github.com/rooklift/kikashi"
)

type Color int

const (
	NONE  Color = 0
	BLACK Color = 1
	WHITE Color = 2
)

func toColor(str string) Color {
	switch str {
	case "none":
		return NONE
	case "black":
		return BLACK
	case "white":
		return WHITE
	}
	panic("toColor: invalid color str")
}

func (c Color) String() string {
	switch c {
	case NONE:
		return "none"
	case BLACK:
		return "black"
	case WHITE:
		return "white"
	}
	panic("Invalid client type")
}

func (c Color) ToKikashi() k.Colour {
	switch c {
	case NONE:
		return k.EMPTY
	case BLACK:
		return k.BLACK
	case WHITE:
		return k.WHITE
	}
	panic("Invalid color")
}

func (c Color) Opposite() Color {
	switch c {
	case NONE:
		return NONE
	case BLACK:
		return WHITE
	case WHITE:
		return BLACK
	}
	panic("Invalid color")
}

type ClientType int

const (
	PLAYER    ClientType = 1
	SPECTATOR ClientType = 0
)

func (t ClientType) String() string {
	switch t {
	case PLAYER:
		return "player"
	case SPECTATOR:
		return "spectator"
	}
	panic("Invalid client type")
}

type Client struct {
	id         string
	name       string
	color      Color
	clientType ClientType
}

func (c Client) String() string {
	return fmt.Sprintf("[%v] %v %v", c.name, c.clientType, c.color)
}

func NewClient(id, name string) Client {
	color := NONE
	clientType := SPECTATOR

	return Client{id, name, color, clientType}
}

func (c Client) Send(server *socketio.Server, event string, args ...interface{}) {
	server.BroadcastToRoom("/", c.id, event, args[0])
}

type RoomState int

const (
	WAITING RoomState = iota
	SELECTING_COLORS
	PLAYING
)

func (r RoomState) String() string {
	switch r {
	case WAITING:
		return "waiting"
	case SELECTING_COLORS:
		return "selecting_colors"
	case PLAYING:
		return "playing"
	}
	panic("Invalid roomstate")
}

type Room struct {
	name    string
	state   RoomState
	clients []*Client
	game    Game
}

func NewRoom(name string) Room {
	state := WAITING
	clients := make([]*Client, 0)
	game := NewGame(9)
	return Room{name, state, clients, game}
}

func (r Room) findClient(id string) int {
	for i := range r.clients {
		if id == r.clients[i].id {
			return i
		}
	}
	return -1
}

func (r Room) findByName(name string) int {
	for i := range r.clients {
		if name == r.clients[i].name {
			return i
		}
	}
	return -1
}

func (r Room) ClientCount() int {
	return len(r.clients)
}

func (r Room) PlayerCount() int {
	return len(r.Players())
}

func (r Room) Players() []*Client {
	players := make([]*Client, 0)
	for _, client := range r.clients {
		if client.clientType == PLAYER {
			players = append(players, client)
		}
	}
	return players
}

func (r *Room) AddClient(client *Client) error {
	// Check if client already exists
	if r.findClient(client.id) != -1 {
		return fmt.Errorf("AddClient: Client already exists")
	}

	if r.findByName(client.name) != -1 {
		return fmt.Errorf("AddClient: Client name already exists")
	}

	if r.PlayerCount() < 2 {
		client.clientType = PLAYER
	}

	r.clients = append(r.clients, client)
	return nil
}

func (r *Room) RemoveClient(id string) error {
	index := r.findClient(id)

	if index == -1 {
		return fmt.Errorf("RemoveClient: client not found")
	}

	r.clients[index] = r.clients[len(r.clients)-1]
	r.clients = r.clients[:len(r.clients)-1]
	return nil
}

func (r Room) Client(id string) (*Client, bool) {
	if i := r.findClient(id); i == -1 {
		return nil, false
	} else {
		return r.clients[i], true
	}
}

func (r Room) String() string {
	str := fmt.Sprintf("Room: %v\n  State: %v\n", r.name, r.state)
	for i := range r.clients {
		str += fmt.Sprintf("    %v\n", *r.clients[i])
	}

	str += "\n"
	return str
}

func (r Room) BroadcastExcept(server *socketio.Server, id string, event string, args ...interface{}) {
	if r.findClient(id) == -1 {
		panic("BroadcastExcept: id doesn't exist")
	}

	for _, client := range r.clients {
		if client.id != id {
			server.BroadcastToRoom("/", client.id, event, args[0])
		}
	}
}

func (r Room) Broadcast(server *socketio.Server, event string, args ...interface{}) {
	server.BroadcastToRoom("/", r.name, event, args[0])
}

func (r *Room) StartColorSelection(server *socketio.Server) error {
	if r.state != WAITING {
		panic("StartColorSelection: Room state is not in WAITING")
	}

	r.state = SELECTING_COLORS

	const waitSeconds = 5

	for i := waitSeconds; i > 0; i-- {
		r.Broadcast(server, "colorCountDown", fmt.Sprintf("%v", i))
		time.Sleep(time.Second)
	}

	players := r.Players()
	if len(players) != 2 {
		return fmt.Errorf("StartColorSelection: Not enough/too many players: %v", len(players))
	}

	p1 := players[0]
	p2 := players[1]

	c1 := p1.color
	c2 := p2.color

	if c1 == NONE {
		c1 = p2.color.Opposite()
	}

	if c2 == NONE {
		c2 = p1.color.Opposite()
	}

	if c2 == c1 {
		// Pick from random
		randFloat := rand.Float64()
		if randFloat < 0.5 {
			c2 = BLACK
		} else {
			c2 = WHITE
		}

		c1 = c2.Opposite()
	}

	p1.Send(server, "color", c1.String())
	p2.Send(server, "color", c2.String())

	p1.color = c1
	p2.color = c2

	r.Broadcast(server, "colorCountDown", 0)
	r.state = PLAYING

	time.Sleep(time.Second)
	r.Broadcast(server, "board", r.game.Packet())

	return nil
}

func (r *Room) processStoneRequest(server *socketio.Server, client *Client, index int) error {
	if r.findClient(client.id) == -1 {
		return fmt.Errorf("processStoneRequest: Client doesn't belong to room")
	}

	if r.game.turnColor != client.color {
		return fmt.Errorf("processStoneRequest: Player is incorrect color. Wanted %v got %v", r.game.turnColor, client.color)
	}

	x, y := r.game.IndexToCoords(index)

	err := r.game.TryMove(x, y)

	if err != nil {
		return err
	}

	// Move has been made. Broadcast to room.
	r.Broadcast(server, "board", r.game.Packet())

	return nil
}

type Manager struct {
	rooms []*Room
}

func NewManager() Manager {
	return Manager{rooms: make([]*Room, 0)}
}

func (m Manager) String() string {
	str := "--------------------\n"
	for i := range m.rooms {
		str += fmt.Sprintf("%v", *m.rooms[i])
	}
	return str + "--------------------\n"
}

func (m Manager) find(name string) int {
	for i := range m.rooms {
		if m.rooms[i].name == name {
			return i
		}
	}
	return -1
}

func (m Manager) Get(name string) (*Room, error) {
	index := m.find(name)
	if index == -1 {
		return nil, fmt.Errorf("Manager.get: Room doesn't exist")
	}

	return m.rooms[index], nil
}

func (m *Manager) Add(name string) error {
	index := m.find(name)
	if index != -1 {
		return fmt.Errorf("Manager.Add: Room already exists")
	}

	room := NewRoom(name)
	m.rooms = append(m.rooms, &room)
	return nil
}

func (m *Manager) Remove(name string) error {
	index := m.find(name)
	if index == -1 {
		return fmt.Errorf("Manager.Remove: Room doesn't exist")
	}

	m.rooms[index] = m.rooms[len(m.rooms)-1]
	m.rooms = m.rooms[:len(m.rooms)-1]
	return nil
}

func (m *Manager) RemoveClient(id string) error {
	for _, room := range m.rooms {
		if room.findClient(id) != -1 {
			room.RemoveClient(id)
			if len(room.clients) == 0 {
				m.Remove(room.name)
			}
			return nil
		}
	}
	return fmt.Errorf("RemoveClient: Client not found")
}

func (m Manager) Client(id string) (*Client, bool) {
	for _, room := range m.rooms {
		if client, has := room.Client(id); has {
			return client, true
		}
	}
	return nil, false
}

func (m Manager) ForceClient(id string) *Client {
	client, has := m.Client(id)
	if !has {
		panic("ForceClient: Client doesn't exist")
	}
	return client
}

func (m Manager) ClientRoom(id string) (*Room, bool) {
	for _, room := range m.rooms {
		if _, has := room.Client(id); has {
			return room, true
		}
	}
	return nil, false
}

func (m Manager) ForceClientRoom(id string) *Room {
	if room, has := m.ClientRoom(id); has {
		return room
	}
	panic("ForceClientRoom: Room not found")
}

func (m Manager) FromConn(s socketio.Conn) (*Room, *Client, error) {
	id := s.ID()
	room, has := m.ClientRoom(id)

	if !has {
		return nil, nil, fmt.Errorf("FromConn: No matching room")
	}

	client, has := room.Client(id)

	if !has {
		return nil, nil, fmt.Errorf("FromConn: No matching Client")
	}

	return room, client, nil
}
