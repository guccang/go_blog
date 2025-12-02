const canvas = document.getElementById('board');
const ctx = canvas.getContext('2d');
const statusDisplay = document.getElementById('status');
const modeSelect = document.getElementById('game-mode');

const BOARD_SIZE = 15;
const CELL_SIZE = 40;
const PADDING = 20;

let board = [];
let currentPlayer = 1; // 1: Black, 2: White
let gameActive = false;
let gameMode = 'pve'; // pve, evp, pvp
let moveHistory = [];

// Initialize board
function initBoard() {
    board = Array(BOARD_SIZE).fill().map(() => Array(BOARD_SIZE).fill(0));
    moveHistory = [];
}

// Draw the board grid
function drawBoard() {
    ctx.fillStyle = '#DEB887';
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    ctx.strokeStyle = '#000';
    ctx.lineWidth = 1;

    for (let i = 0; i < BOARD_SIZE; i++) {
        // Horizontal lines
        ctx.beginPath();
        ctx.moveTo(PADDING, PADDING + i * CELL_SIZE);
        ctx.lineTo(PADDING + (BOARD_SIZE - 1) * CELL_SIZE, PADDING + i * CELL_SIZE);
        ctx.stroke();

        // Vertical lines
        ctx.beginPath();
        ctx.moveTo(PADDING + i * CELL_SIZE, PADDING);
        ctx.lineTo(PADDING + i * CELL_SIZE, PADDING + (BOARD_SIZE - 1) * CELL_SIZE);
        ctx.stroke();
    }

    // Draw star points (Tian Yuan and others)
    const starPoints = [3, 7, 11];
    ctx.fillStyle = '#000';
    for (let i of starPoints) {
        for (let j of starPoints) {
            ctx.beginPath();
            ctx.arc(PADDING + i * CELL_SIZE, PADDING + j * CELL_SIZE, 4, 0, Math.PI * 2);
            ctx.fill();
        }
    }
}

// Draw a piece
function drawPiece(x, y, player) {
    ctx.beginPath();
    const cx = PADDING + x * CELL_SIZE;
    const cy = PADDING + y * CELL_SIZE;

    // Shadow
    ctx.shadowColor = 'rgba(0, 0, 0, 0.5)';
    ctx.shadowBlur = 4;
    ctx.shadowOffsetX = 2;
    ctx.shadowOffsetY = 2;

    ctx.arc(cx, cy, CELL_SIZE / 2 - 2, 0, Math.PI * 2);

    // Gradient for 3D effect
    const gradient = ctx.createRadialGradient(cx - 5, cy - 5, 2, cx, cy, CELL_SIZE / 2 - 2);
    if (player === 1) { // Black
        gradient.addColorStop(0, '#666');
        gradient.addColorStop(1, '#000');
    } else { // White
        gradient.addColorStop(0, '#fff');
        gradient.addColorStop(1, '#ddd');
    }

    ctx.fillStyle = gradient;
    ctx.fill();

    // Reset shadow
    ctx.shadowColor = 'transparent';
    ctx.shadowBlur = 0;
    ctx.shadowOffsetX = 0;
    ctx.shadowOffsetY = 0;

    // Mark the last move
    if (moveHistory.length > 0) {
        const lastMove = moveHistory[moveHistory.length - 1];
        if (lastMove.x === x && lastMove.y === y) {
            ctx.beginPath();
            ctx.strokeStyle = player === 1 ? '#fff' : '#000';
            ctx.lineWidth = 2;
            ctx.moveTo(cx - 5, cy);
            ctx.lineTo(cx + 5, cy);
            ctx.moveTo(cx, cy - 5);
            ctx.lineTo(cx, cy + 5);
            ctx.stroke();
        }
    }
}

// Redraw everything
function render() {
    drawBoard();
    for (let i = 0; i < BOARD_SIZE; i++) {
        for (let j = 0; j < BOARD_SIZE; j++) {
            if (board[i][j] !== 0) {
                drawPiece(i, j, board[i][j]);
            }
        }
    }
}

// Start game
function startGame() {
    initBoard();
    gameActive = true;
    gameMode = modeSelect.value;
    currentPlayer = 1; // Black always starts

    render();

    if (gameMode === 'evp') {
        // AI starts (Black)
        statusDisplay.textContent = '电脑思考中...';
        makeAIMove();
    } else {
        statusDisplay.textContent = '黑方执子';
    }
}

