// OwlMail Web Application
// API Base URL - 使用新的 API v1 端点
const API_BASE = `${window.location.origin}/api/v1`;

// Internationalization (i18n)
const i18n = {
    'zh-CN': {
        title: 'OwlMail - 邮件开发测试工具',
        refresh: '刷新',
        markAllRead: '标记全部已读',
        deleteAll: '删除全部',
        searchPlaceholder: '搜索邮件...',
        search: '搜索',
        emailList: '邮件列表',
        emailCount: '{count} 封邮件',
        loading: '加载中...',
        noEmails: '暂无邮件',
        selectEmail: '选择一个邮件查看详情',
        unknown: '未知',
        noSubject: '(无主题)',
        attachments: '{count} 个附件',
        downloadEml: '下载 .eml',
        viewSource: '查看源码',
        delete: '删除',
        from: '发件人:',
        to: '收件人:',
        cc: '抄送:',
        time: '时间:',
        attachmentsTitle: '附件 ({count})',
        download: '下载',
        prevPage: '上一页',
        nextPage: '下一页',
        pageInfo: '第 {current} 页 / 共 {total} 页',
        confirmTitle: '确认操作',
        confirm: '确认',
        cancel: '取消',
        deleteConfirm: '确定要删除这封邮件吗？',
        deleteAllConfirm: '确定要删除所有邮件吗？此操作不可恢复！',
        markAllReadSuccess: '已标记 {count} 封邮件为已读',
        loadEmailsError: '加载邮件失败: {error}',
        loadEmailDetailError: '加载邮件详情失败: {error}',
        deleteEmailError: '删除邮件失败: {error}',
        deleteAllEmailsError: '删除所有邮件失败: {error}',
        markAllReadError: '标记失败: {error}',
        justNow: '刚刚',
        minutesAgo: '{minutes} 分钟前',
        hoursAgo: '{hours} 小时前',
        daysAgo: '{days} 天前',
        toggleTheme: '切换主题',
        switchLanguage: '切换语言'
    },
    'en': {
        title: 'OwlMail - Email Development Testing Tool',
        refresh: 'Refresh',
        markAllRead: 'Mark All Read',
        deleteAll: 'Delete All',
        searchPlaceholder: 'Search emails...',
        search: 'Search',
        emailList: 'Email List',
        emailCount: '{count} emails',
        loading: 'Loading...',
        noEmails: 'No emails',
        selectEmail: 'Select an email to view details',
        unknown: 'Unknown',
        noSubject: '(No Subject)',
        attachments: '{count} attachments',
        downloadEml: 'Download .eml',
        viewSource: 'View Source',
        delete: 'Delete',
        from: 'From:',
        to: 'To:',
        cc: 'CC:',
        time: 'Time:',
        attachmentsTitle: 'Attachments ({count})',
        download: 'Download',
        prevPage: 'Previous',
        nextPage: 'Next',
        pageInfo: 'Page {current} of {total}',
        confirmTitle: 'Confirm Action',
        confirm: 'Confirm',
        cancel: 'Cancel',
        deleteConfirm: 'Are you sure you want to delete this email?',
        deleteAllConfirm: 'Are you sure you want to delete all emails? This action cannot be undone!',
        markAllReadSuccess: 'Marked {count} emails as read',
        loadEmailsError: 'Failed to load emails: {error}',
        loadEmailDetailError: 'Failed to load email details: {error}',
        deleteEmailError: 'Failed to delete email: {error}',
        deleteAllEmailsError: 'Failed to delete all emails: {error}',
        markAllReadError: 'Failed to mark as read: {error}',
        justNow: 'Just now',
        minutesAgo: '{minutes} minutes ago',
        hoursAgo: '{hours} hours ago',
        daysAgo: '{days} days ago',
        toggleTheme: 'Toggle Theme',
        switchLanguage: 'Switch Language'
    }
};

// Current language
let currentLang = 'en';

// Detect browser language
function detectLanguage() {
    // Check localStorage first
    const savedLang = localStorage.getItem('language');
    if (savedLang && i18n[savedLang]) {
        return savedLang;
    }
    
    // Detect from browser
    const browserLang = navigator.language || navigator.userLanguage;
    if (browserLang) {
        // Check exact match
        if (i18n[browserLang]) {
            return browserLang;
        }
        // Check language code (e.g., 'zh' from 'zh-CN')
        const langCode = browserLang.split('-')[0];
        if (langCode === 'zh') {
            return 'zh-CN';
        }
        if (langCode === 'en') {
            return 'en';
        }
    }
    
    // Default to English
    return 'en';
}

