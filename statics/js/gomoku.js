const canvas = document.getElementById('board');
const ctx = canvas.getContext('2d');
const statusDisplay = document.getElementById('status');
const modeSelect = document.getElementById('game-mode');
const player1NameDisplay = document.getElementById('player1-name');
const player2NameDisplay = document.getElementById('player2-name');
const currentPlayerDisplay = document.getElementById('current-player-display');
const pvpInfo = document.getElementById('pvp-info');
const pvpStatus = document.getElementById('pvp-status');
const gameIdDisplay = document.getElementById('game-id');
const playerNumberDisplay = document.getElementById('player-number');
const roomConfig = document.getElementById('room-config');
const roomCreation = document.getElementById('room-creation');
const roomListContainer = document.getElementById('room-list-container');

const BOARD_SIZE = 15;
const CELL_SIZE = 40;
const PADDING = 20;

let board = [];
let currentPlayer = 1; // 1: Black, 2: White
let gameActive = false;
let gameMode = 'pve'; // pve, evp, pvp
let moveHistory = [];

// Room state
let playerNumber = 0; // 0: not assigned, 1: player1 (black), 2: player2 (white)
let currentGameId = '';
let roomPollInterval = null;

// Room list management
let selectedRoomId = '';
let selectedRoomInfo = null;

// Initialize event listeners
function initEventListeners() {
    modeSelect.addEventListener('change', function() {
        const isPvP = this.value === 'pvp';
        roomConfig.style.display = isPvP ? 'flex' : 'none';
        roomCreation.style.display = isPvP ? 'flex' : 'none';
        roomListContainer.style.display = isPvP ? 'block' : 'none';
        pvpInfo.style.display = isPvP ? 'block' : 'none';

        if (!isPvP) {
            stopRoomPolling();
            // Reset room state
            playerNumber = 0;
            currentGameId = '';
            pvpStatus.textContent = '等待玩家2加入...';
            gameIdDisplay.textContent = '-';
            playerNumberDisplay.textContent = '-';
            player1NameDisplay.textContent = '-';
            player2NameDisplay.textContent = '-';
            currentPlayerDisplay.textContent = '-';
        } else {
            // Load room list
            refreshRoomList();
        }
    });

    // Password modal event listeners
    document.getElementById('password-modal').addEventListener('click', function(e) {
        if (e.target === this) {
            closePasswordModal();
        }
    });

    document.getElementById('modal-password-input').addEventListener('keypress', function(e) {
        if (e.key === 'Enter') {
            joinRoomWithPassword();
        }
    });
}

// Create a new room
async function createRoom() {
    const roomName = document.getElementById('room-name').value.trim();
    const password = document.getElementById('room-password').value.trim();

    // Use a default creator name
    const creator = '玩家' + Math.floor(Math.random() * 1000);

    const response = await fetch('/api/gomoku/room/create', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            password: password,
            creator: creator,
            roomName: roomName
        })
    });

    if (!response.ok) {
        alert('创建房间失败');
        return;
    }

    const data = await response.json();
    currentGameId = data.gameId;
    playerNumber = data.player; // Should be 1 for creator
    // We don't have gameState yet, need to wait for opponent
    gameIdDisplay.textContent = currentGameId;
    playerNumberDisplay.textContent = playerNumber;
    pvpStatus.textContent = '等待玩家2加入...';

    // Start polling for room state
    startRoomPolling();

    // Refresh room list to update status
    refreshRoomList();

    statusDisplay.textContent = '房间已创建，等待对手加入...';
}

// Join room using input field
async function joinRoom() {
    const gameId = document.getElementById('join-room-id').value.trim();
    if (!gameId) {
        alert('请输入游戏ID');
        return;
    }
    await joinRoomById(gameId, '');
}

