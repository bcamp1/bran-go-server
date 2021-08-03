package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/gin-gonic/gin"

	socketio "github.com/googollee/go-socket.io"
)

const serveDir = "../../../js/bran-go/build/"
const notFoundPath = serveDir + "notFound.html"

var server = socketio.NewServer(nil)
var manager = NewManager()

func main() {
	startServer()
}

func startSocket() {
	server.OnEvent("/", "join", func(s socketio.Conn, data map[string]string) error {
		fmt.Println("Join")
		s.SetContext(data)

		name := data["name"]
		roomName := data["room"]

		client := NewClient(s.ID(), name)
		manager.Add(roomName)
		room, err := manager.Get(roomName)
		if err != nil {
			panic(err)
		}

		err = room.AddClient(&client)
		if err != nil {
			fmt.Println(err)
			return err
		}

		s.Join(roomName)
		s.Join(s.ID())

		// Emit player type
		client.Send(server, "type", client.clientType.String())

		fmt.Println(room.state)
		if room.PlayerCount() == 2 && room.state == WAITING {
			go room.StartColorSelection(server)
		}

		fmt.Println(manager)
		return nil
	})

	server.OnEvent("/", "colorChoice", func(s socketio.Conn, data string) error {
		s.SetContext(data)

		room, client, err := manager.FromConn(s)

		if err != nil {
			fmt.Println(err)
			return err
		}

		if client.clientType == PLAYER {
			room.BroadcastExcept(server, client.id, "enemyColor", data)
			client.color = toColor(data)
		}
		return nil
	})

	server.OnEvent("/", "stoneRequest", func(s socketio.Conn, data int) error {
		s.SetContext(data)

		room, client, err := manager.FromConn(s)

		if err != nil {
			fmt.Println(err)
			return err
		}

		fmt.Println("Stone request:", data)

		err = room.processStoneRequest(server, client, data)
		if err != nil {
			fmt.Println(err)
		}

		return nil
	})

	server.OnEvent("/", "boardRequest", func(s socketio.Conn, data string) error {
		s.SetContext(data)

		room, _, err := manager.FromConn(s)

		if err != nil {
			fmt.Println(err)
			return err
		}

		room.Broadcast(server, "board", room.game.Packet())

		return nil
	})

	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		log.Println("disconnected:", s.ID())
		room, _, err := manager.FromConn(s)

		if err != nil {
			fmt.Println(err)
			return
		}

		err = manager.RemoveClient(s.ID())
		if err != nil {
			fmt.Println(err)
		}

		if room.state == PLAYING && room.PlayerCount() < 2 {
			room.state = WAITING
			room.Broadcast(server, "colorCountDown", 10)
		}

	})
}

func startServer() {

	r := gin.New()
	r.GET("/", home)
	r.GET("/build/:file", build)
	r.GET("/current-rooms", currentRooms)
	r.GET("/room/:room", room)

	// Socket IO stuff
	server.OnConnect("/", func(s socketio.Conn) error {

		s.SetContext("")
		log.Println("connected:", s.ID())
		return nil
	})

	startSocket()

	go func() {
		if err := server.Serve(); err != nil {
			log.Fatalf("socketio listen error: %s\n", err)
		}
	}()
	defer server.Close()

	r.GET("/socket.io/*any", gin.WrapH(server))
	r.POST("/socket.io/*any", gin.WrapH(server))

	if err := r.Run(":80"); err != nil {
		log.Fatal("failed run app: ", err)
	}
}

func build(c *gin.Context) {
	file := c.Param("file")
	path := serveDir + file
	parts := strings.Split(file, ".")

	ext := parts[len(parts)-1]

	if ext != "html" {
		c.File(path)
	}
}

func home(c *gin.Context) {
	c.File(serveDir + "home.html")
}

func room(c *gin.Context) {
	c.File(serveDir + "room.html")
}

func currentRooms(c *gin.Context) {
	c.String(200, `{"room1": 4}`)
}