// Translation function
function t(key, params = {}) {
    const translation = i18n[currentLang][key] || i18n['en'][key] || key;
    return translation.replace(/\{(\w+)\}/g, (match, paramKey) => {
        return params[paramKey] !== undefined ? params[paramKey] : match;
    });
}

// Set language
function setLanguage(lang) {
    if (!i18n[lang]) {
        lang = 'en';
    }
    currentLang = lang;
    localStorage.setItem('language', lang);
    document.documentElement.lang = lang;
    updateUI();
}

// Global State
let state = {
    emails: [],
    currentEmail: null,
    currentPage: 0,
    pageSize: 50,
    total: 0,
    searchQuery: '',
    ws: null
};

// API Functions - 使用新的 RESTful API 设计
const API = {
    async getEmails(offset = 0, limit = 50, query = '') {
        const params = new URLSearchParams({
            offset: offset.toString(),
            limit: limit.toString()
        });
        if (query) {
            params.append('q', query);
        }
        const response = await fetch(`${API_BASE}/emails?${params}`);
        if (!response.ok) throw new Error('Failed to fetch emails');
        return await response.json();
    },

    async getEmail(id) {
        const response = await fetch(`${API_BASE}/emails/${id}`);
        if (!response.ok) throw new Error('Failed to fetch email');
        return await response.json();
    },

    async getEmailHTML(id) {
        const response = await fetch(`${API_BASE}/emails/${id}/html`);
        if (!response.ok) throw new Error('Failed to fetch email HTML');
        return await response.text();
    },

    async deleteEmail(id) {
        const response = await fetch(`${API_BASE}/emails/${id}`, {
            method: 'DELETE'
        });
        if (!response.ok) throw new Error('Failed to delete email');
        return await response.json();
    },

    async deleteAllEmails() {
        const response = await fetch(`${API_BASE}/emails`, {
            method: 'DELETE'
        });
        if (!response.ok) throw new Error('Failed to delete all emails');
        return await response.json();
    },

    async markAllRead() {
        const response = await fetch(`${API_BASE}/emails/read`, {
            method: 'PATCH'
        });
        if (!response.ok) throw new Error('Failed to mark all as read');
        return await response.json();
    },

    async relayEmail(id, relayTo = '') {
        const url = relayTo 
            ? `${API_BASE}/emails/${id}/actions/relay/${encodeURIComponent(relayTo)}`
            : `${API_BASE}/emails/${id}/actions/relay`;
        const response = await fetch(url, {
            method: 'POST'
        });
        if (!response.ok) throw new Error('Failed to relay email');
        return await response.json();
    }
};

