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
        switchLanguage: '切换语言',
        // API Error Codes
        'EMAIL_NOT_FOUND': '邮件未找到',
        'EMAIL_FILE_NOT_FOUND': '邮件文件未找到',
        'NO_EMAILS_FOUND': '未找到邮件',
        'NO_EMAILS_TO_EXPORT': '没有可导出的邮件',
        'INVALID_EMAIL_ID': '无效的邮件ID',
        'NO_EMAIL_IDS_PROVIDED': '未提供邮件ID',
        'INVALID_REQUEST': '无效的请求',
        'INVALID_EMAIL_ADDRESS': '无效的邮箱地址',
        'HOST_REQUIRED': '主机地址是必需的',
        'PORT_OUT_OF_RANGE': '端口必须在1到65535之间',
        'INVALID_PORT': '无效的端口',
        'RELAY_FAILED': '转发失败',
        // API Success Codes
        'EMAIL_DELETED': '邮件已删除',
        'ALL_EMAILS_DELETED': '所有邮件已删除',
        'EMAIL_MARKED_READ': '邮件已标记为已读',
        'ALL_EMAILS_MARKED_READ': '所有邮件已标记为已读',
        'EMAIL_RELAYED': '邮件转发成功',
        'MAILS_RELOADED': '邮件重新加载成功',
        'BATCH_DELETE_COMPLETED': '批量删除完成',
        'BATCH_READ_COMPLETED': '批量标记已读完成',
        'CONFIG_UPDATED': '配置已更新'
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
        switchLanguage: 'Switch Language',
        // API Error Codes
        'EMAIL_NOT_FOUND': 'Email not found',
        'EMAIL_FILE_NOT_FOUND': 'Email file not found',
        'NO_EMAILS_FOUND': 'No emails found',
        'NO_EMAILS_TO_EXPORT': 'No emails found to export',
        'INVALID_EMAIL_ID': 'Invalid email ID',
        'NO_EMAIL_IDS_PROVIDED': 'No email IDs provided',
        'INVALID_REQUEST': 'Invalid request',
        'INVALID_EMAIL_ADDRESS': 'Invalid email address',
        'HOST_REQUIRED': 'Host is required',
        'PORT_OUT_OF_RANGE': 'Port must be between 1 and 65535',
        'INVALID_PORT': 'Invalid port',
        'RELAY_FAILED': 'Relay failed',
        // API Success Codes
        'EMAIL_DELETED': 'Email deleted',
        'ALL_EMAILS_DELETED': 'All emails deleted',
        'EMAIL_MARKED_READ': 'Email marked as read',
        'ALL_EMAILS_MARKED_READ': 'All emails marked as read',
        'EMAIL_RELAYED': 'Email relayed successfully',
        'MAILS_RELOADED': 'Mails reloaded successfully',
        'BATCH_DELETE_COMPLETED': 'Batch delete completed',
        'BATCH_READ_COMPLETED': 'Batch read completed',
        'CONFIG_UPDATED': 'Configuration updated'
    },
    'de': {
        title: 'OwlMail - E-Mail-Entwicklungstest-Tool',
        refresh: 'Aktualisieren',
        markAllRead: 'Alle als gelesen markieren',
        deleteAll: 'Alle löschen',
        searchPlaceholder: 'E-Mails suchen...',
        search: 'Suchen',
        emailList: 'E-Mail-Liste',
        emailCount: '{count} E-Mails',
        loading: 'Laden...',
        noEmails: 'Keine E-Mails',
        selectEmail: 'Wählen Sie eine E-Mail aus, um Details anzuzeigen',
        unknown: 'Unbekannt',
        noSubject: '(Kein Betreff)',
        attachments: '{count} Anhänge',
        downloadEml: '.eml herunterladen',
        viewSource: 'Quelle anzeigen',
        delete: 'Löschen',
        from: 'Von:',
        to: 'An:',
        cc: 'CC:',
        time: 'Zeit:',
        attachmentsTitle: 'Anhänge ({count})',
        download: 'Herunterladen',
        prevPage: 'Zurück',
        nextPage: 'Weiter',
        pageInfo: 'Seite {current} von {total}',
        confirmTitle: 'Aktion bestätigen',
        confirm: 'Bestätigen',
        cancel: 'Abbrechen',
        deleteConfirm: 'Möchten Sie diese E-Mail wirklich löschen?',
        deleteAllConfirm: 'Möchten Sie wirklich alle E-Mails löschen? Diese Aktion kann nicht rückgängig gemacht werden!',
        markAllReadSuccess: '{count} E-Mails als gelesen markiert',
        loadEmailsError: 'E-Mails konnten nicht geladen werden: {error}',
        loadEmailDetailError: 'E-Mail-Details konnten nicht geladen werden: {error}',
        deleteEmailError: 'E-Mail konnte nicht gelöscht werden: {error}',
        deleteAllEmailsError: 'Alle E-Mails konnten nicht gelöscht werden: {error}',
        markAllReadError: 'Als gelesen markieren fehlgeschlagen: {error}',
        justNow: 'Gerade eben',
        minutesAgo: 'vor {minutes} Minuten',
        hoursAgo: 'vor {hours} Stunden',
        daysAgo: 'vor {days} Tagen',
        toggleTheme: 'Design umschalten',
        switchLanguage: 'Sprache wechseln',
        // API Error Codes
        'EMAIL_NOT_FOUND': 'E-Mail nicht gefunden',
        'EMAIL_FILE_NOT_FOUND': 'E-Mail-Datei nicht gefunden',
        'NO_EMAILS_FOUND': 'Keine E-Mails gefunden',
        'NO_EMAILS_TO_EXPORT': 'Keine E-Mails zum Exportieren gefunden',
        'INVALID_EMAIL_ID': 'Ungültige E-Mail-ID',
        'NO_EMAIL_IDS_PROVIDED': 'Keine E-Mail-IDs angegeben',
        'INVALID_REQUEST': 'Ungültige Anfrage',
        'INVALID_EMAIL_ADDRESS': 'Ungültige E-Mail-Adresse',
        'HOST_REQUIRED': 'Host ist erforderlich',
        'PORT_OUT_OF_RANGE': 'Port muss zwischen 1 und 65535 liegen',
        'INVALID_PORT': 'Ungültiger Port',
        'RELAY_FAILED': 'Weiterleitung fehlgeschlagen',
        // API Success Codes
        'EMAIL_DELETED': 'E-Mail gelöscht',
        'ALL_EMAILS_DELETED': 'Alle E-Mails gelöscht',
        'EMAIL_MARKED_READ': 'E-Mail als gelesen markiert',
        'ALL_EMAILS_MARKED_READ': 'Alle E-Mails als gelesen markiert',
        'EMAIL_RELAYED': 'E-Mail erfolgreich weitergeleitet',
        'MAILS_RELOADED': 'E-Mails erfolgreich neu geladen',
        'BATCH_DELETE_COMPLETED': 'Batch-Löschung abgeschlossen',
        'BATCH_READ_COMPLETED': 'Batch-Lesevorgang abgeschlossen',
        'CONFIG_UPDATED': 'Konfiguration aktualisiert'
    },
    'it': {
        title: 'OwlMail - Strumento di Test per lo Sviluppo Email',
        refresh: 'Aggiorna',
        markAllRead: 'Segna Tutto come Letto',
        deleteAll: 'Elimina Tutto',
        searchPlaceholder: 'Cerca email...',
        search: 'Cerca',
        emailList: 'Elenco Email',
        emailCount: '{count} email',
        loading: 'Caricamento...',
        noEmails: 'Nessuna email',
        selectEmail: 'Seleziona un\'email per visualizzare i dettagli',
        unknown: 'Sconosciuto',
        noSubject: '(Nessun oggetto)',
        attachments: '{count} allegati',
        downloadEml: 'Scarica .eml',
        viewSource: 'Visualizza Sorgente',
        delete: 'Elimina',
        from: 'Da:',
        to: 'A:',
        cc: 'CC:',
        time: 'Ora:',
        attachmentsTitle: 'Allegati ({count})',
        download: 'Scarica',
        prevPage: 'Precedente',
        nextPage: 'Successivo',
        pageInfo: 'Pagina {current} di {total}',
        confirmTitle: 'Conferma Azione',
        confirm: 'Conferma',
        cancel: 'Annulla',
        deleteConfirm: 'Sei sicuro di voler eliminare questa email?',
        deleteAllConfirm: 'Sei sicuro di voler eliminare tutte le email? Questa azione non può essere annullata!',
        markAllReadSuccess: '{count} email contrassegnate come lette',
        loadEmailsError: 'Impossibile caricare le email: {error}',
        loadEmailDetailError: 'Impossibile caricare i dettagli dell\'email: {error}',
        deleteEmailError: 'Impossibile eliminare l\'email: {error}',
        deleteAllEmailsError: 'Impossibile eliminare tutte le email: {error}',
        markAllReadError: 'Impossibile contrassegnare come letto: {error}',
        justNow: 'Proprio ora',
        minutesAgo: '{minutes} minuti fa',
        hoursAgo: '{hours} ore fa',
        daysAgo: '{days} giorni fa',
        toggleTheme: 'Cambia Tema',
        switchLanguage: 'Cambia Lingua',
        // API Error Codes
        'EMAIL_NOT_FOUND': 'Email non trovata',
        'EMAIL_FILE_NOT_FOUND': 'File email non trovato',
        'NO_EMAILS_FOUND': 'Nessuna email trovata',
        'NO_EMAILS_TO_EXPORT': 'Nessuna email da esportare',
        'INVALID_EMAIL_ID': 'ID email non valido',
        'NO_EMAIL_IDS_PROVIDED': 'Nessun ID email fornito',
        'INVALID_REQUEST': 'Richiesta non valida',
        'INVALID_EMAIL_ADDRESS': 'Indirizzo email non valido',
        'HOST_REQUIRED': 'Host richiesto',
        'PORT_OUT_OF_RANGE': 'La porta deve essere compresa tra 1 e 65535',
        'INVALID_PORT': 'Porta non valida',
        'RELAY_FAILED': 'Inoltro fallito',
        // API Success Codes
        'EMAIL_DELETED': 'Email eliminata',
        'ALL_EMAILS_DELETED': 'Tutte le email eliminate',
        'EMAIL_MARKED_READ': 'Email contrassegnata come letta',
        'ALL_EMAILS_MARKED_READ': 'Tutte le email contrassegnate come lette',
        'EMAIL_RELAYED': 'Email inoltrata con successo',
        'MAILS_RELOADED': 'Email ricaricate con successo',
        'BATCH_DELETE_COMPLETED': 'Eliminazione batch completata',
        'BATCH_READ_COMPLETED': 'Lettura batch completata',
        'CONFIG_UPDATED': 'Configurazione aggiornata'
    },
    'fr': {
        title: 'OwlMail - Outil de Test de Développement Email',
        refresh: 'Actualiser',
        markAllRead: 'Tout Marquer comme Lu',
        deleteAll: 'Tout Supprimer',
        searchPlaceholder: 'Rechercher des emails...',
        search: 'Rechercher',
        emailList: 'Liste des Emails',
        emailCount: '{count} emails',
        loading: 'Chargement...',
        noEmails: 'Aucun email',
        selectEmail: 'Sélectionnez un email pour voir les détails',
        unknown: 'Inconnu',
        noSubject: '(Sans objet)',
        attachments: '{count} pièces jointes',
        downloadEml: 'Télécharger .eml',
        viewSource: 'Voir la Source',
        delete: 'Supprimer',
        from: 'De:',
        to: 'À:',
        cc: 'CC:',
        time: 'Heure:',
        attachmentsTitle: 'Pièces jointes ({count})',
        download: 'Télécharger',
        prevPage: 'Précédent',
        nextPage: 'Suivant',
        pageInfo: 'Page {current} sur {total}',
        confirmTitle: 'Confirmer l\'Action',
        confirm: 'Confirmer',
        cancel: 'Annuler',
        deleteConfirm: 'Êtes-vous sûr de vouloir supprimer cet email?',
        deleteAllConfirm: 'Êtes-vous sûr de vouloir supprimer tous les emails? Cette action ne peut pas être annulée!',
        markAllReadSuccess: '{count} emails marqués comme lus',
        loadEmailsError: 'Échec du chargement des emails: {error}',
        loadEmailDetailError: 'Échec du chargement des détails de l\'email: {error}',
        deleteEmailError: 'Échec de la suppression de l\'email: {error}',
        deleteAllEmailsError: 'Échec de la suppression de tous les emails: {error}',
        markAllReadError: 'Échec du marquage comme lu: {error}',
        justNow: 'À l\'instant',
        minutesAgo: 'il y a {minutes} minutes',
        hoursAgo: 'il y a {hours} heures',
        daysAgo: 'il y a {days} jours',
        toggleTheme: 'Changer le Thème',
        switchLanguage: 'Changer la Langue',
        // API Error Codes
        'EMAIL_NOT_FOUND': 'Email introuvable',
        'EMAIL_FILE_NOT_FOUND': 'Fichier email introuvable',
        'NO_EMAILS_FOUND': 'Aucun email trouvé',
        'NO_EMAILS_TO_EXPORT': 'Aucun email à exporter',
        'INVALID_EMAIL_ID': 'ID email invalide',
        'NO_EMAIL_IDS_PROVIDED': 'Aucun ID email fourni',
        'INVALID_REQUEST': 'Requête invalide',
        'INVALID_EMAIL_ADDRESS': 'Adresse email invalide',
        'HOST_REQUIRED': 'Hôte requis',
        'PORT_OUT_OF_RANGE': 'Le port doit être entre 1 et 65535',
        'INVALID_PORT': 'Port invalide',
        'RELAY_FAILED': 'Relais échoué',
        // API Success Codes
        'EMAIL_DELETED': 'Email supprimé',
        'ALL_EMAILS_DELETED': 'Tous les emails supprimés',
        'EMAIL_MARKED_READ': 'Email marqué comme lu',
        'ALL_EMAILS_MARKED_READ': 'Tous les emails marqués comme lus',
        'EMAIL_RELAYED': 'Email relayé avec succès',
        'MAILS_RELOADED': 'Emails rechargés avec succès',
        'BATCH_DELETE_COMPLETED': 'Suppression par lot terminée',
        'BATCH_READ_COMPLETED': 'Lecture par lot terminée',
        'CONFIG_UPDATED': 'Configuration mise à jour'
    },
    'ko': {
        title: 'OwlMail - 이메일 개발 테스트 도구',
        refresh: '새로고침',
        markAllRead: '모두 읽음으로 표시',
        deleteAll: '모두 삭제',
        searchPlaceholder: '이메일 검색...',
        search: '검색',
        emailList: '이메일 목록',
        emailCount: '{count}개의 이메일',
        loading: '로딩 중...',
        noEmails: '이메일 없음',
        selectEmail: '이메일을 선택하여 세부 정보 보기',
        unknown: '알 수 없음',
        noSubject: '(제목 없음)',
        attachments: '{count}개의 첨부파일',
        downloadEml: '.eml 다운로드',
        viewSource: '소스 보기',
        delete: '삭제',
        from: '보낸 사람:',
        to: '받는 사람:',
        cc: '참조:',
        time: '시간:',
        attachmentsTitle: '첨부파일 ({count})',
        download: '다운로드',
        prevPage: '이전',
        nextPage: '다음',
        pageInfo: '{current}페이지 / 총 {total}페이지',
        confirmTitle: '작업 확인',
        confirm: '확인',
        cancel: '취소',
        deleteConfirm: '이 이메일을 삭제하시겠습니까?',
        deleteAllConfirm: '모든 이메일을 삭제하시겠습니까? 이 작업은 되돌릴 수 없습니다!',
        markAllReadSuccess: '{count}개의 이메일을 읽음으로 표시했습니다',
        loadEmailsError: '이메일 로드 실패: {error}',
        loadEmailDetailError: '이메일 세부 정보 로드 실패: {error}',
        deleteEmailError: '이메일 삭제 실패: {error}',
        deleteAllEmailsError: '모든 이메일 삭제 실패: {error}',
        markAllReadError: '읽음 표시 실패: {error}',
        justNow: '방금',
        minutesAgo: '{minutes}분 전',
        hoursAgo: '{hours}시간 전',
        daysAgo: '{days}일 전',
        toggleTheme: '테마 전환',
        switchLanguage: '언어 전환',
        // API Error Codes
        'EMAIL_NOT_FOUND': '이메일을 찾을 수 없습니다',
        'EMAIL_FILE_NOT_FOUND': '이메일 파일을 찾을 수 없습니다',
        'NO_EMAILS_FOUND': '이메일을 찾을 수 없습니다',
        'NO_EMAILS_TO_EXPORT': '내보낼 이메일이 없습니다',
        'INVALID_EMAIL_ID': '잘못된 이메일 ID',
        'NO_EMAIL_IDS_PROVIDED': '이메일 ID가 제공되지 않았습니다',
        'INVALID_REQUEST': '잘못된 요청',
        'INVALID_EMAIL_ADDRESS': '잘못된 이메일 주소',
        'HOST_REQUIRED': '호스트가 필요합니다',
        'PORT_OUT_OF_RANGE': '포트는 1에서 65535 사이여야 합니다',
        'INVALID_PORT': '잘못된 포트',
        'RELAY_FAILED': '전달 실패',
        // API Success Codes
        'EMAIL_DELETED': '이메일이 삭제되었습니다',
        'ALL_EMAILS_DELETED': '모든 이메일이 삭제되었습니다',
        'EMAIL_MARKED_READ': '이메일이 읽음으로 표시되었습니다',
        'ALL_EMAILS_MARKED_READ': '모든 이메일이 읽음으로 표시되었습니다',
        'EMAIL_RELAYED': '이메일이 성공적으로 전달되었습니다',
        'MAILS_RELOADED': '이메일이 성공적으로 다시 로드되었습니다',
        'BATCH_DELETE_COMPLETED': '일괄 삭제가 완료되었습니다',
        'BATCH_READ_COMPLETED': '일괄 읽기 표시가 완료되었습니다',
        'CONFIG_UPDATED': '설정이 업데이트되었습니다'
    },
    'ja': {
        title: 'OwlMail - メール開発テストツール',
        refresh: '更新',
        markAllRead: 'すべて既読にする',
        deleteAll: 'すべて削除',
        searchPlaceholder: 'メールを検索...',
        search: '検索',
        emailList: 'メール一覧',
        emailCount: '{count}通のメール',
        loading: '読み込み中...',
        noEmails: 'メールなし',
        selectEmail: 'メールを選択して詳細を表示',
        unknown: '不明',
        noSubject: '(件名なし)',
        attachments: '{count}個の添付ファイル',
        downloadEml: '.emlをダウンロード',
        viewSource: 'ソースを表示',
        delete: '削除',
        from: '送信者:',
        to: '宛先:',
        cc: 'CC:',
        time: '時刻:',
        attachmentsTitle: '添付ファイル ({count})',
        download: 'ダウンロード',
        prevPage: '前へ',
        nextPage: '次へ',
        pageInfo: '{current}ページ / 全{total}ページ',
        confirmTitle: '操作の確認',
        confirm: '確認',
        cancel: 'キャンセル',
        deleteConfirm: 'このメールを削除してもよろしいですか?',
        deleteAllConfirm: 'すべてのメールを削除してもよろしいですか? この操作は元に戻せません!',
        markAllReadSuccess: '{count}通のメールを既読にしました',
        loadEmailsError: 'メールの読み込みに失敗しました: {error}',
        loadEmailDetailError: 'メール詳細の読み込みに失敗しました: {error}',
        deleteEmailError: 'メールの削除に失敗しました: {error}',
        deleteAllEmailsError: 'すべてのメールの削除に失敗しました: {error}',
        markAllReadError: '既読マークに失敗しました: {error}',
        justNow: 'たった今',
        minutesAgo: '{minutes}分前',
        hoursAgo: '{hours}時間前',
        daysAgo: '{days}日前',
        toggleTheme: 'テーマを切り替え',
        switchLanguage: '言語を切り替え',
        // API Error Codes
        'EMAIL_NOT_FOUND': 'メールが見つかりません',
        'EMAIL_FILE_NOT_FOUND': 'メールファイルが見つかりません',
        'NO_EMAILS_FOUND': 'メールが見つかりません',
        'NO_EMAILS_TO_EXPORT': 'エクスポートするメールがありません',
        'INVALID_EMAIL_ID': '無効なメールID',
        'NO_EMAIL_IDS_PROVIDED': 'メールIDが提供されていません',
        'INVALID_REQUEST': '無効なリクエスト',
        'INVALID_EMAIL_ADDRESS': '無効なメールアドレス',
        'HOST_REQUIRED': 'ホストが必要です',
        'PORT_OUT_OF_RANGE': 'ポートは1から65535の間である必要があります',
        'INVALID_PORT': '無効なポート',
        'RELAY_FAILED': 'リレーに失敗しました',
        // API Success Codes
        'EMAIL_DELETED': 'メールが削除されました',
        'ALL_EMAILS_DELETED': 'すべてのメールが削除されました',
        'EMAIL_MARKED_READ': 'メールが既読としてマークされました',
        'ALL_EMAILS_MARKED_READ': 'すべてのメールが既読としてマークされました',
        'EMAIL_RELAYED': 'メールが正常にリレーされました',
        'MAILS_RELOADED': 'メールが正常に再読み込みされました',
        'BATCH_DELETE_COMPLETED': '一括削除が完了しました',
        'BATCH_READ_COMPLETED': '一括既読マークが完了しました',
        'CONFIG_UPDATED': '設定が更新されました'
    }
};

