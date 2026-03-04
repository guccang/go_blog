package tetris

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
	"view"
)

// HandleTetris renders the tetris game page
func HandleTetris(w http.ResponseWriter, r *http.Request) {
	view.RenderTemplate(w, view.GetTemplatePath("tetris.template"), nil)
}

// PieceState represents the state of a falling piece
type PieceState struct {
	Matrix [][]int `json:"matrix"`
	X      int     `json:"x"`
	Y      int     `json:"y"`
}

// PlayerState represents the state of a player in the game
type PlayerState struct {
	Board        [][]int    `json:"board"` // 20x10 board
	CurrentPiece PieceState `json:"currentPiece"`
	Score        int        `json:"score"`
	GameOver     bool       `json:"gameOver"`
	Ready        bool       `json:"ready"`
	Name         string     `json:"name"`
	Level        int        `json:"level"`
	Lines        int        `json:"lines"`
}

// Room represents a tetris game room for two players
type Room struct {
	GameID     string
	Players    map[int]*PlayerState // 1: player1, 2: player2
	GameActive bool
	Winner     int // 0: no winner yet, 1: player1 wins, 2: player2 wins
	CreatedAt  int64
	Password   string
	Creator    string
	RoomName   string
	LastUpdate int64
}

// RoomManager manages all tetris rooms
type RoomManager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

var roomManager = &RoomManager{
	rooms: make(map[string]*Room),
}

// CreateRoomRequest represents a request to create a new room
type CreateRoomRequest struct {
	Password string `json:"password"`
	Creator  string `json:"creator"`
	RoomName string `json:"roomName"`
}

// CreateRoomResponse represents the response after creating a room
type CreateRoomResponse struct {
	GameID string `json:"gameId"`
	Player int    `json:"player"`
}

// JoinRoomRequest represents a request to join a room
type JoinRoomRequest struct {
	GameID     string `json:"gameId"`
	Player     int    `json:"player"`
	Password   string `json:"password"`
	PlayerName string `json:"playerName"`
}

// JoinRoomResponse represents the response after joining a room
type JoinRoomResponse struct {
	Success    bool   `json:"success"`
	Player     int    `json:"player"`
	Message    string `json:"message"`
	GameActive bool   `json:"gameActive"`
}

// UpdateStateRequest represents a request to update player state
type UpdateStateRequest struct {
	GameID string      `json:"gameId"`
	Player int         `json:"player"`
	State  PlayerState `json:"state"`
}

// RoomStateResponse represents the room state response
type RoomStateResponse struct {
	Players    map[int]*PlayerState `json:"players"`
	GameActive bool                 `json:"gameActive"`
	Winner     int                  `json:"winner"`
}

// RoomInfo represents information about a room for listing
type RoomInfo struct {
	GameID       string `json:"gameId"`
	RoomName     string `json:"roomName"`
	Creator      string `json:"creator"`
	HasPassword  bool   `json:"hasPassword"`
	Player1Ready bool   `json:"player1Ready"`
	Player2Ready bool   `json:"player2Ready"`
	CreatedAt    int64  `json:"createdAt"`
}

// RoomListResponse represents the response for room list
type RoomListResponse struct {
	Rooms []RoomInfo `json:"rooms"`
}

func generateGameID() string {
	return fmt.Sprintf("tetris_%d", time.Now().UnixNano())
}

// HandleCreateRoom creates a new tetris room
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
		Board: make([][]int, 20), // Init empty board
	}
	for i := range room.Players[1].Board {
		room.Players[1].Board[i] = make([]int, 10)
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

// HandleJoinRoom handles a player joining a room
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
			Board: make([][]int, 20),
		}
		for i := range room.Players[2].Board {
			room.Players[2].Board[i] = make([]int, 10)
		}

		resp.Success = true
		resp.Player = 2

		if room.Players[1].Ready && room.Players[2].Ready {
			room.GameActive = true
		}
	} else {
		resp.Message = "Invalid player number or logic"
	}

	resp.GameActive = room.GameActive

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleRoomList returns a list of available rooms
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

// HandleUpdateState updates the player's state and returns the room state
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

	if !room.GameActive {
		// allow updates even if game not active? maybe for ready state?
		// for now, just return
	}

	// Update player state
	if p, ok := room.Players[req.Player]; ok {
		p.Board = req.State.Board
		p.CurrentPiece = req.State.CurrentPiece
		p.Score = req.State.Score
		p.GameOver = req.State.GameOver
		p.Lines = req.State.Lines
		p.Level = req.State.Level

		if p.GameOver {
			// If one player loses, the other wins
			room.GameActive = false
			room.Winner = 3 - req.Player
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

// HandleRoomState gets the current room state
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