// WebSocket Connection - 使用新的 API v1 WebSocket 端点
function connectWebSocket() {
    try {
        // Use ws:// or wss:// based on current protocol
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/api/v1/ws`;
        const ws = new WebSocket(wsUrl);
        
        ws.onopen = () => {
            console.log('WebSocket connected');
        };

        ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                handleWebSocketMessage(data);
            } catch (e) {
                console.error('Failed to parse WebSocket message:', e);
            }
        };

        ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };

        ws.onclose = () => {
            console.log('WebSocket disconnected, reconnecting...');
            setTimeout(connectWebSocket, 3000);
        };

        state.ws = ws;
    } catch (e) {
        console.error('Failed to connect WebSocket:', e);
        // Retry after 3 seconds
        setTimeout(connectWebSocket, 3000);
    }
}

function handleWebSocketMessage(data) {
    if (data.type === 'new') {
        // Add new email to the list
        state.emails.unshift(data.email);
        state.total++;
        renderEmailList();
        updateEmailCount();
    } else if (data.type === 'delete') {
        // Remove deleted email from the list
        state.emails = state.emails.filter(e => e.id !== data.id);
        state.total--;
        renderEmailList();
        updateEmailCount();
        if (state.currentEmail && state.currentEmail.id === data.id) {
            state.currentEmail = null;
            renderEmailDetail();
        }
    }
}

// Update UI with current language
function updateUI() {
    // Update title
    document.title = t('title');
    
    // Update header buttons
    const refreshBtn = document.getElementById('refreshBtn');
    if (refreshBtn) refreshBtn.textContent = t('refresh');
    
    const markAllReadBtn = document.getElementById('markAllReadBtn');
    if (markAllReadBtn) markAllReadBtn.textContent = t('markAllRead');
    
    const deleteAllBtn = document.getElementById('deleteAllBtn');
    if (deleteAllBtn) deleteAllBtn.textContent = t('deleteAll');
    
    // Update search
    const searchInput = document.getElementById('searchInput');
    if (searchInput) searchInput.placeholder = t('searchPlaceholder');
    
    const searchBtn = document.getElementById('searchBtn');
    if (searchBtn) searchBtn.textContent = t('search');
    
    // Update email list header
    const emailListHeader = document.querySelector('.email-list-header h2');
    if (emailListHeader) emailListHeader.textContent = t('emailList');
    
    // Update pagination
    const prevPageBtn = document.getElementById('prevPage');
    if (prevPageBtn) prevPageBtn.textContent = t('prevPage');
    
    const nextPageBtn = document.getElementById('nextPage');
    if (nextPageBtn) nextPageBtn.textContent = t('nextPage');
    
    // Update theme toggle title
    const themeToggle = document.getElementById('themeToggle');
    if (themeToggle) themeToggle.title = t('toggleTheme');
    
    // Update language toggle title
    const langToggle = document.getElementById('langToggle');
    if (langToggle) langToggle.title = t('switchLanguage');
    
    // Re-render dynamic content
    updateEmailCount();
    updatePagination();
    renderEmailList();
    renderEmailDetail();
}

// UI Rendering Functions
function renderEmailList() {
    const container = document.getElementById('emailList');
    if (!container) return;

    if (state.emails.length === 0) {
        container.innerHTML = `<div class="loading">${t('noEmails')}</div>`;
        return;
    }

    container.innerHTML = state.emails.map(email => {
        const from = email.from && email.from.length > 0 
            ? formatAddress(email.from[0])
            : t('unknown');
        const time = formatTime(email.time);
        const preview = email.text ? email.text.substring(0, 100) : '';
        const unreadClass = email.read ? '' : 'unread';
        const selectedClass = state.currentEmail && state.currentEmail.id === email.id ? 'selected' : '';
        const attachments = email.attachments && email.attachments.length > 0
            ? `<div class="email-item-attachments">📎 ${t('attachments', { count: email.attachments.length })}</div>`
            : '';

        return `
            <div class="email-item ${unreadClass} ${selectedClass}" data-id="${email.id}">
                <div class="email-item-header">
                    <span class="email-item-from">${escapeHtml(from)}</span>
                    <span class="email-item-time">${time}</span>
                </div>
                <div class="email-item-subject">${escapeHtml(email.subject || t('noSubject'))}</div>
                ${preview ? `<div class="email-item-preview">${escapeHtml(preview)}</div>` : ''}
                ${attachments}
            </div>
        `;
    }).join('');

    // Add click handlers
    container.querySelectorAll('.email-item').forEach(item => {
        item.addEventListener('click', () => {
            const id = item.dataset.id;
            loadEmailDetail(id);
        });
    });
}

function renderEmailDetail() {
    const container = document.getElementById('emailDetail');
    if (!container) return;

    if (!state.currentEmail) {
        container.innerHTML = `<div class="empty-state"><p>${t('selectEmail')}</p></div>`;
        return;
    }

    const email = state.currentEmail;
    const from = email.from && email.from.length > 0 
        ? formatAddress(email.from[0])
        : t('unknown');
    const to = email.to && email.to.length > 0
        ? email.to.map(addr => formatAddress(addr)).join(', ')
        : t('unknown');
    const cc = email.cc && email.cc.length > 0
        ? email.cc.map(addr => formatAddress(addr)).join(', ')
        : '';
    const time = formatTime(email.time);
    const attachments = email.attachments && email.attachments.length > 0
        ? renderAttachments(email.attachments, email.id)
        : '';

    container.innerHTML = `
        <div class="email-detail-actions">
            <button class="btn btn-primary" onclick="downloadEmail('${email.id}')">${t('downloadEml')}</button>
            <button class="btn btn-secondary" onclick="viewEmailSource('${email.id}')">${t('viewSource')}</button>
            <button class="btn btn-danger" onclick="deleteEmail('${email.id}')">${t('delete')}</button>
        </div>
        <div class="email-detail-header">
            <h2 class="email-detail-subject">${escapeHtml(email.subject || t('noSubject'))}</h2>
            <div class="email-detail-meta">
                <span class="email-detail-meta-label">${t('from')}</span>
                <span>${escapeHtml(from)}</span>
                <span class="email-detail-meta-label">${t('to')}</span>
                <span>${escapeHtml(to)}</span>
                ${cc ? `
                    <span class="email-detail-meta-label">${t('cc')}</span>
                    <span>${escapeHtml(cc)}</span>
                ` : ''}
                <span class="email-detail-meta-label">${t('time')}</span>
                <span>${time}</span>
            </div>
        </div>
        <div class="email-detail-body">
            ${email.html ? renderHTML(email.html) : renderText(email.text || '')}
        </div>
        ${attachments}
    `;
}

function renderHTML(html) {
    // Create a safe iframe for HTML content
    const iframeId = 'email-html-' + Date.now();
    return `
        <div class="email-detail-html">
            <iframe id="${iframeId}" srcdoc="${escapeHtml(html)}"></iframe>
        </div>
    `;
}

function renderText(text) {
    return `<div class="email-detail-text">${escapeHtml(text)}</div>`;
}

function renderAttachments(attachments, emailId) {
    return `
        <div class="email-detail-attachments">
            <h3>${t('attachmentsTitle', { count: attachments.length })}</h3>
            ${attachments.map(att => {
                // 使用新的 API v1 端点：/api/v1/emails/:id/attachments/:filename
                const url = `${API_BASE}/emails/${emailId}/attachments/${encodeURIComponent(att.generatedFileName)}`;
                return `
                    <div class="attachment-item">
                        <div class="attachment-item-info">
                            <div class="attachment-item-name">${escapeHtml(att.fileName || att.generatedFileName)}</div>
                            <div class="attachment-item-size">${att.sizeHuman || formatBytes(att.size || 0)}</div>
                        </div>
                        <a href="${url}" class="attachment-item-download" download>${t('download')}</a>
                    </div>
                `;
            }).join('')}
        </div>
    `;
}

// Action Functions
async function loadEmails() {
    try {
        showLoading();
        const data = await API.getEmails(
            state.currentPage * state.pageSize,
            state.pageSize,
            state.searchQuery
        );
        state.emails = data.emails || [];
        state.total = data.total || 0;
        renderEmailList();
        updateEmailCount();
        updatePagination();
    } catch (error) {
        console.error('Failed to load emails:', error);
        alert(t('loadEmailsError', { error: error.message }));
    } finally {
        hideLoading();
    }
}

async function loadEmailDetail(id) {
    try {
        showLoading();
        const email = await API.getEmail(id);
        state.currentEmail = email;
        renderEmailDetail();
        renderEmailList(); // Update selected state
    } catch (error) {
        console.error('Failed to load email detail:', error);
        alert(t('loadEmailDetailError', { error: error.message }));
    } finally {
        hideLoading();
    }
}

async function deleteEmail(id) {
    if (!confirm(t('deleteConfirm'))) return;

    try {
        showLoading();
        await API.deleteEmail(id);
        // Remove from list
        state.emails = state.emails.filter(e => e.id !== id);
        state.total--;
        if (state.currentEmail && state.currentEmail.id === id) {
            state.currentEmail = null;
            renderEmailDetail();
        }
        renderEmailList();
        updateEmailCount();
    } catch (error) {
        console.error('Failed to delete email:', error);
        alert(t('deleteEmailError', { error: error.message }));
    } finally {
        hideLoading();
    }
}

async function deleteAllEmails() {
    if (!confirm(t('deleteAllConfirm'))) return;

    try {
        showLoading();
        await API.deleteAllEmails();
        state.emails = [];
        state.total = 0;
        state.currentEmail = null;
        renderEmailList();
        renderEmailDetail();
        updateEmailCount();
    } catch (error) {
        console.error('Failed to delete all emails:', error);
        alert(t('deleteAllEmailsError', { error: error.message }));
    } finally {
        hideLoading();
    }
}

async function markAllRead() {
    try {
        showLoading();
        const result = await API.markAllRead();
        // Reload emails to update read status
        await loadEmails();
        alert(t('markAllReadSuccess', { count: result.count || 0 }));
    } catch (error) {
        console.error('Failed to mark all as read:', error);
        alert(t('markAllReadError', { error: error.message }));
    } finally {
        hideLoading();
    }
}

function downloadEmail(id) {
    // 使用新的 API v1 端点：/api/v1/emails/:id/raw (替代 /download)
    window.open(`${API_BASE}/emails/${id}/raw`, '_blank');
}

function viewEmailSource(id) {
    // 使用新的 API v1 端点：/api/v1/emails/:id/source
    window.open(`${API_BASE}/emails/${id}/source`, '_blank');
}

function searchEmails() {
    const query = document.getElementById('searchInput').value.trim();
    state.searchQuery = query;
    state.currentPage = 0;
    loadEmails();
}

function nextPage() {
    const maxPage = Math.ceil(state.total / state.pageSize) - 1;
    if (state.currentPage < maxPage) {
        state.currentPage++;
        loadEmails();
    }
}

function prevPage() {
    if (state.currentPage > 0) {
        state.currentPage--;
        loadEmails();
    }
}

// Utility Functions
function formatTime(timeStr) {
    if (!timeStr) return '';
    const date = new Date(timeStr);
    const now = new Date();
    const diff = now - date;
    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (days > 0) {
        return t('daysAgo', { days });
    } else if (hours > 0) {
        return t('hoursAgo', { hours });
    } else if (minutes > 0) {
        return t('minutesAgo', { minutes });
    } else {
        return t('justNow');
    }
}

function formatAddress(addr) {
    if (typeof addr === 'string') return addr;
    
    // 支持大小写两种字段名格式（Name/Address 或 name/address）
    const name = addr.Name || addr.name || '';
    const address = addr.Address || addr.address || '';
    
    // 如果名称和地址都存在，显示为 "名称 <地址>"
    if (name && address) {
        return `${name} <${address}>`;
    }
    // 如果只有地址，只显示地址
    if (address) {
        return address;
    }
    // 如果只有名称，只显示名称
    if (name) {
        return name;
    }
    // 两者都为空时显示"未知"
    return t('unknown');
}

function formatBytes(bytes) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function updateEmailCount() {
    const countEl = document.getElementById('emailCount');
    if (countEl) {
        countEl.textContent = t('emailCount', { count: state.total });
    }
}

function updatePagination() {
    const pageInfo = document.getElementById('pageInfo');
    const maxPage = Math.ceil(state.total / state.pageSize) - 1;
    if (pageInfo) {
        pageInfo.textContent = t('pageInfo', { current: state.currentPage + 1, total: maxPage + 1 });
    }

    const prevBtn = document.getElementById('prevPage');
    const nextBtn = document.getElementById('nextPage');
    if (prevBtn) prevBtn.disabled = state.currentPage === 0;
    if (nextBtn) nextBtn.disabled = state.currentPage >= maxPage;
}

function showLoading() {
    const overlay = document.getElementById('loadingOverlay');
    if (overlay) overlay.style.display = 'flex';
}

function hideLoading() {
    const overlay = document.getElementById('loadingOverlay');
    if (overlay) overlay.style.display = 'none';
}

// Theme Management
function initTheme() {
    const savedTheme = localStorage.getItem('theme') || 'light';
    setTheme(savedTheme);
}

function setTheme(theme) {
    const body = document.body;
    const themeToggle = document.getElementById('themeToggle');
    
    if (theme === 'dark') {
        body.classList.remove('light-theme');
        body.classList.add('dark-theme');
        if (themeToggle) themeToggle.textContent = '☀️';
    } else {
        body.classList.remove('dark-theme');
        body.classList.add('light-theme');
        if (themeToggle) themeToggle.textContent = '🌙';
    }
    
    localStorage.setItem('theme', theme);
}

function toggleTheme() {
    const currentTheme = localStorage.getItem('theme') || 'light';
    const newTheme = currentTheme === 'light' ? 'dark' : 'light';
    setTheme(newTheme);
}

// Language toggle function
function toggleLanguage() {
    const newLang = currentLang === 'zh-CN' ? 'en' : 'zh-CN';
    setLanguage(newLang);
}

// Event Listeners
document.addEventListener('DOMContentLoaded', () => {
    // Initialize language
    currentLang = detectLanguage();
    setLanguage(currentLang);
    
    // Initialize theme
    initTheme();

    // Load initial emails
    loadEmails();

    // Connect WebSocket
    connectWebSocket();

    // Button event listeners
    document.getElementById('refreshBtn').addEventListener('click', loadEmails);
    document.getElementById('markAllReadBtn').addEventListener('click', markAllRead);
    document.getElementById('deleteAllBtn').addEventListener('click', deleteAllEmails);
    document.getElementById('searchBtn').addEventListener('click', searchEmails);
    document.getElementById('prevPage').addEventListener('click', prevPage);
    document.getElementById('nextPage').addEventListener('click', nextPage);
    document.getElementById('themeToggle').addEventListener('click', toggleTheme);
    document.getElementById('langToggle').addEventListener('click', toggleLanguage);

    // Search input enter key
    document.getElementById('searchInput').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            searchEmails();
        }
    });
});

// Make functions available globally for onclick handlers
window.deleteEmail = deleteEmail;
window.downloadEmail = downloadEmail;
window.viewEmailSource = viewEmailSource;
window.t = t; // Make translation function available globally

