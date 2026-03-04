const canvas = document.getElementById('board');
const ctx = canvas.getContext('2d');
const statusDisplay = document.getElementById('status');
const modeSelect = document.getElementById('game-mode');
const boardSizeSelect = document.getElementById('board-size');
const iconTypesSelect = document.getElementById('icon-types');
const aiDifficultySelect = document.getElementById('ai-difficulty');
const player1ScoreDisplay = document.getElementById('player1-score');
const player2ScoreDisplay = document.getElementById('player2-score');
const remainingPairsDisplay = document.getElementById('remaining-pairs');
const currentPlayerDisplay = document.getElementById('current-player');
const pvpInfo = document.getElementById('pvp-info');
const pvpStatus = document.getElementById('pvp-status');
const gameIdDisplay = document.getElementById('game-id');
const playerNumberDisplay = document.getElementById('player-number');
const opponentScoreDisplay = document.getElementById('opponent-score');
const opponentPairsDisplay = document.getElementById('opponent-pairs');

// Game state
let gameState = null;
let gameActive = false;
let playerNumber = 0; // 0: not assigned, 1: player1, 2: player2
let currentGameId = '';
let pvpPollInterval = null;

// Board configuration
let rows = 8;
let cols = 10;
let iconTypes = 8;
let cellSize = 40;

// Icon colors (for visualization)
const iconColors = [
    '#FF6B6B', '#4ECDC4', '#FFD166', '#06D6A0', '#118AB2', '#EF476F',
    '#073B4C', '#7209B7', '#3A86FF', '#FB5607', '#8338EC', '#FF006E',
    '#FF9E00', '#FFBE0B', '#3A86FF', '#FB5607', '#8338EC', '#FF006E'
];

// Initialize event listeners
function initEventListeners() {
    // Board size selector
    boardSizeSelect.addEventListener('change', function() {
        if (this.value === 'custom') {
            document.getElementById('custom-size').style.display = 'flex';
        } else {
            document.getElementById('custom-size').style.display = 'none';
            const [r, c] = this.value.split('x').map(Number);
            rows = r;
            cols = c;
        }
    });

    // Game mode selector
    modeSelect.addEventListener('change', function() {
        const isRace = this.value === 'race';
        document.getElementById('race-config').style.display = isRace ? 'flex' : 'none';
        document.getElementById('race-room-creation').style.display = isRace ? 'flex' : 'none';
        document.getElementById('race-room-list-container').style.display = isRace ? 'block' : 'none';
        document.getElementById('ai-difficulty-row').style.display = isRace ? 'none' : 'flex';
        pvpInfo.style.display = isRace ? 'block' : 'none';

        if (!isRace) {
            stopPvPPolling();
        }

        // Update info display for the new mode
        updateGameInfo();

        // If race mode is selected, load room list
        if (isRace) {
            refreshRoomList();
        }
    });

    // Custom size inputs
    document.getElementById('rows').addEventListener('change', function() {
        rows = parseInt(this.value) || 8;
    });
    document.getElementById('cols').addEventListener('change', function() {
        cols = parseInt(this.value) || 10;
    });

    // Icon types selector
    iconTypesSelect.addEventListener('change', function() {
        iconTypes = parseInt(this.value) || 8;
    });
}

// Start a new game
async function startGame() {
    const gameMode = modeSelect.value;

    if (gameMode === 'race') {
        alert('竞速对战请使用"创建竞速房间"或"加入竞速"功能');
        return;
    }

    // Start PvE game (人机对战)
    const boardConfig = getBoardConfig();

    const response = await fetch('/api/linkup/new-game', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            rows: boardConfig.rows,
            cols: boardConfig.cols,
            gameMode: gameMode,
            icons: boardConfig.iconTypes
        })
    });

    if (!response.ok) {
        alert('创建游戏失败');
        return;
    }

    const data = await response.json();
    gameState = data.gameState;
    currentGameId = data.gameId;
    gameActive = true;
    playerNumber = 1; // Player is always player 1 in PvE

    updateGameInfo();
    renderBoard();

    // Player always starts in PvE mode
    statusDisplay.textContent = '请选择第一个图标';
}