// Join a room with optional password
async function joinRoomById(gameId, password) {
    const response = await fetch('/api/gomoku/room/join', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            gameId: gameId,
            player: 2,
            password: password,
            playerName: '' // optional
        })
    });

    if (!response.ok) {
        const errorText = await response.text();
        if (response.status === 401) {
            alert('密码错误');
        } else {
            alert('加入房间失败: ' + errorText);
        }
        return;
    }

    const data = await response.json();
    if (!data.success) {
        alert(data.message);
        return;
    }

    currentGameId = gameId;
    playerNumber = data.player; // Should be 2 for joiner

    // Ensure board is initialized
    if (!board || board.length === 0) {
        initBoard();
    }

    // Update local board with room board
    if (data.board) {
        // board is 15x15 array
        for (let i = 0; i < BOARD_SIZE; i++) {
            for (let j = 0; j < BOARD_SIZE; j++) {
                board[i][j] = data.board[i][j] || 0;
            }
        }
    }
    gameActive = data.gameActive;

    gameIdDisplay.textContent = currentGameId;
    playerNumberDisplay.textContent = playerNumber;

    // Set game mode to pvp (online)
    modeSelect.value = 'pvp';
    // Trigger UI update
    modeSelect.dispatchEvent(new Event('change'));

    // Update board display
    render();

    // Start polling for game state
    startRoomPolling();

    statusDisplay.textContent = gameActive ? '游戏开始，黑方执子' : '等待另一位玩家...';

    // Refresh room list to update status
    refreshRoomList();
}

// Refresh room list
async function refreshRoomList() {
    const response = await fetch('/api/gomoku/room/list', {
        method: 'GET'
    });

    if (!response.ok) {
        console.error('Failed to fetch room list');
        return;
    }

    const data = await response.json();
    renderRoomList(data.rooms || []);
}

// Render room list
function renderRoomList(rooms) {
    const roomListContainer = document.getElementById('room-list');
    if (!roomListContainer) return;

    if (rooms.length === 0) {
        roomListContainer.innerHTML = `
            <div style="text-align: center; color: #999; padding: 20px;">
                暂无可用房间，点击"刷新"按钮获取房间列表
            </div>
        `;
        return;
    }

    let html = '';
    rooms.forEach(room => {
        const lockIcon = room.hasPassword ? '<span class="room-lock">🔒</span>' : '';
        const roomDisplayName = room.roomName ? room.roomName : `房间 ${room.gameId.substring(0, 8)}`;
        const createdAt = new Date(room.createdAt * 1000).toLocaleTimeString();
        const playerStatus = room.player1Ready ? (room.player2Ready ? '已满员' : '等待玩家2') : '等待玩家1';

        html += `
            <div class="room-item" data-game-id="${room.gameId}">
                <div class="room-info">
                    <div class="room-name">
                        ${roomDisplayName} ${lockIcon}
                    </div>
                    <div class="room-details">
                        创建者: ${room.creator} | 状态: ${playerStatus} | 创建时间: ${createdAt}<br>
                        玩家1: ${room.player1Name || '等待'} | 玩家2: ${room.player2Name || '等待'}
                    </div>
                </div>
                <div class="room-actions">
                    <button class="btn btn-secondary" onclick="attemptJoinRoom('${room.gameId}', ${room.hasPassword})"
                            style="padding: 6px 12px; font-size: 14px;" ${room.player1Ready && room.player2Ready ? 'disabled' : ''}>
                        ${room.player1Ready && room.player2Ready ? '已满员' : '加入'}
                    </button>
                </div>
            </div>
        `;
    });

    roomListContainer.innerHTML = html;
}

