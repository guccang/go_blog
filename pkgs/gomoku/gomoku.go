package gomoku

import (
	"encoding/json"
	"net/http"
	"view"
)

// HandleGomoku renders the gomoku game page
func HandleGomoku(w http.ResponseWriter, r *http.Request) {
	view.RenderTemplate(w, view.GetTemplatePath("gomoku.template"), nil)
}

// Point represents a coordinate on the board
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// MoveRequest represents the request for an AI move
type MoveRequest struct {
	Board      [][]int `json:"board"`      // 0: empty, 1: black (player), 2: white (AI)
	PlayerRole int     `json:"playerRole"` // 1 or 2
	Level      int     `json:"level"`      // Difficulty level
}

// MoveResponse represents the AI's move
type MoveResponse struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// HandleAIMove calculates the best move for the AI
func HandleAIMove(w http.ResponseWriter, r *http.Request) {
	var req MoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Simple AI logic (placeholder for now, will implement Minimax later)
	// For now, find the first empty spot or a random one
	x, y := findBestMove(req.Board, 2) // Assuming AI is 2 (White)

	resp := MoveResponse{X: x, Y: y}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// findBestMove finds the best move for the given player
func findBestMove(board [][]int, player int) (int, int) {
	size := len(board)
	if size == 0 {
		return 0, 0
	}

	// If board is empty, play center
	if board[size/2][size/2] == 0 {
		return size / 2, size / 2
	}

	opponent := 1
	if player == 1 {
		opponent = 2
	}

	bestScore := -1
	bestX, bestY := -1, -1

	// Evaluate each empty cell
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			if board[i][j] == 0 {
				// Calculate score for this position
				score := evaluatePosition(board, i, j, player, opponent)
				if score > bestScore {
					bestScore = score
					bestX = i
					bestY = j
				}
			}
		}
	}

	if bestX == -1 {
		// Fallback to random empty spot
		for i := 0; i < size; i++ {
			for j := 0; j < size; j++ {
				if board[i][j] == 0 {
					return i, j
				}
			}
		}
		return -1, -1
	}

	return bestX, bestY
}

// evaluatePosition calculates the score of a move at (x, y)
func evaluatePosition(board [][]int, x, y, player, opponent int) int {
	score := 0

	// Evaluate for attack (AI) and defense (Player)
	// Defense is usually more important to prevent losing
	score += evaluateDirection(board, x, y, player, 1, 0)  // Horizontal
	score += evaluateDirection(board, x, y, player, 0, 1)  // Vertical
	score += evaluateDirection(board, x, y, player, 1, 1)  // Diagonal \
	score += evaluateDirection(board, x, y, player, 1, -1) // Diagonal /

	score += evaluateDirection(board, x, y, opponent, 1, 0)  // Horizontal
	score += evaluateDirection(board, x, y, opponent, 0, 1)  // Vertical
	score += evaluateDirection(board, x, y, opponent, 1, 1)  // Diagonal \
	score += evaluateDirection(board, x, y, opponent, 1, -1) // Diagonal /

	return score
}

// evaluateDirection counts consecutive pieces in a direction
func evaluateDirection(board [][]int, x, y, targetPlayer, dx, dy int) int {
	count := 0
	size := len(board)

	// Check forward
	for i := 1; i < 5; i++ {
		nx, ny := x+dx*i, y+dy*i
		if nx < 0 || nx >= size || ny < 0 || ny >= size {
			break
		}
		if board[nx][ny] == targetPlayer {
			count++
		} else if board[nx][ny] == 0 {
			break
		} else {
			// Blocked by opponent
			break
		}
	}

	// Check backward
	for i := 1; i < 5; i++ {
		nx, ny := x-dx*i, y-dy*i
		if nx < 0 || nx >= size || ny < 0 || ny >= size {
			break
		}
		if board[nx][ny] == targetPlayer {
			count++
		} else if board[nx][ny] == 0 {
			break
		} else {
			// Blocked by opponent
			break
		}
	}

	// Simple scoring
	switch count {
	case 4:
		return 100000 // Win or block win
	case 3:
		return 1000
	case 2:
		return 100
	case 1:
		return 10
	default:
		return 0
	}
}
