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

    function applyTheme(theme) {
        const dark = theme === 'dark';
        document.body.classList.toggle('dark-theme', dark);
        document.body.classList.toggle('light-theme', !dark);
        themeButton.textContent = dark ? '☀️' : '🌙';
        themeButton.setAttribute('aria-label', dark ? 'Use light theme' : 'Use dark theme');
        savePreference('theme', dark ? 'dark' : 'light');
    }

    languageSelect.addEventListener('change', (event) => applyLanguage(event.target.value, true));
    themeButton.addEventListener('click', () => {
        applyTheme(document.body.classList.contains('dark-theme') ? 'light' : 'dark');
    });

    applyLanguage(detectLanguage());
    applyTheme(readPreference('theme') === 'dark' ? 'dark' : 'light');
})();
