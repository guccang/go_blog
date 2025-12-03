package fruitcrush

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
	"view"
)

// HandleFruitCrush renders the fruit crush game page
func HandleFruitCrush(w http.ResponseWriter, r *http.Request) {
	view.RenderTemplate(w, view.GetTemplatePath("fruitcrush.template"), nil)
}

// PlayerState represents the state of a player
type PlayerState struct {
	Board    [][]int `json:"board"` // 8x8 board with fruit types
	Score    int     `json:"score"`
	Moves    int     `json:"moves"`
	GameOver bool    `json:"gameOver"`
	Won      bool    `json:"won"`
	Ready    bool    `json:"ready"`
	Name     string  `json:"name"`
}

// Room represents a game room
type Room struct {
	GameID     string
	Players    map[int]*PlayerState
	GameActive bool
	Winner     int
	CreatedAt  int64
	Password   string
	Creator    string
	RoomName   string
	LastUpdate int64
}

type RoomManager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

var roomManager = &RoomManager{
	rooms: make(map[string]*Room),
}

// Requests and Responses
type CreateRoomRequest struct {
	Password string `json:"password"`
	Creator  string `json:"creator"`
	RoomName string `json:"roomName"`
}

type CreateRoomResponse struct {
	GameID string `json:"gameId"`
	Player int    `json:"player"`
}

type JoinRoomRequest struct {
	GameID     string `json:"gameId"`
	Player     int    `json:"player"`
	Password   string `json:"password"`
	PlayerName string `json:"playerName"`
}

type JoinRoomResponse struct {
	Success    bool   `json:"success"`
	Player     int    `json:"player"`
	Message    string `json:"message"`
	GameActive bool   `json:"gameActive"`
}

type UpdateStateRequest struct {
	GameID string      `json:"gameId"`
	Player int         `json:"player"`
	State  PlayerState `json:"state"`
}

type RoomStateResponse struct {
	Players    map[int]*PlayerState `json:"players"`
	GameActive bool                 `json:"gameActive"`
	Winner     int                  `json:"winner"`
}

type RoomInfo struct {
	GameID       string `json:"gameId"`
	RoomName     string `json:"roomName"`
	Creator      string `json:"creator"`
	HasPassword  bool   `json:"hasPassword"`
	Player1Ready bool   `json:"player1Ready"`
	Player2Ready bool   `json:"player2Ready"`
	CreatedAt    int64  `json:"createdAt"`
}

type RoomListResponse struct {
	Rooms []RoomInfo `json:"rooms"`
}

func generateGameID() string {
	return fmt.Sprintf("fruit_%d", time.Now().UnixNano())
}

func generateBoard() [][]int {
	board := make([][]int, 8)
	for i := range board {
		board[i] = make([]int, 8)
		for j := range board[i] {
			board[i][j] = rand.Intn(6) + 1 // 6 types of fruits (1-6)
		}
	}
	return board
}

func HandleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Creator == "" {
		req.Creator = "Player" + fmt.Sprintf("%d", time.Now().Unix()%1000)
	}

	room := &Room{
		GameID:     generateGameID(),
		Players:    make(map[int]*PlayerState),
		GameActive: false,
		Winner:     0,
		CreatedAt:  time.Now().Unix(),
		Password:   req.Password,
		Creator:    req.Creator,
		RoomName:   req.RoomName,
		LastUpdate: time.Now().Unix(),
	}

	room.Players[1] = &PlayerState{
		Name:  req.Creator,
		Ready: true,
		Board: generateBoard(),
	}

	roomManager.mu.Lock()
	roomManager.rooms[room.GameID] = room
	roomManager.mu.Unlock()

	resp := CreateRoomResponse{
		GameID: room.GameID,
		Player: 1,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func HandleJoinRoom(w http.ResponseWriter, r *http.Request) {
	var req JoinRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	roomManager.mu.Lock()
	defer roomManager.mu.Unlock()

	room, ok := roomManager.rooms[req.GameID]
	if !ok {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	if room.Password != "" && room.Password != req.Password {
		http.Error(w, "Incorrect password", http.StatusUnauthorized)
		return
	}

	resp := JoinRoomResponse{
		Success: false,
		Player:  0,
		Message: "",
	}

	if req.Player == 2 {
		if _, ok := room.Players[2]; ok {
			resp.Message = "Player 2 already joined"
			json.NewEncoder(w).Encode(resp)
			return
		}

		room.Players[2] = &PlayerState{
			Name:  req.PlayerName,
			Ready: true,
			Board: generateBoard(),
		}

		resp.Success = true
		resp.Player = 2

		if room.Players[1].Ready && room.Players[2].Ready {
			room.GameActive = true
		}
	} else {
		resp.Message = "Invalid player number"
	}

	resp.GameActive = room.GameActive

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func HandleRoomList(w http.ResponseWriter, r *http.Request) {
	roomManager.mu.RLock()
	defer roomManager.mu.RUnlock()

	rooms := []RoomInfo{}
	for gameID, room := range roomManager.rooms {
		if room.GameActive {
			continue
		}

		p2Ready := false
		if _, ok := room.Players[2]; ok {
			p2Ready = room.Players[2].Ready
		}

		roomInfo := RoomInfo{
			GameID:       gameID,
			RoomName:     room.RoomName,
			Creator:      room.Creator,
			HasPassword:  room.Password != "",
			Player1Ready: room.Players[1].Ready,
			Player2Ready: p2Ready,
			CreatedAt:    room.CreatedAt,
		}
		rooms = append(rooms, roomInfo)
	}

	resp := RoomListResponse{
		Rooms: rooms,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func HandleUpdateState(w http.ResponseWriter, r *http.Request) {
	var req UpdateStateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	roomManager.mu.Lock()
	defer roomManager.mu.Unlock()

	room, ok := roomManager.rooms[req.GameID]
	if !ok {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	if p, ok := room.Players[req.Player]; ok {
		p.Board = req.State.Board
		p.Score = req.State.Score
		p.Moves = req.State.Moves
		p.GameOver = req.State.GameOver
		p.Won = req.State.Won

		if p.Won {
			room.GameActive = false
			room.Winner = req.Player
		}
	}

	room.LastUpdate = time.Now().Unix()

	resp := RoomStateResponse{
		Players:    room.Players,
		GameActive: room.GameActive,
		Winner:     room.Winner,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func HandleRoomState(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Query().Get("gameId")

	roomManager.mu.RLock()
	defer roomManager.mu.RUnlock()

	room, ok := roomManager.rooms[gameID]
	if !ok {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	resp := RoomStateResponse{
		Players:    room.Players,
		GameActive: room.GameActive,
		Winner:     room.Winner,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