// Attempt to join a room (check if password needed)
function attemptJoinRoom(gameId, hasPassword) {
    // Convert string 'true'/'false' to boolean
    const needsPassword = hasPassword === true || hasPassword === 'true';

    // Find room info for display
    const roomItems = document.querySelectorAll('.room-item');
    let roomInfo = null;
    roomItems.forEach(item => {
        if (item.dataset.gameId === gameId) {
            const roomNameEl = item.querySelector('.room-name');
            const roomDetailsEl = item.querySelector('.room-details');
            if (roomNameEl && roomDetailsEl) {
                const roomName = roomNameEl.textContent.replace('🔒', '').trim();
                const detailsText = roomDetailsEl.textContent;
                const creatorMatch = detailsText.match(/创建者:\s*([^|]+)/);
                const creator = creatorMatch ? creatorMatch[1].trim() : '未知';
                roomInfo = { roomName, creator };
            }
        }
    });

    if (needsPassword) {
        // Show password modal
        if (roomInfo) {
            openPasswordModal(gameId, roomInfo.roomName, roomInfo.creator);
        } else {
            // Fallback if room info not found
            openPasswordModal(gameId, '房间 ' + gameId.substring(0, 8), '未知');
        }
    } else {
        // Join directly without password
        joinRoomById(gameId, '');
    }
}

// Open password modal for a specific room
function openPasswordModal(gameId, roomName, creator) {
    selectedRoomId = gameId;
    selectedRoomInfo = { roomName, creator };
    document.getElementById('modal-room-name').textContent = roomName;
    document.getElementById('modal-room-creator').textContent = creator;
    document.getElementById('modal-password-input').value = '';
    document.getElementById('password-modal').style.display = 'flex';
}

// Close password modal
function closePasswordModal() {
    document.getElementById('password-modal').style.display = 'none';
    selectedRoomId = '';
    selectedRoomInfo = null;
}

// Join room with password from modal
async function joinRoomWithPassword() {
    const password = document.getElementById('modal-password-input').value.trim();
    if (!selectedRoomId) {
        alert('未选择房间');
        return;
    }

    closePasswordModal();
    await joinRoomById(selectedRoomId, password);
}

// Start polling for room/game state
function startRoomPolling() {
    if (roomPollInterval) {
        clearInterval(roomPollInterval);
    }

    roomPollInterval = setInterval(async () => {
        if (!currentGameId || !playerNumber) return;

        const response = await fetch('/api/gomoku/room/state', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                gameId: currentGameId,
                player: playerNumber
            })
        });

        if (!response.ok) return;

        const data = await response.json();

        // Update player names
        player1NameDisplay.textContent = data.player1Name || '等待';
        player2NameDisplay.textContent = data.player2Name || '等待';

        // Update board if changed
        if (JSON.stringify(board) !== JSON.stringify(data.board)) {
            // Ensure board is initialized
            if (!board || board.length === 0) {
                initBoard();
            }
            // Update local board
            for (let i = 0; i < BOARD_SIZE; i++) {
                for (let j = 0; j < BOARD_SIZE; j++) {
                    board[i][j] = data.board[i][j] || 0;
                }
            }
            render();
        }

        // Update current player
        currentPlayer = data.currentPlayer;
        currentPlayerDisplay.textContent = currentPlayer === 1 ? '黑方' : '白方';

        // Update game active status
        gameActive = data.gameActive;
        if (!gameActive) {
            if (data.winner === 0) {
                // Game not started yet (waiting)
                pvpStatus.textContent = '等待另一位玩家...';
            } else if (data.winner === playerNumber) {
                pvpStatus.textContent = '你赢了！';
                statusDisplay.textContent = '你赢了！';
                stopRoomPolling();
            } else {
                pvpStatus.textContent = '对手赢了！';
                statusDisplay.textContent = '对手赢了！';
                stopRoomPolling();
            }
        } else {
            pvpStatus.textContent = data.yourTurn ? '轮到你了' : '等待对手';
            statusDisplay.textContent = data.yourTurn ? (currentPlayer === 1 ? '黑方执子' : '白方执子') : '等待对手下子';
        }

        // Update status display
        if (!data.yourTurn && gameActive) {
            statusDisplay.textContent = '等待对手下子';
        }
    }, 2000); // Poll every 2 seconds
}

// Stop room polling
function stopRoomPolling() {
    if (roomPollInterval) {
        clearInterval(roomPollInterval);
        roomPollInterval = null;
    }
}

