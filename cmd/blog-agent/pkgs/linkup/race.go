package linkup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// RaceGame represents a race game session where two players compete independently
type RaceGame struct {
	GameID           string
	Player1State     GameState
	Player2State     GameState
	Player1Ready     bool
	Player2Ready     bool
	Player1StartTime int64 // When player 1 started (unix timestamp)
	Player2StartTime int64 // When player 2 started (unix timestamp)
	Player1FinishTime int64 // When player 1 finished (0 if not finished)
	Player2FinishTime int64 // When player 2 finished (0 if not finished)
	CreatedAt        int64
	GameActive       bool // True when both players are ready
	Winner           int  // 0: no winner yet, 1: player1 won, 2: player2 won
	Password         string // Empty string means no password
	Creator          string // Creator's name or identifier
	RoomName         string // Optional room name
}

// RaceGameManager manages all race games
type RaceGameManager struct {
	games map[string]*RaceGame
	mu    sync.RWMutex
}

// Global race game manager
var raceGameManager = &RaceGameManager{
	games: make(map[string]*RaceGame),
}

// CreateRaceRequest represents a request to create a new race game
type CreateRaceRequest struct {
	Rows      int    `json:"rows"`
	Cols      int    `json:"cols"`
	Icons     int    `json:"icons"`
	Password  string `json:"password"`  // Optional password
	Creator   string `json:"creator"`   // Creator's name
	RoomName  string `json:"roomName"`  // Optional room name
}

// CreateRaceResponse represents the response after creating a race game
type CreateRaceResponse struct {
	GameID    string    `json:"gameId"`
	GameState GameState `json:"gameState"`
	Player    int       `json:"player"` // player number (always 1 for creator)
}

// JoinRaceRequest represents a request to join a race game
type JoinRaceRequest struct {
	GameID   string `json:"gameId"`
	Player   int    `json:"player"` // 1 or 2
	Password string `json:"password"` // Required if room has password
}

// JoinRaceResponse represents the response after joining a race game
type JoinRaceResponse struct {
	Success     bool      `json:"success"`
	GameState   GameState `json:"gameState"`
	Player      int       `json:"player"` // assigned player number
	Message     string    `json:"message"`
}

// RaceStateRequest represents a request for race game state
type RaceStateRequest struct {
	GameID string `json:"gameId"`
	Player int    `json:"player"` // requesting player (1 or 2)
}

// RaceStateResponse represents the race game state response
type RaceStateResponse struct {
	YourGameState        GameState `json:"yourGameState"`
	OpponentGameState    GameState `json:"opponentGameState"`
	OpponentScore        int       `json:"opponentScore"`
	OpponentRemainingPairs int     `json:"opponentRemainingPairs"`
	OpponentFinished     bool      `json:"opponentFinished"`
	OpponentFinishTime   int64     `json:"opponentFinishTime"` // 0 if not finished
	GameActive           bool      `json:"gameActive"`
	Winner               int       `json:"winner"` // 0: no winner yet, 1: player1 won, 2: player2 won
	YourFinishTime       int64     `json:"yourFinishTime"` // 0 if not finished
}

// RaceRoomInfo represents information about a race room for listing
type RaceRoomInfo struct {
	GameID      string `json:"gameId"`
	RoomName    string `json:"roomName"`    // Optional room name
	Creator     string `json:"creator"`     // Creator's name
	Rows        int    `json:"rows"`        // Board rows
	Cols        int    `json:"cols"`        // Board columns
	Icons       int    `json:"icons"`       // Number of icon types
	HasPassword bool   `json:"hasPassword"` // Whether room has password
	Player1Ready bool  `json:"player1Ready"` // Is player 1 ready?
	Player2Ready bool  `json:"player2Ready"` // Is player 2 ready?
	CreatedAt   int64  `json:"createdAt"`   // When the room was created
}

// RaceListResponse represents the response for race room list
type RaceListResponse struct {
	Rooms []RaceRoomInfo `json:"rooms"`
}