// Get board configuration from UI
function getBoardConfig() {
    if (boardSizeSelect.value === 'custom') {
        const customRows = parseInt(document.getElementById('rows').value) || 8;
        const customCols = parseInt(document.getElementById('cols').value) || 10;
        return {
            rows: customRows,
            cols: customCols,
            iconTypes: parseInt(iconTypesSelect.value) || 8
        };
    } else {
        const [r, c] = boardSizeSelect.value.split('x').map(Number);
        return {
            rows: r,
            cols: c,
            iconTypes: parseInt(iconTypesSelect.value) || 8
        };
    }
}

// Update game information display
function updateGameInfo() {
    // Control visibility of info items based on game mode
    const player2ScoreItem = document.getElementById('player2-score-item');
    const currentPlayerItem = document.getElementById('current-player-item');
    if (player2ScoreItem && currentPlayerItem) {
        const currentMode = modeSelect.value; // Use current selected mode
        if (currentMode === 'race') {
            // Hide player2 score and current player in race mode
            player2ScoreItem.style.display = 'none';
            currentPlayerItem.style.display = 'none';
        } else {
            // Show in other modes
            player2ScoreItem.style.display = 'flex';
            currentPlayerItem.style.display = 'flex';
        }
    }

    if (!gameState) return;

    const gameMode = gameState.gameMode || modeSelect.value;

    if (gameMode === 'race') {
        // Race mode: player's score is in player1Score, opponent's score is shown separately
        player1ScoreDisplay.textContent = gameState.player1Score;
        player2ScoreDisplay.textContent = ''; // Not used in race mode
        remainingPairsDisplay.textContent = gameState.remainingPairs;
        currentPlayerDisplay.textContent = ''; // Race mode doesn't use current player concept
    } else {
        // PvE mode
        player1ScoreDisplay.textContent = gameState.player1Score;
        player2ScoreDisplay.textContent = gameState.player2Score;
        remainingPairsDisplay.textContent = gameState.remainingPairs;
        currentPlayerDisplay.textContent = gameState.currentPlayer === 1 ? '玩家' : '电脑';
    }
}

// Render the game board
function renderBoard() {
    if (!gameState) return;

    // Use board dimensions from gameState
    const boardRows = gameState.rows || 8;
    const boardCols = gameState.cols || 10;

    // Update global variables for consistency
    rows = boardRows;
    cols = boardCols;

    // Check if board exists
    if (!gameState.board || !Array.isArray(gameState.board) || gameState.board.length === 0) {
        console.error('Game board is missing or invalid:', gameState.board);
        console.error('Full gameState:', JSON.stringify(gameState, null, 2));
        // Draw empty board
        ctx.clearRect(0, 0, canvas.width, canvas.height);
        ctx.fillStyle = '#f0f0f0';
        ctx.fillRect(0, 0, canvas.width, canvas.height);
        ctx.fillStyle = '#333';
        ctx.font = '20px Arial';
        ctx.textAlign = 'center';
        ctx.fillText('游戏棋盘加载中...', canvas.width / 2, canvas.height / 2);
        return;
    }

    // Calculate cell size based on board dimensions
    const maxWidth = canvas.width - 40;
    const maxHeight = canvas.height - 40;
    cellSize = Math.min(
        maxWidth / boardCols,
        maxHeight / boardRows,
        60 // Maximum cell size
    );

    const boardWidth = boardCols * cellSize;
    const boardHeight = boardRows * cellSize;
    const offsetX = (canvas.width - boardWidth) / 2;
    const offsetY = (canvas.height - boardHeight) / 2;

    // Clear canvas
    ctx.clearRect(0, 0, canvas.width, canvas.height);

    // Draw background
    ctx.fillStyle = '#f0f0f0';
    ctx.fillRect(offsetX - 10, offsetY - 10, boardWidth + 20, boardHeight + 20);

    // Draw grid
    ctx.strokeStyle = '#ccc';
    ctx.lineWidth = 1;

    for (let r = 0; r <= boardRows; r++) {
        ctx.beginPath();
        ctx.moveTo(offsetX, offsetY + r * cellSize);
        ctx.lineTo(offsetX + boardWidth, offsetY + r * cellSize);
        ctx.stroke();
    }

    for (let c = 0; c <= boardCols; c++) {
        ctx.beginPath();
        ctx.moveTo(offsetX + c * cellSize, offsetY);
        ctx.lineTo(offsetX + c * cellSize, offsetY + boardHeight);
        ctx.stroke();
    }

    // Draw icons
    for (let r = 0; r < boardRows; r++) {
        // Check if row exists
        if (!gameState.board[r] || !Array.isArray(gameState.board[r])) {
            continue;
        }
        for (let c = 0; c < boardCols; c++) {
            const iconType = gameState.board[r][c];
            if (iconType > 0) {
                drawIcon(r, c, iconType, offsetX, offsetY);
            }
        }
    }

    // Draw selected cell highlight
    if (gameState.selectedCell) {
        const { row, col } = gameState.selectedCell;
        ctx.strokeStyle = '#FF0000';
        ctx.lineWidth = 3;
        ctx.strokeRect(
            offsetX + col * cellSize + 2,
            offsetY + row * cellSize + 2,
            cellSize - 4,
            cellSize - 4
        );
    }
}

