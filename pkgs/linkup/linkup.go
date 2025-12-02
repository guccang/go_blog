package linkup

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"
	"view"
)

// HandleLinkup renders the linkup game page
func HandleLinkup(w http.ResponseWriter, r *http.Request) {
	view.RenderTemplate(w, view.GetTemplatePath("linkup.template"), nil)
}

// GameMode represents the game mode
type GameMode string

const (
	ModePvE GameMode = "pve" // Player vs AI
	ModePvP GameMode = "pvp" // Player vs Player
	ModeRace GameMode = "race" // Race mode: two players compete independently
)

// GameState represents the current game state
type GameState struct {
	Board                [][]int  `json:"board"`                  // 0: empty, >0: icon type
	Rows                 int      `json:"rows"`                   // number of rows
	Cols                 int      `json:"cols"`                   // number of columns
	SelectedCell         *Cell    `json:"selectedCell"`           // currently selected cell, nil if none (for backward compatibility)
	Player1SelectedCell  *Cell    `json:"player1SelectedCell"`    // player 1's selected cell
	Player2SelectedCell  *Cell    `json:"player2SelectedCell"`    // player 2's selected cell
	GameMode             GameMode `json:"gameMode"`
	CurrentPlayer        int      `json:"currentPlayer"`          // 1: player1, 2: player2 (for backward compatibility)
	Player1Score         int      `json:"player1Score"`
	Player2Score         int      `json:"player2Score"`
	TotalPairs           int      `json:"totalPairs"`             // total icon pairs at game start
	RemainingPairs       int      `json:"remainingPairs"`         // remaining icon pairs
	GameActive           bool     `json:"gameActive"`
	GameID               string   `json:"gameId"`                 // unique game ID for PvP
	LastMoveTime         int64    `json:"lastMoveTime"`           // timestamp of last move
	Winner               int      `json:"winner"`                 // 0: no winner yet, 1: player1 won, 2: player2 won
}

// Cell represents a coordinate on the board
type Cell struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// NewGameRequest represents a request to start a new game
type NewGameRequest struct {
	Rows     int      `json:"rows"`
	Cols     int      `json:"cols"`
	GameMode GameMode `json:"gameMode"`
	Icons    int      `json:"icons"` // number of different icon types
}

// NewGameResponse represents the response for a new game
type NewGameResponse struct {
	GameState GameState `json:"gameState"`
	GameID    string    `json:"gameId"`
}

// SelectCellRequest represents a request to select a cell
type SelectCellRequest struct {
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	GameID string `json:"gameId"`
	Player int    `json:"player"` // player making the move (1 or 2)
}

// SelectCellResponse represents the response after selecting a cell
type SelectCellResponse struct {
	GameState      GameState `json:"gameState"`
	Matched        bool      `json:"matched"`        // whether the selection resulted in a match
	MatchCells     []Cell    `json:"matchCells"`     // cells that were matched (if any)
	GameOver       bool      `json:"gameOver"`       // whether the game is over
	CurrentPlayer  int       `json:"currentPlayer"`  // updated current player
	Player1Score   int       `json:"player1Score"`   // updated player 1 score
	Player2Score   int       `json:"player2Score"`   // updated player 2 score
	RemainingPairs int       `json:"remainingPairs"` // updated remaining pairs
}

// HintRequest represents a request for a hint
type HintRequest struct {
	GameID string `json:"gameId"`
}

// HintResponse represents a hint response
type HintResponse struct {
	Cell1 Cell `json:"cell1"`
	Cell2 Cell `json:"cell2"`
}

// AIMoveRequest represents a request for AI to make a move
type AIMoveRequest struct {
	GameState GameState `json:"gameState"`
	Level     int       `json:"level"` // difficulty level (1-3)
}

// AIMoveResponse represents the AI's move
type AIMoveResponse struct {
	Cell1     Cell      `json:"cell1"`
	Cell2     Cell      `json:"cell2"`
	GameState GameState `json:"gameState"`
}

// PvPStateRequest represents a request for PvP game state
type PvPStateRequest struct {
	GameID string `json:"gameId"`
	Player int    `json:"player"` // requesting player (1 or 2)
}

// PvPStateResponse represents the PvP game state response
type PvPStateResponse struct {
	GameState              GameState `json:"gameState"`
	OpponentScore          int       `json:"opponentScore"`
	OpponentRemainingPairs int       `json:"opponentRemainingPairs"`
	YourTurn               bool      `json:"yourTurn"` // whether it's the requesting player's turn
}

// In-memory game storage (for simplicity, in production use Redis)
var games = make(map[string]GameState)

