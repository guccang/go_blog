package linkup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// PvPGame represents a PvP game session
type PvPGame struct {
	GameState      GameState
	Player1Ready   bool
	Player2Ready   bool
	Player1LastSeen int64
	Player2LastSeen int64
	CreatedAt      int64
}

// PvPGameManager manages all PvP games
type PvPGameManager struct {
	games map[string]*PvPGame
	mu    sync.RWMutex
}

// Global PvP game manager
var pvpGameManager = &PvPGameManager{
	games: make(map[string]*PvPGame),
}

// JoinPvPRequest represents a request to join a PvP game
type JoinPvPRequest struct {
	GameID string `json:"gameId"`
	Player int    `json:"player"` // 1 or 2
}

// JoinPvPResponse represents the response after joining a PvP game
type JoinPvPResponse struct {
	Success     bool      `json:"success"`
	GameState   GameState `json:"gameState"`
	Player      int       `json:"player"` // assigned player number
	Message     string    `json:"message"`
}

// CreatePvPRequest represents a request to create a new PvP game
type CreatePvPRequest struct {
	Rows  int `json:"rows"`
	Cols  int `json:"cols"`
	Icons int `json:"icons"`
}

// CreatePvPResponse represents the response after creating a PvP game
type CreatePvPResponse struct {
	GameID    string    `json:"gameId"`
	GameState GameState `json:"gameState"`
	Player    int       `json:"player"` // player number (always 1 for creator)
}

// HandleCreatePvP creates a new PvP game
func HandleCreatePvP(w http.ResponseWriter, r *http.Request) {
	var req CreatePvPRequest
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

	// Generate game board
	board := generateBoard(req.Rows, req.Cols, req.Icons)
	fmt.Printf("[DEBUG] HandleCreatePvP: generating board rows=%d, cols=%d, icons=%d, board rows=%d, board[0] cols=%d\n",
		req.Rows, req.Cols, req.Icons, len(board), len(board[0]))

	// Calculate total pairs
	totalPairs := (req.Rows * req.Cols) / 2

	gameState := GameState{
		Board:               board,
		Rows:                req.Rows,
		Cols:                req.Cols,
		SelectedCell:        nil,
		Player1SelectedCell: nil,
		Player2SelectedCell: nil,
		GameMode:            ModePvP,
		CurrentPlayer:       1, // Player 1 starts (for backward compatibility)
		Player1Score:        0,
		Player2Score:        0,
		TotalPairs:          totalPairs,
		RemainingPairs:      totalPairs,
		GameActive:          false, // Game starts when both players are ready
		GameID:              generateGameID(),
		LastMoveTime:        time.Now().Unix(),
		Winner:              0,
	}

	// Create PvP game
	pvpGame := &PvPGame{
		GameState:      gameState,
		Player1Ready:   true, // Creator is ready
		Player2Ready:   false,
		Player1LastSeen: time.Now().Unix(),
		Player2LastSeen: 0,
		CreatedAt:      time.Now().Unix(),
	}

	// Store in manager
	pvpGameManager.mu.Lock()
	pvpGameManager.games[gameState.GameID] = pvpGame
	pvpGameManager.mu.Unlock()

	// Also store in global games map for backward compatibility
	games[gameState.GameID] = gameState

	resp := CreatePvPResponse{
		GameID:    gameState.GameID,
		GameState: gameState,
		Player:    1,
	}

	fmt.Printf("[DEBUG] HandleCreatePvP: created game %s for player 1\n", gameState.GameID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleJoinPvP handles a player joining a PvP game
func HandleJoinPvP(w http.ResponseWriter, r *http.Request) {
	var req JoinPvPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pvpGameManager.mu.Lock()
	defer pvpGameManager.mu.Unlock()

	pvpGame, ok := pvpGameManager.games[req.GameID]
	if !ok {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	resp := JoinPvPResponse{
		Success:   false,
		GameState: pvpGame.GameState,
		Player:    0,
		Message:   "",
	}

	// Check if player 1 is joining
	if req.Player == 1 {
		if pvpGame.Player1Ready {
			resp.Message = "Player 1 already joined"
			json.NewEncoder(w).Encode(resp)
			return
		}
		pvpGame.Player1Ready = true
		pvpGame.Player1LastSeen = time.Now().Unix()
		resp.Success = true
		resp.Player = 1
		resp.GameState = pvpGame.GameState
	} else if req.Player == 2 {
		if pvpGame.Player2Ready {
			resp.Message = "Player 2 already joined"
			json.NewEncoder(w).Encode(resp)
			return
		}
		pvpGame.Player2Ready = true
		pvpGame.Player2LastSeen = time.Now().Unix()

		// If both players are ready, start the game
		if pvpGame.Player1Ready && pvpGame.Player2Ready {
			pvpGame.GameState.GameActive = true
			// Update global games map
			games[req.GameID] = pvpGame.GameState
		}

		resp.Success = true
		resp.Player = 2
		resp.GameState = pvpGame.GameState
		// Debug logging
		fmt.Printf("[DEBUG] HandleJoinPvP: player %d joined game %s, board rows=%d, cols=%d, board exists=%v\n",
			req.Player, req.GameID, pvpGame.GameState.Rows, pvpGame.GameState.Cols,
			pvpGame.GameState.Board != nil)
	} else {
		resp.Message = "Invalid player number"
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Update the game
	pvpGameManager.games[req.GameID] = pvpGame

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandlePvPReady checks if both players are ready and starts the game
func HandlePvPReady(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Query().Get("gameId")
	if gameID == "" {
		http.Error(w, "Game ID required", http.StatusBadRequest)
		return
	}

	pvpGameManager.mu.RLock()
	defer pvpGameManager.mu.RUnlock()

	pvpGame, ok := pvpGameManager.games[gameID]
	if !ok {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	response := struct {
		Player1Ready bool `json:"player1Ready"`
		Player2Ready bool `json:"player2Ready"`
		GameActive   bool `json:"gameActive"`
	}{
		Player1Ready: pvpGame.Player1Ready,
		Player2Ready: pvpGame.Player2Ready,
		GameActive:   pvpGame.GameState.GameActive,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CleanupStaleGames removes games that have been inactive for too long
func CleanupStaleGames() {
	pvpGameManager.mu.Lock()
	defer pvpGameManager.mu.Unlock()

	now := time.Now().Unix()
	staleThreshold := int64(3600) // 1 hour

	for gameID, pvpGame := range pvpGameManager.games {
		// Remove if game was created more than threshold ago and no activity
		lastActivity := max64(pvpGame.Player1LastSeen, pvpGame.Player2LastSeen)
		if now-pvpGame.CreatedAt > staleThreshold && now-lastActivity > staleThreshold {
			delete(pvpGameManager.games, gameID)
			delete(games, gameID)
		}
	}
}

// Helper function
func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}