// Draw an icon
function drawIcon(row, col, iconType, offsetX, offsetY) {
    const x = offsetX + col * cellSize + cellSize / 2;
    const y = offsetY + row * cellSize + cellSize / 2;
    const radius = cellSize / 2 - 6;

    // Color based on icon type
    const colorIndex = (iconType - 1) % iconColors.length;
    const color = iconColors[colorIndex];

    // Draw icon background
    ctx.beginPath();
    ctx.arc(x, y, radius, 0, Math.PI * 2);
    ctx.fillStyle = color;
    ctx.fill();

    // Draw border
    ctx.strokeStyle = '#333';
    ctx.lineWidth = 2;
    ctx.stroke();

    // Draw icon number (temporary - in real game would use images)
    ctx.fillStyle = '#FFF';
    ctx.font = `${Math.floor(radius)}px Arial`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(iconType.toString(), x, y);
}

// Handle canvas click
canvas.addEventListener('click', async (e) => {
    if (!gameState || !gameState.gameActive) return;

    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;

    // Calculate cell size and offset
    const boardWidth = cols * cellSize;
    const boardHeight = rows * cellSize;
    const offsetX = (canvas.width - boardWidth) / 2;
    const offsetY = (canvas.height - boardHeight) / 2;

    // Check if click is within board
    if (x < offsetX || x > offsetX + boardWidth ||
        y < offsetY || y > offsetY + boardHeight) {
        return;
    }

    // Calculate grid coordinates
    const col = Math.floor((x - offsetX) / cellSize);
    const row = Math.floor((y - offsetY) / cellSize);

    if (row >= 0 && row < rows && col >= 0 && col < cols) {
        await selectCell(row, col);
    }
});

