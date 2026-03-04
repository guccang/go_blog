package minesweeper

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
	"view"
)

// HandleMinesweeper renders the minesweeper game page
func HandleMinesweeper(w http.ResponseWriter, r *http.Request) {
	view.RenderTemplate(w, view.GetTemplatePath("minesweeper.template"), nil)
}

// CellState represents the state of a single cell
type CellState struct {
	IsMine        bool `json:"isMine"`
	IsRevealed    bool `json:"isRevealed"`
	IsFlagged     bool `json:"isFlagged"`
	NeighborMines int  `json:"neighborMines"`
}

// PlayerState represents the state of a player
type PlayerState struct {
	Board    [][]CellState `json:"board"`
	Score    int           `json:"score"` // Percentage cleared or time taken
	GameOver bool          `json:"gameOver"`
	Won      bool          `json:"won"`
	Ready    bool          `json:"ready"`
	Name     string        `json:"name"`
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
	// Shared configuration for fairness
	MineLocations [][2]int `json:"mineLocations"`
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
	Success       bool     `json:"success"`
	Player        int      `json:"player"`
	Message       string   `json:"message"`
	GameActive    bool     `json:"gameActive"`
	MineLocations [][2]int `json:"mineLocations"` // Send mines to P2 upon join
}

type UpdateStateRequest struct {
	GameID string      `json:"gameId"`
	Player int         `json:"player"`
	State  PlayerState `json:"state"`
}

type RoomStateResponse struct {
	Players       map[int]*PlayerState `json:"players"`
	GameActive    bool                 `json:"gameActive"`
	Winner        int                  `json:"winner"`
	MineLocations [][2]int             `json:"mineLocations"`
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
	return fmt.Sprintf("mines_%d", time.Now().UnixNano())
}

func generateMines(rows, cols, count int) [][2]int {
	mines := make([][2]int, 0, count)
	seen := make(map[string]bool)

	for len(mines) < count {
		r := rand.Intn(rows)
		c := rand.Intn(cols)
		key := fmt.Sprintf("%d,%d", r, c)
		if !seen[key] {
			seen[key] = true
			mines = append(mines, [2]int{r, c})
		}
	}
	return mines
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

	// Generate standard board: 10x10, 15 mines
	mines := generateMines(10, 10, 15)

	room := &Room{
		GameID:        generateGameID(),
		Players:       make(map[int]*PlayerState),
		GameActive:    false,
		Winner:        0,
		CreatedAt:     time.Now().Unix(),
		Password:      req.Password,
		Creator:       req.Creator,
		RoomName:      req.RoomName,
		LastUpdate:    time.Now().Unix(),
		MineLocations: mines,
	}

	room.Players[1] = &PlayerState{
		Name:  req.Creator,
		Ready: true,
		Board: make([][]CellState, 10),
	}
	for i := range room.Players[1].Board {
		room.Players[1].Board[i] = make([]CellState, 10)
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
			Board: make([][]CellState, 10),
		}
		for i := range room.Players[2].Board {
			room.Players[2].Board[i] = make([]CellState, 10)
		}

		resp.Success = true
		resp.Player = 2
		resp.MineLocations = room.MineLocations

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
		p.GameOver = req.State.GameOver
		p.Won = req.State.Won

		if p.Won {
			room.GameActive = false
			room.Winner = req.Player
		} else if p.GameOver {
			// If one player explodes, does the other win immediately?
			// Or just that player loses?
			// Let's say if you explode, you lose. If both explode?
			// For now: if you explode, you are out. If opponent is still playing, they can win.
			// If both explode, draw?
			// Simple logic: First to Win sets Winner.
		}
	}

	room.LastUpdate = time.Now().Unix()

	resp := RoomStateResponse{
		Players:       room.Players,
		GameActive:    room.GameActive,
		Winner:        room.Winner,
		MineLocations: room.MineLocations,
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
		Players:       room.Players,
		GameActive:    room.GameActive,
		Winner:        room.Winner,
		MineLocations: room.MineLocations,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