// HandleCreateRace creates a new race game
func HandleCreateRace(w http.ResponseWriter, r *http.Request) {
	var req CreateRaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Rows < 4 || req.Rows > 20 {
		req.Rows = 8
	}
	if req.Cols < 4 || req.Cols > 20 {
		req.Cols = 8
	}
	if req.Icons < 4 || req.Icons > 20 {
		req.Icons = 8
	}

	// Generate game board (same for both players)
	board := generateBoard(req.Rows, req.Cols, req.Icons)
	totalPairs := (req.Rows * req.Cols) / 2

	// Create game states for both players (identical initial boards)
	player1State := GameState{
		Board:               copyBoard(board),
		Rows:                req.Rows,
		Cols:                req.Cols,
		SelectedCell:        nil,
		Player1SelectedCell: nil,
		Player2SelectedCell: nil,
		GameMode:            ModeRace,
		CurrentPlayer:       1, // Not used in race mode but kept for compatibility
		Player1Score:        0,
		Player2Score:        0, // Not used in race mode
		TotalPairs:          totalPairs,
		RemainingPairs:      totalPairs,
		GameActive:          false, // Will be active when both players ready
		GameID:              generateGameID(),
		LastMoveTime:        time.Now().Unix(),
		Winner:              0,
	}

	player2State := GameState{
		Board:               copyBoard(board),
		Rows:                req.Rows,
		Cols:                req.Cols,
		SelectedCell:        nil,
		Player1SelectedCell: nil,
		Player2SelectedCell: nil,
		GameMode:            ModeRace,
		CurrentPlayer:       1, // Not used in race mode
		Player1Score:        0,
		Player2Score:        0,
		TotalPairs:          totalPairs,
		RemainingPairs:      totalPairs,
		GameActive:          false,
		GameID:              player1State.GameID, // Same game ID
		LastMoveTime:        time.Now().Unix(),
		Winner:              0,
	}

	// Create race game
	raceGame := &RaceGame{
		GameID:           player1State.GameID,
		Player1State:     player1State,
		Player2State:     player2State,
		Player1Ready:     true, // Creator is ready
		Player2Ready:     false,
		Player1StartTime: 0, // Will be set when game starts
		Player2StartTime: 0,
		Player1FinishTime: 0,
		Player2FinishTime: 0,
		CreatedAt:        time.Now().Unix(),
		GameActive:       false,
		Winner:           0,
		Password:         req.Password,
		Creator:          req.Creator,
		RoomName:         req.RoomName,
	}

	// Store in manager
	raceGameManager.mu.Lock()
	raceGameManager.games[player1State.GameID] = raceGame
	raceGameManager.mu.Unlock()

	// Also store individual game states in global games map for backward compatibility
	games[player1State.GameID+"_p1"] = player1State
	games[player2State.GameID+"_p2"] = player2State

	resp := CreateRaceResponse{
		GameID:    player1State.GameID,
		GameState: player1State,
		Player:    1,
	}

	fmt.Printf("[DEBUG] HandleCreateRace: created race game %s for player 1\n", player1State.GameID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleJoinRace handles a player joining a race game
func HandleJoinRace(w http.ResponseWriter, r *http.Request) {
	var req JoinRaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	raceGameManager.mu.Lock()
	defer raceGameManager.mu.Unlock()

	raceGame, ok := raceGameManager.games[req.GameID]
	if !ok {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Check password if required
	if raceGame.Password != "" && raceGame.Password != req.Password {
		http.Error(w, "Incorrect password", http.StatusUnauthorized)
		return
	}

	resp := JoinRaceResponse{
		Success:   false,
		GameState: GameState{},
		Player:    0,
		Message:   "",
	}

	// Check if player 1 is joining
	if req.Player == 1 {
		if raceGame.Player1Ready {
			resp.Message = "Player 1 already joined"
			json.NewEncoder(w).Encode(resp)
			return
		}
		raceGame.Player1Ready = true
		resp.Success = true
		resp.Player = 1
		resp.GameState = raceGame.Player1State
	} else if req.Player == 2 {
		if raceGame.Player2Ready {
			resp.Message = "Player 2 already joined"
			json.NewEncoder(w).Encode(resp)
			return
		}
		raceGame.Player2Ready = true
		resp.Success = true
		resp.Player = 2
		resp.GameState = raceGame.Player2State

		// If both players are ready, start the game
		if raceGame.Player1Ready && raceGame.Player2Ready {
			raceGame.GameActive = true
			raceGame.Player1StartTime = time.Now().Unix()
			raceGame.Player2StartTime = time.Now().Unix()
			// Update individual game states
			raceGame.Player1State.GameActive = true
			raceGame.Player2State.GameActive = true
			// Update global games map
			games[raceGame.GameID+"_p1"] = raceGame.Player1State
			games[raceGame.GameID+"_p2"] = raceGame.Player2State
		}
	} else {
		resp.Message = "Invalid player number"
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Update the game
	raceGameManager.games[req.GameID] = raceGame

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleRaceState gets the current race game state
func HandleRaceState(w http.ResponseWriter, r *http.Request) {
	var req RaceStateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	raceGameManager.mu.RLock()
	defer raceGameManager.mu.RUnlock()

	raceGame, ok := raceGameManager.games[req.GameID]
	if !ok {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	var yourState, opponentState GameState
	var yourFinishTime, opponentFinishTime int64
	var opponentFinished bool
	var opponentScore, opponentRemainingPairs int

	if req.Player == 1 {
		yourState = raceGame.Player1State
		opponentState = raceGame.Player2State
		yourFinishTime = raceGame.Player1FinishTime
		opponentFinishTime = raceGame.Player2FinishTime
		opponentFinished = raceGame.Player2FinishTime > 0
		opponentScore = opponentState.Player1Score
		opponentRemainingPairs = opponentState.RemainingPairs
	} else {
		yourState = raceGame.Player2State
		opponentState = raceGame.Player1State
		yourFinishTime = raceGame.Player2FinishTime
		opponentFinishTime = raceGame.Player1FinishTime
		opponentFinished = raceGame.Player1FinishTime > 0
		opponentScore = opponentState.Player1Score
		opponentRemainingPairs = opponentState.RemainingPairs
	}

	// Determine winner if both finished
	winner := 0
	if raceGame.Player1FinishTime > 0 && raceGame.Player2FinishTime > 0 {
		if raceGame.Player1FinishTime < raceGame.Player2FinishTime {
			winner = 1 // Player 1 finished first
		} else if raceGame.Player2FinishTime < raceGame.Player1FinishTime {
			winner = 2 // Player 2 finished first
		} else {
			winner = 0 // Tie
		}
	} else if raceGame.Player1FinishTime > 0 {
		winner = 1 // Player 1 finished, player 2 hasn't
	} else if raceGame.Player2FinishTime > 0 {
		winner = 2 // Player 2 finished, player 1 hasn't
	}

	resp := RaceStateResponse{
		YourGameState:         yourState,
		OpponentGameState:     opponentState,
		OpponentScore:         opponentScore,
		OpponentRemainingPairs: opponentRemainingPairs,
		OpponentFinished:      opponentFinished,
		OpponentFinishTime:    opponentFinishTime,
		GameActive:            raceGame.GameActive,
		Winner:                winner,
		YourFinishTime:        yourFinishTime,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleRaceList returns a list of available race rooms
func HandleRaceList(w http.ResponseWriter, r *http.Request) {
	raceGameManager.mu.RLock()
	defer raceGameManager.mu.RUnlock()

	rooms := []RaceRoomInfo{}
	for gameID, raceGame := range raceGameManager.games {
		// Only list rooms that are waiting for players (not active yet)
		// and not full (both players ready)
		if raceGame.GameActive || (raceGame.Player1Ready && raceGame.Player2Ready) {
			continue
		}

		roomInfo := RaceRoomInfo{
			GameID:      gameID,
			RoomName:    raceGame.RoomName,
			Creator:     raceGame.Creator,
			Rows:        raceGame.Player1State.Rows,
			Cols:        raceGame.Player1State.Cols,
			Icons:       raceGame.Player1State.TotalPairs * 2 / (raceGame.Player1State.Rows * raceGame.Player1State.Cols), // Calculate icons count
			HasPassword: raceGame.Password != "",
			Player1Ready: raceGame.Player1Ready,
			Player2Ready: raceGame.Player2Ready,
			CreatedAt:   raceGame.CreatedAt,
		}
		rooms = append(rooms, roomInfo)
	}

	resp := RaceListResponse{
		Rooms: rooms,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Helper function to copy a board
func copyBoard(board [][]int) [][]int {
	if board == nil {
		return nil
	}
	copied := make([][]int, len(board))
	for i := range board {
		copied[i] = make([]int, len(board[i]))
		copy(copied[i], board[i])
	}
	return copied
}

// handleRaceSelectCell handles cell selection for race mode
func handleRaceSelectCell(w http.ResponseWriter, req SelectCellRequest) {
	raceGameManager.mu.Lock()
	defer raceGameManager.mu.Unlock()

	raceGame, ok := raceGameManager.games[req.GameID]
	if !ok {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Get the player's game state
	var playerState *GameState
	if req.Player == 1 {
		playerState = &raceGame.Player1State
	} else if req.Player == 2 {
		playerState = &raceGame.Player2State
	} else {
		http.Error(w, "Invalid player number", http.StatusBadRequest)
		return
	}

	// Validate cell
	if req.Row < 0 || req.Row >= playerState.Rows || req.Col < 0 || req.Col >= playerState.Cols {
		http.Error(w, "Invalid cell", http.StatusBadRequest)
		return
	}

	// Check if cell is empty
	if playerState.Board[req.Row][req.Col] == 0 {
		http.Error(w, "Cell is empty", http.StatusBadRequest)
		return
	}

	cell := Cell{Row: req.Row, Col: req.Col}
	resp := SelectCellResponse{
		GameState:      *playerState,
		Matched:        false,
		MatchCells:     []Cell{},
		GameOver:       false,
		CurrentPlayer:  playerState.CurrentPlayer,
		Player1Score:   playerState.Player1Score,
		Player2Score:   playerState.Player2Score,
		RemainingPairs: playerState.RemainingPairs,
	}

	// In race mode, use the shared SelectedCell (not player-specific)
	selectedCellPtr := &playerState.SelectedCell

	// If no cell is selected yet for this player, select this cell
	if *selectedCellPtr == nil {
		*selectedCellPtr = &cell
		resp.GameState = *playerState
	} else {
		// Check if trying to select the same cell
		if (*selectedCellPtr).Row == req.Row && (*selectedCellPtr).Col == req.Col {
			// Deselect cell
			*selectedCellPtr = nil
			resp.GameState = *playerState
		} else {
			// Try to match with previously selected cell
			cell1 := **selectedCellPtr
			cell2 := cell

			if canConnect(playerState.Board, playerState.Rows, playerState.Cols, cell1, cell2) {
				// Match successful
				playerState.Board[cell1.Row][cell1.Col] = 0
				playerState.Board[cell2.Row][cell2.Col] = 0
				*selectedCellPtr = nil
				playerState.RemainingPairs--
				playerState.Player1Score++ // In race mode, player's score is stored in Player1Score

				// Check if player has finished (board is cleared)
				if playerState.RemainingPairs == 0 {
					playerState.GameActive = false
					resp.GameOver = true
					// Record finish time
					now := time.Now().Unix()
					if req.Player == 1 {
						raceGame.Player1FinishTime = now
					} else {
						raceGame.Player2FinishTime = now
					}
					// Determine winner if both finished
					if raceGame.Player1FinishTime > 0 && raceGame.Player2FinishTime > 0 {
						if raceGame.Player1FinishTime < raceGame.Player2FinishTime {
							raceGame.Winner = 1
						} else if raceGame.Player2FinishTime < raceGame.Player1FinishTime {
							raceGame.Winner = 2
						} else {
							raceGame.Winner = 0 // Tie
						}
					}
				}

				playerState.LastMoveTime = time.Now().Unix()
				resp.GameState = *playerState
				resp.Matched = true
				resp.MatchCells = []Cell{cell1, cell2}
				resp.Player1Score = playerState.Player1Score
				resp.Player2Score = playerState.Player2Score
				resp.RemainingPairs = playerState.RemainingPairs
				resp.CurrentPlayer = playerState.CurrentPlayer
			} else {
				// Match failed, select new cell
				*selectedCellPtr = &cell
				resp.GameState = *playerState
			}
		}
	}

	// Update global games map for backward compatibility
	games[req.GameID+"_p"+fmt.Sprint(req.Player)] = *playerState

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}