// Select a cell
async function selectCell(row, col) {
    if (!gameState || !gameState.gameActive) return;

    console.log('selectCell:', { row, col, playerNumber, gameState: {
        currentPlayer: gameState.currentPlayer,
        gameMode: gameState.gameMode,
        gameActive: gameState.gameActive,
        selectedCell: gameState.selectedCell
    }});

    // Check game mode (prefer gameState.gameMode, fallback to modeSelect.value)
    const gameMode = gameState.gameMode || modeSelect.value;

    // In PvE, ignore click when it's AI's turn
    if (gameMode === 'pve' && gameState.currentPlayer === 2) {
        return;
    }

    const response = await fetch('/api/linkup/select', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            row: row,
            col: col,
            gameId: currentGameId,
            player: playerNumber
        })
    });

    if (!response.ok) {
        const error = await response.text();
        console.error('Select cell error:', error);
        if (error.includes('Not your turn')) {
            alert('请等待对手的回合');
        } else if (error.includes('Cell is empty')) {
            alert('此位置没有图标');
        } else if (error.includes('Invalid cell')) {
            alert('无效的位置');
        } else {
            alert('操作失败: ' + error);
        }
        return;
    }

    const data = await response.json();
    console.log('selectCell response:', data);
    gameState = data.gameState;
    console.log('Updated gameState:', gameState);
    updateGameInfo();
    renderBoard();

    if (data.gameOver) {
        gameActive = false;
        const gameMode = gameState.gameMode || modeSelect.value;

        if (gameMode === 'race') {
            statusDisplay.textContent = '你已完成！等待最终结果...';
            alert('你已完成！等待对手完成，最终结果将通过轮询显示。');
        } else {
            // PvE mode
            statusDisplay.textContent = '游戏结束！';
            alert(`游戏结束！你的得分: ${gameState.player1Score}`);
        }
        return;
    }

    if (data.matched) {
        // Show match animation
        highlightMatch(data.matchCells);

        if (modeSelect.value === 'pve' && gameState.currentPlayer === 2) {
            statusDisplay.textContent = '电脑思考中...';
            setTimeout(makeAIMove, 1000);
        } else {
            statusDisplay.textContent = '匹配成功！请选择下一个图标';
        }
    } else {
        statusDisplay.textContent = gameState.selectedCell ?
            '已选择第一个图标，请选择第二个图标' :
            '请选择第一个图标';
    }
}

// Highlight matched cells
function highlightMatch(cells) {
    const boardWidth = cols * cellSize;
    const boardHeight = rows * cellSize;
    const offsetX = (canvas.width - boardWidth) / 2;
    const offsetY = (canvas.height - boardHeight) / 2;

    cells.forEach(cell => {
        const x = offsetX + cell.col * cellSize + cellSize / 2;
        const y = offsetY + cell.row * cellSize + cellSize / 2;

        // Draw highlight animation
        ctx.beginPath();
        ctx.arc(x, y, cellSize / 2, 0, Math.PI * 2);
        ctx.strokeStyle = '#FFD700';
        ctx.lineWidth = 4;
        ctx.stroke();
    });

    // Remove highlight after delay
    setTimeout(() => {
        renderBoard();
    }, 500);
}

// Make AI move
async function makeAIMove() {
    if (!gameState || !gameState.gameActive) return;

    const difficulty = parseInt(aiDifficultySelect.value) || 2;

    const response = await fetch('/api/linkup/ai-move', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            gameState: gameState,
            level: difficulty
        })
    });

    if (!response.ok) {
        const error = await response.text();
        console.error('AI move error:', error);
        statusDisplay.textContent = 'AI出错了';
        return;
    }

    const data = await response.json();
    gameState = data.gameState;
    updateGameInfo();
    renderBoard();

    if (data.gameState.gameActive) {
        statusDisplay.textContent = '电脑已行动，轮到你了';
    } else {
        gameActive = false;
        statusDisplay.textContent = '游戏结束！';
        alert(`游戏结束！电脑得分: ${gameState.player2Score}, 你的得分: ${gameState.player1Score}`);
    }
}

// Get a hint
async function getHint() {
    if (!gameState || !gameState.gameActive) return;

    const response = await fetch('/api/linkup/hint', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            gameId: currentGameId
        })
    });

    if (!response.ok) {
        alert('获取提示失败');
        return;
    }

    const data = await response.json();

    // Highlight the hinted cells
    const boardWidth = cols * cellSize;
    const boardHeight = rows * cellSize;
    const offsetX = (canvas.width - boardWidth) / 2;
    const offsetY = (canvas.height - boardHeight) / 2;

    [data.cell1, data.cell2].forEach(cell => {
        const x = offsetX + cell.col * cellSize + cellSize / 2;
        const y = offsetY + cell.row * cellSize + cellSize / 2;

        ctx.beginPath();
        ctx.arc(x, y, cellSize / 2, 0, Math.PI * 2);
        ctx.strokeStyle = '#00FF00';
        ctx.lineWidth = 4;
        ctx.stroke();
    });

    // Remove highlight after 2 seconds
    setTimeout(() => {
        renderBoard();
    }, 2000);

    statusDisplay.textContent = '已显示提示（绿色高亮）';
}