// Current language
let currentLang = 'en';

// Language code mapping for browser language detection
const languageCodeMap = {
    'zh': 'zh-CN',
    'de': 'de',
    'it': 'it',
    'fr': 'fr',
    'ko': 'ko',
    'ja': 'ja',
    'en': 'en'
};

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
        // Check language code and map to supported language
        const langCode = browserLang.split('-')[0].toLowerCase();
        if (languageCodeMap[langCode]) {
            return languageCodeMap[langCode];
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

// Parse API error response and return translated message
function parseAPIError(error) {
    // If error is a string, try to parse it as JSON
    let errorObj = error;
    if (typeof error === 'string') {
        try {
            errorObj = JSON.parse(error);
        } catch (e) {
            // If not JSON, return the string as is
            return error;
        }
    }
    
    // Check if it's an Error object with response
    if (error.response) {
        try {
            errorObj = typeof error.response === 'string' 
                ? JSON.parse(error.response) 
                : error.response;
        } catch (e) {
            // If parsing fails, use error message
            return error.message || error.toString();
        }
    }
    
    // Extract error code from response
    const errorCode = errorObj.error || errorObj.code || errorObj.Error || errorObj.Code;
    if (errorCode && i18n[currentLang][errorCode]) {
        return t(errorCode);
    }
    
    // Extract message from response
    const message = errorObj.message || errorObj.Message || errorObj.error || errorObj.Error;
    if (message) {
        // Check if message is an error code
        if (i18n[currentLang][message]) {
            return t(message);
        }
        return message;
    }
    
    // Fallback to error message or toString
    return error.message || error.toString();
}

// Parse API success response and return translated message
function parseAPISuccess(response) {
    if (!response) return '';
    
    // Extract success code from response
    const successCode = response.code || response.Code;
    if (successCode && i18n[currentLang][successCode]) {
        return t(successCode);
    }
    
    // Extract message from response
    const message = response.message || response.Message;
    if (message) {
        // Check if message is a success code
        if (i18n[currentLang][message]) {
            return t(message);
        }
        return message;
    }
    
    return '';
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

// Helper function to handle API errors
async function handleAPIResponse(response) {
    const contentType = response.headers.get('content-type');
    const isJSON = contentType && contentType.includes('application/json');
    
    if (!response.ok) {
        let errorData;
        if (isJSON) {
            errorData = await response.json();
        } else {
            const text = await response.text();
            try {
                errorData = JSON.parse(text);
            } catch (e) {
                errorData = { error: text || 'Unknown error' };
            }
        }
        const error = new Error(errorData.message || errorData.error || 'Request failed');
        error.response = errorData;
        error.status = response.status;
        throw error;
    }
    
    if (isJSON) {
        return await response.json();
    } else {
        return await response.text();
    }
}

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
        return await handleAPIResponse(response);
    },

    async getEmail(id) {
        const response = await fetch(`${API_BASE}/emails/${id}`);
        return await handleAPIResponse(response);
    },

    async getEmailHTML(id) {
        const response = await fetch(`${API_BASE}/emails/${id}/html`);
        return await handleAPIResponse(response);
    },

    async deleteEmail(id) {
        const response = await fetch(`${API_BASE}/emails/${id}`, {
            method: 'DELETE'
        });
        return await handleAPIResponse(response);
    },

    async deleteAllEmails() {
        const response = await fetch(`${API_BASE}/emails`, {
            method: 'DELETE'
        });
        return await handleAPIResponse(response);
    },

    async markAllRead() {
        const response = await fetch(`${API_BASE}/emails/read`, {
            method: 'PATCH'
        });
        return await handleAPIResponse(response);
    },

    async relayEmail(id, relayTo = '') {
        const url = relayTo 
            ? `${API_BASE}/emails/${id}/actions/relay/${encodeURIComponent(relayTo)}`
            : `${API_BASE}/emails/${id}/actions/relay`;
        const response = await fetch(url, {
            method: 'POST'
        });
        return await handleAPIResponse(response);
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
    
    // Update language selector
    updateLanguageSelector();
    
    // Update modal texts
    const confirmTitle = document.getElementById('confirmTitle');
    if (confirmTitle) confirmTitle.textContent = t('confirmTitle');
    
    const confirmYes = document.getElementById('confirmYes');
    if (confirmYes) confirmYes.textContent = t('confirm');
    
    const confirmNo = document.getElementById('confirmNo');
    if (confirmNo) confirmNo.textContent = t('cancel');
    
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
        const errorMsg = parseAPIError(error);
        alert(t('loadEmailsError', { error: errorMsg }));
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
        const errorMsg = parseAPIError(error);
        alert(t('loadEmailDetailError', { error: errorMsg }));
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
        const errorMsg = parseAPIError(error);
        alert(t('deleteEmailError', { error: errorMsg }));
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
        const errorMsg = parseAPIError(error);
        alert(t('deleteAllEmailsError', { error: errorMsg }));
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
        const successMsg = parseAPISuccess(result) || t('markAllReadSuccess', { count: result.count || 0 });
        alert(successMsg);
    } catch (error) {
        console.error('Failed to mark all as read:', error);
        const errorMsg = parseAPIError(error);
        alert(t('markAllReadError', { error: errorMsg }));
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

// Language names in their own language (for display in selector)
const languageNames = {
    'en': 'English',
    'zh-CN': '简体中文',
    'de': 'Deutsch',
    'it': 'Italiano',
    'fr': 'Français',
    'ko': '한국어',
    'ja': '日本語'
};

// Initialize language selector
function initLanguageSelector() {
    const langSelect = document.getElementById('langSelect');
    if (!langSelect) return;
    
    // Populate language options
    Object.keys(i18n).forEach(lang => {
        const option = document.createElement('option');
        option.value = lang;
        option.textContent = languageNames[lang] || lang;
        if (lang === currentLang) {
            option.selected = true;
        }
        langSelect.appendChild(option);
    });
    
    // Add change event listener
    langSelect.addEventListener('change', (e) => {
        setLanguage(e.target.value);
    });
}

// Update language selector
function updateLanguageSelector() {
    const langSelect = document.getElementById('langSelect');
    if (langSelect) {
        langSelect.value = currentLang;
    }
}

// Event Listeners
document.addEventListener('DOMContentLoaded', () => {
    // Initialize language
    currentLang = detectLanguage();
    setLanguage(currentLang);
    
    // Initialize language selector
    initLanguageSelector();
    
    // Initialize theme
    initTheme();

    // Load initial emails
    loadEmails();

    // Connect WebSocket
    connectWebSocket();

    // Button event listeners
    const refreshBtn = document.getElementById('refreshBtn');
    if (refreshBtn) refreshBtn.addEventListener('click', loadEmails);
    
    const markAllReadBtn = document.getElementById('markAllReadBtn');
    if (markAllReadBtn) markAllReadBtn.addEventListener('click', markAllRead);
    
    const deleteAllBtn = document.getElementById('deleteAllBtn');
    if (deleteAllBtn) deleteAllBtn.addEventListener('click', deleteAllEmails);
    
    const searchBtn = document.getElementById('searchBtn');
    if (searchBtn) searchBtn.addEventListener('click', searchEmails);
    
    const prevPageBtn = document.getElementById('prevPage');
    if (prevPageBtn) prevPageBtn.addEventListener('click', prevPage);
    
    const nextPageBtn = document.getElementById('nextPage');
    if (nextPageBtn) nextPageBtn.addEventListener('click', nextPage);
    
    const themeToggle = document.getElementById('themeToggle');
    if (themeToggle) themeToggle.addEventListener('click', toggleTheme);

    // Search input enter key
    const searchInput = document.getElementById('searchInput');
    if (searchInput) {
        searchInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                searchEmails();
            }
        });
    }
});

// Make functions available globally for onclick handlers
window.deleteEmail = deleteEmail;
window.downloadEmail = downloadEmail;
window.viewEmailSource = viewEmailSource;
window.t = t; // Make translation function available globally