// HandleNewGame starts a new Linkup game
func HandleNewGame(w http.ResponseWriter, r *http.Request) {
	var req NewGameRequest
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

	// Calculate total pairs
	totalPairs := (req.Rows * req.Cols) / 2

	// Determine starting player
	currentPlayer := 1 // Player always starts in PvE mode

	gameState := GameState{
		Board:               board,
		Rows:                req.Rows,
		Cols:                req.Cols,
		SelectedCell:        nil,
		Player1SelectedCell: nil,
		Player2SelectedCell: nil,
		GameMode:            req.GameMode,
		CurrentPlayer:       currentPlayer,
		Player1Score:        0,
		Player2Score:        0,
		TotalPairs:          totalPairs,
		RemainingPairs:      totalPairs,
		GameActive:          true,
		GameID:              generateGameID(),
		LastMoveTime:        time.Now().Unix(),
		Winner:              0,
	}

	// Store game
	games[gameState.GameID] = gameState

	resp := NewGameResponse{
		GameState: gameState,
		GameID:    gameState.GameID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleSelectCell handles cell selection
func HandleSelectCell(w http.ResponseWriter, r *http.Request) {
	var req SelectCellRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Load game state - try multiple ID formats
	var gameState GameState
	var ok bool

	// First try the provided ID
	gameState, ok = games[req.GameID]
	if !ok {
		// Try removing _p1 or _p2 suffix
		if len(req.GameID) > 3 && (req.GameID[len(req.GameID)-3:] == "_p1" || req.GameID[len(req.GameID)-3:] == "_p2") {
			originalGameID := req.GameID[:len(req.GameID)-3]
			gameState, ok = games[originalGameID]
		}
		// If still not found, try adding _p1 and _p2 suffixes
		if !ok {
			gameState, ok = games[req.GameID+"_p1"]
			if !ok {
				gameState, ok = games[req.GameID+"_p2"]
			}
		}
		if !ok {
			http.Error(w, "Game not found", http.StatusNotFound)
			return
		}
	}

	// Check if this is a race game
	if gameState.GameMode == ModeRace {
		// Extract original game ID (remove _p1 or _p2 suffix if present)
		originalGameID := req.GameID
		if len(originalGameID) > 3 && (originalGameID[len(originalGameID)-3:] == "_p1" || originalGameID[len(originalGameID)-3:] == "_p2") {
			originalGameID = originalGameID[:len(originalGameID)-3]
		}
		// Update request with original game ID
		req.GameID = originalGameID
		// Call race-specific handler
		handleRaceSelectCell(w, req)
		return
	}

	// Validate cell
	if req.Row < 0 || req.Row >= gameState.Rows || req.Col < 0 || req.Col >= gameState.Cols {
		http.Error(w, "Invalid cell", http.StatusBadRequest)
		return
	}

	// Check if cell is empty
	if gameState.Board[req.Row][req.Col] == 0 {
		http.Error(w, "Cell is empty", http.StatusBadRequest)
		return
	}

	cell := Cell{Row: req.Row, Col: req.Col}
	resp := SelectCellResponse{
		GameState:      gameState,
		Matched:        false,
		MatchCells:     []Cell{},
		GameOver:       false,
		CurrentPlayer:  gameState.CurrentPlayer,
		Player1Score:   gameState.Player1Score,
		Player2Score:   gameState.Player2Score,
		RemainingPairs: gameState.RemainingPairs,
	}

	// Get the selected cell pointer for this player
	var selectedCellPtr **Cell
	if gameState.GameMode == ModePvP {
		// PvP mode: each player has their own selected cell
		if req.Player == 1 {
			selectedCellPtr = &gameState.Player1SelectedCell
		} else {
			selectedCellPtr = &gameState.Player2SelectedCell
		}
		// Note: For PvP mode, we use separate selected cells for each player
		// The legacy SelectedCell field is not used in PvP mode
	} else {
		// Non-PvP mode: use the shared SelectedCell
		selectedCellPtr = &gameState.SelectedCell
	}

	// If no cell is selected yet for this player, select this cell
	if *selectedCellPtr == nil {
		*selectedCellPtr = &cell
		games[req.GameID] = gameState
		resp.GameState = gameState
	} else {
		// Check if trying to select the same cell
		if (*selectedCellPtr).Row == req.Row && (*selectedCellPtr).Col == req.Col {
			// Deselect cell
			*selectedCellPtr = nil
			games[req.GameID] = gameState
			resp.GameState = gameState
		} else {
			// Try to match with previously selected cell
			cell1 := **selectedCellPtr
			cell2 := cell

			if canConnect(gameState.Board, gameState.Rows, gameState.Cols, cell1, cell2) {
				// Match successful
				gameState.Board[cell1.Row][cell1.Col] = 0
				gameState.Board[cell2.Row][cell2.Col] = 0
				*selectedCellPtr = nil
				gameState.RemainingPairs--

				// Update score
				if req.Player == 1 {
					gameState.Player1Score++
				} else {
					gameState.Player2Score++
				}

				// Check if game is over (board is cleared)
				if gameState.RemainingPairs == 0 {
					gameState.GameActive = false
					resp.GameOver = true
					// Determine winner
					if gameState.Player1Score > gameState.Player2Score {
						gameState.Winner = 1
					} else if gameState.Player2Score > gameState.Player1Score {
						gameState.Winner = 2
					} else {
						// Tie
						gameState.Winner = 0
					}
				}

				gameState.LastMoveTime = time.Now().Unix()
				games[req.GameID] = gameState

				resp.GameState = gameState
				resp.Matched = true
				resp.MatchCells = []Cell{cell1, cell2}
				resp.Player1Score = gameState.Player1Score
				resp.Player2Score = gameState.Player2Score
				resp.RemainingPairs = gameState.RemainingPairs
				resp.CurrentPlayer = gameState.CurrentPlayer
			} else {
				// Match failed, select new cell
				*selectedCellPtr = &cell
				games[req.GameID] = gameState
				resp.GameState = gameState
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleAIMove handles AI move request
func HandleAIMove(w http.ResponseWriter, r *http.Request) {
	var req AIMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Find a match using AI based on difficulty level
	cell1, cell2, found := AIMove(req.GameState, req.Level)
	if !found {
		// No possible moves (shouldn't happen if game is not over)
		http.Error(w, "No possible moves", http.StatusBadRequest)
		return
	}

	// Update game state
	req.GameState.Board[cell1.Row][cell1.Col] = 0
	req.GameState.Board[cell2.Row][cell2.Col] = 0
	req.GameState.RemainingPairs--
	req.GameState.Player2Score++ // AI is player 2
	req.GameState.LastMoveTime = time.Now().Unix()

	// Check if game is over
	if req.GameState.RemainingPairs == 0 {
		req.GameState.GameActive = false
	}

	// Switch back to player
	req.GameState.CurrentPlayer = 1

	resp := AIMoveResponse{
		Cell1:     cell1,
		Cell2:     cell2,
		GameState: req.GameState,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleHint provides a hint for the player
func HandleHint(w http.ResponseWriter, r *http.Request) {
	var req HintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Load game state
	gameState, ok := games[req.GameID]
	if !ok {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	cell1, cell2, found := findPossibleMatch(gameState.Board, gameState.Rows, gameState.Cols)
	if !found {
		http.Error(w, "No possible matches", http.StatusBadRequest)
		return
	}

	resp := HintResponse{
		Cell1: cell1,
		Cell2: cell2,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandlePvPState gets the current PvP game state
func HandlePvPState(w http.ResponseWriter, r *http.Request) {
	var req PvPStateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Load game state
	gameState, ok := games[req.GameID]
	if !ok {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Determine opponent's score and remaining pairs
	opponentScore := 0
	if req.Player == 1 {
		opponentScore = gameState.Player2Score
	} else {
		opponentScore = gameState.Player1Score
	}

	// Estimate opponent's remaining pairs (same for both players)
	opponentRemainingPairs := gameState.RemainingPairs

	// Check if it's the requesting player's turn
	yourTurn := (req.Player == gameState.CurrentPlayer)

	resp := PvPStateResponse{
		GameState:              gameState,
		OpponentScore:          opponentScore,
		OpponentRemainingPairs: opponentRemainingPairs,
		YourTurn:               yourTurn,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// generateBoard creates a random board with pairs of icons
func generateBoard(rows, cols, iconTypes int) [][]int {
	totalCells := rows * cols
	iconCount := totalCells
	if totalCells%2 != 0 {
		// Odd number of cells: leave one cell empty (0)
		iconCount = totalCells - 1
	}

	// Create pairs of icons
	icons := make([]int, iconCount)
	for i := 0; i < iconCount; i += 2 {
		iconType := (i/2)%iconTypes + 1
		icons[i] = iconType
		icons[i+1] = iconType
	}

	// Shuffle icons
	for i := range icons {
		j := i + randInt(0, len(icons)-i-1)
		icons[i], icons[j] = icons[j], icons[i]
	}

	// Convert to 2D board
	board := make([][]int, rows)
	for i := 0; i < rows; i++ {
		board[i] = make([]int, cols)
		for j := 0; j < cols; j++ {
			idx := i*cols + j
			if idx < len(icons) {
				board[i][j] = icons[idx]
			} else {
				board[i][j] = 0 // empty cell
			}
		}
	}

	return board
}

// generateGameID generates a unique game ID
func generateGameID() string {
	b := make([]byte, 6)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback to timestamp
		return fmt.Sprintf("linkup_%d", time.Now().UnixNano())
	}
	return "linkup_" + base64.URLEncoding.EncodeToString(b)
}

// randInt returns a random integer in [min, max]
func randInt(min, max int) int {
	if min > max {
		min, max = max, min
	}
	if min == max {
		return min
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		// Fallback to pseudo-random
		return min + (time.Now().Nanosecond() % (max - min + 1))
	}
	return min + int(n.Int64())
}