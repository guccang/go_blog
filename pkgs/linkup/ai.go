package linkup

import (
	"math/rand"
	"time"
)

// Difficulty levels
const (
	DifficultyEasy   = 1
	DifficultyMedium = 2
	DifficultyHard   = 3
)

// AIMove finds the best move for AI based on difficulty level
func AIMove(gameState GameState, level int) (Cell, Cell, bool) {
	switch level {
	case DifficultyEasy:
		return findRandomMatch(gameState.Board, gameState.Rows, gameState.Cols)
	case DifficultyMedium:
		return findMediumMatch(gameState.Board, gameState.Rows, gameState.Cols)
	case DifficultyHard:
		cell1, cell2, score := findBestMatch(gameState.Board, gameState.Rows, gameState.Cols)
		if score >= 0 {
			return cell1, cell2, true
		}
		return Cell{}, Cell{}, false
	default:
		return findRandomMatch(gameState.Board, gameState.Rows, gameState.Cols)
	}
}

// findRandomMatch finds a random possible match
func findRandomMatch(board [][]int, rows, cols int) (Cell, Cell, bool) {
	// Collect all non-empty cells
	cells := make([]Cell, 0)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if board[r][c] != 0 {
				cells = append(cells, Cell{Row: r, Col: c})
			}
		}
	}

	// Shuffle cells
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(cells), func(i, j int) {
		cells[i], cells[j] = cells[j], cells[i]
	})

	// Try to find a match
	for i := 0; i < len(cells); i++ {
		for j := i + 1; j < len(cells); j++ {
			if board[cells[i].Row][cells[i].Col] != board[cells[j].Row][cells[j].Col] {
				continue
			}
			if canConnect(board, rows, cols, cells[i], cells[j]) {
				return cells[i], cells[j], true
			}
		}
	}

	return Cell{}, Cell{}, false
}

// findMediumMatch finds a match with preference for easier connections
func findMediumMatch(board [][]int, rows, cols int) (Cell, Cell, bool) {
	// Collect all non-empty cells
	cells := make([]Cell, 0)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if board[r][c] != 0 {
				cells = append(cells, Cell{Row: r, Col: c})
			}
		}
	}

	// Try to find straight line matches first
	for i := 0; i < len(cells); i++ {
		for j := i + 1; j < len(cells); j++ {
			if board[cells[i].Row][cells[i].Col] != board[cells[j].Row][cells[j].Col] {
				continue
			}
			if canConnectStraight(board, rows, cols, cells[i], cells[j]) {
				return cells[i], cells[j], true
			}
		}
	}

	// Then try one-corner matches
	for i := 0; i < len(cells); i++ {
		for j := i + 1; j < len(cells); j++ {
			if board[cells[i].Row][cells[i].Col] != board[cells[j].Row][cells[j].Col] {
				continue
			}
			if canConnectOneCorner(board, rows, cols, cells[i], cells[j]) {
				return cells[i], cells[j], true
			}
		}
	}

	// Finally try two-corner matches
	for i := 0; i < len(cells); i++ {
		for j := i + 1; j < len(cells); j++ {
			if board[cells[i].Row][cells[i].Col] != board[cells[j].Row][cells[j].Col] {
				continue
			}
			if canConnectTwoCorners(board, rows, cols, cells[i], cells[j]) {
				return cells[i], cells[j], true
			}
		}
	}

	return Cell{}, Cell{}, false
}

// findBestMatch finds the best match based on heuristic (already implemented in logic.go)
// This function is already defined in logic.go, so we'll just call it