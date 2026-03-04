package gomoku

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Room represents a gomoku game room for two players
type Room struct {
	GameID        string
	Board         [][]int // 0: empty, 1: black, 2: white
	Player1Ready  bool
	Player2Ready  bool
	CurrentPlayer int // 1: black (player1), 2: white (player2)
	GameActive    bool
	Winner        int // 0: no winner yet, 1: black wins, 2: white wins
	CreatedAt     int64
	Password      string // Empty string means no password
	Creator       string // Creator's name or identifier
	RoomName      string // Optional room name
	Player1Name   string // Player 1 display name
	Player2Name   string // Player 2 display name
	LastMoveTime  int64  // Timestamp of last move
}

// RoomManager manages all gomoku rooms
type RoomManager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

// Global room manager
var roomManager = &RoomManager{
	rooms: make(map[string]*Room),
}

// CreateRoomRequest represents a request to create a new room
type CreateRoomRequest struct {
	Password string `json:"password"` // Optional password
	Creator  string `json:"creator"`  // Creator's name
	RoomName string `json:"roomName"` // Optional room name
}

// CreateRoomResponse represents the response after creating a room
type CreateRoomResponse struct {
	GameID    string `json:"gameId"`
	Player    int    `json:"player"` // player number (always 1 for creator)
}

// JoinRoomRequest represents a request to join a room
type JoinRoomRequest struct {
	GameID     string `json:"gameId"`
	Player     int    `json:"player"` // 1 or 2
	Password   string `json:"password"` // Required if room has password
	PlayerName string `json:"playerName"` // Player's display name
}

// JoinRoomResponse represents the response after joining a room
type JoinRoomResponse struct {
	Success    bool     `json:"success"`
	Player     int      `json:"player"` // assigned player number
	Message    string   `json:"message"`
	Board      [][]int  `json:"board"` // current board state
	GameActive bool     `json:"gameActive"`
}

// RoomStateRequest represents a request for room state
type RoomStateRequest struct {
	GameID string `json:"gameId"`
	Player int    `json:"player"` // requesting player (1 or 2)
}

// RoomStateResponse represents the room state response
type RoomStateResponse struct {
	Board         [][]int `json:"board"`
	CurrentPlayer int     `json:"currentPlayer"`
	GameActive    bool    `json:"gameActive"`
	Winner        int     `json:"winner"`
	Player1Name   string  `json:"player1Name"`
	Player2Name   string  `json:"player2Name"`
	YourTurn      bool    `json:"yourTurn"` // whether it's the requesting player's turn
}

// MakeMoveRequest represents a request to make a move
type MakeMoveRequest struct {
	GameID string `json:"gameId"`
	Player int    `json:"player"` // 1 or 2
	X      int    `json:"x"`
	Y      int    `json:"y"`
}

// MakeMoveResponse represents the response after making a move
type MakeMoveResponse struct {
	Success       bool     `json:"success"`
	GameActive    bool     `json:"gameActive"`
	Winner        int      `json:"winner"` // 0: no winner yet, 1: black wins, 2: white wins
	Message       string   `json:"message"`
	Board         [][]int  `json:"board"`
	CurrentPlayer int      `json:"currentPlayer"`
}

// RoomInfo represents information about a room for listing
type RoomInfo struct {
	GameID       string `json:"gameId"`
	RoomName     string `json:"roomName"`    // Optional room name
	Creator      string `json:"creator"`     // Creator's name
	HasPassword  bool   `json:"hasPassword"` // Whether room has password
	Player1Ready bool   `json:"player1Ready"` // Is player 1 ready?
	Player2Ready bool   `json:"player2Ready"` // Is player 2 ready?
	CreatedAt    int64  `json:"createdAt"`   // When the room was created
	Player1Name  string `json:"player1Name"` // Player 1 display name
	Player2Name  string `json:"player2Name"` // Player 2 display name
}

// RoomListResponse represents the response for room list
type RoomListResponse struct {
	Rooms []RoomInfo `json:"rooms"`
}

// Helper function to generate a unique game ID
func generateGameID() string {
	return fmt.Sprintf("gomoku_%d", time.Now().UnixNano())
}

// Helper function to create initial board
func createInitialBoard() [][]int {
	board := make([][]int, 15)
	for i := range board {
		board[i] = make([]int, 15)
	}
	return board
}

