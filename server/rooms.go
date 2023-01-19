package server

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Participant struct which contains the connection details of the participant
type Participant struct {
	Host bool
	Conn *websocket.Conn
}

// Room Struct contains details of room and its particpants
type RoomMap struct {
	Mutex sync.RWMutex
	Map   map[string][]Participant
}

// Initializes the map which contains rooms
func (r *RoomMap) Init() {
	r.Map = make(map[string][]Participant)
}

// Gets the details of the participants in a specific room
func (r *RoomMap) Get(room_id string) []Participant {
	r.Mutex.RLock()
	defer r.Mutex.RUnlock()

	return r.Map[room_id]
}

// Creates a new room
func (r *RoomMap) CreateRoom() string {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	r.Init()
	rand.Seed(time.Now().UnixNano())
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456789")
	bArr := make([]rune, 8)
	for i := range bArr {
		bArr[i] = letters[rand.Intn(len(letters))]
	}
	room_id := string(bArr)
	r.Map[room_id] = []Participant{}
	return room_id
}

// Inserts a new participant in the room
func (r *RoomMap) InsertIntoRoom(room_id string, host bool, conn *websocket.Conn) {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	b := Participant{host, conn}

	log.Println("Inserting into the Room, RoomID : ", room_id)
	r.Map[room_id] = append(r.Map[room_id], b)
}

// Deletes a room
func (r *RoomMap) DeleteRoom(room_id string) {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	delete(r.Map, room_id)
}
