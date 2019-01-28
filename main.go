package main;

import(
	"github.com/gorilla/websocket"
	"encoding/json"
	"fmt"
	"net/http"
	"github.com/satori/go.uuid"
)

type ClientManager struct {
    clients    map[*Client]bool //connected clients
    broadcast  chan []byte      //messages that are going to be broadcasted to and from connected clients
    register   chan *Client     //clients trying to become registered
    unregister chan *Client     //clients that 
}

type Client struct {
    id     string               //id
    socket *websocket.Conn      //websocket connection
    send   chan []byte          //message to be sent
}

type Message struct {
    Sender    string `json:"sender,omitempty"`
    Recipient string `json:"recipient,omitempty"`
    Content   string `json:"content,omitempty"`
}


var manager = ClientManager{
    clients:    make(map[*Client]bool), 
    broadcast:  make(chan []byte),   
    register:   make(chan *Client),        
    unregister: make(chan *Client),       
}

func (manager *ClientManager) start() {
    for {
        select {
        case conn := <-manager.register:
            manager.clients[conn] = true
            jsonMessage, _ := json.Marshal(&Message{Content: "/A new socket has connected."})
            manager.send(jsonMessage, conn)
        case conn := <-manager.unregister:
            if _, ok := manager.clients[conn]; ok {
                close(conn.send)
                delete(manager.clients, conn)
                jsonMessage, _ := json.Marshal(&Message{Content: "/A socket has disconnected."})
                manager.send(jsonMessage, conn)
            }
        case message := <-manager.broadcast:
            for conn := range manager.clients {
                select {
                case conn.send <- message:
                default:
                    close(conn.send)
                    delete(manager.clients, conn)
                }
            }
        }
    }
}

func (manager *ClientManager) send(message []byte, ignore *Client) {
    for conn := range manager.clients {
        if conn != ignore {
            conn.send <- message
        }
    }
}

func (c *Client) read() {
    defer func() {
        manager.unregister <- c
        c.socket.Close()
    }()

    for {
        _, message, err := c.socket.ReadMessage()
        if err != nil {
            manager.unregister <- c
            c.socket.Close()
            break
        }
        jsonMessage, _ := json.Marshal(&Message{Sender: c.id, Content: string(message)})
        manager.broadcast <- jsonMessage
    }
}

func (c *Client) write() {
    defer func() {
        c.socket.Close()
    }()

    for {
        select {
        case message, ok := <-c.send:
            if !ok {
                c.socket.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }

            c.socket.WriteMessage(websocket.TextMessage, message)
        }
    }
}

func main() {
    port := ":6789"
    fmt.Println("Starting application on port " + port + "....")
    go manager.start()
    http.HandleFunc("/ws", wsPage)
    http.ListenAndServe(port, nil)
}

func wsPage(res http.ResponseWriter, req *http.Request) {
    conn, error := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(res, req, nil)
    if error != nil {
        http.NotFound(res, req)
        return
    }
	
	u, err := uuid.NewV4()

	if err != nil {
		fmt.Println(err)
	}

	

	client := &Client{id: u.String(), socket: conn, send: make(chan []byte)}

    manager.register <- client

    go client.read()
    go client.write()
}