// Handle click
canvas.addEventListener('click', (e) => {
    if (!gameActive) return;

    // If it's AI's turn in PvE, ignore click
    if ((gameMode === 'pve' && currentPlayer === 2) ||
        (gameMode === 'evp' && currentPlayer === 1)) {
        return;
    }

    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;

    // Convert to grid coordinates
    // Use Math.round to find the nearest intersection
    const i = Math.round((x - PADDING) / CELL_SIZE);
    const j = Math.round((y - PADDING) / CELL_SIZE);

    if (i >= 0 && i < BOARD_SIZE && j >= 0 && j < BOARD_SIZE) {
        if (board[i][j] === 0) {
            makeMove(i, j);
        }
    }
});

// Make a move
function makeMove(x, y) {
    board[x][y] = currentPlayer;
    moveHistory.push({ x, y, player: currentPlayer });
    render();

    if (checkWin(x, y, currentPlayer)) {
        statusDisplay.textContent = (currentPlayer === 1 ? '黑方' : '白方') + '获胜!';
        gameActive = false;
        return;
    }

    // Switch player
    currentPlayer = currentPlayer === 1 ? 2 : 1;

    // Update status
    if (gameMode === 'pvp') {
        statusDisplay.textContent = (currentPlayer === 1 ? '黑方' : '白方') + '执子';
    } else {
        // Check if it's AI's turn
        if ((gameMode === 'pve' && currentPlayer === 2) ||
            (gameMode === 'evp' && currentPlayer === 1)) {
            statusDisplay.textContent = '电脑思考中...';
            setTimeout(makeAIMove, 500); // Small delay for better UX
        } else {
            statusDisplay.textContent = '请下子';
        }
    }
}

// AI Move
function makeAIMove() {
    if (!gameActive) return;

    fetch('/api/gomoku/ai-move', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            board: board,
            playerRole: currentPlayer,
            level: 1
        })
    })
        .then(response => response.json())
        .then(data => {
            if (data.x !== undefined && data.y !== undefined) {
                makeMove(data.x, data.y);
            } else {
                console.error('Invalid AI move');
            }
        })
        .catch(error => {
            console.error('Error:', error);
            statusDisplay.textContent = 'AI出错了';
        });
}

// Undo move
function undoMove() {
    if (!gameActive && moveHistory.length === 0) return;

    // In PvE, undo 2 steps (player + AI)
    // In PvP, undo 1 step

    let stepsToUndo = 1;
    if (gameMode !== 'pvp') {
        stepsToUndo = 2;
    }

    // If only 1 move made in PvE (e.g. AI started), undo 1
    if (moveHistory.length < stepsToUndo) {
        stepsToUndo = moveHistory.length;
    }

    for (let k = 0; k < stepsToUndo; k++) {
        const lastMove = moveHistory.pop();
        if (lastMove) {
            board[lastMove.x][lastMove.y] = 0;
            // Switch player back
            currentPlayer = lastMove.player;
        }
    }

    gameActive = true;
    render();

    if (gameMode === 'pvp') {
        statusDisplay.textContent = (currentPlayer === 1 ? '黑方' : '白方') + '执子';
    } else {
        statusDisplay.textContent = '请下子';
    }
}

// Check win
function checkWin(x, y, player) {
    const directions = [
        [1, 0],  // Horizontal
        [0, 1],  // Vertical
        [1, 1],  // Diagonal \
        [1, -1]  // Diagonal /
    ];

    for (let [dx, dy] of directions) {
        let count = 1;

        // Check forward
        let i = 1;
        while (true) {
            const nx = x + dx * i;
            const ny = y + dy * i;
            if (nx < 0 || nx >= BOARD_SIZE || ny < 0 || ny >= BOARD_SIZE || board[nx][ny] !== player) break;
            count++;
            i++;
        }

        // Check backward
        i = 1;
        while (true) {
            const nx = x - dx * i;
            const ny = y - dy * i;
            if (nx < 0 || nx >= BOARD_SIZE || ny < 0 || ny >= BOARD_SIZE || board[nx][ny] !== player) break;
            count++;
            i++;
        }

        if (count >= 5) return true;
    }
    return false;
}

// Initial draw
drawBoard();
