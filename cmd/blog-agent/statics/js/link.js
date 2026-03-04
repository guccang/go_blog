		function onSearch(){
			var match = document.getElementById('search').value;
			if (match.trim() === '') return;

			// Check if it's a reload command
			var isReloadCommand = match.toLowerCase().startsWith('@reload');

			var xhr = new XMLHttpRequest();
			xhr.onreadystatechange = function() {
				if (xhr.readyState == 4 && xhr.status == 200) {
					if (isReloadCommand) {
						// Show browser notification for reload completion
						showReloadNotification();
						// Clear the search input
						document.getElementById('search').value = '';
						// Still redirect to show the reload confirmation
						setTimeout(function() {
							window.location.href = xhr.responseURL;
						}, 1000);
					} else {
						window.location.href = xhr.responseURL;
					}
				}
			};
			xhr.open('GET', '/search?match=' + encodeURIComponent(match), true);
			xhr.send();
		}

		function showReloadNotification() {
			// Try to use browser notification API
			if ("Notification" in window) {
				if (Notification.permission === "granted") {
					new Notification("系统重新加载完成", {
						body: "配置文件已重新加载完成！",
						icon: "/statics/logo/favicon.ico"
					});
				} else if (Notification.permission !== "denied") {
					Notification.requestPermission().then(function(permission) {
						if (permission === "granted") {
							new Notification("系统重新加载完成", {
								body: "配置文件已重新加载完成！",
								icon: "/statics/logo/favicon.ico"
							});
						}
					});
				}
			}
			
			// Fallback: show a toast notification
			if (typeof showToast === 'function') {
				showToast('系统重新加载完成！', 'success');
			} else {
				// Simple alert as last resort
				alert('系统重新加载完成！');
			}
		}

		PageHistoryBack()

		document.addEventListener('keydown', function(event) {
			if (event.key === "Enter") {
				event.preventDefault();
				onSearch()
			}
		});

		let isGridView = true;

		function toggleView() {
			const container = document.querySelector('.container');
			const viewIcon = document.getElementById('view-icon');
			const viewText = document.getElementById('view-text');
			
			isGridView = !isGridView;
			container.classList.toggle('list-view');
			
			if (isGridView) {
				viewIcon.textContent = '📑';
				viewText.textContent = '网格视图';
			} else {
				viewIcon.textContent = '📋';
				viewText.textContent = '列表视图';
			}
			
			// Save preference to localStorage
			localStorage.setItem('blogViewPreference', isGridView ? 'grid' : 'list');
		}

		// Load saved preference on page load
		document.addEventListener('DOMContentLoaded', function() {
			const savedView = localStorage.getItem('blogViewPreference');
			if (savedView === 'list') {
				toggleView();
			}
			
			// 设置圆形头像中的首字符
			const titleSpans = document.querySelectorAll('.circle-text');
			titleSpans.forEach(span => {
				const titleText = span.getAttribute('data-title');
				if (titleText && titleText.length > 0) {
					// 提取第一个字符，适用于英文和中文
					span.textContent = titleText.charAt(0);
				}
			});
			
			// Add animation for link cards on page load
			const cards = document.querySelectorAll('.link-card');
			cards.forEach((card, index) => {
				card.style.opacity = '0';
				card.style.transform = 'translateY(20px)';
				setTimeout(() => {
					card.style.transition = 'opacity 0.5s ease, transform 0.5s ease';
					card.style.opacity = '1';
					card.style.transform = 'translateY(0)';
				}, 100 * index);
			});
		});

		function navigateToTodolist(event) {
			event.preventDefault();
			const today = new Date().toISOString().split('T')[0];
			window.location.href = `/todolist?date=${today}`;
		}