// Reset game
function resetGame() {
    gameState = null;
    gameActive = false;
    currentGameId = '';
    playerNumber = 0;
    stopPvPPolling();

    statusDisplay.textContent = '请选择游戏模式并开始游戏';
    player1ScoreDisplay.textContent = '0';
    player2ScoreDisplay.textContent = '0';
    remainingPairsDisplay.textContent = '0';
    currentPlayerDisplay.textContent = '-';

    // Clear canvas
    ctx.clearRect(0, 0, canvas.width, canvas.height);
}


// Start polling for PvP game state
function startPvPPolling() {
    if (pvpPollInterval) {
        clearInterval(pvpPollInterval);
    }

    pvpPollInterval = setInterval(async () => {
        if (!currentGameId || !playerNumber) return;

        // Determine game mode (prefer gameState.gameMode, fallback to modeSelect.value)
        const gameMode = (gameState && gameState.gameMode) || modeSelect.value;

        // Race mode polling (only mode now)
        const response = await fetch('/api/linkup/race/state', {
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

        // Update opponent info for race mode
        opponentScoreDisplay.textContent = data.opponentScore;
        opponentPairsDisplay.textContent = data.opponentRemainingPairs;

        // Update game state if changed
        if (JSON.stringify(gameState) !== JSON.stringify(data.yourGameState)) {
            gameState = data.yourGameState;
            gameActive = data.yourGameState.gameActive;
            updateGameInfo();
            renderBoard();

            // Update status for race mode
            if (!data.yourGameState.gameActive) {
                // Player has finished
                if (data.opponentFinished) {
                    // Both players finished
                    if (data.winner === 0) {
                        statusDisplay.textContent = '游戏结束，平局！';
                        alert('游戏结束，平局！');
                    } else if (data.winner === playerNumber) {
                        statusDisplay.textContent = '恭喜，你赢了！';
                        alert('恭喜，你赢了！');
                    } else {
                        statusDisplay.textContent = '对手赢了！';
                        alert('对手赢了！');
                    }
                    stopPvPPolling();
                } else {
                    // Player finished but opponent hasn't
                    statusDisplay.textContent = '你已完成！等待对手...';
                    // Don't stop polling - need to wait for opponent
                }
            } else {
                // Player hasn't finished yet
                if (data.opponentFinished) {
                    statusDisplay.textContent = '对手已完成，加油！';
                } else {
                    statusDisplay.textContent = '竞速进行中...';
                }
            }
        }

        // Update PvP status for race mode
        if (!data.yourGameState.gameActive) {
            // Player has finished
            if (data.opponentFinished) {
                // Both players finished
                if (data.winner === 0) {
                    pvpStatus.textContent = '游戏结束，平局！';
                } else if (data.winner === playerNumber) {
                    pvpStatus.textContent = '你赢了！';
                } else {
                    pvpStatus.textContent = '对手赢了！';
                }
            } else {
                // Player finished but opponent hasn't
                pvpStatus.textContent = '你已完成';
            }
        } else {
            // Player hasn't finished yet
            if (data.opponentFinished) {
                pvpStatus.textContent = '对手已完成';
            } else {
                pvpStatus.textContent = '竞速进行中';
            }
        }
    }, 2000); // Poll every 2 seconds
}

// Stop PvP polling
function stopPvPPolling() {
    if (pvpPollInterval) {
        clearInterval(pvpPollInterval);
        pvpPollInterval = null;
    }
}

// Race functions
async function createRaceGame() {
    const boardConfig = getBoardConfig();
    const roomName = document.getElementById('room-name').value.trim();
    const password = document.getElementById('room-password').value.trim();

    // Use a default creator name, could be enhanced with user system
    const creator = '玩家' + Math.floor(Math.random() * 1000);

    const response = await fetch('/api/linkup/race/create', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            rows: boardConfig.rows,
            cols: boardConfig.cols,
            icons: boardConfig.iconTypes,
            password: password,
            creator: creator,
            roomName: roomName
        })
    });

    if (!response.ok) {
        alert('创建竞速游戏失败');
        return;
    }

    const data = await response.json();
    currentGameId = data.gameId;
    gameState = data.gameState;
    playerNumber = data.player;
    gameActive = gameState.gameActive;

    gameIdDisplay.textContent = currentGameId;
    playerNumberDisplay.textContent = playerNumber;
    pvpStatus.textContent = '等待玩家2加入...';

    // Set game mode to race
    modeSelect.value = 'race';

    // Start polling for opponent
    startPvPPolling();

    // Refresh room list to update status
    refreshRoomList();

    statusDisplay.textContent = '竞速游戏已创建，等待对手加入...';
}