// HandleCreateRoom creates a new gomoku room
func HandleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Use default creator name if not provided
	if req.Creator == "" {
		req.Creator = "玩家" + fmt.Sprintf("%d", time.Now().Unix()%1000)
	}

	// Create initial board
	board := createInitialBoard()

	// Create room
	room := &Room{
		GameID:        generateGameID(),
		Board:         board,
		Player1Ready:  true, // Creator is ready
		Player2Ready:  false,
		CurrentPlayer: 1, // Black (player1) starts
		GameActive:    false, // Will be active when both players ready
		Winner:        0,
		CreatedAt:     time.Now().Unix(),
		Password:      req.Password,
		Creator:       req.Creator,
		RoomName:      req.RoomName,
		Player1Name:   req.Creator,
		Player2Name:   "", // Will be set when player 2 joins
		LastMoveTime:  time.Now().Unix(),
	}

	// Store room
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

	// Check password if required
	if room.Password != "" && room.Password != req.Password {
		http.Error(w, "Incorrect password", http.StatusUnauthorized)
		return
	}

	resp := JoinRoomResponse{
		Success:    false,
		Player:     0,
		Message:    "",
		Board:      nil,
		GameActive: false,
	}

	// Check if player 1 is joining
	if req.Player == 1 {
		if room.Player1Ready {
			resp.Message = "Player 1 already joined"
			json.NewEncoder(w).Encode(resp)
			return
		}
		room.Player1Ready = true
		if req.PlayerName != "" {
			room.Player1Name = req.PlayerName
		}
		resp.Success = true
		resp.Player = 1
	} else if req.Player == 2 {
		if room.Player2Ready {
			resp.Message = "Player 2 already joined"
			json.NewEncoder(w).Encode(resp)
			return
		}
		room.Player2Ready = true
		if req.PlayerName != "" {
			room.Player2Name = req.PlayerName
		}
		resp.Success = true
		resp.Player = 2

		// If both players are ready, start the game
		if room.Player1Ready && room.Player2Ready {
			room.GameActive = true
			// Set player 2 name if not set
			if room.Player2Name == "" {
				room.Player2Name = "玩家" + fmt.Sprintf("%d", time.Now().Unix()%1000)
			}
		}
	} else {
		resp.Message = "Invalid player number"
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp.Board = room.Board
	resp.GameActive = room.GameActive

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleRoomState gets the current room state
func HandleRoomState(w http.ResponseWriter, r *http.Request) {
	var req RoomStateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	roomManager.mu.RLock()
	defer roomManager.mu.RUnlock()

	room, ok := roomManager.rooms[req.GameID]
	if !ok {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	yourTurn := (room.GameActive && room.CurrentPlayer == req.Player)

	resp := RoomStateResponse{
		Board:         room.Board,
		CurrentPlayer: room.CurrentPlayer,
		GameActive:    room.GameActive,
		Winner:        room.Winner,
		Player1Name:   room.Player1Name,
		Player2Name:   room.Player2Name,
		YourTurn:      yourTurn,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleMakeMove handles a player making a move
func HandleMakeMove(w http.ResponseWriter, r *http.Request) {
	var req MakeMoveRequest
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

	resp := MakeMoveResponse{
		Success:       false,
		GameActive:    room.GameActive,
		Winner:        room.Winner,
		Message:       "",
		Board:         room.Board,
		CurrentPlayer: room.CurrentPlayer,
	}

	// Check if game is active
	if !room.GameActive {
		resp.Message = "Game is not active"
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Check if it's the player's turn
	if room.CurrentPlayer != req.Player {
		resp.Message = "Not your turn"
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Validate move coordinates
	if req.X < 0 || req.X >= 15 || req.Y < 0 || req.Y >= 15 {
		resp.Message = "Invalid move coordinates"
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Check if cell is empty
	if room.Board[req.X][req.Y] != 0 {
		resp.Message = "Cell is already occupied"
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Make the move
	room.Board[req.X][req.Y] = req.Player
	room.LastMoveTime = time.Now().Unix()

	// Check for win
	if checkWinOnBoard(room.Board, req.X, req.Y, req.Player) {
		room.Winner = req.Player
		room.GameActive = false
		resp.Success = true
		resp.Winner = req.Player
		resp.Message = "Win!"
	} else {
		// Switch player
		room.CurrentPlayer = 3 - req.Player // 1->2, 2->1
		resp.Success = true
		resp.Message = "Move successful"
	}

	resp.Board = room.Board
	resp.CurrentPlayer = room.CurrentPlayer
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
		// Only list rooms that are waiting for players (not active yet)
		// and not full (both players ready)
		if room.GameActive || (room.Player1Ready && room.Player2Ready) {
			continue
		}

		roomInfo := RoomInfo{
			GameID:       gameID,
			RoomName:     room.RoomName,
			Creator:      room.Creator,
			HasPassword:  room.Password != "",
			Player1Ready: room.Player1Ready,
			Player2Ready: room.Player2Ready,
			CreatedAt:    room.CreatedAt,
			Player1Name:  room.Player1Name,
			Player2Name:  room.Player2Name,
		}
		rooms = append(rooms, roomInfo)
	}

	resp := RoomListResponse{
		Rooms: rooms,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// checkWinOnBoard checks if a move at (x,y) results in a win for the player
func checkWinOnBoard(board [][]int, x, y, player int) bool {
	directions := [][2]int{
		{1, 0},  // Horizontal
		{0, 1},  // Vertical
		{1, 1},  // Diagonal \
		{1, -1}, // Diagonal /
	}

	for _, dir := range directions {
		dx, dy := dir[0], dir[1]
		count := 1

		// Check forward
		for i := 1; i < 5; i++ {
			nx, ny := x+dx*i, y+dy*i
			if nx < 0 || nx >= 15 || ny < 0 || ny >= 15 || board[nx][ny] != player {
				break
			}
			count++
		}

		// Check backward
		for i := 1; i < 5; i++ {
			nx, ny := x-dx*i, y-dy*i
			if nx < 0 || nx >= 15 || ny < 0 || ny >= 15 || board[nx][ny] != player {
				break
			}
			count++
		}

		if count >= 5 {
			return true
		}
	}

	return false
}