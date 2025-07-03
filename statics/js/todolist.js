        // 数据存储和状态管理
        // 使用模板中的日期初始化currentDate
        let currentDate = templateDate || new Date().toISOString().split('T')[0];
        let isReadOnly = false;
        let localTodos = {}; // 用于在只读模式下存储本地数据
        let timeRange = {
            start: "09:30",
            end: "18:30"
        };

        // 初始化日期选择器
        flatpickr("#datePicker", {
            dateFormat: "Y-m-d",
            defaultDate: currentDate,
            locale: "zh",
            onChange: function(selectedDates, dateStr, instance) {
                currentDate = dateStr;
                loadTodos(currentDate);
            },
            minDate: "2020-01-01",
            maxDate: "2030-12-31",
            disable: [
                function(date) {
                    // 禁用周末
                    return (date.getDay() === 0 || date.getDay() === 6);
                }
            ],
            // 设置工作日
            enableTime: false,
            time_24hr: true,
            // 设置工作时间
            start: "09:30",
            end: "18:30"
        });

        // 初始化
        const datePicker = document.getElementById('datePicker');
        datePicker.value = currentDate;
        
        // 检查是否有本地存储的待办事项
        if (localStorage.getItem('localTodos')) {
            try {
                localTodos = JSON.parse(localStorage.getItem('localTodos'));
            } catch (e) {
                console.error("Failed to parse local todos:", e);
                localTodos = {};
            }
        }

        // 检查是否有本地存储的时间范围
        if (localStorage.getItem('timeRange')) {
            try {
                timeRange = JSON.parse(localStorage.getItem('timeRange'));
                
                // 解析时间范围并设置选择器的值
                const [startHour, startMinute] = timeRange.start.split(':');
                const [endHour, endMinute] = timeRange.end.split(':');
                
                document.getElementById('startHour').value = startHour;
                document.getElementById('startMinute').value = startMinute;
                document.getElementById('endHour').value = endHour;
                document.getElementById('endMinute').value = endMinute;
            } catch (e) {
                console.error("Failed to parse time range:", e);
            }
        }

        // 加载待办事项
        loadTodos(currentDate);

        // 加载历史记录
        loadHistoricalTodos();

        // 事件监听器
        datePicker.addEventListener('change', (e) => {
            currentDate = e.target.value;
            loadTodos(currentDate);
        });

        document.getElementById('newTodo').addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                addTodo();
            }
        });

        // 页面加载动画
        document.addEventListener('DOMContentLoaded', function() {
            animateItems('.card', 50);
            
            // 设置默认时间值为0
            const hoursDisplay = document.getElementById('hours_display');
            const minutesDisplay = document.getElementById('minutes_display');
            const hoursValue = document.getElementById('hours_value');
            const minutesValue = document.getElementById('minutes_value');
            
            hoursDisplay.textContent = '0';
            minutesDisplay.textContent = '0';
            hoursValue.textContent = '0';
            minutesValue.textContent = '0';
        });

        // 对元素应用加载动画
        function animateItems(selector, delay) {
            const items = document.querySelectorAll(selector);
            items.forEach((item, index) => {
                item.style.opacity = '0';
                item.style.transform = 'translateY(20px)';
                setTimeout(() => {
                    item.style.transition = 'opacity 0.5s ease, transform 0.5s ease';
                    item.style.opacity = '1';
                    item.style.transform = 'translateY(0)';
                }, delay * index);
            });
        }

        // 使用 XMLHttpRequest 加载待办事项
        function loadTodos(date) {
            const url = `/api/todos?date=${date}`;
            
            const xhr = new XMLHttpRequest();
            xhr.open('GET', url, true);
            xhr.setRequestHeader('Accept', 'application/json');
            
            xhr.onreadystatechange = function() {
                if (xhr.readyState === 4) {
                    if (xhr.status === 200) {
                        try {
                            let response = JSON.parse(xhr.responseText);
                            let todos;
                            
                            // 处理响应格式
                            if (response.items && Array.isArray(response.items)) {
                                // 响应是完整的TodoList对象
                                todos = response.items;
                                
                                // 提取任务顺序
                                if (response.order && Array.isArray(response.order)) {
                                    todoOrder = response.order;
                                    console.log("从服务器加载任务顺序:", todoOrder);
                                    
                                    // 同时更新本地存储的顺序
                                    localStorage.setItem(`todoOrder_${currentDate}`, JSON.stringify(todoOrder));
                                } else {
                                    // 没有服务器顺序，尝试从本地存储加载
                                    loadOrderFromLocalStorage();
                                }
                            } else if (Array.isArray(response)) {
                                // 兼容旧API，响应是任务数组
                                todos = response;
                                // 尝试从本地存储加载顺序
                                loadOrderFromLocalStorage();
                            } else {
                                console.error("Unexpected response format:", response);
                                todos = [];
                                loadOrderFromLocalStorage();
                            }
                            
                            renderTodos(todos);
                            updateStatusIndicator(false);
                            
                            // 应用动画效果
                            setTimeout(() => {
                                animateItems('.todo-item', 50);
                            }, 100);
                            
                        } catch (parseError) {
                            console.error("Failed to parse todos:", parseError);
                            console.error("Invalid JSON:", xhr.responseText);
                            showToast('加载待办事项失败，解析错误', 'error');
                            renderTodos([]);
                        }
                    } else if (xhr.status === 401) {
                        // 未授权
                        window.location.href = "/index";
                    } else {
                        handleErrorResponse(xhr, '加载待办事项失败');
                        if (xhr.responseText && xhr.responseText.includes('read-only')) {
                            updateStatusIndicator(true);
                        }
                        renderTodos([]);
                    }
                }
            };
            
            xhr.send();
        }

        // 更新状态指示器
        function updateStatusIndicator(readOnly) {
            isReadOnly = readOnly;
            const indicator = document.getElementById('status-indicator');
            
            if (readOnly) {
                indicator.className = 'status-indicator read-only';
                indicator.textContent = '只读模式';
            } else {
                indicator.className = 'status-indicator read-write';
                indicator.textContent = '可编辑模式';
            }
        }

        // 使用 XMLHttpRequest 加载历史记录
        function loadHistoricalTodos() {
            // 计算日期范围：今天到30天前
            const endDate = new Date().toISOString().split('T')[0];
            const startDate = new Date();
            startDate.setDate(startDate.getDate() - 30); // 获取过去30天的历史记录
            const startDateStr = startDate.toISOString().split('T')[0];
            
            const url = `/api/todos/history?start_date=${startDateStr}&end_date=${endDate}`;
            console.log("加载历史记录:", url);
            
            const xhr = new XMLHttpRequest();
            xhr.open('GET', url, true);
            xhr.setRequestHeader('Accept', 'application/json');
            
            xhr.onreadystatechange = function() {
                if (xhr.readyState === 4) {
                    if (xhr.status === 200) {
                        try {
                            const responseText = xhr.responseText;
                            // 确保响应不为空
                            if (!responseText || responseText.trim() === '') {
                                console.log("接收到空响应");
                                renderHistory({});
                                return;
                            }
                            
                            let history;
                            try {
                                history = JSON.parse(responseText);
                                console.log("历史记录数据:", history);
                            } catch (parseError) {
                                console.error("JSON解析错误:", parseError);
                                console.error("无效的JSON:", responseText);
                                // 尝试修复可能的JSON格式问题
                                try {
                                    const fixedJson = responseText.replace(/,\s*}/g, '}').replace(/,\s*]/g, ']');
                                    history = JSON.parse(fixedJson);
                                    console.log("使用修复的JSON格式解析成功:", history);
                                } catch (e) {
                                    console.error("无法修复JSON格式:", e);
                                    renderHistory({});
                                    showToast('加载历史记录失败，无效数据格式', 'error');
                                    return;
                                }
                            }
                            
                            // 渲染历史记录，无论它是对象还是数组
                            renderHistory(history);
                        } catch (e) {
                            console.error("处理历史记录失败:", e);
                            renderHistory({});
                            showToast('加载历史记录失败', 'error');
                        }
                    } else {
                        console.error("加载历史记录失败, 状态码:", xhr.status);
                        renderHistory({});
                        showToast('加载历史记录失败', 'error');
                    }
                }
            };
            
            xhr.onerror = function() {
                console.error("加载历史记录请求失败");
                renderHistory({});
                showToast('加载历史记录失败，网络错误', 'error');
            };
            
            xhr.send();
        }

        // 使用 XMLHttpRequest 添加待办事项
        function addTodo() {
            const input = document.getElementById('newTodo');
            const content = input.value.trim();
            
            if (!content) {
                showToast('请输入任务内容', 'warning');
                return;
            }

            // 获取耗时信息
            const hours = parseInt(document.getElementById('hours_display').textContent) || 0;
            const minutes = parseInt(document.getElementById('minutes_display').textContent) || 0;
            
            // 验证时间输入
            if (hours < 0 || hours > 24) {
                showToast('小时数必须在0-24之间', 'error');
                return;
            }
            
            if (minutes < 0 || minutes > 59) {
                showToast('分钟数必须在0-59之间', 'error');
                return;
            }

            console.log("Sending request to add todo:", content);
        
            const xhr = new XMLHttpRequest();
            xhr.open('POST', '/api/todos', true);
            xhr.setRequestHeader('Content-Type', 'application/json');
            xhr.setRequestHeader('Accept', 'application/json');
            
            xhr.onreadystatechange = function() {
                if (xhr.readyState === 4) {
                    if (xhr.status === 201 || xhr.status === 200) {
                        try {
                            console.log("Raw response:", xhr.responseText);
                            const responseData = JSON.parse(xhr.responseText);
                            console.log("Response data:", responseData);
                            
                            input.value = '';
                            // 重置时间显示
                            document.getElementById('hours_display').textContent = '0';
                            document.getElementById('minutes_display').textContent = '0';
                            document.getElementById('hours_value').textContent = '0';
                            document.getElementById('minutes_value').textContent = '0';
                            
                            loadTodos(currentDate);
                            loadHistoricalTodos();
                            showToast('任务已添加', 'success');
                        } catch (parseError) {
                            console.error("Failed to parse response:", parseError);
                            showToast('服务器返回的数据无法解析，但可能已成功添加', 'warning');
                            
                            input.value = '';
                            // 重置时间显示
                            document.getElementById('hours_display').textContent = '0';
                            document.getElementById('minutes_display').textContent = '0';
                            document.getElementById('hours_value').textContent = '0';
                            document.getElementById('minutes_value').textContent = '0';
                            
                            document.getElementById('hours-display').textContent = '0';
                            document.getElementById('minutes-display').textContent = '0';
                            document.getElementById('hours-value').textContent = '0';
                            document.getElementById('minutes-value').textContent = '0';
                            
                            loadTodos(currentDate);
                        }
                    } else if (xhr.status === 401) {
                        // 未授权
                        window.location.href = "/index";
                    } else if (xhr.status === 405) {
                        // 方法不允许 - 可能是因为处于只读模式
                        console.error("405 Method Not Allowed");
                        updateStatusIndicator(true);
                        showToast('服务器处于只读模式，无法添加任务', 'warning');
                    } else {
                        handleErrorResponse(xhr, '添加失败');
                    }
                }
            };
            
            const data = JSON.stringify({
                content: content,
                date: currentDate,
                hours: hours,
                minutes: minutes
            });
            
            xhr.send(data);
        }

        // 切换待办事项完成状态
        function toggleTodo(id) {
            if (isReadOnly) {
                toggleLocalTodo(id);
                return;
            }
            
            const xhr = new XMLHttpRequest();
            xhr.open('PUT', `/api/todos/toggle?id=${id}&date=${currentDate}`, true);
            xhr.setRequestHeader('Accept', 'application/json');
            
            xhr.onreadystatechange = function() {
                if (xhr.readyState === 4) {
                    if (xhr.status === 200) {
                        loadTodos(currentDate);
                        loadHistoricalTodos();
                        
                        // 状态切换成功后刷新周统计数据
                        loadWeeklyStats(new Date());
                        
                        // 不显示提示，避免频繁提示干扰用户
                    } else if (xhr.status === 401) {
                        // 未授权
                        window.location.href = "/index";
                    } else if (xhr.status === 405) {
                        // 方法不允许 - 可能是因为处于只读模式
                        console.error("405 Method Not Allowed");
                        updateStatusIndicator(true);
                        toggleLocalTodo(id);
                    } else {
                        handleErrorResponse(xhr, '修改待办事项状态失败');
                    }
                }
            };
            
            xhr.send();
        }

        // 使用 XMLHttpRequest 更新待办事项耗时
        function updateTodoTime(id, hours, minutes) {
            if (isReadOnly) {
                updateLocalTodoTime(id, hours, minutes);
                return;
            }

            // 确保hours和minutes是整数
            const hoursInt = parseInt(hours, 10);
            const minutesInt = parseInt(minutes, 10);
            
            // 验证时间值
            if (isNaN(hoursInt) || hoursInt < 0 || hoursInt > 24) {
                showToast('小时数必须在0-24之间', 'error');
                return;
            }
            
            if (isNaN(minutesInt) || minutesInt < 0 || minutesInt > 59) {
                showToast('分钟数必须在0-59之间', 'error');
                return;
            }

            const xhr = new XMLHttpRequest();
            xhr.open('PUT', `/api/todos/time?id=${id}&date=${currentDate}`, true);
            xhr.setRequestHeader('Content-Type', 'application/json');
            xhr.setRequestHeader('Accept', 'application/json');
            
            xhr.onreadystatechange = function() {
                if (xhr.readyState === 4) {
                    if (xhr.status === 200) {
                        loadTodos(currentDate);
                        loadHistoricalTodos();
                        showToast('耗时已更新', 'success');
                    } else if (xhr.status === 401) {
                        // 未授权
                        window.location.href = "/index";
                    } else if (xhr.status === 405) {
                        // 方法不允许 - 可能是因为处于只读模式
                        updateStatusIndicator(true);
                        updateLocalTodoTime(id, hoursInt, minutesInt);
                    } else {
                        handleErrorResponse(xhr, '更新耗时失败');
                    }
                }
            };
            
            const data = JSON.stringify({
                hours: hoursInt,
                minutes: minutesInt
            });
            
            xhr.send(data);
        }

        // 删除待办事项
        function deleteTodo(id) {
            if (confirm('确定要删除这个任务吗？')) {
                if (isReadOnly) {
                    deleteLocalTodo(id);
                    return;
                }
                
                const xhr = new XMLHttpRequest();
                xhr.open('DELETE', `/api/todos?id=${id}&date=${currentDate}`, true);
                xhr.setRequestHeader('Accept', 'application/json');
                
                xhr.onreadystatechange = function() {
                    if (xhr.readyState === 4) {
                        if (xhr.status === 200 || xhr.status === 204) {
                            loadTodos(currentDate);
                            loadHistoricalTodos();
                            
                            // 删除成功后刷新周统计数据
                            loadWeeklyStats(new Date());
                            
                            showToast('任务已删除', 'success');
                        } else if (xhr.status === 401) {
                            window.location.href = "/index";
                        } else if (xhr.status === 405) {
                            showToast('服务器处于只读模式，无法删除任务', 'warning');
                            updateStatusIndicator(true);
                        } else {
                            handleErrorResponse(xhr, '删除待办事项失败');
                        }
                    }
                };
                
                xhr.send();
            }
        }

        // 处理错误响应
        function handleErrorResponse(xhr, defaultMessage) {
            try {
                const errorText = xhr.responseText;
                let errorMsg = `HTTP错误 ${xhr.status}`;
                console.log(errorText);
                
                if (errorText && errorText.includes('read-only')) {
                    // 特殊处理只读错误
                    errorMsg = '服务器处于只读模式';
                    updateStatusIndicator(true);
                } else {
                    try {
                        const errorData = JSON.parse(errorText);
                        errorMsg = errorData.error || errorMsg;
                    } catch (e) {
                        // 如果无法解析为JSON，使用原始文本
                        if (errorText) errorMsg = errorText;
                    }
                }
                
                showToast(`${defaultMessage}: ${errorMsg}`, 'error');
            } catch (error) {
                showToast(`${defaultMessage}: 未知错误`, 'error');
            }
        }

        // 在本地存储中保存待办事项
        function saveToLocalStorage(content, hours, minutes) {
            if (!localTodos[currentDate]) {
                localTodos[currentDate] = [];
            }
            
            const newTodo = {
                id: 'local_' + Date.now(),
                content: content,
                completed: false,
                created_at: new Date().toISOString(),
                hours: hours,
                minutes: minutes
            };
            
            localTodos[currentDate].push(newTodo);
            localStorage.setItem('localTodos', JSON.stringify(localTodos));
            return newTodo;
        }

        // 在本地切换待办事项状态
        function toggleLocalTodo(id) {
            if (!localTodos[currentDate]) return;
            
            const todos = localTodos[currentDate];
            for (let i = 0; i < todos.length; i++) {
                if (todos[i].id === id) {
                    todos[i].completed = !todos[i].completed;
                    break;
                }
            }
            
            localStorage.setItem('localTodos', JSON.stringify(localTodos));
            loadTodos(currentDate);
            showToast('使用本地模式更新任务状态', 'warning');
        }

        // 在本地更新待办事项耗时
        function updateLocalTodoTime(id, hours, minutes) {
            if (!localTodos[currentDate]) return;
            
            // 确保hours和minutes是整数
            const hoursInt = parseInt(hours, 10);
            const minutesInt = parseInt(minutes, 10);
            
            const todos = localTodos[currentDate];
            for (let i = 0; i < todos.length; i++) {
                if (todos[i].id === id) {
                    todos[i].hours = hoursInt;
                    todos[i].minutes = minutesInt;
                    break;
                }
            }
            
            localStorage.setItem('localTodos', JSON.stringify(localTodos));
            loadTodos(currentDate);
            showToast('使用本地模式更新耗时', 'warning');
        }

        // 在本地删除待办事项
        function deleteLocalTodo(id) {
            if (!localTodos[currentDate]) return;
            
            localTodos[currentDate] = localTodos[currentDate].filter(todo => todo.id !== id);
            localStorage.setItem('localTodos', JSON.stringify(localTodos));
            loadTodos(currentDate);
            showToast('使用本地模式删除任务', 'warning');
        }

        // 格式化时间显示
        function formatTime(hours, minutes) {
            // Convert to numbers to handle correctly
            const h = parseInt(hours) || 0;
            const m = parseInt(minutes) || 0;
            
            // If both are 0, return empty string
            if (h === 0 && m === 0) return '';
            
            // If only hours are set
            if (h > 0 && m === 0) {
                return `${h}小时`;
            }
            
            // If only minutes are set
            if (h === 0 && m > 0) {
                return `${m}分钟`;
            }
            
            // If both are set
            return `${h}小时${m}分钟`;
        }

        // 渲染待办事项列表
        function renderTodos(todos) {
            const todoList = document.getElementById('todoList');
            todoList.innerHTML = '';

            // 创建任务容器
            const todoItemsContainer = document.createElement('div');
            todoItemsContainer.className = 'todo-items-container';
            todoList.appendChild(todoItemsContainer);

            // 创建进度条容器 - 现在在card-body的顶部
            const progressBarContainer = document.getElementById('progressBarContainer');
            progressBarContainer.innerHTML = '';

            // 创建进度条
            const progressBar = document.createElement('div');
            progressBar.className = 'progress-bar';
            progressBarContainer.appendChild(progressBar);

            // 创建进度填充
            const progressFill = document.createElement('div');
            progressFill.className = 'progress-fill';
            progressBar.appendChild(progressFill);

            // 创建时间标记容器
            const progressTimeMarkers = document.createElement('div');
            progressTimeMarkers.className = 'progress-time-markers';
            progressBarContainer.appendChild(progressTimeMarkers);

            // 添加时间标记
            const startMarker = document.createElement('div');
            startMarker.className = 'progress-marker';
            startMarker.textContent = formatTimeDisplay(timeRange.start);
            progressTimeMarkers.appendChild(startMarker);

            const endMarker = document.createElement('div');
            endMarker.className = 'progress-marker';
            endMarker.textContent = formatTimeDisplay(timeRange.end);
            progressTimeMarkers.appendChild(endMarker);

            // 创建总时间显示
            const progressTotal = document.createElement('div');
            progressTotal.className = 'progress-total';
            progressTotal.textContent = '已完成任务总时间: 0小时0分钟';
            progressBarContainer.appendChild(progressTotal);

            // 合并服务器数据和本地数据
            let combinedTodos = [...(todos || [])];
            
            if (isReadOnly && localTodos[currentDate]) {
                combinedTodos = [...combinedTodos, ...localTodos[currentDate]];
            }

            if (!combinedTodos || combinedTodos.length === 0) {
                const emptyState = document.createElement('div');
                emptyState.className = 'empty-state';
                emptyState.textContent = '暂无任务';
                todoItemsContainer.appendChild(emptyState);
                updateTodoCounters(0, 0); // 更新计数器
                
                // 清空今日任务总时长统计
                updateTodaySummary(0, 0, 0, 0);
                return;
            }

            // 计算完成的任务数
            let completedCount = 0;
            combinedTodos.forEach(todo => {
                if (todo.completed) {
                    completedCount++;
                }
            });

            // 更新计数器
            updateTodoCounters(combinedTodos.length, completedCount);

            // 按自定义顺序排序任务
            const sortedTodos = sortTodosByOrder([...combinedTodos]);
            
            // 计算已完成任务的总时间
            let totalHours = 0;
            let totalMinutes = 0;
            
            // 计算所有任务的总时间
            let allTasksHours = 0;
            let allTasksMinutes = 0;
            
            sortedTodos.forEach(todo => {
                const hours = parseInt(todo.hours) || 0;
                const minutes = parseInt(todo.minutes) || 0;
                
                // 添加到所有任务总时间
                allTasksHours += hours;
                allTasksMinutes += minutes;
                
                if (todo.completed) {  // 只计算已完成的任务
                    totalHours += hours;
                    totalMinutes += minutes;
                }
            });

            // 调整总时间（处理分钟溢出）
            totalHours += Math.floor(totalMinutes / 60);
            totalMinutes = totalMinutes % 60;
            
            // 调整所有任务总时间（处理分钟溢出）
            allTasksHours += Math.floor(allTasksMinutes / 60);
            allTasksMinutes = allTasksMinutes % 60;

            // 更新总时间显示
            progressTotal.textContent = `已完成任务总时间: ${totalHours}小时${totalMinutes}分钟`;
            
            // 更新今日任务总时长统计
            updateTodaySummary(allTasksHours, allTasksMinutes, totalHours, totalMinutes);

            // 计算进度条高度（基于总时间）
            const totalTimeInMinutes = totalHours * 60 + totalMinutes;
            
            // 计算工作时间范围（小时）
            const startHours = timeToHours(timeRange.start);
            const endHours = timeToHours(timeRange.end);
            const workHours = endHours - startHours;
            const maxTimeInMinutes = workHours * 60;
            
            const progressPercent = Math.min(100, (totalTimeInMinutes / maxTimeInMinutes) * 100);
            
            // 设置进度条宽度（现在是水平进度条）
            progressFill.style.width = `${progressPercent}%`;
            
            // 检查是否超过可用时间，如果是则改变颜色
            if (totalTimeInMinutes > maxTimeInMinutes) {
                progressFill.style.backgroundColor = 'var(--danger-color)'; // 使用危险颜色（红色）
                progressTotal.style.color = 'var(--danger-color)'; // 总时间也变为红色
                
                // 添加警告图标
                progressTotal.innerHTML = `
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" style="vertical-align: middle; margin-right: 5px;">
                        <path d="M1 21h22L12 2 1 21zm12-3h-2v-2h2v2zm0-4h-2v-4h2v4z" fill="currentColor"/>
                    </svg>
                    已完成任务总时间: ${totalHours}小时${totalMinutes}分钟
                    <span style="font-size: 0.8em; display: block; margin-top: 3px;">已超出可用时间 ${Math.floor((totalTimeInMinutes - maxTimeInMinutes) / 60)}小时${(totalTimeInMinutes - maxTimeInMinutes) % 60}分钟</span>
                `;
            } else {
                progressFill.style.backgroundColor = 'var(--accent-color)'; // 使用默认强调色
                progressTotal.style.color = 'var(--accent-color)'; // 总时间使用默认强调色
            }

            // 判断当前日期是否是今天
            const today = new Date().toISOString().split('T')[0]; // 格式化为YYYY-MM-DD
            const isToday = currentDate === today;

            // 渲染任务列表
            sortedTodos.forEach((todo, index) => {
                const todoItem = document.createElement('div');
                todoItem.className = `todo-item ${todo.completed ? 'completed' : ''}`;
                todoItem.dataset.index = index; // 保存索引用于重新排序
                todoItem.dataset.id = todo.id; // 保存任务ID
                
                // 如果是今天的任务，添加today类
                if (isToday) {
                    todoItem.classList.add('today');
                }
                
                // 检查是否有耗时信息
                const hours = parseInt(todo.hours) || 0;
                const minutes = parseInt(todo.minutes) || 0;
                const hasTime = hours > 0 || minutes > 0;
                const timeDisplay = hasTime ? formatTime(hours, minutes) : '';
                
                todoItem.innerHTML = `
                    <div class="todo-order-controls">
                        <button class="order-btn move-up" onclick="moveTodoUp('${todo.id}', ${index})">
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M7.41 15.41L12 10.83L16.59 15.41L18 14L12 8L6 14L7.41 15.41Z" fill="currentColor"/>
                            </svg>
                        </button>
                        <button class="order-btn move-down" onclick="moveTodoDown('${todo.id}', ${index})">
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M7.41 8.59L12 13.17L16.59 8.59L18 10L12 16L6 10L7.41 8.59Z" fill="currentColor"/>
                            </svg>
                        </button>
                    </div>
                    <input type="checkbox" class="todo-checkbox" 
                           ${todo.completed ? 'checked' : ''} 
                           onchange="toggleTodo('${todo.id}')">
                    <div class="todo-content-wrapper">
                        <div class="todo-content">${todo.content}</div>
                        <div class="todo-time">
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M12 2C6.5 2 2 6.5 2 12C2 17.5 6.5 22 12 22C17.5 22 22 17.5 22 12C22 6.5 17.5 2 12 2ZM12 20C7.59 20 4 16.41 4 12C4 7.59 7.59 4 12 4C16.41 4 20 7.59 20 12C20 16.41 16.41 20 12 20ZM12.5 7H11V13L16.2 16.2L17 14.9L12.5 12.2V7Z" fill="currentColor"/>
                            </svg>
                            ${timeDisplay || '未设置时间'}
                        </div>
                    </div>
                    <div class="todo-actions">
                        <button class="edit-time-btn" onclick="toggleTimeControls('${todo.id}')">
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M3 17.25V21H6.75L17.81 9.94L14.06 6.19L3 17.25ZM20.71 7.04C21.1 6.65 21.1 6.02 20.71 5.63L18.37 3.29C17.98 2.9 17.35 2.9 16.96 3.29L15.13 5.12L18.88 8.87L20.71 7.04Z" fill="currentColor"/>
                            </svg>
                            修改
                        </button>
                        <button class="delete-btn" onclick="deleteTodo('${todo.id}')">×</button>
                    </div>
                    <div id="time-controls-${todo.id}" class="todo-time-slider-container">
                        <div class="todo-time-slider-group">
                            <div class="todo-time-slider-label">
                                <span>小时</span>
                                <span id="hours_value_${todo.id}" class="todo-time-slider-value">${todo.hours || 0}</span>
                            </div>
                            <div class="time-input-controls">
                                <div class="time-input-display" id="hours_display_${todo.id}" 
                                    onwheel="adjustTimeWithWheel(event, 'hours_${todo.id}', 1, 0, 24)"
                                    ontouchstart="handleTouchStart(event, 'hours_${todo.id}', 1, 0, 24)"
                                    ontouchmove="handleTouchMove(event)"
                                    ontouchend="handleTouchEnd(event)">${todo.hours || 0}</div>
                            </div>
                        </div>
                        <div class="todo-time-slider-group">
                            <div class="todo-time-slider-label">
                                <span>分钟</span>
                                <span id="minutes_value_${todo.id}" class="todo-time-slider-value">${todo.minutes || 0}</span>
                            </div>
                            <div class="time-input-controls">
                                <div class="time-input-display" id="minutes_display_${todo.id}" 
                                    onwheel="adjustTimeWithWheel(event, 'minutes_${todo.id}', 5, 0, 59)"
                                    ontouchstart="handleTouchStart(event, 'minutes_${todo.id}', 5, 0, 59)"
                                    ontouchmove="handleTouchMove(event)"
                                    ontouchend="handleTouchEnd(event)">${todo.minutes || 0}</div>
                            </div>
                        </div>
                        <button class="todo-time-update-btn" onclick="updateTodoTime('${todo.id}', document.getElementById('hours_display_${todo.id}').textContent, document.getElementById('minutes_display_${todo.id}').textContent); toggleTimeControls('${todo.id}')">更新</button>
                    </div>
                `;
                todoItemsContainer.appendChild(todoItem);
            });
        }

        // 更新今日任务总时长统计
        function updateTodaySummary(allHours, allMinutes, completedHours, completedMinutes) {
            const todaySummary = document.getElementById('todaySummary');
            if (!todaySummary) return;
            
            // 格式化总时间
            const totalTimeText = allHours > 0 ? 
                `${allHours}小时${allMinutes > 0 ? allMinutes + '分钟' : ''}` : 
                (allMinutes > 0 ? `${allMinutes}分钟` : '0分钟');
                
            // 格式化完成时间
            const completedTimeText = completedHours > 0 ? 
                `${completedHours}小时${completedMinutes > 0 ? completedMinutes + '分钟' : ''}` : 
                (completedMinutes > 0 ? `${completedMinutes}分钟` : '0分钟');
            
            // 更新显示
            todaySummary.innerHTML = `
                <div class="time-stat total-time">
                    <span class="time-stat-icon">
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M11.99 2C6.47 2 2 6.48 2 12C2 17.52 6.47 22 11.99 22C17.52 22 22 17.52 22 12C22 6.48 17.52 2 11.99 2ZM12 20C7.58 20 4 16.42 4 12C4 7.58 7.58 4 12 4C16.42 4 20 7.58 20 12C20 16.42 16.42 20 12 20ZM12.5 7H11V13L16.25 16.15L17 14.92L12.5 12.25V7Z" fill="currentColor"/>
                        </svg>
                    </span>
                    <span class="time-stat-value">${totalTimeText}</span>
                </div>
                <div class="time-stat completed-time">
                    <span class="time-stat-icon">
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M9 16.17L4.83 12L3.41 13.41L9 19L21 7L19.59 5.59L9 16.17Z" fill="currentColor"/>
                        </svg>
                    </span>
                    <span class="time-stat-value">${completedTimeText}</span>
                </div>
            `;
        }

        // 更新任务计数器
        function updateTodoCounters(total, completed) {
            document.getElementById('total-todos').textContent = `总任务数: ${total}`;
            document.getElementById('completed-todos').textContent = `已完成: ${completed}`;
        }

        // 渲染历史记录
        function renderHistory(history) {
            const historyList = document.getElementById('historyList');
            historyList.innerHTML = '';
            
            // 检查历史记录是否为空
            if (!history || 
                (Array.isArray(history) && history.length === 0) || 
                (typeof history === 'object' && Object.keys(history).length === 0)) {
                historyList.innerHTML = '<div class="empty-history">暂无历史记录</div>';
                return;
            }
            
            // 将历史数据转换为统一格式的数组
            let historyArray = [];
            
            if (Array.isArray(history)) {
                // 如果已经是数组格式
                historyArray = history;
            } else if (typeof history === 'object') {
                // 如果是对象格式 {date1: {...}, date2: {...}}
                for (const date in history) {
                    if (Object.prototype.hasOwnProperty.call(history, date)) {
                        const entry = history[date];
                        
                        // 处理不同的数据结构
                        if (entry.todos || entry.items) {
                            // 如果条目已经有todos或items字段
                            historyArray.push({
                                date: date,
                                todos: entry.todos || entry.items || []
                            });
                        } else if (Array.isArray(entry)) {
                            // 如果条目是数组
                            historyArray.push({
                                date: date,
                                todos: entry
                            });
                        } else {
                            // 其他格式，尝试保持兼容
                            historyArray.push({
                                date: date,
                                todos: entry.items || []
                            });
                        }
                    }
                }
            }
            
            // 按日期倒序排序
            historyArray.sort((a, b) => {
                return new Date(b.date) - new Date(a.date);
            });
            
            historyArray.forEach(item => {
                const historyItem = document.createElement('div');
                historyItem.className = 'history-item';
                
                // 计算统计数据
                let totalHours = 0;
                let totalMinutes = 0;
                let completedHours = 0;
                let completedMinutes = 0;
                let completedCount = 0;
                const todoArray = Array.isArray(item.todos) ? item.todos : [];
                const totalTasks = todoArray.length;
                
                todoArray.forEach(todo => {
                    const hours = parseInt(todo.hours) || 0;
                    const minutes = parseInt(todo.minutes) || 0;
                    
                    // 添加到任务总时间
                    totalHours += hours;
                    totalMinutes += minutes;
                    
                    if (todo.completed) {
                        // 添加到已完成任务时间
                        completedHours += hours;
                        completedMinutes += minutes;
                        completedCount++;
                    }
                });
                
                // 处理分钟溢出
                totalHours += Math.floor(totalMinutes / 60);
                totalMinutes = totalMinutes % 60;
                
                completedHours += Math.floor(completedMinutes / 60);
                completedMinutes = completedMinutes % 60;
                
                // 格式化日期
                const formattedDate = formatDate(item.date);
                
                // 计算完成率百分比
                const completionRate = totalTasks > 0 ? Math.round((completedCount / totalTasks) * 100) : 0;
                
                // 创建历史记录卡片
                historyItem.innerHTML = `
                    <div class="history-date">${formattedDate}</div>
                    <div class="history-summary">
                        <div class="history-stat tasks-stat">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M19 3H5C3.9 3 3 3.9 3 5V19C3 20.1 3.9 21 5 21H19C20.1 21 21 20.1 21 19V5C21 3.9 20.1 3 19 3ZM9 17H7V10H9V17ZM13 17H11V7H13V17ZM17 17H15V13H17V17Z" fill="currentColor"/>
                            </svg>
                            <span class="task-total">${totalTasks}</span>
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" style="margin-left: 8px;">
                                <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z" fill="var(--success-color)"/>
                            </svg>
                            <span class="task-completed">${completedCount}</span>
                            <span class="task-percentage">${completionRate}%</span>
                        </div>
                        <div class="history-stat time-stat">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M12 2C6.5 2 2 6.5 2 12C2 17.5 6.5 22 12 22C17.5 22 22 17.5 22 12C22 6.5 17.5 2 12 2ZM12 20C7.59 20 4 16.41 4 12C4 7.59 7.59 4 12 4C16.41 4 20 7.59 20 12C20 16.41 16.41 20 12 20ZM12.5 7H11V13L16.2 16.2L17 14.9L12.5 12.2V7Z" fill="currentColor"/>
                            </svg>
                            <span class="time-completed">${completedHours}h${completedMinutes}m</span>
                            <span class="time-separator">/</span>
                            <span class="time-total">${totalHours}h${totalMinutes}m</span>
                        </div>
                    </div>
                `;
                
                // 点击历史记录项显示该日期的详细待办事项
                historyItem.addEventListener('click', function(e) {
                    e.preventDefault();
                    e.stopPropagation();
                    
                    // 保存当前显示的日期，以便恢复
                    const previousDate = currentDate;
                    
                    // 显示待办事项详情
                    showDailyTodoModal(item.date, item.todos);
                });
                
                historyList.appendChild(historyItem);
            });
            
            // 应用加载动画
            animateItems('.history-item', 0.1);
        }

        // 格式化日期
        function formatDate(dateStr) {
            const date = new Date(dateStr);
            return date.toLocaleDateString('zh-CN', {
                year: 'numeric',
                month: 'long',
                day: 'numeric',
                weekday: 'long'
            });
        }

        // 显示提示信息
        function showToast(message, type = 'info') {
            const toast = document.createElement('div');
            toast.className = `toast ${type}`;
            toast.innerHTML = `<span class="toast-message">${message}</span>`;
            
            const toastContainer = document.getElementById('toast-container');
            toastContainer.appendChild(toast);
            
            // 4秒后移除提示
            setTimeout(() => {
                toast.style.opacity = '0';
                setTimeout(() => {
                    toast.remove();
                }, 300);
            }, 3000);
        }
        
        // 调整时间值的函数（用于鼠标滚轮）
        function adjustTimeWithWheel(event, elementId, step, min, max) {
            // 阻止页面滚动
            event.preventDefault();
            
            console.log('Adjusting time with wheel:', elementId, step, min, max);
            
            let displayElement, valueElement;
            
            // Handle both new tasks and existing tasks
            if (elementId.includes('_')) {
                // Existing task
                const [type, todoId] = elementId.split('_');
                displayElement = document.getElementById(`${type}_display_${todoId}`);
                valueElement = document.getElementById(`${type}_value_${todoId}`);
            } else {
                // New task
                displayElement = document.getElementById(`${elementId}_display`);
                valueElement = document.getElementById(`${elementId}_value`);
            }
            
            if (!displayElement || !valueElement) {
                console.error('Elements not found:', elementId);
                return;
            }
            
            // Get current value
            let currentValue = parseInt(displayElement.textContent) || 0;
            
            // Determine direction (up or down)
            const direction = event.deltaY < 0 ? 1 : -1;
            
            // Calculate new value
            let newValue = currentValue + (direction * step);
            
            // Ensure value stays within bounds
            newValue = Math.max(min, Math.min(max, newValue));
            
            // Update both display and value elements
            displayElement.textContent = newValue;
            valueElement.textContent = newValue;
            
            console.log('New value:', newValue);
        }
        
        // 调整新任务时间值的函数
        function adjustNewTaskTime(type, amount, min, max) {
            console.log('Adjusting new task time:', type, amount, min, max);
            
            const displayElement = document.getElementById(`${type}_display`);
            const valueElement = document.getElementById(`${type}_value`);
            
            if (!displayElement || !valueElement) {
                console.error('Elements not found:', type);
                return;
            }
            
            // Get current value and calculate new value
            let currentValue = parseInt(displayElement.textContent) || 0;
            let newValue = currentValue + amount;
            
            // Ensure value stays within bounds
            newValue = Math.max(min, Math.min(max, newValue));
            
            // Update both display and value elements
            displayElement.textContent = newValue;
            valueElement.textContent = newValue;
            
            console.log('New value:', newValue);
        }

        // 切换时间控制面板显示/隐藏
        function toggleTimeControls(todoId) {
            const timeControls = document.getElementById(`time-controls-${todoId}`);
            if (timeControls) {
                timeControls.classList.toggle('expanded');
                
                // 找到触发按钮并更新文本
                const editBtn = document.querySelector(`[onclick="toggleTimeControls('${todoId}')"]`);
                if (editBtn) {
                    if (timeControls.classList.contains('expanded')) {
                        editBtn.innerHTML = `
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" style="vertical-align: middle; margin-right: 2px;">
                                <path d="M19 6.41L17.59 5L12 10.59L6.41 5L5 6.41L10.59 12L5 17.59L6.41 19L12 13.41L17.59 19L19 17.59L13.41 12L19 6.41Z" fill="currentColor"/>
                            </svg>
                            收起`;
                    } else {
                        editBtn.innerHTML = `
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" style="vertical-align: middle; margin-right: 2px;">
                                <path d="M3 17.25V21H6.75L17.81 9.94L14.06 6.19L3 17.25ZM20.71 7.04C21.1 6.65 21.1 6.02 20.71 5.63L18.37 3.29C17.98 2.9 17.35 2.9 16.96 3.29L15.13 5.12L18.88 8.87L20.71 7.04Z" fill="currentColor"/>
                            </svg>
                            修改`;
                    }
                }
            }
        }

        // 应用时间范围设置
        function applyTimeRange() {
            // 获取小时和分钟选择器的值
            const startHour = document.getElementById('startHour').value;
            const startMinute = document.getElementById('startMinute').value;
            const endHour = document.getElementById('endHour').value;
            const endMinute = document.getElementById('endMinute').value;
            
            // 格式化时间为 HH:MM 格式
            timeRange.start = `${startHour.padStart(2, '0')}:${startMinute.padStart(2, '0')}`;
            timeRange.end = `${endHour.padStart(2, '0')}:${endMinute.padStart(2, '0')}`;
            
            // 保存到本地存储
            localStorage.setItem('timeRange', JSON.stringify(timeRange));
            
            // 重新渲染待办事项列表以更新进度条
            loadTodos(currentDate);
            
            showToast('工作时间范围已更新', 'success');
        }

        // 将时间字符串转换为小时数
        function timeToHours(timeStr) {
            const [hours, minutes] = timeStr.split(':').map(Number);
            return hours + minutes / 60;
        }

        // 将小时数转换为时间字符串
        function hoursToTime(hours) {
            const h = Math.floor(hours);
            const m = Math.round((hours - h) * 60);
            return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}`;
        }

        // 格式化时间显示
        function formatTimeDisplay(timeStr) {
            // 直接返回24小时制格式，不做AM/PM转换
            return timeStr;
        }

        // 触摸事件相关变量
        let touchStartY = 0;
        let touchElementId = '';
        let touchStep = 0;
        let touchMin = 0;
        let touchMax = 0;
        let touchThreshold = 10; // 触摸移动阈值，超过此值才调整时间
        let touchMoved = false;

        // 添加周时间统计相关变量
        let currentWeekDate = new Date(); // 当前显示的周的日期
        const weekDays = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];
        const weeklyStatsData = {}; // 存储周统计数据

        // 历史记录折叠状态
        let historyCollapsed = true; // 默认折叠

        // 页面加载后初始化周统计
        document.addEventListener('DOMContentLoaded', function() {
            console.log('DOMContentLoaded event fired - main initialization');
            
            // 初始化日期选择器
            flatpickr("#datePicker", {
                dateFormat: "Y-m-d",
                defaultDate: currentDate,
                locale: "zh",
                onChange: function(selectedDates, dateStr, instance) {
                    currentDate = dateStr;
                    loadTodos(currentDate);
                }
            });
            
            // 加载待办事项
            loadTodos(currentDate);
            
            // 加载历史记录
            loadHistoricalTodos();
            
            // 初始化周统计
            initWeeklyStats();
            
            // 初始化折叠功能
            console.log('Calling initToggleSections from DOMContentLoaded');
            initToggleSections();
            
            // 初始化模态框
            initModal();
            
            console.log('DOMContentLoaded initialization complete');
        });

        // 初始化折叠/展开功能
        function initToggleSections() {
            console.log('Initializing toggle sections...');
            setTimeout(function() {
                // 延迟执行以确保DOM完全加载
                const toggleHistoryBtn = document.getElementById('toggleHistoryBtn');
                
                console.log('toggleHistoryBtn element (delayed check):', toggleHistoryBtn);
                if (!toggleHistoryBtn) {
                    console.error('toggleHistoryBtn element not found in DOM!');
                    return;
                }
                
                const historyList = document.getElementById('historyList');
                if (!historyList) {
                    console.error('historyList element not found in DOM!');
                    return;
                }
                
                // 检查是否有本地存储的折叠状态
                const savedHistoryState = localStorage.getItem('historyCollapsed');
                if (savedHistoryState !== null) {
                    historyCollapsed = savedHistoryState === 'true';
                } else {
                    // 首次访问，默认为折叠状态
                    historyCollapsed = true;
                    localStorage.setItem('historyCollapsed', 'true');
                }
                
                // 应用当前折叠状态
                updateHistoryCollapsedState();
                
                // 尝试使用原生DOM onclick
                toggleHistoryBtn.onclick = function(e) {
                    console.log('Toggle button clicked via direct onclick property!');
                    // 切换折叠状态
                    historyCollapsed = !historyCollapsed;
                    // 更新UI
                    updateHistoryCollapsedState();
                    // 保存到本地存储
                    localStorage.setItem('historyCollapsed', historyCollapsed);
                    
                    // 阻止事件冒泡和默认行为
                    e.stopPropagation();
                    e.preventDefault();
                    return false;
                };
                
                console.log('Click handler attached via direct onclick property');
                
                // 手动触发一次更新，确保初始状态正确
                updateHistoryCollapsedState();
            }, 500); // 延迟500毫秒，确保DOM完全加载
        }

        // 更新历史记录折叠状态
        function updateHistoryCollapsedState() {
            console.log('updateHistoryCollapsedState called', historyCollapsed);
            const toggleHistoryBtn = document.getElementById('toggleHistoryBtn');
            const historyList = document.getElementById('historyList');
            const toggleText = toggleHistoryBtn.querySelector('.toggle-text');
            
            if (historyCollapsed) {
                // 使用直接样式操作而不是classList
                historyList.style.maxHeight = '0';
                historyList.style.opacity = '0';
                historyList.style.transform = 'translateY(-10px)';
                historyList.style.margin = '0';
                historyList.style.padding = '0';
                
                toggleHistoryBtn.classList.add('collapsed');
                toggleText.textContent = '展开';
            } else {
                // 展开时的样式
                historyList.style.maxHeight = '2000px';
                historyList.style.opacity = '1';
                historyList.style.transform = 'translateY(0)';
                historyList.style.margin = '';
                historyList.style.padding = '';
                
                toggleHistoryBtn.classList.remove('collapsed');
                toggleText.textContent = '折叠';
            }
        }

        // 初始化周时间统计
        function initWeeklyStats() {
            // 获取DOM元素
            const prevWeekBtn = document.getElementById('prevWeekBtn');
            const nextWeekBtn = document.getElementById('nextWeekBtn');
            
            // 添加事件监听
            prevWeekBtn.addEventListener('click', showPreviousWeek);
            nextWeekBtn.addEventListener('click', showNextWeek);
            
            // 添加刷新按钮
            const weekHeader = document.querySelector('.weekly-stats-header');
            if (weekHeader) {
                const refreshBtn = document.createElement('button');
                refreshBtn.className = 'week-refresh-btn';
                refreshBtn.innerHTML = `
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M17.65 6.35C16.2 4.9 14.21 4 12 4C7.58 4 4 7.58 4 12C4 16.42 7.58 20 12 20C15.73 20 18.84 17.45 19.73 14H17.65C16.83 16.33 14.61 18 12 18C8.69 18 6 15.31 6 12C6 8.69 8.69 6 12 6C13.66 6 15.14 6.69 16.22 7.78L13 11H20V4L17.65 6.35Z" fill="currentColor"/>
                    </svg>
                    刷新
                `;
                refreshBtn.addEventListener('click', () => {
                    // 重新加载数据
                    loadWeeklyStats(currentWeekDate);
                    // 显示提示
                    showToast('已刷新周统计数据', 'success');
                });
                
                // 插入到周标题栏中
                weekHeader.appendChild(refreshBtn);
            }
            
            // 加载当前周的数据
            loadWeeklyStats(currentWeekDate);
        }

        // 显示上一周数据
        function showPreviousWeek() {
            // 计算上一周的日期
            const prevWeekDate = new Date(currentWeekDate);
            prevWeekDate.setDate(prevWeekDate.getDate() - 7);
            
            // 更新当前显示的周日期
            currentWeekDate = prevWeekDate;
            
            // 加载数据并更新视图
            loadWeeklyStats(currentWeekDate);
        }

        // 显示下一周数据
        function showNextWeek() {
            // 计算下一周的日期
            const nextWeekDate = new Date(currentWeekDate);
            nextWeekDate.setDate(nextWeekDate.getDate() + 7);
            
            // 确保不超过当前日期
            const today = new Date();
            if (nextWeekDate > today) {
                nextWeekDate.setTime(today.getTime());
            }
            
            // 更新当前显示的周日期
            currentWeekDate = nextWeekDate;
            
            // 加载数据并更新视图
            loadWeeklyStats(currentWeekDate);
        }

        // 加载周统计数据
        function loadWeeklyStats(date) {
            console.log("loadWeeklyStats called with date:", date);
            
            // 计算当前周的开始日期（周一）和结束日期（周日）
            const weekDates = getWeekDates(date);
            const startDate = weekDates.monday;
            const endDate = weekDates.sunday;
            
            console.log("Week dates:", {
                monday: formatDateToYYYYMMDD(startDate),
                sunday: formatDateToYYYYMMDD(endDate)
            });
            
            // 更新周显示
            updateWeekDisplay(weekDates);
            
            // 格式化日期为YYYY-MM-DD
            const startDateStr = formatDateToYYYYMMDD(startDate);
            const endDateStr = formatDateToYYYYMMDD(endDate);
            
            // 直接从API加载数据，不使用缓存
            const url = `/api/todos/history?start_date=${startDateStr}&end_date=${endDateStr}`;
            console.log("Loading weekly stats from:", url);
            
            const xhr = new XMLHttpRequest();
            xhr.open('GET', url, true);
            xhr.setRequestHeader('Accept', 'application/json');
            
            xhr.onreadystatechange = function() {
                if (xhr.readyState === 4) {
                    if (xhr.status === 200) {
                        try {
                            const historyData = JSON.parse(xhr.responseText);
                            
                            // 处理历史数据
                            const processedData = processWeeklyData(historyData, weekDates);
                            
                            // 渲染处理后的数据
                            renderWeeklyStats(processedData);
                        } catch (e) {
                            console.error("Failed to process weekly stats:", e);
                            renderEmptyWeeklyStats();
                        }
                    } else {
                        console.error("Failed to load weekly stats:", xhr.status, xhr.statusText);
                        renderEmptyWeeklyStats();
                    }
                }
            };
            
            xhr.onerror = function() {
                console.error("Network error when loading weekly stats");
                renderEmptyWeeklyStats();
            };
            
            xhr.send();
        }

        // 获取一周的日期（周一到周日）
        function getWeekDates(date) {
            const result = {};
            // 确保使用日期副本，避免修改原始日期
            const currentDate = new Date(date.getTime());
            
            // 重置时间为00:00:00，以确保日期比较的准确性
            currentDate.setHours(0, 0, 0, 0);
            
            // 获取当前是周几（0是周日，1-6是周一到周六）
            const day = currentDate.getDay();
            
            // 计算到本周一的天数差
            const diff = day === 0 ? 6 : day - 1;
            
            // 计算本周一的日期
            const monday = new Date(currentDate.getTime());
            monday.setDate(currentDate.getDate() - diff);
            // 确保时间为00:00:00
            monday.setHours(0, 0, 0, 0);
            result.monday = monday;
            
            // 计算本周其他日期
            for (let i = 1; i <= 6; i++) {
                const nextDay = new Date(monday.getTime());
                nextDay.setDate(monday.getDate() + i);
                // 确保时间为00:00:00
                nextDay.setHours(0, 0, 0, 0);
                
                if (i === 6) {
                    result.sunday = nextDay;
                } else {
                    result[`day${i + 1}`] = nextDay;
                }
            }
            
            console.log(`Week range generated: ${result.monday.toDateString()} to ${result.sunday.toDateString()}`);
            
            // 返回日期对象，包含周一到周日
            return result;
        }

        // 格式化日期为YYYY-MM-DD
        function formatDateToYYYYMMDD(date) {
            // 克隆日期对象，避免修改原始日期
            const d = new Date(date.getTime());
            // 调整为本地时区
            d.setMinutes(d.getMinutes() - d.getTimezoneOffset());
            
            const year = d.getFullYear();
            const month = String(d.getMonth() + 1).padStart(2, '0');
            const day = String(d.getDate()).padStart(2, '0');
            
            return `${year}-${month}-${day}`;
        }

        // 更新周显示
        function updateWeekDisplay(weekDates) {
            const currentWeekDisplay = document.getElementById('currentWeekDisplay');
            const monday = weekDates.monday;
            const sunday = weekDates.sunday;
            
            // 计算本周是一年中的第几周
            const weekNumber = getWeekNumber(monday);
            
            // 格式化日期范围
            const mondayMonth = (monday.getMonth() + 1).toString().padStart(2, '0');
            const mondayDay = monday.getDate().toString().padStart(2, '0');
            const sundayMonth = (sunday.getMonth() + 1).toString().padStart(2, '0');
            const sundayDay = sunday.getDate().toString().padStart(2, '0');
            
            const dateRange = `${mondayMonth}.${mondayDay}-${sundayMonth}.${sundayDay}`;
            
            // 更新显示
            currentWeekDisplay.innerHTML = `
                <span class="week-number">${monday.getFullYear()}年第${weekNumber}周</span>
                <span class="week-date-range">${dateRange}</span>
            `;
        }

        // 计算一年中的第几周
        function getWeekNumber(date) {
            const target = new Date(date.valueOf());
            const dayNr = (date.getDay() + 6) % 7;
            target.setDate(target.getDate() - dayNr + 3);
            const firstThursday = target.valueOf();
            target.setMonth(0, 1);
            if (target.getDay() !== 4) {
                target.setMonth(0, 1 + ((4 - target.getDay()) + 7) % 7);
            }
            return 1 + Math.ceil((firstThursday - target) / 604800000);
        }

        // 处理周数据
        function processWeeklyData(history, weekDates) {
            console.log("processWeeklyData called with:", JSON.stringify(history).substring(0, 200) + "...", "weekDates:", weekDates);
            
            // 创建一个包含周一到周日的数据结构
            const weeklyData = {
                days: {},
                totals: {
                    totalTasks: 0,
                    completedTasks: 0,
                    totalTime: 0,
                    averageDailyTime: 0,
                    mostProductiveDay: null,
                    mostProductiveTime: 0
                }
            };
            
            // 初始化每一天的数据
            for (let i = 0; i < 7; i++) {
                const dayDate = new Date(weekDates.monday);
                dayDate.setDate(weekDates.monday.getDate() + i);
                const dateStr = formatDateToYYYYMMDD(dayDate);
                
                weeklyData.days[dateStr] = {
                    date: dateStr,
                    dayOfWeek: i,
                    totalTasks: 0,
                    completedTasks: 0,
                    totalTimeMinutes: 0
                };
            }
            
            console.log("Initial weeklyData structure:", weeklyData);
            
            // 处理历史数据
            for (const dateStr in history) {
                // 跳过不在本周范围内的日期
                const date = new Date(dateStr + "T00:00:00"); // 强制设置为当天的00:00:00时间
                
                // 复制一份日期对象并设置为00:00:00以确保比较的是日期而不是时间
                const mondayDate = new Date(weekDates.monday);
                mondayDate.setHours(0, 0, 0, 0);
                
                const sundayDate = new Date(weekDates.sunday);
                sundayDate.setHours(23, 59, 59, 999); // 设置为当天结束时间以包含整个周日
                
                // 详细日志输出，帮助诊断问题
                console.log(`Comparing dates: ${dateStr} [${date.toISOString()}] with week range: ${mondayDate.toISOString()} to ${sundayDate.toISOString()}`);
                console.log(`Date comparison: ${date.getTime()} < ${mondayDate.getTime()} = ${date < mondayDate}`);
                console.log(`Date comparison: ${date.getTime()} > ${sundayDate.getTime()} = ${date > sundayDate}`);
                
                if (date < mondayDate || date > sundayDate) {
                    console.log(`Skipping date ${dateStr} - outside of week range (${mondayDate.toDateString()} to ${sundayDate.toDateString()})`);
                    continue;
                }
                
                // 格式化日期确保格式一致
                const formattedDateStr = formatDateToYYYYMMDD(date);
                console.log(`Processing date: ${dateStr}, formatted: ${formattedDateStr}`);
                
                // 获取该日期的任务
                let items;
                if (Array.isArray(history[dateStr])) {
                    items = history[dateStr];
                    console.log(`Date ${dateStr} has ${items.length} tasks (from array)`);
                } else if (history[dateStr] && history[dateStr].items) {
                    items = history[dateStr].items;
                    console.log(`Date ${dateStr} has ${items.length} tasks (from history.items)`);
                } else {
                    items = [];
                    console.log(`Date ${dateStr} has no tasks`);
                }
                
                // 初始化该日期的数据（如果不存在）
                if (!weeklyData.days[formattedDateStr]) {
                    console.log(`Creating new day data for ${formattedDateStr} which wasn't in initialized days`);
                    weeklyData.days[formattedDateStr] = {
                        date: formattedDateStr,
                        dayOfWeek: (date.getDay() + 6) % 7, // 转换为0表示周一
                        totalTasks: 0,
                        completedTasks: 0,
                        totalTimeMinutes: 0
                    };
                }
                
                // 计算统计数据
                items.forEach((todo, index) => {
                    // 计算总任务数
                    weeklyData.days[formattedDateStr].totalTasks++;
                    weeklyData.totals.totalTasks++;
                    
                    console.log(`Task ${index} for ${formattedDateStr}: content=${todo.content?.substring(0, 20) || 'n/a'}, completed=${todo.completed}`);
                    
                    // 如果是已完成任务
                    if (todo.completed) {
                        weeklyData.days[formattedDateStr].completedTasks++;
                        weeklyData.totals.completedTasks++;
                        
                        // 计算任务时间（分钟）
                        const hours = parseInt(todo.hours) || 0;
                        const minutes = parseInt(todo.minutes) || 0;
                        const totalMinutes = hours * 60 + minutes;
                        
                        console.log(`Completed task time: ${hours}h ${minutes}m = ${totalMinutes} minutes`);
                        
                        // 添加到日统计
                        weeklyData.days[formattedDateStr].totalTimeMinutes += totalMinutes;
                        
                        // 添加到周统计
                        weeklyData.totals.totalTime += totalMinutes;
                        
                        // 检查是否是最高效的一天
                        if (weeklyData.days[formattedDateStr].totalTimeMinutes > weeklyData.totals.mostProductiveTime) {
                            weeklyData.totals.mostProductiveDay = formattedDateStr;
                            weeklyData.totals.mostProductiveTime = weeklyData.days[formattedDateStr].totalTimeMinutes;
                        }
                    }
                });
                
                console.log(`After processing ${formattedDateStr}: totalTasks=${weeklyData.days[formattedDateStr].totalTasks}, completedTasks=${weeklyData.days[formattedDateStr].completedTasks}`);
            }
            
            // 计算平均每日任务时间（仅计算工作日，不包括周末）
            let workdayCount = 0;
            let workdayTotalTime = 0;
            
            for (const dateStr in weeklyData.days) {
                const day = new Date(dateStr).getDay();
                // 只计算工作日（1-5表示周一到周五）
                if (day >= 1 && day <= 5) {
                    workdayCount++;
                    workdayTotalTime += weeklyData.days[dateStr].totalTimeMinutes;
                }
            }
            
            if (workdayCount > 0) {
                weeklyData.totals.averageDailyTime = Math.round(workdayTotalTime / workdayCount);
            }
            
            console.log("Final weeklyData:", weeklyData);
            return weeklyData;
        }

        // 渲染周统计数据
        function renderWeeklyStats(data) {
            const chartContainer = document.getElementById('weeklyStatsChart');
            const summaryContainer = document.getElementById('weeklyStatsSummary');
            
            // 清空容器
            chartContainer.innerHTML = '';
            summaryContainer.innerHTML = '';
            
            // 渲染图表
            renderWeeklyChart(data, chartContainer);
            
            // 渲染总结
            renderWeeklySummary(data, summaryContainer);
        }

        // 渲染周统计图表
        function renderWeeklyChart(data, container) {
            console.log("renderWeeklyChart called with data:", data);
            
            // 找出最大值用于计算比例
            let maxTimeMinutes = 0;
            let maxTasks = 0;
            for (const dateStr in data.days) {
                maxTimeMinutes = Math.max(maxTimeMinutes, data.days[dateStr].totalTimeMinutes);
                maxTasks = Math.max(maxTasks, data.days[dateStr].totalTasks);
            }
            
            // 如果最大值为0，设置一个默认值以便绘制空图表
            if (maxTimeMinutes === 0) {
                maxTimeMinutes = 60; // 默认1小时
            }
            if (maxTasks === 0) {
                maxTasks = 5; // 默认5个任务
            }
            
            // 遍历周一到周日
            const mondayDate = new Date(Object.keys(data.days)[0]);
            for (let i = 0; i < 7; i++) {
                const currentDate = new Date(mondayDate);
                currentDate.setDate(mondayDate.getDate() + i);
                const dateStr = formatDateToYYYYMMDD(currentDate);
                const dayData = data.days[dateStr] || { totalTimeMinutes: 0, totalTasks: 0, completedTasks: 0, todos: [] };
                
                // 创建日期柱状图容器
                const dayBarContainer = document.createElement('div');
                dayBarContainer.className = 'day-bar-container';
                dayBarContainer.dataset.date = dateStr;
                
                // 创建柱状图
                const dayBar = document.createElement('div');
                dayBar.className = 'day-bar';
                
                // 计算高度百分比（最大高度180px）
                const heightPercent = (dayData.totalTimeMinutes / maxTimeMinutes) * 100;
                dayBar.style.height = `${heightPercent}%`;
                
                // 添加悬停时显示的详细信息
                const dayInfoPopup = document.createElement('div');
                dayInfoPopup.className = 'day-info-popup';
                
                // 格式化时间显示
                const hours = Math.floor(dayData.totalTimeMinutes / 60);
                const minutes = dayData.totalTimeMinutes % 60;
                const timeText = hours > 0 ? 
                    `${hours}小时${minutes > 0 ? minutes + '分钟' : ''}` : 
                    (minutes > 0 ? `${minutes}分钟` : '0分钟');
                
                // 构建详细信息HTML
                dayInfoPopup.innerHTML = `
                    <div class="popup-item">
                        <span class="popup-label">总任务:</span>
                        <span class="popup-value">${dayData.totalTasks}个</span>
                    </div>
                    <div class="popup-item">
                        <span class="popup-label">已完成:</span>
                        <span class="popup-value">${dayData.completedTasks}个</span>
                    </div>
                    <div class="popup-item">
                        <span class="popup-label">总用时:</span>
                        <span class="popup-value">${timeText}</span>
                    </div>
                `;
                
                dayBar.appendChild(dayInfoPopup);
                
                // 创建任务数量指示器
                const taskCount = document.createElement('div');
                taskCount.className = 'task-count';
                taskCount.textContent = dayData.totalTasks;
                
                // 格式化日期（例如：04-10）
                const month = (currentDate.getMonth() + 1).toString().padStart(2, '0');
                const day = currentDate.getDate().toString().padStart(2, '0');
                const formattedDate = `${month}-${day}`;
                
                // 创建日期标签
                const dayLabel = document.createElement('div');
                dayLabel.className = 'day-label';
                dayLabel.innerHTML = `
                    <div class="day-name">${weekDays[i]}</div>
                    <div class="day-date">${formattedDate}</div>
                `;
                
                // 组合元素
                dayBarContainer.appendChild(dayBar);
                dayBarContainer.appendChild(taskCount);
                dayBarContainer.appendChild(dayLabel);
                
                // 添加点击事件 - 显示当天详情
                dayBarContainer.addEventListener('click', function() {
                    const clickedDate = this.dataset.date;
                    
                    // 如果该日有任务，显示详情
                    if (dayData.totalTasks > 0) {
                        // 如果数据已经包含todos，直接使用
                        if (dayData.todos && dayData.todos.length > 0) {
                            showDailyTodoModal(clickedDate, dayData.todos);
                        } else {
                            // 否则从服务器或本地存储获取
                            fetchDailyTodos(clickedDate);
                        }
                    } else {
                        showToast('该日无任务数据', 'info');
                    }
                });
                
                // 添加到容器
                container.appendChild(dayBarContainer);
            }
        }

        // 渲染周统计总结
        function renderWeeklySummary(data, container) {
            // 创建统计项
            const createStatItem = (value, label, icon = null) => {
                const item = document.createElement('div');
                item.className = 'weekly-stat-item';
                
                const valueElem = document.createElement('div');
                valueElem.className = 'weekly-stat-value';
                
                if (icon) {
                    const iconElem = document.createElement('span');
                    iconElem.className = 'stat-icon';
                    iconElem.innerHTML = icon;
                    valueElem.appendChild(iconElem);
                }
                
                const valueText = document.createElement('span');
                valueText.textContent = value;
                valueElem.appendChild(valueText);
                
                const labelElem = document.createElement('div');
                labelElem.className = 'weekly-stat-label';
                labelElem.textContent = label;
                
                item.appendChild(valueElem);
                item.appendChild(labelElem);
                
                return item;
            };
            
            // 计算任务完成率
            const completionRate = data.totals.totalTasks > 0 
                ? Math.round((data.totals.completedTasks / data.totals.totalTasks) * 100) 
                : 0;
            
            // 计算每天平均任务数
            const avgTasksPerDay = data.totals.totalTasks > 0 
                ? (data.totals.totalTasks / 5).toFixed(1) 
                : 0;
            
            // 统计图标
            const icons = {
                tasks: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M19 3H5C3.9 3 3 3.9 3 5V19C3 20.1 3.9 21 5 21H19C20.1 21 21 20.1 21 19V5C21 3.9 20.1 3 19 3ZM9 17H7V10H9V17ZM13 17H11V7H13V17ZM17 17H15V13H17V17Z" fill="currentColor"/></svg>',
                completed: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M9 16.17L4.83 12L3.41 13.41L9 19L21 7L19.59 5.59L9 16.17Z" fill="currentColor"/></svg>',
                time: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M11.99 2C6.47 2 2 6.48 2 12C2 17.52 6.47 22 11.99 22C17.52 22 22 17.52 22 12C22 6.48 17.52 2 11.99 2ZM12 20C7.58 20 4 16.42 4 12C4 7.58 7.58 4 12 4C16.42 4 20 7.58 20 12C20 16.42 16.42 20 12 20ZM12.5 7H11V13L16.25 16.15L17 14.92L12.5 12.25V7Z" fill="currentColor"/></svg>',
                rate: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M19 3H5C3.9 3 3 3.9 3 5V19C3 20.1 3.9 21 5 21H19C20.1 21 21 20.1 21 19V5C21 3.9 20.1 3 19 3ZM9 17H7V10H9V17ZM13 17H11V7H13V17ZM17 17H15V13H17V17Z" fill="currentColor"/></svg>',
                avg: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M19 3H4.99C3.89 3 3 3.9 3 5V19C3 20.1 3.89 21 4.99 21H19C20.1 21 21 20.1 21 19V5C21 3.9 20.1 3 19 3ZM19 15H15.87C15.4 15 15.02 15.34 14.89 15.8C14.54 17.07 13.37 18 12 18C10.63 18 9.46 17.07 9.11 15.8C8.98 15.34 8.6 15 8.13 15H4.99V5H19V15Z" fill="currentColor"/></svg>',
                efficient: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M15 1H9V3H15V1ZM11 14H13V8H11V14ZM19.03 7.39L20.45 5.97C20.02 5.46 19.55 4.98 19.04 4.56L17.62 5.98C16.07 4.74 14.12 4 12 4C7.03 4 3 8.03 3 13C3 17.97 7.02 22 12 22C16.98 22 21 17.97 21 13C21 10.88 20.26 8.93 19.03 7.39ZM12 20C8.13 20 5 16.87 5 13C5 9.13 8.13 6 12 6C15.87 6 19 9.13 19 13C19 16.87 15.87 20 12 20Z" fill="currentColor"/></svg>'
            };
            
            // 总任务数
            container.appendChild(createStatItem(
                data.totals.totalTasks,
                '总任务数',
                icons.tasks
            ));
            
            // 已完成任务数
            container.appendChild(createStatItem(
                data.totals.completedTasks,
                '已完成任务',
                icons.completed
            ));
            
            // 完成率
            container.appendChild(createStatItem(
                `${completionRate}%`,
                '任务完成率',
                icons.rate
            ));
            
            // 每天平均任务数
            container.appendChild(createStatItem(
                avgTasksPerDay,
                '工作日均任务',
                icons.avg
            ));
            
            // 总用时
            const totalHours = Math.floor(data.totals.totalTime / 60);
            const totalMinutes = data.totals.totalTime % 60;
            const totalTimeText = totalHours > 0 ? 
                `${totalHours}小时${totalMinutes > 0 ? totalMinutes + '分钟' : ''}` : 
                (totalMinutes > 0 ? `${totalMinutes}分钟` : '0分钟');
            
            container.appendChild(createStatItem(
                totalTimeText,
                '总用时',
                icons.time
            ));
            
            // 平均每日用时
            const avgHours = Math.floor(data.totals.averageDailyTime / 60);
            const avgMinutes = data.totals.averageDailyTime % 60;
            const avgTimeText = avgHours > 0 ? 
                `${avgHours}小时${avgMinutes > 0 ? avgMinutes + '分钟' : ''}` : 
                (avgMinutes > 0 ? `${avgMinutes}分钟` : '0分钟');
            
            container.appendChild(createStatItem(
                avgTimeText,
                '平均工作日用时',
                icons.time
            ));
            
            // 最高效的一天
            if (data.totals.mostProductiveDay) {
                const mostProductiveDate = new Date(data.totals.mostProductiveDay);
                const dayOfWeek = weekDays[mostProductiveDate.getDay() === 0 ? 6 : mostProductiveDate.getDay() - 1];
                
                container.appendChild(createStatItem(
                    dayOfWeek,
                    '最高效的一天',
                    icons.efficient
                ));
            }
        }

        // 渲染空的周统计
        function renderEmptyWeeklyStats() {
            const chartContainer = document.getElementById('weeklyStatsChart');
            const summaryContainer = document.getElementById('weeklyStatsSummary');
            
            // 清空容器
            chartContainer.innerHTML = '';
            summaryContainer.innerHTML = '';
            
            // 添加空状态消息
            const emptyState = document.createElement('div');
            emptyState.className = 'empty-state';
            emptyState.textContent = '暂无周统计数据';
            chartContainer.appendChild(emptyState);
            
            // 创建空的统计摘要
            const weeklyData = {
                days: {},
                totals: {
                    totalTasks: 0,
                    completedTasks: 0,
                    totalTime: 0,
                    averageDailyTime: 0
                }
            };
            
            renderWeeklySummary(weeklyData, summaryContainer);
        }

        // 处理触摸开始事件
        function handleTouchStart(event, elementId, step, min, max) {
            // 阻止默认行为
            event.preventDefault();
            
            // 记录初始触摸位置和元素信息
            touchStartY = event.touches[0].clientY;
            touchElementId = elementId;
            touchStep = step;
            touchMin = min;
            touchMax = max;
            touchMoved = false;
            
            console.log('Touch start:', touchStartY, elementId);
        }

        // 处理触摸移动事件
        function handleTouchMove(event) {
            // 阻止默认行为
            event.preventDefault();
            
            // 计算移动距离
            const touchY = event.touches[0].clientY;
            const deltaY = touchStartY - touchY;
            
            // 如果移动距离超过阈值，调整时间值
            if (Math.abs(deltaY) > touchThreshold && !touchMoved) {
                // 确定方向（向上滑动增加，向下滑动减少）
                const direction = deltaY > 0 ? 1 : -1;
                
                // 调整时间值
                adjustTimeWithTouch(touchElementId, direction * touchStep, touchMin, touchMax);
                
                // 更新初始位置，以便连续调整
                touchStartY = touchY;
                touchMoved = true;
            }
        }

        // 处理触摸结束事件
        function handleTouchEnd(event) {
            // 重置触摸状态
            touchElementId = '';
            touchMoved = false;
        }

        // 使用触摸调整时间值
        function adjustTimeWithTouch(elementId, step, min, max) {
            console.log('Adjusting time with touch:', elementId, step, min, max);
            
            let displayElement, valueElement;
            
            // Handle both new tasks and existing tasks
            if (elementId.includes('_')) {
                // Existing task
                const [type, todoId] = elementId.split('_');
                displayElement = document.getElementById(`${type}_display_${todoId}`);
                valueElement = document.getElementById(`${type}_value_${todoId}`);
            } else {
                // New task
                displayElement = document.getElementById(`${elementId}_display`);
                valueElement = document.getElementById(`${elementId}_value`);
            }
            
            if (!displayElement || !valueElement) {
                console.error('Elements not found:', elementId);
                return;
            }
            
            // Get current value
            let currentValue = parseInt(displayElement.textContent) || 0;
            
            // Calculate new value
            let newValue = currentValue + step;
            
            // Ensure value stays within bounds
            newValue = Math.max(min, Math.min(max, newValue));
            
            // Update both display and value elements
            displayElement.textContent = newValue;
            valueElement.textContent = newValue;
            
            console.log('New value:', newValue);
        }

        // 任务排序相关变量
        let todoOrder = []; // 存储任务排序顺序

        // 上移任务
        function moveTodoUp(id, index) {
            // 如果已经是第一个任务，不做任何操作
            if (index === 0) {
                showToast('已经是第一个任务', 'info');
                return;
            }
            
            // 获取所有任务元素
            const todoContainer = document.querySelector('.todo-items-container');
            const todoItems = todoContainer.querySelectorAll('.todo-item');
            
            // 获取当前任务和上一个任务
            const currentItem = todoItems[index];
            const prevItem = todoItems[index - 1];
            
            // 插入当前任务到上一个任务之前
            todoContainer.insertBefore(currentItem, prevItem);
            
            // 更新任务顺序
            updateTodoOrder();
            
            // 显示提示
            showToast('任务顺序已更新', 'success');
        }

        // 下移任务
        function moveTodoDown(id, index) {
            // 获取所有任务元素
            const todoContainer = document.querySelector('.todo-items-container');
            const todoItems = todoContainer.querySelectorAll('.todo-item');
            
            // 如果已经是最后一个任务，不做任何操作
            if (index === todoItems.length - 1) {
                showToast('已经是最后一个任务', 'info');
                return;
            }
            
            // 获取当前任务和下一个任务
            const currentItem = todoItems[index];
            const nextItem = todoItems[index + 1];
            
            // 插入当前任务到下一个任务之后
            todoContainer.insertBefore(nextItem, currentItem);
            
            // 更新任务顺序
            updateTodoOrder();
            
            // 显示提示
            showToast('任务顺序已更新', 'success');
        }

        // 更新任务顺序
        function updateTodoOrder() {
            // 获取所有任务元素的顺序
            const todoItems = document.querySelectorAll('.todo-item');
            todoOrder = Array.from(todoItems).map(item => item.dataset.id);
            
            // 更新本地存储中的顺序
            saveOrderToLocalStorage();
            
            // 更新任务索引
            todoItems.forEach((item, index) => {
                item.dataset.index = index;
                
                // 更新上下移动按钮的onclick属性
                const upBtn = item.querySelector('.move-up');
                const downBtn = item.querySelector('.move-down');
                
                if (upBtn) {
                    upBtn.setAttribute('onclick', `moveTodoUp('${item.dataset.id}', ${index})`);
                }
                
                if (downBtn) {
                    downBtn.setAttribute('onclick', `moveTodoDown('${item.dataset.id}', ${index})`);
                }
            });
        }

        // 保存任务顺序到本地存储
        function saveOrderToLocalStorage() {
            // 先保存到本地存储作为备份
            localStorage.setItem(`todoOrder_${currentDate}`, JSON.stringify(todoOrder));
            
            // 如果不是只读模式，则将顺序保存到服务器
            if (!isReadOnly) {
                const xhr = new XMLHttpRequest();
                xhr.open('PUT', '/api/todos/order', true);
                xhr.setRequestHeader('Content-Type', 'application/json');
                xhr.setRequestHeader('Accept', 'application/json');
                
                xhr.onreadystatechange = function() {
                    if (xhr.readyState === 4) {
                        if (xhr.status === 200) {
                            console.log('任务顺序已保存到服务器');
                        } else if (xhr.status === 405) {
                            console.warn('服务器处于只读模式，无法保存任务顺序');
                            updateStatusIndicator(true);
                        } else {
                            console.error('保存任务顺序失败:', xhr.status);
                        }
                    }
                };
                
                const data = JSON.stringify({
                    date: currentDate,
                    order: todoOrder
                });
                
                xhr.send(data);
            }
        }

        // 从本地存储加载任务顺序
        function loadOrderFromLocalStorage() {
            const savedOrder = localStorage.getItem(`todoOrder_${currentDate}`);
            if (savedOrder) {
                try {
                    todoOrder = JSON.parse(savedOrder);
                    return true;
                } catch (e) {
                    console.error("Failed to parse todo order:", e);
                    todoOrder = [];
                    return false;
                }
            }
            todoOrder = [];
            return false;
        }

        // 根据保存的顺序排序任务
        function sortTodosByOrder(todos) {
            // 如果没有保存的顺序或加载失败，使用默认排序
            if (!loadOrderFromLocalStorage() || todoOrder.length === 0) {
                return todos.sort((a, b) => {
                    // 使用创建时间排序
                    const timeA = new Date(a.created_at || 0).getTime();
                    const timeB = new Date(b.created_at || 0).getTime();
                    return timeA - timeB;
                });
            }
            
            // 创建一个ID到任务的映射
            const todoMap = {};
            todos.forEach(todo => {
                todoMap[todo.id] = todo;
            });
            
            // 根据保存的顺序构建新的任务数组
            const orderedTodos = [];
            
            // 首先添加有序的任务
            todoOrder.forEach(id => {
                if (todoMap[id]) {
                    orderedTodos.push(todoMap[id]);
                    delete todoMap[id];
                }
            });
            
            // 添加剩余的任务（可能是新添加的）
            Object.values(todoMap).forEach(todo => {
                orderedTodos.push(todo);
            });
            
            return orderedTodos;
        }
        
        // 修改渲染待办事项列表函数，使用自定义排序
        function renderTodos(todos) {
            const todoList = document.getElementById('todoList');
            todoList.innerHTML = '';

            // 创建任务容器
            const todoItemsContainer = document.createElement('div');
            todoItemsContainer.className = 'todo-items-container';
            todoList.appendChild(todoItemsContainer);

            // 创建进度条容器 - 现在在card-body的顶部
            const progressBarContainer = document.getElementById('progressBarContainer');
            progressBarContainer.innerHTML = '';

            // 创建进度条
            const progressBar = document.createElement('div');
            progressBar.className = 'progress-bar';
            progressBarContainer.appendChild(progressBar);

            // 创建进度填充
            const progressFill = document.createElement('div');
            progressFill.className = 'progress-fill';
            progressBar.appendChild(progressFill);

            // 创建时间标记容器
            const progressTimeMarkers = document.createElement('div');
            progressTimeMarkers.className = 'progress-time-markers';
            progressBarContainer.appendChild(progressTimeMarkers);

            // 添加时间标记
            const startMarker = document.createElement('div');
            startMarker.className = 'progress-marker';
            startMarker.textContent = formatTimeDisplay(timeRange.start);
            progressTimeMarkers.appendChild(startMarker);

            const endMarker = document.createElement('div');
            endMarker.className = 'progress-marker';
            endMarker.textContent = formatTimeDisplay(timeRange.end);
            progressTimeMarkers.appendChild(endMarker);

            // 创建总时间显示
            const progressTotal = document.createElement('div');
            progressTotal.className = 'progress-total';
            progressTotal.textContent = '已完成任务总时间: 0小时0分钟';
            progressBarContainer.appendChild(progressTotal);

            // 合并服务器数据和本地数据
            let combinedTodos = [...(todos || [])];
            
            if (isReadOnly && localTodos[currentDate]) {
                combinedTodos = [...combinedTodos, ...localTodos[currentDate]];
            }

            if (!combinedTodos || combinedTodos.length === 0) {
                const emptyState = document.createElement('div');
                emptyState.className = 'empty-state';
                emptyState.textContent = '暂无任务';
                todoItemsContainer.appendChild(emptyState);
                updateTodoCounters(0, 0); // 更新计数器
                
                // 清空今日任务总时长统计
                updateTodaySummary(0, 0, 0, 0);
                return;
            }

            // 计算完成的任务数
            let completedCount = 0;
            combinedTodos.forEach(todo => {
                if (todo.completed) {
                    completedCount++;
                }
            });

            // 更新计数器
            updateTodoCounters(combinedTodos.length, completedCount);

            // 按自定义顺序排序任务
            const sortedTodos = sortTodosByOrder([...combinedTodos]);
            
            // 计算已完成任务的总时间
            let totalHours = 0;
            let totalMinutes = 0;
            
            // 计算所有任务的总时间
            let allTasksHours = 0;
            let allTasksMinutes = 0;
            
            sortedTodos.forEach(todo => {
                const hours = parseInt(todo.hours) || 0;
                const minutes = parseInt(todo.minutes) || 0;
                
                // 添加到所有任务总时间
                allTasksHours += hours;
                allTasksMinutes += minutes;
                
                if (todo.completed) {  // 只计算已完成的任务
                    totalHours += hours;
                    totalMinutes += minutes;
                }
            });

            // 调整总时间（处理分钟溢出）
            totalHours += Math.floor(totalMinutes / 60);
            totalMinutes = totalMinutes % 60;
            
            // 调整所有任务总时间（处理分钟溢出）
            allTasksHours += Math.floor(allTasksMinutes / 60);
            allTasksMinutes = allTasksMinutes % 60;

            // 更新总时间显示
            progressTotal.textContent = `已完成任务总时间: ${totalHours}小时${totalMinutes}分钟`;
            
            // 更新今日任务总时长统计
            updateTodaySummary(allTasksHours, allTasksMinutes, totalHours, totalMinutes);

            // 计算进度条高度（基于总时间）
            const totalTimeInMinutes = totalHours * 60 + totalMinutes;
            
            // 计算工作时间范围（小时）
            const startHours = timeToHours(timeRange.start);
            const endHours = timeToHours(timeRange.end);
            const workHours = endHours - startHours;
            const maxTimeInMinutes = workHours * 60;
            
            const progressPercent = Math.min(100, (totalTimeInMinutes / maxTimeInMinutes) * 100);
            
            // 设置进度条宽度（现在是水平进度条）
            progressFill.style.width = `${progressPercent}%`;
            
            // 检查是否超过可用时间，如果是则改变颜色
            if (totalTimeInMinutes > maxTimeInMinutes) {
                progressFill.style.backgroundColor = 'var(--danger-color)'; // 使用危险颜色（红色）
                progressTotal.style.color = 'var(--danger-color)'; // 总时间也变为红色
                
                // 添加警告图标
                progressTotal.innerHTML = `
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" style="vertical-align: middle; margin-right: 5px;">
                        <path d="M1 21h22L12 2 1 21zm12-3h-2v-2h2v2zm0-4h-2v-4h2v4z" fill="currentColor"/>
                    </svg>
                    已完成任务总时间: ${totalHours}小时${totalMinutes}分钟
                    <span style="font-size: 0.8em; display: block; margin-top: 3px;">已超出可用时间 ${Math.floor((totalTimeInMinutes - maxTimeInMinutes) / 60)}小时${(totalTimeInMinutes - maxTimeInMinutes) % 60}分钟</span>
                `;
            } else {
                progressFill.style.backgroundColor = 'var(--accent-color)'; // 使用默认强调色
                progressTotal.style.color = 'var(--accent-color)'; // 总时间使用默认强调色
            }

            // 判断当前日期是否是今天
            const today = new Date().toISOString().split('T')[0]; // 格式化为YYYY-MM-DD
            const isToday = currentDate === today;

            // 渲染任务列表
            sortedTodos.forEach((todo, index) => {
                const todoItem = document.createElement('div');
                todoItem.className = `todo-item ${todo.completed ? 'completed' : ''}`;
                todoItem.dataset.index = index; // 保存索引用于重新排序
                todoItem.dataset.id = todo.id; // 保存任务ID
                
                // 如果是今天的任务，添加today类
                if (isToday) {
                    todoItem.classList.add('today');
                }
                
                // 检查是否有耗时信息
                const hours = parseInt(todo.hours) || 0;
                const minutes = parseInt(todo.minutes) || 0;
                const hasTime = hours > 0 || minutes > 0;
                const timeDisplay = hasTime ? formatTime(hours, minutes) : '';
                
                todoItem.innerHTML = `
                    <div class="todo-order-controls">
                        <button class="order-btn move-up" onclick="moveTodoUp('${todo.id}', ${index})">
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M7.41 15.41L12 10.83L16.59 15.41L18 14L12 8L6 14L7.41 15.41Z" fill="currentColor"/>
                            </svg>
                        </button>
                        <button class="order-btn move-down" onclick="moveTodoDown('${todo.id}', ${index})">
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M7.41 8.59L12 13.17L16.59 8.59L18 10L12 16L6 10L7.41 8.59Z" fill="currentColor"/>
                            </svg>
                        </button>
                    </div>
                    <input type="checkbox" class="todo-checkbox" 
                           ${todo.completed ? 'checked' : ''} 
                           onchange="toggleTodo('${todo.id}')">
                    <div class="todo-content-wrapper">
                        <div class="todo-content">${todo.content}</div>
                        <div class="todo-time">
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M12 2C6.5 2 2 6.5 2 12C2 17.5 6.5 22 12 22C17.5 22 22 17.5 22 12C22 6.5 17.5 2 12 2ZM12 20C7.59 20 4 16.41 4 12C4 7.59 7.59 4 12 4C16.41 4 20 7.59 20 12C20 16.41 16.41 20 12 20ZM12.5 7H11V13L16.2 16.2L17 14.9L12.5 12.2V7Z" fill="currentColor"/>
                            </svg>
                            ${timeDisplay || '未设置时间'}
                        </div>
                    </div>
                    <div class="todo-actions">
                        <button class="edit-time-btn" onclick="toggleTimeControls('${todo.id}')">
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M3 17.25V21H6.75L17.81 9.94L14.06 6.19L3 17.25ZM20.71 7.04C21.1 6.65 21.1 6.02 20.71 5.63L18.37 3.29C17.98 2.9 17.35 2.9 16.96 3.29L15.13 5.12L18.88 8.87L20.71 7.04Z" fill="currentColor"/>
                            </svg>
                            修改
                        </button>
                        <button class="delete-btn" onclick="deleteTodo('${todo.id}')">×</button>
                    </div>
                    <div id="time-controls-${todo.id}" class="todo-time-slider-container">
                        <div class="todo-time-slider-group">
                            <div class="todo-time-slider-label">
                                <span>小时</span>
                                <span id="hours_value_${todo.id}" class="todo-time-slider-value">${todo.hours || 0}</span>
                            </div>
                            <div class="time-input-controls">
                                <div class="time-input-display" id="hours_display_${todo.id}" 
                                    onwheel="adjustTimeWithWheel(event, 'hours_${todo.id}', 1, 0, 24)"
                                    ontouchstart="handleTouchStart(event, 'hours_${todo.id}', 1, 0, 24)"
                                    ontouchmove="handleTouchMove(event)"
                                    ontouchend="handleTouchEnd(event)">${todo.hours || 0}</div>
                            </div>
                        </div>
                        <div class="todo-time-slider-group">
                            <div class="todo-time-slider-label">
                                <span>分钟</span>
                                <span id="minutes_value_${todo.id}" class="todo-time-slider-value">${todo.minutes || 0}</span>
                            </div>
                            <div class="time-input-controls">
                                <div class="time-input-display" id="minutes_display_${todo.id}" 
                                    onwheel="adjustTimeWithWheel(event, 'minutes_${todo.id}', 5, 0, 59)"
                                    ontouchstart="handleTouchStart(event, 'minutes_${todo.id}', 5, 0, 59)"
                                    ontouchmove="handleTouchMove(event)"
                                    ontouchend="handleTouchEnd(event)">${todo.minutes || 0}</div>
                            </div>
                        </div>
                        <button class="todo-time-update-btn" onclick="updateTodoTime('${todo.id}', document.getElementById('hours_display_${todo.id}').textContent, document.getElementById('minutes_display_${todo.id}').textContent); toggleTimeControls('${todo.id}')">更新</button>
                    </div>
                `;
                todoItemsContainer.appendChild(todoItem);
            });
        }

        // 添加待办事项后，清除相关缓存
        function addTodoAndClearCache() {
            // 先正常添加待办事项
            addTodo();
            
            // 添加完成后刷新周统计
            loadWeeklyStats(new Date());
        }

        // 完成待办事项后，清除相关缓存
        function toggleTodoAndClearCache(id) {
            // 先正常切换待办事项状态
            toggleTodo(id);
            
            // 切换完成后刷新周统计
            loadWeeklyStats(new Date());
        }

        // 初始化模态框
        function initModal() {
            const modal = document.getElementById('dailyTodoModal');
            const closeBtn = modal.querySelector('.close-modal');
            
            // 点击关闭按钮关闭模态框
            closeBtn.addEventListener('click', function() {
                modal.style.display = 'none';
            });
            
            // 点击模态框外部关闭
            window.addEventListener('click', function(event) {
                if (event.target === modal) {
                    modal.style.display = 'none';
                }
            });
        }

        // 显示每日任务详情模态框
        function showDailyTodoModal(date, todos) {
            // 获取模态框元素
            const modal = document.getElementById('dailyTodoModal');
            const modalDate = document.getElementById('modalDate');
            const modalTotalTasks = document.getElementById('modalTotalTasks');
            const modalCompletedTasks = document.getElementById('modalCompletedTasks');
            const modalTotalTime = document.getElementById('modalTotalTime');
            const modalTodoList = document.getElementById('modalTodoList');
            
            // 清空任务列表
            modalTodoList.innerHTML = '';
            
            // 格式化日期显示
            const dateObj = new Date(date);
            const formattedDate = dateObj.toLocaleDateString('zh-CN', {
                year: 'numeric',
                month: 'long',
                day: 'numeric',
                weekday: 'long'
            });
            
            // 设置模态框标题
            modalDate.textContent = formattedDate;
            
            // 计算统计数据
            let totalTasks = todos.length;
            let completedTasks = 0;
            let totalHours = 0;
            let totalMinutes = 0;
            
            // 处理任务数据
            todos.forEach(todo => {
                // 统计已完成任务
                if (todo.completed) {
                    completedTasks++;
                }
                
                // 累计总时间
                totalHours += parseInt(todo.hours) || 0;
                totalMinutes += parseInt(todo.minutes) || 0;
                
                // 创建任务项
                const todoItem = document.createElement('div');
                todoItem.className = `modal-todo-item ${todo.completed ? 'completed' : ''}`;
                
                // 计算时间显示
                const hours = parseInt(todo.hours) || 0;
                const minutes = parseInt(todo.minutes) || 0;
                const timeDisplay = (hours > 0 || minutes > 0) ? 
                    formatTime(hours, minutes) : '未设置时间';
                    
                // 设置任务内容
                todoItem.innerHTML = `
                    <input type="checkbox" ${todo.completed ? 'checked' : ''} disabled>
                    <div class="modal-todo-content">${todo.content}</div>
                    <div class="modal-todo-time">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M12 2C6.5 2 2 6.5 2 12C2 17.5 6.5 22 12 22C17.5 22 22 17.5 22 12C22 6.5 17.5 2 12 2ZM12 20C7.59 20 4 16.41 4 12C4 7.59 7.59 4 12 4C16.41 4 20 7.59 20 12C20 16.41 16.41 20 12 20ZM12.5 7H11V13L16.2 16.2L17 14.9L12.5 12.2V7Z" fill="currentColor"/>
                        </svg>
                        ${timeDisplay}
                    </div>
                `;
                
                // 添加到列表
                modalTodoList.appendChild(todoItem);
            });
            
            // 处理时间溢出
            totalHours += Math.floor(totalMinutes / 60);
            totalMinutes = totalMinutes % 60;
            
            // 更新统计显示
            modalTotalTasks.textContent = totalTasks;
            modalCompletedTasks.textContent = completedTasks;
            modalTotalTime.textContent = `${totalHours}小时${totalMinutes}分钟`;
            
            // 显示模态框
            modal.style.display = 'block';
        }

        // 从历史记录或API加载特定日期的任务
        function fetchDailyTodos(date) {
            // 先检查本地历史记录中是否有数据
            if (localStorage.getItem('todoHistory')) {
                try {
                    const history = JSON.parse(localStorage.getItem('todoHistory'));
                    if (history[date]) {
                        const todos = history[date].todos || history[date].items || history[date];
                        if (Array.isArray(todos) && todos.length > 0) {
                            // 显示本地数据
                            showDailyTodoModal(date, todos);
                            return; // 找到了本地数据，直接返回
                        }
                    }
                } catch (e) {
                    console.error("Failed to parse local history:", e);
                }
            }
            
            // 如果本地没有数据，从服务器获取
            const url = `/api/todos?date=${date}`;
            
            const xhr = new XMLHttpRequest();
            xhr.open('GET', url, true);
            xhr.setRequestHeader('Accept', 'application/json');
            
            xhr.onreadystatechange = function() {
                if (xhr.readyState === 4) {
                    if (xhr.status === 200) {
                        try {
                            let response = JSON.parse(xhr.responseText);
                            let todos;
                            
                            // 处理响应格式
                            if (response.items && Array.isArray(response.items)) {
                                todos = response.items;
                            } else if (Array.isArray(response)) {
                                todos = response;
                            } else {
                                todos = [];
                            }
                            
                            // 显示待办事项
                            showDailyTodoModal(date, todos);
                            
                        } catch (parseError) {
                            console.error("Failed to parse todos:", parseError);
                            showToast('加载任务失败，解析错误', 'error');
                            showDailyTodoModal(date, []); // 显示空数据
                        }
                    } else {
                        showToast('无法加载该日期的任务', 'error');
                        showDailyTodoModal(date, []); // 显示空数据
                    }
                }
            };
            
            xhr.send();
        }

        // 用于内联点击处理的手动切换函数
        function toggleHistoryManually() {
            console.log('toggleHistoryManually function called');
            console.log('Current historyCollapsed state:', historyCollapsed);
            
            // 切换折叠状态
            historyCollapsed = !historyCollapsed;
            console.log('New historyCollapsed state:', historyCollapsed);
            
            // 更新UI
            updateHistoryCollapsedState();
            
            // 保存到本地存储
            localStorage.setItem('historyCollapsed', historyCollapsed);
        }