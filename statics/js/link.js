		function onSearch(){
			var match = document.getElementById('search').value;
			if (match.trim() === '') return;

			var xhr = new XMLHttpRequest();
			xhr.onreadystatechange = function() {
				if (xhr.readyState == 4 && xhr.status == 200) {
					window.location.href = xhr.responseURL;
				}
			};
			xhr.open('GET', '/search?match=' + encodeURIComponent(match), true);
			xhr.send();
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


