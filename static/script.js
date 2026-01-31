document.addEventListener('DOMContentLoaded', () => {
    const fileListDiv = document.getElementById('file-list');
    const loadingText = document.getElementById('loading-text');
    const mergeBtn = document.getElementById('merge-btn');
    const statusDiv = document.getElementById('status');
    const selectedListDiv = document.getElementById('selected-list');

    let selectedFiles = [];

    // 1. 获取并渲染列表
    async function fetchAndRenderVideos() {
        try {
            const response = await fetch('/api/videos');
            const allFiles = await response.json();
            loadingText.style.display = 'none';

            if (!allFiles || allFiles.length === 0) {
                fileListDiv.innerHTML = '<p>未找到 mp4/flv/ts 文件</p>';
                return;
            }

            // 按文件夹分组
            const filesByDir = {};
            allFiles.forEach(path => {
                const parts = path.split('/');
                const fileName = parts.pop();
                const dir = parts.join('/') || '根目录';
                if (!filesByDir[dir]) filesByDir[dir] = [];
                filesByDir[dir].push({ path, fileName });
            });

            // 渲染 HTML
            fileListDiv.innerHTML = '';
            for (const dir in filesByDir) {
                const group = document.createElement('div');
                group.className = 'folder-group';
                
                const title = document.createElement('div');
                title.className = 'folder-name';
                title.textContent = `📂 ${dir}`;
                // 点击文件夹名折叠/展开
                title.onclick = () => {
                    const container = title.nextElementSibling;
                    container.style.display = container.style.display === 'none' ? 'block' : 'none';
                };
                group.appendChild(title);

                const container = document.createElement('div');
                filesByDir[dir].forEach(file => {
                    const label = document.createElement('label');
                    label.className = 'file-item';
                    const cb = document.createElement('input');
                    cb.type = 'checkbox';
                    cb.value = file.path;
                    cb.checked = selectedFiles.includes(file.path);
                    
                    label.appendChild(cb);
                    label.appendChild(document.createTextNode(file.fileName));
                    container.appendChild(label);
                });
                group.appendChild(container);
                fileListDiv.appendChild(group);
            }
        } catch (e) {
            loadingText.textContent = "加载失败: " + e.message;
        }
    }

    // 2. 更新右侧选中列表
    function updateSelectedView() {
        selectedListDiv.innerHTML = '';
        if (selectedFiles.length === 0) {
            selectedListDiv.innerHTML = '<p class="empty-text">请从左侧勾选视频</p>';
        } else {
            selectedFiles.forEach(path => {
                const item = document.createElement('div');
                item.className = 'selected-item';
                item.draggable = true;
                item.dataset.path = path;
                item.textContent = path.split('/').pop();
                selectedListDiv.appendChild(item);
            });
        }
        mergeBtn.disabled = selectedFiles.length < 2;
        mergeBtn.textContent = selectedFiles.length < 2 ? '请至少选2个文件' : `合并 (${selectedFiles.length})`;
    }

    // 3. 监听复选框点击
    fileListDiv.addEventListener('change', (e) => {
        if (e.target.type === 'checkbox') {
            const val = e.target.value;
            if (e.target.checked) {
                if (!selectedFiles.includes(val)) selectedFiles.push(val);
            } else {
                selectedFiles = selectedFiles.filter(f => f !== val);
            }
            updateSelectedView();
        }
    });

    // 4. 拖拽排序逻辑
    let dragSrcEl = null;
    selectedListDiv.addEventListener('dragstart', e => {
        dragSrcEl = e.target;
        e.target.classList.add('dragging');
    });
    selectedListDiv.addEventListener('dragend', e => {
        e.target.classList.remove('dragging');
        // 更新数组顺序
        const newOrder = [];
        document.querySelectorAll('.selected-item').forEach(el => newOrder.push(el.dataset.path));
        selectedFiles = newOrder;
    });
    selectedListDiv.addEventListener('dragover', e => {
        e.preventDefault();
        const afterElement = getDragAfterElement(selectedListDiv, e.clientY);
        const dragging = document.querySelector('.dragging');
        if (afterElement == null) {
            selectedListDiv.appendChild(dragging);
        } else {
            selectedListDiv.insertBefore(dragging, afterElement);
        }
    });

    function getDragAfterElement(container, y) {
        const draggableElements = [...container.querySelectorAll('.selected-item:not(.dragging)')];
        return draggableElements.reduce((closest, child) => {
            const box = child.getBoundingClientRect();
            const offset = y - box.top - box.height / 2;
            if (offset < 0 && offset > closest.offset) return { offset: offset, element: child };
            else return closest;
        }, { offset: Number.NEGATIVE_INFINITY }).element;
    }

    // 5. 合并按钮逻辑
    mergeBtn.addEventListener('click', async () => {
        if (selectedFiles.length < 2) return;
        
        mergeBtn.disabled = true;
        mergeBtn.textContent = '合并中...';
        statusDiv.className = '';
        statusDiv.textContent = '⏳ 正在处理，请稍候...';

        try {
            const res = await fetch('/api/merge', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ files: selectedFiles })
            });
            const text = await res.text();
            
            if (res.ok) {
                statusDiv.className = 'success';
                statusDiv.textContent = text;
            } else {
                throw new Error(text);
            }
        } catch (e) {
            statusDiv.className = 'error';
            statusDiv.textContent = '❌ ' + e.message;
        } finally {
            // --- 优化点：完成后自动刷新列表并重置 ---
            selectedFiles = [];
            updateSelectedView();
            // 重新同步左侧复选框
            document.querySelectorAll('input[type=checkbox]').forEach(cb => cb.checked = false);
            // 刷新文件列表以显示新生成的文件
            await fetchAndRenderVideos();
            
            mergeBtn.textContent = '合并 (0)';
        }
    });

    // 初始化
    fetchAndRenderVideos();
});