async function joinRaceGame() {
    const gameId = document.getElementById('join-race-id').value.trim();
    if (!gameId) {
        alert('请输入游戏ID');
        return;
    }
    await joinRaceRoom(gameId, '');
}

// Join a race room with optional password
async function joinRaceRoom(gameId, password) {
    const response = await fetch('/api/linkup/race/join', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            gameId: gameId,
            player: 2,
            password: password
        })
    });

    if (!response.ok) {
        const errorText = await response.text();
        if (response.status === 401) {
            alert('密码错误');
        } else {
            alert('加入游戏失败: ' + errorText);
        }
        return;
    }

    const data = await response.json();
    if (!data.success) {
        alert(data.message);
        return;
    }

    currentGameId = gameId;
    gameState = data.gameState;
    playerNumber = data.player;
    gameActive = gameState.gameActive;

    console.log('Joined race game:', data);
    console.log('Game state:', gameState);

    gameIdDisplay.textContent = currentGameId;
    playerNumberDisplay.textContent = playerNumber;

    // Set game mode to race
    modeSelect.value = 'race';

    // Update game state
    updateGameInfo();
    renderBoard();

    // Start polling for game state
    startPvPPolling();

    statusDisplay.textContent = gameState.gameActive ?
        '游戏开始，竞速开始！' :
        '等待另一位玩家...';

    // Refresh room list to update status
    refreshRoomList();
}

// Room list management
let selectedRoomId = '';
let selectedRoomInfo = null;

// Refresh room list
async function refreshRoomList() {
    const response = await fetch('/api/linkup/race/list', {
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
    const roomListContainer = document.getElementById('race-room-list');
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
        const boardSize = `${room.rows}×${room.cols}`;
        const iconCount = room.icons;
        const createdAt = new Date(room.createdAt * 1000).toLocaleTimeString();
        const playerStatus = room.player1Ready ? (room.player2Ready ? '已满员' : '等待玩家2') : '等待玩家1';

        html += `
            <div class="room-item" data-game-id="${room.gameId}">
                <div class="room-info">
                    <div class="room-name">
                        ${roomDisplayName} ${lockIcon}
                    </div>
                    <div class="room-details">
                        创建者: ${room.creator} | 棋盘: ${boardSize} | 图标: ${iconCount}种<br>
                        状态: ${playerStatus} | 创建时间: ${createdAt}
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
        joinRaceRoom(gameId, '');
    }
}

// Join room with password from modal
async function joinRoomWithPassword() {
    const password = document.getElementById('modal-password-input').value.trim();
    if (!selectedRoomId) {
        alert('未选择房间');
        return;
    }

    closePasswordModal();
    await joinRaceRoom(selectedRoomId, password);
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

// Initialize

// Initialize event listeners for password modal
document.addEventListener('DOMContentLoaded', function() {
    // Close modal when clicking outside
    document.getElementById('password-modal').addEventListener('click', function(e) {
        if (e.target === this) {
            closePasswordModal();
        }
    });

    // Handle Enter key in password input
    document.getElementById('modal-password-input').addEventListener('keypress', function(e) {
        if (e.key === 'Enter') {
            joinRoomWithPassword();
        }
    });
});

// Initialize
initEventListeners();
renderBoard();
updateGameInfo(); // Initialize display based on default mode