// Send move to server
async function sendMoveToServer(x, y) {
    if (!currentGameId || !playerNumber) return false;

    const response = await fetch('/api/gomoku/room/move', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            gameId: currentGameId,
            player: playerNumber,
            x: x,
            y: y
        })
    });

    if (!response.ok) {
        const error = await response.text();
        console.error('Move failed:', error);
        alert('落子失败: ' + error);
        return false;
    }

    const data = await response.json();
    if (!data.success) {
        alert(data.message);
        return false;
    }

    // Update board from response
    if (data.board) {
        // Ensure board is initialized
        if (!board || board.length === 0) {
            initBoard();
        }
        for (let i = 0; i < BOARD_SIZE; i++) {
            for (let j = 0; j < BOARD_SIZE; j++) {
                board[i][j] = data.board[i][j] || 0;
            }
        }
    }

    // Update game state
    gameActive = data.gameActive;
    currentPlayer = data.currentPlayer;

    // Check for win
    if (data.winner !== 0) {
        // Game over
        if (data.winner === playerNumber) {
            statusDisplay.textContent = '你赢了！';
            pvpStatus.textContent = '你赢了！';
        } else {
            statusDisplay.textContent = '对手赢了！';
            pvpStatus.textContent = '对手赢了！';
        }
        gameActive = false;
        stopRoomPolling();
    }

    render();
    return true;
}

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
    // Ensure board is initialized before accessing
    if (!board || board.length === 0) {
        return; // Just draw empty board
    }
    for (let i = 0; i < BOARD_SIZE; i++) {
        // Check if row exists
        if (!board[i]) continue;
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
canvas.addEventListener('click', async (e) => {
    if (!gameActive) return;

    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;

    // Convert to grid coordinates
    // Use Math.round to find the nearest intersection
    const i = Math.round((x - PADDING) / CELL_SIZE);
    const j = Math.round((y - PADDING) / CELL_SIZE);

    if (i < 0 || i >= BOARD_SIZE || j < 0 || j >= BOARD_SIZE) return;
    if (board[i][j] !== 0) return; // Cell occupied

    // Determine game mode
    const mode = modeSelect.value;

    if (mode === 'pvp') {
        // Online PvP mode
        if (playerNumber === 0) {
            // Local PvP (fallback)
            makeMove(i, j);
            return;
        }
        // Check if it's player's turn
        if (playerNumber !== currentPlayer) {
            alert('请等待对手的回合');
            return;
        }
        // Send move to server
        await sendMoveToServer(i, j);
    } else {
        // PvE or evp mode
        // If it's AI's turn, ignore click
        if ((mode === 'pve' && currentPlayer === 2) ||
            (mode === 'evp' && currentPlayer === 1)) {
            return;
        }
        makeMove(i, j);
    }
});

// Make a move
function makeMove(x, y) {
    // Ensure board is initialized
    if (!board || board.length === 0) {
        initBoard();
    }
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

    // Ensure board is initialized
    if (!board || board.length === 0) {
        initBoard();
    }

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
            // Ensure row exists before accessing
            if (board[lastMove.x]) {
                board[lastMove.x][lastMove.y] = 0;
            }
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
    // Ensure board is initialized
    if (!board || board.length === 0) {
        return false;
    }

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
            if (nx < 0 || nx >= BOARD_SIZE || ny < 0 || ny >= BOARD_SIZE || !board[nx] || board[nx][ny] !== player) break;
            count++;
            i++;
        }

        // Check backward
        i = 1;
        while (true) {
            const nx = x - dx * i;
            const ny = y - dy * i;
            if (nx < 0 || nx >= BOARD_SIZE || ny < 0 || ny >= BOARD_SIZE || !board[nx] || board[nx][ny] !== player) break;
            count++;
            i++;
        }

        if (count >= 5) return true;
    }
    return false;
}

// Initial draw
drawBoard();

// Initialize event listeners
initEventListeners();
