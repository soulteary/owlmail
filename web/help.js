(() => {
    const supportedLanguages = new Set(['en', 'zh-CN']);
    const languageSelect = document.getElementById('helpLanguage');
    const themeButton = document.getElementById('helpTheme');

    function readPreference(key) {
        try {
            return localStorage.getItem(key);
        } catch (_) {
            return null;
        }
    }

    function savePreference(key, value) {
        try {
            localStorage.setItem(key, value);
        } catch (_) {
            // The help page still works when storage is blocked.
        }
    }

    function detectLanguage() {
        const saved = readPreference('language');
        if (saved && supportedLanguages.has(saved)) return saved;
        return (navigator.language || '').toLowerCase().startsWith('zh') ? 'zh-CN' : 'en';
    }

    function applyLanguage(language, persist = false) {
        const selected = supportedLanguages.has(language) ? language : 'en';
        document.documentElement.lang = selected;
        document.documentElement.dataset.language = selected;
        document.title = selected === 'zh-CN' ? 'OwlMail 本地帮助' : 'OwlMail Help';
        languageSelect.value = selected;
        document.querySelectorAll('.language-panel').forEach((panel) => {
            panel.hidden = panel.dataset.language !== selected;
        });
        if (persist) savePreference('language', selected);
    }

    let sessionThemePreference = null;
    function applyTheme(theme, persist = true) {
        const dark = theme === 'dark';
        if (persist) sessionThemePreference = dark ? 'dark' : 'light';
        document.body.classList.toggle('dark-theme', dark);
        document.body.classList.toggle('light-theme', !dark);
        themeButton.textContent = dark ? '☀️' : '🌙';
        themeButton.setAttribute('aria-label', dark ? 'Use light theme' : 'Use dark theme');
        if (persist) savePreference('theme', dark ? 'dark' : 'light');
    }


    function watchTheme() {
        if (window.matchMedia) {
            window.matchMedia('(prefers-color-scheme: dark)').addEventListener?.('change', () => {
                const saved = readTheme();
                if (saved !== 'dark' && saved !== 'light') applyTheme(initialTheme(), false);
            });
        }
        window.addEventListener('storage', (event) => {
            if (event.key === 'theme' || event.key === null) {
                sessionThemePreference = null;
                applyTheme(initialTheme(), false);
            }
        });
    }
    function readTheme() {
        try {
            const saved = localStorage.getItem('theme');
            return saved === 'dark' || saved === 'light' ? saved : sessionThemePreference;
        } catch (_) { return sessionThemePreference; }
    }
    function initialTheme() {
        const saved = readTheme();
        if (saved === 'dark' || saved === 'light') return saved;
        return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }

    languageSelect.addEventListener('change', (event) => applyLanguage(event.target.value, true));
    themeButton.addEventListener('click', () => {
        applyTheme(document.body.classList.contains('dark-theme') ? 'light' : 'dark');
    });

    applyLanguage(detectLanguage());
    applyTheme(initialTheme(), false);
    watchTheme();
})();
