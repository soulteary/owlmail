// OwlMail Web Application
// API Base URL - 使用新的 API v1 端点
const BASE_PATHNAME = document.querySelector('meta[name="owlmail-base-pathname"]')?.content || '';
const API_BASE = `${window.location.origin}${BASE_PATHNAME}/api/v1`;

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
        emailContentViews: '邮件内容视图',
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
        help: '帮助',
        webhooks: 'Webhook 配置',
        toggleTheme: '切换主题',
        switchLanguage: '切换语言',
        remoteContentBlocked: '为保护隐私，远程图片、字体和样式默认已阻止。',
        loadRemoteContent: '加载远程内容',
        emailPreviewTitle: '隔离的邮件 HTML 预览',
        emailViewportPresets: '预览宽度',
        emailViewportWidth: '邮件预览宽度：{width}',
        relayOriginal: '按原收件人中继',
        relayOverride: '中继到…',
        relayRecipientPrompt: '输入中继收件人地址：',
        relayConfirm: '确定要中继这封邮件吗？实际邮件将发送到 {recipient}。',
        relayOriginalRecipients: '原始收件人',
        relayNoOriginalRecipients: '这封邮件没有可供中继的 SMTP 信封收件人。',
        relayNoEffectiveRecipients: '当前中继规则排除了这封邮件的所有 SMTP 信封收件人。',
        relayQueued: '中继任务已提交。任务 ID：{id}',
        relayError: '中继失败：{error}',
        contentHTML: 'HTML',
        contentText: '纯文本',
        contentHeaders: '邮件头',
        contentSource: '源码',
        sourceLoading: '正在加载源码…',
        sourceTooLarge: '邮件源码过大，无法在页面内安全显示。',
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
        emailContentViews: 'Email content views',
        emailCount: '{count} emails',
        emailCount_one: '{count} email',
        loading: 'Loading...',
        noEmails: 'No emails',
        selectEmail: 'Select an email to view details',
        unknown: 'Unknown',
        noSubject: '(No Subject)',
        attachments: '{count} attachments',
        attachments_one: '{count} attachment',
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
        markAllReadSuccess_one: 'Marked {count} email as read',
        loadEmailsError: 'Failed to load emails: {error}',
        loadEmailDetailError: 'Failed to load email details: {error}',
        deleteEmailError: 'Failed to delete email: {error}',
        deleteAllEmailsError: 'Failed to delete all emails: {error}',
        markAllReadError: 'Failed to mark as read: {error}',
        justNow: 'Just now',
        minutesAgo: '{minutes} minutes ago',
        minutesAgo_one: '{minutes} minute ago',
        hoursAgo: '{hours} hours ago',
        hoursAgo_one: '{hours} hour ago',
        daysAgo: '{days} days ago',
        daysAgo_one: '{days} day ago',
        help: 'Help',
        webhooks: 'Webhooks',
        toggleTheme: 'Toggle Theme',
        switchLanguage: 'Switch Language',
        remoteContentBlocked: 'Remote images, fonts, and styles are blocked by default for privacy.',
        loadRemoteContent: 'Load remote content',
        emailPreviewTitle: 'Isolated email HTML preview',
        emailViewportPresets: 'Preview width',
        emailViewportWidth: 'Email preview width: {width}',
        relayOriginal: 'Relay to original recipients',
        relayOverride: 'Relay to…',
        relayRecipientPrompt: 'Enter the relay recipient address:',
        relayConfirm: 'Relay this message? A real email will be sent to {recipient}.',
        relayOriginalRecipients: 'the original recipients',
        relayNoOriginalRecipients: 'This message has no SMTP envelope recipients to relay to.',
        relayNoEffectiveRecipients: 'The current relay rules exclude every SMTP envelope recipient for this message.',
        relayQueued: 'Relay job accepted. Job ID: {id}',
        relayError: 'Relay failed: {error}',
        contentHTML: 'HTML',
        contentText: 'Plain text',
        contentHeaders: 'Headers',
        contentSource: 'Source',
        contentInspect: 'Verify',
        contentDeliveries: 'Deliveries',
        inspectSubject: 'Expected subject text',
        inspectText: 'Expected plain-text body',
        inspectPlainTextOnly: 'This message has no plain-text part. Body text checks and code extraction require plain text; subject and link checks remain available.',
        inspectHosts: 'Allowed link hosts (comma separated, optional)',
        inspectCodes: 'Extract six-digit codes',
        inspectRun: 'Run checks',
        inspectAssertionTooLarge: 'Subject and plain-text assertions are limited to 1024 UTF-8 bytes each.',
        inspectPass: 'Checks passed',
        inspectFail: 'Checks failed',
        inspectTruncated: 'Results were truncated. Complete verification needs the full message.',
        workflowEmpty: 'No retained delivery records for this message.',
        workflowRetry: 'Retry failed delivery',
        workflowRetryConfirm: 'Retry this failed delivery? A real outgoing request will be sent again.',
        workflowQueued: 'Retry accepted. Refresh to see the latest outcome.',
        workflowHealth: 'Delivery history could not be persisted. Check server storage.',
        workflowMore: 'More records exist. Showing the most recent 100.',
        workflowDisabled: 'Webhook forwarding is disabled.',
        emailRunID: 'Test run:',

        sourceLoading: 'Loading source…',
        sourceTooLarge: 'This message source is too large to display safely inline.',
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
        emailContentViews: 'E-Mail-Inhaltsansichten',
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
        help: 'Hilfe',
        webhooks: 'Webhooks',
        toggleTheme: 'Design umschalten',
        switchLanguage: 'Sprache wechseln',
        remoteContentBlocked: 'Externe Bilder, Schriftarten und Stile sind aus Datenschutzgründen blockiert.',
        loadRemoteContent: 'Externe Inhalte laden',
        emailPreviewTitle: 'Isolierte HTML-E-Mail-Vorschau',
        emailViewportPresets: 'Vorschaubreite',
        emailViewportWidth: 'Breite der E-Mail-Vorschau: {width}',
        contentHTML: 'HTML',
        contentText: 'Klartext',
        contentHeaders: 'Kopfzeilen',
        contentSource: 'Quelltext',
        sourceLoading: 'Quelltext wird geladen…',
        sourceTooLarge: 'Der Nachrichtenquelltext ist zu groß für eine sichere Anzeige auf dieser Seite.',
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
        emailContentViews: 'Visualizzazioni contenuto email',
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
        help: 'Aiuto',
        webhooks: 'Webhook',
        toggleTheme: 'Cambia Tema',
        switchLanguage: 'Cambia Lingua',
        remoteContentBlocked: 'Immagini, font e stili remoti sono bloccati per impostazione predefinita.',
        loadRemoteContent: 'Carica contenuti remoti',
        emailPreviewTitle: 'Anteprima HTML email isolata',
        emailViewportPresets: 'Larghezza anteprima',
        emailViewportWidth: 'Larghezza anteprima email: {width}',
        contentHTML: 'HTML',
        contentText: 'Testo semplice',
        contentHeaders: 'Intestazioni',
        contentSource: 'Sorgente',
        sourceLoading: 'Caricamento sorgente…',
        sourceTooLarge: 'Il sorgente del messaggio è troppo grande per essere visualizzato in modo sicuro nella pagina.',
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
        emailContentViews: 'Vues du contenu de l\'email',
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
        help: 'Aide',
        webhooks: 'Webhooks',
        toggleTheme: 'Changer le Thème',
        switchLanguage: 'Changer la Langue',
        remoteContentBlocked: 'Les images, polices et styles distants sont bloqués par défaut.',
        loadRemoteContent: 'Charger le contenu distant',
        emailPreviewTitle: 'Aperçu HTML isolé de l’e-mail',
        emailViewportPresets: 'Largeur de l’aperçu',
        emailViewportWidth: 'Largeur de l’aperçu de l’e-mail : {width}',
        contentHTML: 'HTML',
        contentText: 'Texte brut',
        contentHeaders: 'En-têtes',
        contentSource: 'Source',
        sourceLoading: 'Chargement de la source…',
        sourceTooLarge: 'La source du message est trop volumineuse pour être affichée en toute sécurité dans la page.',
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
        emailContentViews: '이메일 콘텐츠 보기',
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
        help: '도움말',
        webhooks: '웹훅',
        toggleTheme: '테마 전환',
        switchLanguage: '언어 전환',
        remoteContentBlocked: '개인정보 보호를 위해 원격 이미지, 글꼴 및 스타일이 기본적으로 차단됩니다.',
        loadRemoteContent: '원격 콘텐츠 불러오기',
        emailPreviewTitle: '격리된 이메일 HTML 미리보기',
        emailViewportPresets: '미리보기 너비',
        emailViewportWidth: '이메일 미리보기 너비: {width}',
        contentHTML: 'HTML',
        contentText: '일반 텍스트',
        contentHeaders: '헤더',
        contentSource: '원본',
        sourceLoading: '원본을 불러오는 중…',
        sourceTooLarge: '메시지 원본이 너무 커서 페이지 안에 안전하게 표시할 수 없습니다.',
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
        emailContentViews: 'メール内容表示',
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
        help: 'ヘルプ',
        webhooks: 'Webhook',
        toggleTheme: 'テーマを切り替え',
        switchLanguage: '言語を切り替え',
        remoteContentBlocked: 'プライバシー保護のため、外部の画像、フォント、スタイルは既定でブロックされます。',
        loadRemoteContent: '外部コンテンツを読み込む',
        emailPreviewTitle: '隔離されたメール HTML プレビュー',
        emailViewportPresets: 'プレビュー幅',
        emailViewportWidth: 'メールプレビュー幅: {width}',
        contentHTML: 'HTML',
        contentText: 'プレーンテキスト',
        contentHeaders: 'ヘッダー',
        contentSource: 'ソース',
        sourceLoading: 'ソースを読み込み中…',
        sourceTooLarge: 'メッセージのソースが大きすぎるため、ページ内に安全に表示できません。',
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

// Browser notification strings are kept separate from the main UI dictionary
// so notification behavior remains self-contained.
const notificationI18n = {
    'zh-CN': {
        off: '通知已关闭',
        on: '通知已开启',
        blocked: '通知被阻止',
        unavailable: '通知不可用',
        deniedHelp: '通知权限已被阻止，请在浏览器的网站设置中允许通知。',
        unavailableHelp: '浏览器通知需要浏览器支持，并通过 HTTPS 或 localhost 访问。',
        error: '无法更新浏览器通知设置。',
        newEmailFrom: '来自 {sender} 的新邮件'
    },
    'en': {
        off: 'Notifications off',
        on: 'Notifications on',
        blocked: 'Notifications blocked',
        unavailable: 'Notifications unavailable',
        deniedHelp: 'Notification permission is blocked. Allow it in the browser site settings.',
        unavailableHelp: 'Browser notifications require support and a secure context (HTTPS or localhost).',
        error: 'Could not update browser notifications.',
        newEmailFrom: 'New email from {sender}'
    },
    'de': {
        off: 'Benachrichtigungen aus',
        on: 'Benachrichtigungen an',
        blocked: 'Benachrichtigungen blockiert',
        unavailable: 'Benachrichtigungen nicht verfügbar',
        deniedHelp: 'Benachrichtigungen sind blockiert. Erlauben Sie sie in den Website-Einstellungen des Browsers.',
        unavailableHelp: 'Browser-Benachrichtigungen benötigen einen sicheren Kontext (HTTPS oder localhost).',
        error: 'Browser-Benachrichtigungen konnten nicht aktualisiert werden.',
        newEmailFrom: 'Neue E-Mail von {sender}'
    },
    'it': {
        off: 'Notifiche disattivate',
        on: 'Notifiche attivate',
        blocked: 'Notifiche bloccate',
        unavailable: 'Notifiche non disponibili',
        deniedHelp: 'Le notifiche sono bloccate. Consentile nelle impostazioni del sito del browser.',
        unavailableHelp: 'Le notifiche richiedono un contesto sicuro (HTTPS o localhost) e un browser compatibile.',
        error: 'Impossibile aggiornare le notifiche del browser.',
        newEmailFrom: 'Nuova email da {sender}'
    },
    'fr': {
        off: 'Notifications désactivées',
        on: 'Notifications activées',
        blocked: 'Notifications bloquées',
        unavailable: 'Notifications indisponibles',
        deniedHelp: 'Les notifications sont bloquées. Autorisez-les dans les paramètres du site du navigateur.',
        unavailableHelp: 'Les notifications nécessitent un contexte sécurisé (HTTPS ou localhost) et un navigateur compatible.',
        error: 'Impossible de mettre à jour les notifications du navigateur.',
        newEmailFrom: 'Nouvel e-mail de {sender}'
    },
    'ko': {
        off: '알림 꺼짐',
        on: '알림 켜짐',
        blocked: '알림 차단됨',
        unavailable: '알림 사용 불가',
        deniedHelp: '알림 권한이 차단되었습니다. 브라우저 사이트 설정에서 알림을 허용하세요.',
        unavailableHelp: '브라우저 알림은 HTTPS 또는 localhost의 보안 컨텍스트와 브라우저 지원이 필요합니다.',
        error: '브라우저 알림 설정을 변경할 수 없습니다.',
        newEmailFrom: '{sender} 님이 보낸 새 이메일'
    },
    'ja': {
        off: '通知オフ',
        on: '通知オン',
        blocked: '通知がブロックされています',
        unavailable: '通知を利用できません',
        deniedHelp: '通知がブロックされています。ブラウザーのサイト設定で許可してください。',
        unavailableHelp: 'ブラウザー通知には、HTTPS または localhost の安全な接続と対応ブラウザーが必要です。',
        error: 'ブラウザー通知の設定を変更できませんでした。',
        newEmailFrom: '{sender} から新しいメール'
    }
};

function nt(key, params = {}) {
    const dictionary = notificationI18n[currentLang] || notificationI18n.en;
    const translation = dictionary[key] || notificationI18n.en[key] || key;
    return translation.replace(/\{(\w+)\}/g, (match, paramKey) => {
        return params[paramKey] !== undefined ? params[paramKey] : match;
    });
}

// Language code mapping for browser language detection
const languageCodeMap = {
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
    const savedLang = readPreference('language');
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
        const normalizedBrowserLang = browserLang.toLowerCase();
        if (normalizedBrowserLang === 'zh'
            || normalizedBrowserLang === 'zh-cn'
            || normalizedBrowserLang === 'zh-sg'
            || normalizedBrowserLang.startsWith('zh-hans')) {
            return 'zh-CN';
        }
        const langCode = normalizedBrowserLang.split('-')[0];
        if (languageCodeMap[langCode]) {
            return languageCodeMap[langCode];
        }
    }
    
    // Default to English
    return 'en';
}

// Translation function
function t(key, params = {}) {
    const singularValue = [params.count, params.minutes, params.hours, params.days]
        .find(value => value !== undefined);
    const resolvedKey = singularValue === 1 && i18n[currentLang][`${key}_one`]
        ? `${key}_one`
        : key;
    const translation = i18n[currentLang][resolvedKey] || i18n['en'][resolvedKey] || i18n['en'][key] || key;
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
function setLanguage(lang, renderDynamic = true) {
    if (!i18n[lang]) {
        lang = 'en';
    }
    currentLang = lang;
    writePreference('language', lang);
    document.documentElement.lang = lang;
    updateUI(renderDynamic);
    if (browserNotificationsInitialized) updateBrowserNotificationButton();
}

// Global State
Object.assign(i18n["en"], {"relayWaiting": "Waiting for delivery status", "relayPaused": "Status waiting paused", "relayStopWait": "Stop waiting", "relayResumeWait": "Resume waiting", "relayWaitStopped": "Stopped waiting. The queued delivery continues; resume here or check Deliveries.", "relayWaitTimeout": "Waiting timed out. The queued delivery continues; resume here or check Deliveries."});
Object.assign(i18n["zh-CN"], {"relayWaiting": "正在等待投递状态", "relayPaused": "状态等待已暂停", "relayStopWait": "停止等待", "relayResumeWait": "继续等待", "relayWaitStopped": "已停止等待。已提交的投递会继续，可继续等待或查看投递记录。", "relayWaitTimeout": "等待超时。已提交的投递会继续，可继续等待或查看投递记录。"});
Object.assign(i18n["de"], {"relayWaiting": "Warte auf Zustellstatus", "relayPaused": "Statusabfrage pausiert", "relayStopWait": "Warten stoppen", "relayResumeWait": "Warten fortsetzen", "relayWaitStopped": "Warten beendet. Die Zustellung läuft weiter; fortsetzen oder Zustellungen prüfen.", "relayWaitTimeout": "Zeitlimit erreicht. Die Zustellung läuft weiter; fortsetzen oder Zustellungen prüfen."});
Object.assign(i18n["it"], {"relayWaiting": "Attesa dello stato di consegna", "relayPaused": "Attesa sospesa", "relayStopWait": "Interrompi attesa", "relayResumeWait": "Riprendi attesa", "relayWaitStopped": "Attesa interrotta. La consegna continua; riprendi o controlla le consegne.", "relayWaitTimeout": "Attesa scaduta. La consegna continua; riprendi o controlla le consegne."});
Object.assign(i18n["fr"], {"relayWaiting": "En attente du statut de livraison", "relayPaused": "Attente suspendue", "relayStopWait": "Arrêter l’attente", "relayResumeWait": "Reprendre l’attente", "relayWaitStopped": "Attente arrêtée. La livraison continue ; reprenez ou consultez les livraisons.", "relayWaitTimeout": "Délai dépassé. La livraison continue ; reprenez ou consultez les livraisons."});
Object.assign(i18n["ko"], {"relayWaiting": "전송 상태 대기 중", "relayPaused": "상태 대기 일시 중지", "relayStopWait": "대기 중지", "relayResumeWait": "대기 재개", "relayWaitStopped": "대기를 중지했습니다. 전송은 계속됩니다. 대기를 재개하거나 전송 내역을 확인하세요.", "relayWaitTimeout": "대기 시간이 초과되었습니다. 전송은 계속됩니다. 대기를 재개하거나 전송 내역을 확인하세요."});
Object.assign(i18n["ja"], {"relayWaiting": "配信状態を待機中", "relayPaused": "状態の待機を停止中", "relayStopWait": "待機を停止", "relayResumeWait": "待機を再開", "relayWaitStopped": "待機を停止しました。配信は続行されます。待機を再開するか配信履歴を確認してください。", "relayWaitTimeout": "待機がタイムアウトしました。配信は続行されます。待機を再開するか配信履歴を確認してください。"});
Object.assign(i18n["en"], {"mailFilters": "Filters", "filterFrom": "Sender", "filterTo": "Recipient", "filterDateFrom": "From date", "filterDateTo": "Through date", "filterRead": "Read state", "filterAny": "Any", "filterUnread": "Unread", "filterReadOnly": "Read", "filterSort": "Sort by", "sortDesc": "Descending", "sortAsc": "Ascending", "applyFilters": "Apply filters", "resetFilters": "Reset filters", "selectPage": "Select this page", "selectionCount": "{count} selected", "batchRead": "Mark selected read", "batchDelete": "Delete selected", "exportSelected": "Export selected (.zip)", "exportFiltered": "Export filtered results (.zip)", "selectMessage": "Select message: {subject}", "batchDeleteConfirm": "Delete {count} selected messages? This cannot be undone.", "batchResult": "Completed: {success}; failed: {failed}.", "sortTime": "Time", "sortSubject": "Subject", "sortFrom": "Sender", "sortSize": "Size", "sortStore": "Arrival order"});
Object.assign(i18n["zh-CN"], {"mailFilters": "筛选", "filterFrom": "发件人", "filterTo": "收件人", "filterDateFrom": "起始日期", "filterDateTo": "截止日期", "filterRead": "已读状态", "filterAny": "全部", "filterUnread": "未读", "filterReadOnly": "已读", "filterSort": "排序字段", "sortDesc": "降序", "sortAsc": "升序", "applyFilters": "应用筛选", "resetFilters": "重置筛选", "selectPage": "选择当前页", "selectionCount": "已选择 {count} 封", "batchRead": "所选标为已读", "batchDelete": "删除所选", "exportSelected": "导出所选（.zip）", "exportFiltered": "导出筛选结果（.zip）", "selectMessage": "选择邮件：{subject}", "batchDeleteConfirm": "删除所选 {count} 封邮件？此操作不可撤销。", "batchResult": "成功：{success}；失败：{failed}。", "sortTime": "时间", "sortSubject": "主题", "sortFrom": "发件人", "sortSize": "大小", "sortStore": "接收顺序"});
Object.assign(i18n["de"], {"mailFilters": "Filter", "filterFrom": "Absender", "filterTo": "Empfänger", "filterDateFrom": "Ab Datum", "filterDateTo": "Bis Datum", "filterRead": "Lesestatus", "filterAny": "Alle", "filterUnread": "Ungelesen", "filterReadOnly": "Gelesen", "filterSort": "Sortieren nach", "sortDesc": "Absteigend", "sortAsc": "Aufsteigend", "applyFilters": "Filter anwenden", "resetFilters": "Filter zurücksetzen", "selectPage": "Diese Seite auswählen", "selectionCount": "{count} ausgewählt", "batchRead": "Auswahl als gelesen markieren", "batchDelete": "Auswahl löschen", "exportSelected": "Auswahl exportieren (.zip)", "exportFiltered": "Gefilterte Ergebnisse exportieren (.zip)", "selectMessage": "Nachricht auswählen: {subject}", "batchDeleteConfirm": "{count} ausgewählte Nachrichten unwiderruflich löschen?", "batchResult": "Erfolgreich: {success}; fehlgeschlagen: {failed}.", "sortTime": "Zeit", "sortSubject": "Betreff", "sortFrom": "Absender", "sortSize": "Größe", "sortStore": "Eingangsreihenfolge", "filterSortOrder": "Sortierrichtung", "filterDateRangeError": "Das Enddatum darf nicht vor dem Startdatum liegen."});
Object.assign(i18n["it"], {"mailFilters": "Filtri", "filterFrom": "Mittente", "filterTo": "Destinatario", "filterDateFrom": "Dalla data", "filterDateTo": "Alla data", "filterRead": "Stato di lettura", "filterAny": "Tutti", "filterUnread": "Non letti", "filterReadOnly": "Letti", "filterSort": "Ordina per", "sortDesc": "Decrescente", "sortAsc": "Crescente", "applyFilters": "Applica filtri", "resetFilters": "Reimposta filtri", "selectPage": "Seleziona questa pagina", "selectionCount": "{count} selezionati", "batchRead": "Segna selezionati come letti", "batchDelete": "Elimina selezionati", "exportSelected": "Esporta selezionati (.zip)", "exportFiltered": "Esporta risultati filtrati (.zip)", "selectMessage": "Seleziona messaggio: {subject}", "batchDeleteConfirm": "Eliminare definitivamente {count} messaggi selezionati?", "batchResult": "Completati: {success}; non riusciti: {failed}.", "sortTime": "Data", "sortSubject": "Oggetto", "sortFrom": "Mittente", "sortSize": "Dimensione", "sortStore": "Ordine di arrivo", "filterSortOrder": "Direzione di ordinamento", "filterDateRangeError": "La data finale non può precedere quella iniziale."});
Object.assign(i18n["fr"], {"mailFilters": "Filtres", "filterFrom": "Expéditeur", "filterTo": "Destinataire", "filterDateFrom": "À partir du", "filterDateTo": "Jusqu’au", "filterRead": "État de lecture", "filterAny": "Tous", "filterUnread": "Non lus", "filterReadOnly": "Lus", "filterSort": "Trier par", "sortDesc": "Décroissant", "sortAsc": "Croissant", "applyFilters": "Appliquer les filtres", "resetFilters": "Réinitialiser les filtres", "selectPage": "Sélectionner cette page", "selectionCount": "{count} sélectionnés", "batchRead": "Marquer la sélection comme lue", "batchDelete": "Supprimer la sélection", "exportSelected": "Exporter la sélection (.zip)", "exportFiltered": "Exporter les résultats filtrés (.zip)", "selectMessage": "Sélectionner le message : {subject}", "batchDeleteConfirm": "Supprimer définitivement les {count} messages sélectionnés ?", "batchResult": "Réussis : {success} ; échoués : {failed}.", "sortTime": "Date", "sortSubject": "Objet", "sortFrom": "Expéditeur", "sortSize": "Taille", "sortStore": "Ordre de réception", "filterSortOrder": "Sens du tri", "filterDateRangeError": "La date de fin ne peut pas précéder la date de début."});
Object.assign(i18n["ko"], {"mailFilters": "필터", "filterFrom": "보낸 사람", "filterTo": "받는 사람", "filterDateFrom": "시작 날짜", "filterDateTo": "종료 날짜", "filterRead": "읽음 상태", "filterAny": "전체", "filterUnread": "읽지 않음", "filterReadOnly": "읽음", "filterSort": "정렬 기준", "sortDesc": "내림차순", "sortAsc": "오름차순", "applyFilters": "필터 적용", "resetFilters": "필터 초기화", "selectPage": "현재 페이지 선택", "selectionCount": "{count}개 선택됨", "batchRead": "선택 항목 읽음 처리", "batchDelete": "선택 항목 삭제", "exportSelected": "선택 항목 내보내기 (.zip)", "exportFiltered": "필터 결과 내보내기 (.zip)", "selectMessage": "메일 선택: {subject}", "batchDeleteConfirm": "선택한 메일 {count}개를 영구 삭제할까요?", "batchResult": "성공: {success}; 실패: {failed}.", "sortTime": "시간", "sortSubject": "제목", "sortFrom": "보낸 사람", "sortSize": "크기", "sortStore": "수신 순서", "filterSortOrder": "정렬 방향", "filterDateRangeError": "종료 날짜는 시작 날짜보다 빠를 수 없습니다."});
Object.assign(i18n["ja"], {"mailFilters": "フィルター", "filterFrom": "差出人", "filterTo": "宛先", "filterDateFrom": "開始日", "filterDateTo": "終了日", "filterRead": "既読状態", "filterAny": "すべて", "filterUnread": "未読", "filterReadOnly": "既読", "filterSort": "並べ替え項目", "sortDesc": "降順", "sortAsc": "昇順", "applyFilters": "フィルターを適用", "resetFilters": "フィルターをリセット", "selectPage": "このページを選択", "selectionCount": "{count} 件選択", "batchRead": "選択したメールを既読にする", "batchDelete": "選択したメールを削除", "exportSelected": "選択したメールをエクスポート (.zip)", "exportFiltered": "検索結果をエクスポート (.zip)", "selectMessage": "メールを選択: {subject}", "batchDeleteConfirm": "選択した {count} 件のメールを完全に削除しますか？", "batchResult": "成功: {success}、失敗: {failed}。", "sortTime": "日時", "sortSubject": "件名", "sortFrom": "差出人", "sortSize": "サイズ", "sortStore": "受信順", "filterSortOrder": "並べ替え順序", "filterDateRangeError": "終了日は開始日以降にしてください。"});
let state = {
    emails: [],
    currentEmail: null,
    currentPage: 0,
    pageSize: 50,
    total: 0,
    searchQuery: '',
    ws: null
};
let relayEnabled = false;
const relayPending = new Set();
const relayTracking = new Map();
const completedRelayJobs = new Map();
const deliveryRefreshesInFlight = new Set();
const relayWaitControllers = new Map();

function syncRelayMutationControls() {
    const deleteAllBtn = document.getElementById('deleteAllBtn');
    if (deleteAllBtn) deleteAllBtn.disabled = relayPending.size > 0 || mailboxBatchBusy;
    const markAllReadBtn = document.getElementById('markAllReadBtn');
    if (markAllReadBtn) markAllReadBtn.disabled = mailboxBatchBusy;
    updateMailboxSelectionControls();
}

function relayStatusErrorIsTransient(error) {
    const status = Number(error && error.status);
    return !Number.isFinite(status) || status === 408 || status === 429 || status >= 500;
}
const RELAY_STATUS_POLL_INTERVAL_MS = 1000;

function currentEmailIDFromLocation() {
    try {
        const fallback = `${window.location.origin || ''}${window.location.pathname || '/'}${window.location.search || ''}`;
        const url = new URL(window.location.href || fallback, window.location.origin || undefined);
        return url.searchParams.get('email') || '';
    } catch (_) {
        return '';
    }
}

function updateEmailLocation(emailID, mode = 'push') {
    if (!window.history || typeof window.history.pushState !== 'function') return;
    if (currentEmailIDFromLocation() === (emailID || '')) return;
    try {
        const fallback = `${window.location.origin || ''}${window.location.pathname || '/'}${window.location.search || ''}`;
        const url = new URL(window.location.href || fallback, window.location.origin || undefined);
        if (emailID) url.searchParams.set('email', emailID);
        else url.searchParams.delete('email');
        const method = mode === 'replace' ? 'replaceState' : 'pushState';
        window.history[method]({ emailID: emailID || null }, '', `${url.pathname}${url.search}${url.hash}`);
    } catch (error) {
        console.warn('Unable to update email navigation history:', error);
    }
}

let emailDetailRequestSequence = 0;

function clearEmailSelection(historyMode = 'push') {
    emailDetailRequestSequence += 1;
    remoteContentAllowedEmailID = null;
    emailSourceCache.clear();
    emailSourceErrors.clear();
    emailSourceOversized.clear();
    emailSourceRequests.clear();
    state.currentEmail = null;
    renderEmailDetail();
    renderEmailList();
    if (historyMode !== 'none') updateEmailLocation('', historyMode);
}

function isEditableKeyboardTarget(target) {
    if (!target) return false;
    if (target.isContentEditable) return true;
    const tagName = String(target.tagName || '').toLowerCase();
    return tagName === 'input' || tagName === 'textarea' || tagName === 'select';
}

function handleMailboxKeydown(event) {
    if (!event || event.defaultPrevented || event.isComposing || event.repeat || event.shiftKey || event.altKey || event.ctrlKey || event.metaKey || isEditableKeyboardTarget(event.target)) {
        return;
    }
    if (event.target && event.target.closest && event.target.closest('button, a, [role=button], [role=dialog], [role=menu], [contenteditable]')) return;
    if (event.key === '/') {
        const search = document.getElementById('searchInput');
        if (search) { event.preventDefault(); search.focus(); }
        return;
    }
    if (event.key === 'r') { event.preventDefault(); return loadEmails(); }
    if (event.key === 't') { event.preventDefault(); toggleTheme(); return; }
    if (event.key === 'Delete' && state.currentEmail && !mailboxBatchBusy && !loadingCounts.has('emailDetail')) {
        event.preventDefault();
        return deleteEmail(state.currentEmail.id);
    }
    if (event.key === 'Escape' && state.currentEmail) {
        event.preventDefault();
        clearEmailSelection('push');
        return;
    }

    let direction = 0;
    if (event.key === 'ArrowDown' || event.key === 'j') direction = 1;
    if (event.key === 'ArrowUp' || event.key === 'k') direction = -1;
    if (direction === 0 || state.emails.length === 0) return;

    const currentID = state.currentEmail && state.currentEmail.id;
    const currentIndex = state.emails.findIndex((email) => email.id === currentID);
    const nextIndex = currentIndex < 0
        ? (direction > 0 ? 0 : state.emails.length - 1)
        : Math.max(0, Math.min(state.emails.length - 1, currentIndex + direction));
    const nextEmail = state.emails[nextIndex];
    if (!nextEmail || nextEmail.id === currentID) return;

    event.preventDefault();
    return loadEmailDetail(nextEmail.id);
}

function handleHistoryNavigation() {
    const emailID = currentEmailIDFromLocation();
    if (!emailID) {
        clearEmailSelection('none');
        return;
    }
    if (state.currentEmail && state.currentEmail.id === emailID) return;
    void loadEmailDetail(emailID, { historyMode: 'none' });
}

const EMAIL_VIEWPORT_PRESETS = Object.freeze([
    { key: '100%', label: '100%', width: '100%' },
    { key: '1440', label: '1440 px', width: '1440px' },
    { key: '1024', label: '1024 px', width: '1024px' },
    { key: '768', label: '768 px', width: '768px' },
    { key: '425', label: '425 px', width: '425px' },
    { key: '375', label: '375 px', width: '375px' },
    { key: '320', label: '320 px', width: '320px' }
]);
function readPreference(key) {
    try { return localStorage.getItem(key); } catch (_) { return null; }
}
function writePreference(key, value) {
    try { localStorage.setItem(key, value); } catch (_) { /* Preferences remain usable for this session. */ }
}
const savedViewport = readPreference('owlmail.previewWidth');
let emailViewportPreset = EMAIL_VIEWPORT_PRESETS.some((preset) => preset.key === savedViewport) ? savedViewport : '100%';
let emailContentTab = 'html';
const EMAIL_SOURCE_INLINE_MAX_BYTES = 1024 * 1024;
const emailSourceCache = new Map();
const emailSourceErrors = new Map();
const emailSourceOversized = new Set();
const emailSourceRequests = new Map();

// Remote resources are enabled only for the currently rendered message after
// an explicit user action. The choice is intentionally not persisted.
let remoteContentAllowedEmailID = null;

const BROWSER_NOTIFICATION_STORAGE_KEY = 'owlmail.browserNotifications.enabled';
let browserNotificationsEnabled = false;
let browserNotificationsInitialized = false;
let notificationPermissionPending = false;
let notificationStatusTimer = null;
let notificationServiceWorkerPromise = null;

function notificationServiceWorkerSupported() {
    return typeof navigator !== 'undefined' && navigator.serviceWorker
        && typeof navigator.serviceWorker.register === 'function';
}

function getNotificationServiceWorker() {
    if (!notificationServiceWorkerSupported()) return Promise.resolve(null);
	if (!notificationServiceWorkerPromise) {
		notificationServiceWorkerPromise = navigator.serviceWorker
			.register(`${BASE_PATHNAME}/service-worker.js`, { scope: `${BASE_PATHNAME}/` })
			.then((registration) => navigator.serviceWorker.ready || registration)
			.catch((error) => {
                console.warn('Unable to register notification service worker:', error);
                return null;
            });
    }
    return notificationServiceWorkerPromise;
}

function browserNotificationsSupported() {
    return typeof window.Notification === 'function'
        && typeof window.Notification.requestPermission === 'function'
        && window.isSecureContext !== false
        && (notificationServiceWorkerSupported() || typeof window.Notification === 'function');
}

function readBrowserNotificationPreference() {
    try {
        return localStorage.getItem(BROWSER_NOTIFICATION_STORAGE_KEY) === 'true';
    } catch (error) {
        console.warn('Unable to read browser notification preference:', error);
        return false;
    }
}

function storeBrowserNotificationPreference(enabled) {
    try {
        localStorage.setItem(BROWSER_NOTIFICATION_STORAGE_KEY, enabled ? 'true' : 'false');
    } catch (error) {
        console.warn('Unable to store browser notification preference:', error);
    }
}

function getBrowserNotificationState() {
    if (!browserNotificationsSupported()) return 'unavailable';
    if (window.Notification.permission === 'denied') return 'blocked';
    if (window.Notification.permission === 'granted' && browserNotificationsEnabled) return 'enabled';
    return 'disabled';
}

function showBrowserNotificationStatus(message) {
    const status = document.getElementById('notificationStatus');
    if (!status) return;

    status.textContent = message;
    status.hidden = false;
    if (notificationStatusTimer) clearTimeout(notificationStatusTimer);
    notificationStatusTimer = setTimeout(() => {
        status.hidden = true;
        notificationStatusTimer = null;
    }, 5000);
}

function updateBrowserNotificationButton() {
    const button = document.getElementById('notificationToggle');
    if (!button) return;

    const notificationState = getBrowserNotificationState();
    const labels = {
        enabled: { icon: '🔔', text: nt('on'), title: nt('on') },
        disabled: { icon: '🔕', text: nt('off'), title: nt('off') },
        blocked: { icon: '🔒', text: nt('blocked'), title: nt('deniedHelp') },
        unavailable: { icon: '🚫', text: nt('unavailable'), title: nt('unavailableHelp') }
    };
    const label = labels[notificationState];

    button.textContent = `${label.icon} ${label.text}`;
    button.title = label.title;
    button.disabled = notificationPermissionPending || notificationState === 'unavailable';
    button.setAttribute('aria-pressed', notificationState === 'enabled' ? 'true' : 'false');
    button.classList.toggle('is-enabled', notificationState === 'enabled');
    button.classList.toggle('is-blocked', notificationState === 'blocked');
}

async function toggleBrowserNotifications() {
    if (notificationPermissionPending) return;

    if (!browserNotificationsSupported()) {
        showBrowserNotificationStatus(nt('unavailableHelp'));
        updateBrowserNotificationButton();
        return;
    }

    if (browserNotificationsEnabled && window.Notification.permission === 'granted') {
        browserNotificationsEnabled = false;
        storeBrowserNotificationPreference(false);
        updateBrowserNotificationButton();
        showBrowserNotificationStatus(nt('off'));
        return;
    }

    if (window.Notification.permission === 'denied') {
        browserNotificationsEnabled = false;
        storeBrowserNotificationPreference(false);
        updateBrowserNotificationButton();
        showBrowserNotificationStatus(nt('deniedHelp'));
        return;
    }

    notificationPermissionPending = true;
    updateBrowserNotificationButton();
    try {
        let permission = window.Notification.permission;
        if (permission === 'default') {
            const requestedPermission = await window.Notification.requestPermission();
            permission = requestedPermission || window.Notification.permission;
        }

        browserNotificationsEnabled = permission === 'granted';
        storeBrowserNotificationPreference(browserNotificationsEnabled);
        updateBrowserNotificationButton();
        showBrowserNotificationStatus(
            browserNotificationsEnabled ? nt('on') : (permission === 'denied' ? nt('deniedHelp') : nt('off'))
        );
    } catch (error) {
        browserNotificationsEnabled = false;
        storeBrowserNotificationPreference(false);
        console.error('Failed to request browser notification permission:', error);
        showBrowserNotificationStatus(nt('error'));
    } finally {
        notificationPermissionPending = false;
        updateBrowserNotificationButton();
    }
}

function synchronizeBrowserNotificationPermission() {
    if (browserNotificationsSupported()
        && window.Notification.permission !== 'granted'
        && browserNotificationsEnabled) {
        browserNotificationsEnabled = false;
        storeBrowserNotificationPreference(false);
    }
    updateBrowserNotificationButton();
}

function initializeBrowserNotifications() {
    const button = document.getElementById('notificationToggle');
    if (!button) return;

    const savedPreference = readBrowserNotificationPreference();
    browserNotificationsEnabled = browserNotificationsSupported()
        && window.Notification.permission === 'granted'
        && savedPreference;
    if (browserNotificationsSupported()
        && window.Notification.permission !== 'granted'
        && savedPreference) {
        storeBrowserNotificationPreference(false);
    }

    button.addEventListener('click', toggleBrowserNotifications);
    window.addEventListener('focus', synchronizeBrowserNotificationPermission);
    if (notificationServiceWorkerSupported()) {
        navigator.serviceWorker.addEventListener('message', (event) => {
            if (!event.data || event.data.type !== 'owlmail-notification-click') return;
            if (typeof window.focus === 'function') window.focus();
            if (event.data.emailID) loadEmailDetail(event.data.emailID);
        });
        void getNotificationServiceWorker();
    }
    browserNotificationsInitialized = true;
    updateBrowserNotificationButton();
}

function normalizeNotificationText(value, fallback, maxLength) {
    const normalized = typeof value === 'string' ? value.replace(/\s+/g, ' ').trim() : '';
    const text = normalized || fallback;
    return text.length > maxLength ? `${text.slice(0, maxLength - 1)}…` : text;
}

async function notifyBrowserForEmail(email) {
    if (!email || !browserNotificationsEnabled || !browserNotificationsSupported()) return;
    if (window.Notification.permission !== 'granted') {
        browserNotificationsEnabled = false;
        storeBrowserNotificationPreference(false);
        updateBrowserNotificationButton();
        return;
    }

    const sender = email.from && email.from.length > 0 && email.from[0]
        ? formatAddress(email.from[0])
        : t('unknown');
    const title = normalizeNotificationText(email.subject, t('noSubject'), 160);
    const notificationSender = normalizeNotificationText(sender, t('unknown'), 240);

    try {
        const options = {
            body: nt('newEmailFrom', { sender: notificationSender }),
            tag: email.id ? `owlmail-email-${email.id}` : 'owlmail-new-email',
            renotify: false,
            data: { emailID: email.id || '' }
        };
        const registration = await getNotificationServiceWorker();
        if (registration && typeof registration.showNotification === 'function') {
            await registration.showNotification(title, options);
            return;
        }
        const notification = new window.Notification(title, options);
        notification.onclick = () => {
            if (typeof window.focus === 'function') window.focus();
            notification.close();
            if (email.id) loadEmailDetail(email.id);
        };
    } catch (error) {
        browserNotificationsEnabled = false;
        storeBrowserNotificationPreference(false);
        updateBrowserNotificationButton();
        console.error('Failed to show browser notification:', error);
        showBrowserNotificationStatus(nt('error'));
    }
}

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
    async getEmailPreviews(offset = 0, limit = 50, query = '', filters = {}) {
        const params = new URLSearchParams({
            offset: offset.toString(),
            limit: limit.toString()
        });
        if (query) {
            params.append('q', query);
        }
        for (const [key, value] of Object.entries(filters)) {
            if (value !== '') params.set(key, value);
        }
        const response = await fetch(`${API_BASE}/emails/preview?${params}`);
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

    async getOutgoingConfig() {
        const response = await fetch(`${API_BASE}/settings/outgoing`);
        return await handleAPIResponse(response);
    },

    async getEmailSource(id, maximumBytes = EMAIL_SOURCE_INLINE_MAX_BYTES) {
        const response = await fetch(`${API_BASE}/emails/${id}/source`);
        if (!response.ok) return await handleAPIResponse(response);
        const contentLength = Number(response.headers.get('content-length'));
        if (Number.isFinite(contentLength) && contentLength > maximumBytes) {
            return { oversized: true, source: '' };
        }
        if (!response.body || typeof response.body.getReader !== 'function') {
            const source = await response.text();
            return new Blob([source]).size > maximumBytes
                ? { oversized: true, source: '' }
                : { oversized: false, source };
        }
        const reader = response.body.getReader();
        const chunks = [];
        let received = 0;
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            received += value.byteLength;
            if (received > maximumBytes) {
                await reader.cancel();
                return { oversized: true, source: '' };
            }
            chunks.push(value);
        }
        return { oversized: false, source: await new Blob(chunks).text() };
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

    async getRelayPreflight(id) {
        const response = await fetch(`${API_BASE}/emails/${id}/actions/relay/preflight`);
        return await handleAPIResponse(response);
    },

    async relayEmail(id, relayTo = '', confirmedRecipients = null) {
        const url = relayTo 
            ? `${API_BASE}/emails/${id}/actions/relay/${encodeURIComponent(relayTo)}`
            : `${API_BASE}/emails/${id}/actions/relay`;
        const options = { method: 'POST' };
        if (confirmedRecipients !== null) {
            options.headers = { 'Content-Type': 'application/json' };
            options.body = JSON.stringify({ confirmedRecipients });
        }
        const response = await fetch(url, options);
        return await handleAPIResponse(response);
    },

    async getRelayJob(statusURL, signal) {
        const url = new URL(statusURL, window.location.origin);
        if (url.origin !== window.location.origin) throw new Error("Relay status URL must use the inbox origin");
        const response = await fetch(url.toString(), { signal });
        return await handleAPIResponse(response);
    }
};

// WebSocket Connection - 使用新的 API v1 WebSocket 端点
let websocketHasConnected = false;
function connectWebSocket() {
    try {
        // Use ws:// or wss:// based on current protocol
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}${BASE_PATHNAME}/api/v1/ws`;
        const ws = new WebSocket(wsUrl);
        
        ws.onopen = () => {
            console.log('WebSocket connected');
            if (websocketHasConnected || emailListActiveSequence === null) scheduleMailboxRefresh();
            else mailboxRefreshAfterLoad = true;
            websocketHasConnected = true;
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

let mailboxRefreshTimer = null;
let mailboxRefreshGeneration = 0;
const knownDeletedMailIDs = new Set();
function scheduleMailboxRefresh() {
    // Events invalidate the query result; only the server can correctly apply
    // filtering, sorting, pagination and total counts to the current query.
    if (emailListActiveSequence !== null) {
        mailboxRefreshAfterLoad = true;
        return;
    }
    if (mailboxRefreshTimer !== null) return;
    const generation = ++mailboxRefreshGeneration;
    mailboxRefreshTimer = setTimeout(() => {
        if (generation !== mailboxRefreshGeneration) return;
        mailboxRefreshTimer = null;
        void loadEmails({ background: true });
    }, 100);
}
function handleWebSocketMessage(data) {
    if (!data || (data.type !== 'new' && data.type !== 'delete')) return;
    if (data.type === 'new' && data.email) { knownDeletedMailIDs.delete(data.email.id); notifyBrowserForEmail(data.email); }
    if (data.type === 'delete') removeKnownDeletedEmail(data.id);
    if (data.type === 'delete' && state.currentEmail && state.currentEmail.id === data.id) {
        clearEmailSelection('replace');
    }
    scheduleMailboxRefresh();
}

// Update UI with current language
function updateUI(renderDynamic = true) {
    renderMailboxTools();
    // Update title
    document.title = t('title');
    
    // Update header buttons
    const refreshBtn = document.getElementById('refreshBtn');
    if (refreshBtn) refreshBtn.textContent = t('refresh');
    
    const markAllReadBtn = document.getElementById('markAllReadBtn');
    if (markAllReadBtn) markAllReadBtn.textContent = t('markAllRead');
    
    const deleteAllBtn = document.getElementById('deleteAllBtn');
    if (deleteAllBtn) {
        deleteAllBtn.textContent = t('deleteAll');
        deleteAllBtn.disabled = relayPending.size > 0 || mailboxBatchBusy;
    }

    const webhookBtn = document.getElementById('webhookBtn');
    if (webhookBtn) {
        webhookBtn.textContent = t('webhooks');
        webhookBtn.title = t('webhooks');
    }

    const helpBtn = document.getElementById('helpBtn');
    if (helpBtn) {
        helpBtn.textContent = t('help');
        helpBtn.title = t('help');
    }
    
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
    
    if (renderDynamic) {
        updateEmailCount();
        updatePagination();
        renderEmailList();
        renderEmailDetail();
    } else if (!state.currentEmail) {
        // The empty detail state is safe to render before the initial API load
        // and must not retain the English placeholder from index.html.
        renderEmailDetail();
    }
}

// UI Rendering Functions
function renderEmailList() {
    const visible = new Set(state.emails.map(email => email.id));
    for (const id of selectedMailIDs) if (!visible.has(id)) selectedMailIDs.delete(id);
    updateMailboxSelectionControls();
    const container = document.getElementById('emailList');
    if (!container) return;
    container.inert = emailListActiveSequence !== null;

    if (state.emails.length === 0) {
        container.innerHTML = `<div class="loading">${t('noEmails')}</div>`;
        return;
    }

    container.innerHTML = state.emails.map(email => {
        const from = Array.isArray(email.from)
            ? (email.from.length > 0 ? formatAddress(email.from[0]) : t('unknown'))
            : (email.from ? formatAddress(email.from) : t('unknown'));
        const time = formatTime(email.time);
        const previewText = typeof email.preview === 'string' ? email.preview : email.text;
        const preview = previewText ? previewText.substring(0, 100) : '';
        const unreadClass = email.read ? '' : 'unread';
        const selectedClass = state.currentEmail && state.currentEmail.id === email.id ? 'selected' : '';
        const attachments = Array.isArray(email.attachments) && email.attachments.length > 0
            ? `<div class="email-item-attachments">📎 ${t('attachments', { count: email.attachments.length })}</div>`
            : (email.hasAttachment ? '<div class="email-item-attachments">📎</div>' : '');

        return `
            <div class="email-item ${unreadClass} ${selectedClass}" data-id="${email.id}">
                <label class="email-item-selection"><input type="checkbox" data-mail-select="${escapeHtmlAttribute(email.id)}" aria-label="${escapeHtmlAttribute(t('selectMessage', {subject:email.subject || t('noSubject')}))}" ${selectedMailIDs.has(email.id) ? 'checked' : ''} ${mailboxControlsBusy() ? 'disabled' : ''} onchange="toggleMailSelection(this.dataset.mailSelect, this.checked)"></label>
                <div class="email-item-open" data-id="${email.id}" tabindex="0" role="button">
                <div class="email-item-header">
                    <span class="email-item-from">${escapeHtml(from)}</span>
                    <span class="email-item-time">${time}</span>
                </div>
                <div class="email-item-subject">${escapeHtml(email.subject || t('noSubject'))}</div>
                ${preview ? `<div class="email-item-preview">${escapeHtml(preview)}</div>` : ''}
                ${attachments}
                </div>
            </div>
        `;
    }).join('');

    // Add click handlers
    container.querySelectorAll('.email-item-open').forEach(item => {
        item.addEventListener('click', () => {
            const id = item.dataset.id;
            loadEmailDetail(id);
        });
        item.addEventListener('keydown', (event) => {
            if (event.key !== 'Enter' && event.key !== ' ') return;
            event.preventDefault();
            loadEmailDetail(item.dataset.id);
        });
    });
}

function renderEmailDetail() {
    const container = document.getElementById('emailDetail');
    if (!container) return;

    if (!state.currentEmail) {
        emailTools = null;
        container.innerHTML = `<div class="empty-state"><p>${t('selectEmail')}</p></div>`;
        return;
    }

    const email = state.currentEmail;
    emailToolsFor(email);
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
            <span id="emailRelayActions">${renderEmailRelayActions(email)}</span>
            <button id="emailDeleteAction" class="btn btn-danger" ${relayPending.has(email.id) || mailboxBatchBusy ? 'disabled' : ''} onclick="deleteEmail('${email.id}')">${t('delete')}</button>
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
                ${email.run_id ? `<span class="email-detail-meta-label">${t('emailRunID')}</span><code>${escapeHtml(email.run_id)}</code>` : ''}
                <span class="email-detail-meta-label">${t('time')}</span>
                <span>${time}</span>
            </div>
        </div>
        ${renderEmailContentTabs(email)}
        ${attachments}
    `;
}

function renderEmailContentTabs(email) {
    const availableTabs = ['html', 'text', 'headers', 'source', 'inspect', 'deliveries'];
    if (!availableTabs.includes(emailContentTab) || (emailContentTab === 'html' && !email.html)) {
        emailContentTab = email.html ? 'html' : 'text';
    }
    const labels = { html: 'contentHTML', text: 'contentText', headers: 'contentHeaders', source: 'contentSource', inspect: 'contentInspect', deliveries: 'contentDeliveries' };
    return `
        <div class="email-content-tabs" role="tablist" aria-label="${t('emailContentViews')}">
            ${availableTabs.map((tab) => `
                <button type="button" role="tab" class="email-content-tab"
                    id="email-content-tab-${tab}" aria-controls="email-content-panel"
                    aria-selected="${emailContentTab === tab}" tabindex="${emailContentTab === tab ? '0' : '-1'}"
                    ${tab === 'html' && !email.html ? 'disabled' : ''}
                    onclick="setEmailContentTab('${tab}', true)"
                    onkeydown="handleEmailContentTabKeydown(event, '${tab}')">${t(labels[tab])}</button>
            `).join('')}
        </div>
        <div id="email-content-panel" class="email-detail-body" role="tabpanel"
            aria-labelledby="email-content-tab-${emailContentTab}">${renderEmailContentPanel(email)}</div>
    `;
}

function renderEmailContentPanel(email) {
    switch (emailContentTab) {
    case 'html':
        return renderHTML(email.html || '', email.id, email.attachments || []);
    case 'inspect':
        return renderEmailInspection(email);
    case 'deliveries':
        return renderEmailDeliveries(email);
    case 'headers':
        return `<pre class="email-detail-source">${escapeHtml(JSON.stringify(email.headers || {}, null, 2))}</pre>`;
    case 'source': {
        if (emailSourceOversized.has(email.id)) {
            return `<div class="email-detail-source-notice">${escapeHtml(t('sourceTooLarge'))} <button type="button" class="btn btn-secondary" onclick="viewEmailSource('${email.id}')">${t('viewSource')}</button></div>`;
        }
        const source = emailSourceCache.get(email.id);
        const sourceError = emailSourceErrors.get(email.id);
        if (sourceError !== undefined) {
            return `<div class="error">${escapeHtml(t('loadEmailDetailError', { error: sourceError }))}</div>`;
        }
        return source === undefined
            ? `<div class="loading">${t('sourceLoading')}</div>`
            : `<pre class="email-detail-source">${escapeHtml(source)}</pre>`;
    }
    default:
        return renderText(email.text || '');
    }
}

async function setEmailContentTab(tab, restoreFocus = false) {
    const email = state.currentEmail;
    if (!email || !['html', 'text', 'headers', 'source', 'inspect', 'deliveries'].includes(tab)) return;
    if (tab === 'html' && !email.html) return;
    emailContentTab = tab;
    const shouldLoadSource = tab === 'source' && !emailSourceCache.has(email.id) && !emailSourceOversized.has(email.id);
    if (shouldLoadSource) emailSourceErrors.delete(email.id);
    renderEmailDetail();
    if (restoreFocus) {
        document.getElementById(`email-content-tab-${tab}`)?.focus?.();
    }
    if (tab === 'deliveries' && !emailToolsFor(email).loaded) await refreshEmailDeliveries();
    if (!shouldLoadSource) return;
    if (Number(email.size) > EMAIL_SOURCE_INLINE_MAX_BYTES) {
        emailSourceOversized.add(email.id);
        renderEmailDetail();
        if (restoreFocus) document.getElementById(`email-content-tab-${tab}`)?.focus?.();
        return;
    }
    let request = emailSourceRequests.get(email.id);
    if (!request) {
        request = { restoreFocus: false };
        request.promise = API.getEmailSource(email.id, EMAIL_SOURCE_INLINE_MAX_BYTES)
            .then((result) => {
                if (emailSourceRequests.get(email.id) !== request) return;
                if (result.oversized) emailSourceOversized.add(email.id);
                else emailSourceCache.set(email.id, result.source);
                emailSourceErrors.delete(email.id);
            })
            .catch((error) => {
                if (emailSourceRequests.get(email.id) !== request) return;
                console.error('Failed to load email source:', error);
                const message = parseAPIError(error);
                emailSourceErrors.set(email.id, message);
                alert(t('loadEmailDetailError', { error: message }));
            })
            .finally(() => {
                if (emailSourceRequests.get(email.id) === request) emailSourceRequests.delete(email.id);
                if (state.currentEmail && state.currentEmail.id === email.id && emailContentTab === 'source') {
					const sourceTab = document.getElementById('email-content-tab-source');
					const shouldRestoreFocus = request.restoreFocus && document.activeElement === sourceTab;
                    renderEmailDetail();
					if (shouldRestoreFocus) document.getElementById('email-content-tab-source')?.focus?.();
                }
            });
        emailSourceRequests.set(email.id, request);
    }
    if (restoreFocus) request.restoreFocus = true;
    await request.promise;
}

function handleEmailContentTabKeydown(event, tab) {
    const email = state.currentEmail;
    if (!email) return;
    const tabs = ['html', 'text', 'headers', 'source', 'inspect', 'deliveries'].filter((candidate) => candidate !== 'html' || email.html);
    const current = tabs.indexOf(tab);
    if (current < 0) return;
    let next = current;
    if (event.key === 'ArrowRight') next = (current + 1) % tabs.length;
    else if (event.key === 'ArrowLeft') next = (current - 1 + tabs.length) % tabs.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = tabs.length - 1;
    else return;
    event.preventDefault();
    void setEmailContentTab(tabs[next], true);
}

function hasRemoteEmailResources(html) {
    const isRemoteURL = (value) => {
        for (const match of value.matchAll(/(?:https?:)?\/\/[^\s,'")]+/gi)) {
            try {
                if (new URL(match[0], window.location.origin).origin !== window.location.origin) return true;
            } catch (error) {
                // The server sanitizer removes unparseable resource URLs. If
                // legacy stored content reaches this layer, treat it as remote.
                return true;
            }
        }
        return false;
    };

    for (const tag of html.matchAll(/<(?:img|source|video|audio|link)\b[^>]*>/gi)) {
        for (const attribute of tag[0].matchAll(/(?:src|srcset|href|poster)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))/gi)) {
            if (isRemoteURL(attribute[1] || attribute[2] || attribute[3] || '')) return true;
        }
    }
    for (const cssURL of html.matchAll(/url\(\s*["']?\s*([^"')\s]+)/gi)) {
        if (isRemoteURL(cssURL[1])) return true;
    }
    return false;
}

function resolveCIDReferences(html, emailId, attachments) {
    const cidURLs = new Map();
    attachments.forEach((attachment) => {
        if (!attachment || !attachment.contentId || !attachment.generatedFileName) return;
        const cid = String(attachment.contentId).replace(/^<|>$/g, '').toLowerCase();
        cidURLs.set(cid, `${API_BASE}/emails/${encodeURIComponent(emailId)}/attachments/${encodeURIComponent(attachment.generatedFileName)}`);
    });

    return html.replace(/cid:([^"'\s)>]+)/gi, (reference, cid) => cidURLs.get(cid.toLowerCase()) || reference);
}

function previewContentSecurityPolicy(allowRemote) {
    const localOrigin = window.location.origin;
    const resourceSources = allowRemote
        ? `http: https: data: blob: ${localOrigin}`
        : `data: blob: ${localOrigin}`;
    const fontSources = allowRemote
        ? `http: https: data: ${localOrigin}`
        : `data: ${localOrigin}`;

    return [
        "default-src 'none'",
        "base-uri 'none'",
        "object-src 'none'",
        "frame-src 'none'",
        "form-action 'none'",
        "script-src 'none'",
        `img-src ${resourceSources}`,
        `media-src ${resourceSources}`,
        `font-src ${fontSources}`,
        `style-src 'unsafe-inline' ${localOrigin}${allowRemote ? ' http: https:' : ''}`,
        "connect-src 'none'"
    ].join('; ');
}

function injectPreviewSecurityHead(html, allowRemote) {
    const securityHead = `<meta http-equiv="Content-Security-Policy" content="${escapeHtml(previewContentSecurityPolicy(allowRemote))}">`
        + '<meta name="referrer" content="no-referrer"><meta name="color-scheme" content="light">';
    if (/<head(?:\s[^>]*)?>/i.test(html)) {
        return html.replace(/<head(?:\s[^>]*)?>/i, (head) => `${head}${securityHead}`);
    }
    if (/<html(?:\s[^>]*)?>/i.test(html)) {
        return html.replace(/<html(?:\s[^>]*)?>/i, (root) => `${root}<head>${securityHead}</head>`);
    }
    return `<head>${securityHead}</head>${html}`;
}

function renderEmailViewportPresets() {
    return `
        <div class="email-viewport-toolbar" role="group" aria-label="${t('emailViewportPresets')}">
            <span class="email-viewport-label">${t('emailViewportPresets')}</span>
            <div class="email-viewport-presets">
                ${EMAIL_VIEWPORT_PRESETS.map((preset) => `
                    <button type="button"
                        class="email-viewport-preset"
                        data-viewport-width="${preset.key}"
                        aria-pressed="${emailViewportPreset === preset.key}"
                        title="${t('emailViewportWidth', { width: preset.label })}"
                        onclick="setEmailViewport('${preset.key}')">${preset.label}</button>
                `).join('')}
            </div>
        </div>
    `;
}

function setEmailViewport(presetKey) {
    const preset = EMAIL_VIEWPORT_PRESETS.find((candidate) => candidate.key === presetKey);
    if (!preset) return;

    emailViewportPreset = preset.key;
    writePreference('owlmail.previewWidth', preset.key);
    const viewportFrame = document.getElementById('emailViewportFrame');
    if (!viewportFrame) return;

    const viewportStage = document.getElementById('emailViewportStage');
    const scrollLeft = viewportStage ? viewportStage.scrollLeft : 0;
    const scrollTop = viewportStage ? viewportStage.scrollTop : 0;

    // Resize the existing frame instead of rendering the message again. The
    // iframe node, srcdoc, sandbox, and its own scroll position stay intact.
    viewportFrame.style.width = preset.width;
    document.querySelectorAll('.email-viewport-preset').forEach((button) => {
        button.setAttribute('aria-pressed', button.dataset.viewportWidth === preset.key ? 'true' : 'false');
    });

    if (viewportStage) {
        viewportStage.scrollLeft = scrollLeft;
        viewportStage.scrollTop = scrollTop;
    }
}

function renderHTML(html, emailId, attachments) {
    const allowRemote = remoteContentAllowedEmailID === emailId;
    const remoteContentPresent = hasRemoteEmailResources(html);
    const cidResolvedHTML = resolveCIDReferences(html, emailId, attachments);
    const previewDocument = injectPreviewSecurityHead(cidResolvedHTML, allowRemote);
    const iframeId = 'email-html-' + Date.now();
    return `
        <div class="email-detail-html">
            ${remoteContentPresent && !allowRemote ? `
                <div class="email-remote-content-notice" role="status">
                    <span>${t('remoteContentBlocked')}</span>
                    <button type="button" class="btn btn-secondary" onclick="loadRemoteContent('${emailId}')">${t('loadRemoteContent')}</button>
                </div>
            ` : ''}
            ${renderEmailViewportPresets()}
            <div id="emailViewportStage" class="email-viewport-stage">
                <div id="emailViewportFrame" class="email-viewport-frame" style="width: ${EMAIL_VIEWPORT_PRESETS.find((preset) => preset.key === emailViewportPreset).width};">
                    <iframe id="${iframeId}" title="${t('emailPreviewTitle')}" sandbox="" referrerpolicy="no-referrer" srcdoc="${escapeHtmlAttribute(previewDocument)}"></iframe>
                </div>
            </div>
        </div>
    `;
}

function loadRemoteContent(emailId) {
    if (!state.currentEmail || state.currentEmail.id !== emailId) return;
    remoteContentAllowedEmailID = emailId;
    renderEmailDetail();
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
let emailListRequestSequence = 0;
let emailListActiveSequence = null;
let mailboxRefreshAfterLoad = false;
async function loadEmails({ background = false } = {}) {
    if (!background && mailboxRefreshTimer !== null) {
        clearTimeout(mailboxRefreshTimer);
        mailboxRefreshTimer = null;
        mailboxRefreshGeneration += 1;
    }
    const sequence = ++emailListRequestSequence;
    emailListActiveSequence = sequence;
    knownDeletedMailIDs.clear();
    renderEmailList();
    try {
        if (!background) showLoading('emailList');
        const data = await API.getEmailPreviews(
            state.currentPage * state.pageSize,
            state.pageSize,
            state.searchQuery,
            mailboxFilters
        );
        if (sequence !== emailListRequestSequence) return;
        const previews = data.previews || [];
        const remaining = previews.filter(email => !knownDeletedMailIDs.has(email.id));
        state.total = Math.max(0, (data.total || 0) - (previews.length - remaining.length));
        const lastPage = Math.max(0, Math.ceil(state.total / state.pageSize) - 1);
        if (state.currentPage > lastPage) {
            state.currentPage = lastPage;
            state.emails = [];
            renderEmailList();
            updateEmailCount();
            updatePagination();
            return await loadEmails({ background });
        }
        state.emails = remaining;
        renderEmailList();
        updateEmailCount();
        updatePagination();
    } catch (error) {
        if (sequence !== emailListRequestSequence) return;
        console.error('Failed to load emails:', error);
        if (!background) alert(t('loadEmailsError', { error: parseAPIError(error) }));
    } finally {
        if (!background) hideLoading('emailList');
        if (emailListActiveSequence === sequence) {
            emailListActiveSequence = null;
            renderEmailList();
            if (mailboxRefreshAfterLoad) {
                mailboxRefreshAfterLoad = false;
                scheduleMailboxRefresh();
            }
        }
    }
}

function renderEmailRelayActions(email) {
    if (!relayEnabled) return '';
    const tracking = relayTracking.get(email.id);
    const waiting = relayWaitControllers.has(email.id);
    return `<button class="btn btn-secondary" ${relayPending.has(email.id) || mailboxBatchBusy ? 'disabled' : ''} onclick="relayCurrentEmail('${email.id}')">${t('relayOriginal')}</button>
        <button class="btn btn-secondary" ${relayPending.has(email.id) || mailboxBatchBusy ? 'disabled' : ''} onclick="relayCurrentEmail('${email.id}', true)">${t('relayOverride')}</button>
        ${tracking ? `<span role="status">${t(waiting ? 'relayWaiting' : 'relayPaused')}</span>
            <button class="btn btn-secondary" onclick="${waiting ? 'stopRelayWaiting' : 'resumeRelayWaiting'}('${email.id}')">${t(waiting ? 'relayStopWait' : 'relayResumeWait')}</button>
            <button class="btn btn-secondary" onclick="setEmailContentTab('deliveries')">${t('contentDeliveries')}</button>` : ''}`;
}

function updateEmailRelayControls() {
    if (!state.currentEmail) return;
    const actions = document.getElementById('emailRelayActions');
    if (actions) actions.innerHTML = renderEmailRelayActions(state.currentEmail);
    const deletion = document.getElementById('emailDeleteAction');
    if (deletion) deletion.disabled = relayPending.has(state.currentEmail.id) || mailboxBatchBusy;
}

async function loadRelayAvailability() {
    try {
        const outgoing = await API.getOutgoingConfig();
        relayEnabled = outgoing && outgoing.enabled === true;
        if (state.currentEmail) updateEmailRelayControls();
    } catch (error) {
        relayEnabled = false;
        updateEmailRelayControls();
        console.warn('Unable to inspect outgoing relay configuration:', error);
    }
}

function relayWaitError(code) {
    const error = new Error(code);
    error.code = code;
    return error;
}
function relayWaitOperation(operation, signal, timeoutMs = 10000) {
    return new Promise((resolve, reject) => {
        const controller = new AbortController();
        let settled = false;
        const finish = (error, value) => {
            if (settled) return;
            settled = true;
            clearTimeout(timer);
            signal?.removeEventListener('abort', abort);
            controller.abort();
            if (error) reject(error); else resolve(value);
        };
        const abort = () => finish(relayWaitError(signal.reason || 'RELAY_WAIT_STOPPED'));
        const timer = setTimeout(() => finish(relayWaitError('RELAY_REQUEST_TIMEOUT')), timeoutMs);
        if (signal?.aborted) { abort(); return; }
        signal?.addEventListener('abort', abort, { once: true });
        Promise.resolve().then(() => operation(controller.signal)).then(value => finish(null, value), finish);
    });
}
async function waitForRelayJob(statusUrl, { signal } = {}) {
    while (true) {
        let statusResult;
        try {
            statusResult = await relayWaitOperation(requestSignal => API.getRelayJob(statusUrl, requestSignal), signal);
        } catch (error) {
            if (signal?.aborted || !relayStatusErrorIsTransient(error)) throw error;
            console.warn('Relay status check failed; retrying:', error);
        }
        const current = statusResult && statusResult.data;
        if (current && (current.status === 'succeeded' || current.status === 'failed')) return current;
        await relayWaitOperation(() => new Promise(resolve => setTimeout(resolve, RELAY_STATUS_POLL_INTERVAL_MS)), signal);
    }
}
function stopRelayWaiting(id) {
    relayWaitControllers.get(id)?.abort('RELAY_WAIT_STOPPED');
}
function cacheRelayCompletion(id, completed) {
    const staleRefreshes = new Set([...deliveryRefreshesInFlight].filter(tools => tools.id === id));
    if (completed.id && staleRefreshes.size) completedRelayJobs.set(completed.id, { refreshes: staleRefreshes, job: completed });
    if (emailTools?.id !== id) return;
    const tools = emailTools;
    tools.notice = '';
    const job = tools.relay?.jobs?.find(job => job.id === completed.id);
    if (job) Object.assign(job, completed);
    else tools.loaded = false;
    if (state.currentEmail?.id === id && emailContentTab === 'deliveries') {
        if (job) renderEmailDetailKeepingTabFocus();
        else void refreshEmailDeliveries();
    }
}
async function resumeRelayWaiting(id, timeoutMs = 60000) {
    const tracking = relayTracking.get(id);
    if (!tracking || relayWaitControllers.has(id)) return;
    const controller = new AbortController();
    relayWaitControllers.set(id, controller);
    tracking.paused = false;
    updateEmailRelayControls();
    const timer = setTimeout(() => controller.abort('RELAY_WAIT_TIMEOUT'), timeoutMs);
    try {
        const completed = await waitForRelayJob(tracking.statusUrl, { signal: controller.signal });
        if (relayTracking.get(id) !== tracking) return;
        tracking.onCompleted?.(completed);
        cacheRelayCompletion(id, completed);
        relayTracking.delete(id);
        relayPending.delete(id);
        if (completed.status === 'failed') alert(t('relayError', { error: completed.errorCategory || 'delivery failed' }));
    } catch (error) {
        if (relayTracking.get(id) !== tracking) return;
        tracking.paused = true;
        alert(t(error.code === 'RELAY_WAIT_TIMEOUT' ? 'relayWaitTimeout' : error.code === 'RELAY_WAIT_STOPPED' ? 'relayWaitStopped' : 'relayError', { error: parseAPIError(error) }));
    } finally {
        clearTimeout(timer);
        if (relayWaitControllers.get(id) === controller) relayWaitControllers.delete(id);
        syncRelayMutationControls();
        if (state.currentEmail && state.currentEmail.id === id) updateEmailRelayControls();
    }
}

async function relayCurrentEmail(id, askForRecipient = false) {
    if (!relayEnabled || relayPending.has(id) || mailboxBatchBusy) return;
    let relayTo = '';
    if (askForRecipient) {
        relayTo = (prompt(t('relayRecipientPrompt')) || '').trim();
        if (!relayTo) return;
    }
    let recipient = relayTo;
    let confirmedRecipients = null;
    if (!askForRecipient) {
        const envelopeRecipients = state.currentEmail && state.currentEmail.id === id
            && state.currentEmail.envelope && Array.isArray(state.currentEmail.envelope.to)
            ? state.currentEmail.envelope.to.filter((value) => String(value).trim() !== '')
            : [];
        if (envelopeRecipients.length === 0) {
            alert(t('relayNoOriginalRecipients'));
            return;
        }
        let preflight;
        try {
            preflight = await API.getRelayPreflight(id);
        } catch (error) {
            alert(t('relayError', { error: parseAPIError(error) }));
            return;
        }
        if (!state.currentEmail || state.currentEmail.id !== id || mailboxBatchBusy) return;
        confirmedRecipients = preflight && preflight.data && Array.isArray(preflight.data.recipients)
            ? preflight.data.recipients
            : [];
        if (confirmedRecipients.length === 0) {
            alert(t('relayNoEffectiveRecipients'));
            return;
        }
        recipient = confirmedRecipients.join(', ');
    }
    if (!confirm(t('relayConfirm', { recipient }))) return;

    relayPending.add(id);
    syncRelayMutationControls();
    updateEmailRelayControls();
    try {
        const result = await API.relayEmail(id, relayTo, confirmedRecipients);
        const data = result && result.data;
        const job = data && data.job;
        alert(t('relayQueued', { id: job && job.id ? job.id : t('unknown') }));
        if (!data || !data.statusUrl) {
            throw new Error('Relay status URL is missing');
        }
        relayTracking.set(id, { jobID: job?.id, statusUrl: data.statusUrl, paused: false });
        await resumeRelayWaiting(id);
    } catch (error) {
        alert(t('relayError', { error: parseAPIError(error) }));
    } finally {
        if (!relayTracking.has(id)) relayPending.delete(id);
        syncRelayMutationControls();
        if (state.currentEmail && state.currentEmail.id === id) updateEmailRelayControls();
    }
}

async function loadEmailDetail(id, { historyMode = 'push' } = {}) {
    if (!id) {
        clearEmailSelection(historyMode);
        return;
    }
    const requestSequence = ++emailDetailRequestSequence;
    try {
        showLoading('emailDetail');
        const email = await API.getEmail(id);
        if (requestSequence !== emailDetailRequestSequence) return;
        if (historyMode === 'none' && currentEmailIDFromLocation() !== id) return;
        remoteContentAllowedEmailID = null;
        emailSourceCache.clear();
        emailSourceErrors.clear();
        emailSourceOversized.clear();
        emailSourceRequests.clear();
        emailContentTab = email.html ? 'html' : 'text';
        state.currentEmail = email;
        renderEmailDetail();
        renderEmailList(); // Update selected state
        if (historyMode !== 'none') updateEmailLocation(id, historyMode);
        const selected = Array.from(document.getElementById('emailList')?.querySelectorAll('.email-item') || [])
            .find((item) => item.dataset.id === id);
        if (selected && typeof selected.scrollIntoView === 'function') {
            selected.scrollIntoView({ block: 'nearest' });
        }
    } catch (error) {
        if (requestSequence !== emailDetailRequestSequence) return;
        if (historyMode === 'none' && currentEmailIDFromLocation() === id) {
            clearEmailSelection('none');
        }
        console.error('Failed to load email detail:', error);
        const errorMsg = parseAPIError(error);
        alert(t('loadEmailDetailError', { error: errorMsg }));
    } finally {
        hideLoading('emailDetail');
    }
}

function removeKnownDeletedEmail(id) {
    const deleted = new Set(Array.isArray(id) ? id : [id]);
    for (const value of deleted) { selectedMailIDs.delete(value); knownDeletedMailIDs.add(value); }
    const remaining = state.emails.filter(email => !deleted.has(email.id));
    if (remaining.length === state.emails.length) { updateMailboxSelectionControls(); return; }
    state.total = Math.max(0, state.total - (state.emails.length - remaining.length));
    state.emails = remaining;
    const lastPage = Math.max(0, Math.ceil(state.total / state.pageSize) - 1);
    if (state.currentPage > lastPage) {
        emailListRequestSequence += 1;
        emailListActiveSequence = null;
        state.currentPage = lastPage;
        state.emails = [];
    }
    renderEmailList();
    updateEmailCount();
    updatePagination();
    updateMailboxSelectionControls();
}
async function deleteEmail(id) {
    if (relayPending.has(id) || mailboxBatchBusy) return;
    if (!confirm(t('deleteConfirm'))) return;

    try {
        showLoading();
        await API.deleteEmail(id);
        removeKnownDeletedEmail(id);
        if (state.currentEmail && state.currentEmail.id === id) clearEmailSelection('replace');
        await loadEmails();
    } catch (error) {
        console.error('Failed to delete email:', error);
        const errorMsg = parseAPIError(error);
        alert(t('deleteEmailError', { error: errorMsg }));
    } finally {
        hideLoading();
    }
}

async function deleteAllEmails() {
    if (relayPending.size > 0 || mailboxBatchBusy) return;
    if (!confirm(t('deleteAllConfirm'))) return;

    try {
        showLoading();
        await API.deleteAllEmails();
        emailListRequestSequence += 1;
        emailListActiveSequence = null;
        mailboxRefreshAfterLoad = false;
        state.currentPage = 0;
        state.emails = [];
        selectedMailIDs.clear();
        state.total = 0;
        updateMailboxSelectionControls();
        clearEmailSelection('replace');
        renderEmailList();
        updateEmailCount();
        updatePagination();
        scheduleMailboxRefresh();
    } catch (error) {
        console.error('Failed to delete all emails:', error);
        const errorMsg = parseAPIError(error);
        alert(t('deleteAllEmailsError', { error: errorMsg }));
    } finally {
        hideLoading();
    }
}

async function markAllRead() {
    if (mailboxBatchBusy) return;
    try {
        showLoading();
        const result = await API.markAllRead();
        for (const email of state.emails) email.read = true;
        if (state.currentEmail) state.currentEmail.read = true;
        if (mailboxFilters.read === 'false') {
            state.emails = []; state.total = 0; state.currentPage = 0;
        }
        renderEmailList(); updateEmailCount(); updatePagination();
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
    clearMailboxQueryRows(true);
    loadEmails();
}

function nextPage() {
    const maxPage = Math.ceil(state.total / state.pageSize) - 1;
    if (state.currentPage < maxPage) {
        state.currentPage++;
        clearMailboxQueryRows();
        loadEmails();
    }
}

function prevPage() {
    if (state.currentPage > 0) {
        state.currentPage--;
        clearMailboxQueryRows();
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
    if (!addr || typeof addr !== 'object') return t('unknown');
    
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

function escapeHtmlAttribute(text) {
    return escapeHtml(text)
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
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

const loadingCounts = new Map();
function showLoading(region = 'global') {
    loadingCounts.set(region, (loadingCounts.get(region) || 0) + 1);
    const element = document.getElementById(region === 'global' ? 'loadingOverlay' : region);
    if (!element) return;
    if (region === 'global') element.style.display = 'flex';
    else { element.setAttribute('aria-busy', 'true'); element.classList.add('is-loading'); if (region === 'emailDetail') element.inert = true; }
}
function hideLoading(region = 'global') {
    const count = Math.max(0, (loadingCounts.get(region) || 0) - 1);
    if (count) loadingCounts.set(region, count); else loadingCounts.delete(region);
    if (count) return;
    const element = document.getElementById(region === 'global' ? 'loadingOverlay' : region);
    if (!element) return;
    if (region === 'global') element.style.display = 'none';
    else { element.setAttribute('aria-busy', 'false'); element.classList.remove('is-loading'); if (region === 'emailDetail') element.inert = false; }
}

// Theme Management
let currentTheme = 'light';
let sessionThemePreference = null;
function readThemePreference() {
    const saved = readPreference('theme');
    return saved === 'dark' || saved === 'light' ? saved : sessionThemePreference;
}
function systemTheme() {
    return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}
function initTheme() {
    const saved = readThemePreference();
    setTheme(saved === 'dark' || saved === 'light' ? saved : systemTheme(), false);
    if (window.matchMedia) {
        const media = window.matchMedia('(prefers-color-scheme: dark)');
        media.addEventListener?.('change', () => {
            const preference = readThemePreference();
            if (preference !== 'dark' && preference !== 'light') setTheme(systemTheme(), false);
        });
    }
    window.addEventListener('storage', (event) => {
        if (event.key === 'theme' || event.key === null) {
            sessionThemePreference = null;
            const saved = readThemePreference();
            setTheme(saved === 'dark' || saved === 'light' ? saved : systemTheme(), false);
        }
    });
}
function setTheme(theme, persist = true) {
    currentTheme = theme === 'dark' ? 'dark' : 'light';
    document.body.classList.remove('light-theme', 'dark-theme');
    document.body.classList.add(currentTheme + '-theme');
    const toggle = document.getElementById('themeToggle');
    if (toggle) {
        toggle.textContent = currentTheme === 'dark' ? '☀️' : '🌙';
        toggle.setAttribute('aria-pressed', String(currentTheme === 'dark'));
    }
    if (persist) {
        sessionThemePreference = currentTheme;
        writePreference('theme', currentTheme);
    }
}
function toggleTheme() {
    setTheme(currentTheme === 'light' ? 'dark' : 'light');
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
    setLanguage(detectLanguage(), false);
    
    // Initialize language selector
    initLanguageSelector();
    
    // Initialize theme
    initTheme();

    // Initialize opt-in browser notifications without prompting on page load.
    initializeBrowserNotifications();
    void loadRelayAvailability();

    // Load initial emails, then honor an email deep link without adding a
    // duplicate browser-history entry.
    const initialEmailID = currentEmailIDFromLocation();
    const initialLoad = loadEmails();
    if (initialEmailID) {
        void Promise.resolve(initialLoad).then(() => {
            if (currentEmailIDFromLocation() !== initialEmailID) return;
            return loadEmailDetail(initialEmailID, { historyMode: 'none' });
        });
    }

    window.addEventListener('popstate', handleHistoryNavigation);
    document.addEventListener('keydown', handleMailboxKeydown);

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
window.relayCurrentEmail = relayCurrentEmail;
window.setEmailContentTab = setEmailContentTab;
window.handleEmailContentTabKeydown = handleEmailContentTabKeydown;
window.t = t; // Make translation function available globally

// Only the selected message owns tool state; late responses cannot replace a new selection.
let emailTools = null;
function emailToolsFor(email) {
    if (!emailTools || emailTools.id !== email.id) {
        emailTools = { id: email.id, subject: '', text: '', hosts: '', codes: false, report: null, error: '', revision: 0, checking: false, busy: false, loaded: false, relay: null, webhook: null, notice: '' };
    }
    return emailTools;
}
function updateEmailInspectionField(field, value) {
    if (!state.currentEmail || !['subject', 'text', 'hosts', 'codes'].includes(field)) return;
    const tools = emailToolsFor(state.currentEmail);
    if (tools[field] === value) return;
    tools[field] = value;
    tools.revision++; tools.report = null; tools.error = '';
    // Clear only the result region so typing keeps its focus and caret.
    const status = document.getElementById('emailInspectionStatus');
    if (status) status.innerHTML = '';
}
function renderEmailInspection(email) {
    const tools = emailToolsFor(email);
    const hasPlainText = Boolean(email.text);
    return `<section class="email-tools">
        <label>${t('inspectSubject')}<input maxlength="1024" value="${escapeHtmlAttribute(tools.subject)}" oninput="updateEmailInspectionField('subject', this.value)"></label>
        <label>${t('inspectText')}<input ${hasPlainText ? '' : 'disabled'} maxlength="1024" value="${escapeHtmlAttribute(tools.text)}" oninput="updateEmailInspectionField('text', this.value)"></label>
        <label>${t('inspectHosts')}<input maxlength="2048" value="${escapeHtmlAttribute(tools.hosts)}" oninput="updateEmailInspectionField('hosts', this.value)"></label>
        <label class="email-tools-checkbox"><input type="checkbox" ${hasPlainText ? '' : 'disabled'} ${tools.codes ? 'checked' : ''} onchange="updateEmailInspectionField('codes', this.checked)">${t('inspectCodes')}</label>
        ${hasPlainText ? '' : `<p>${t('inspectPlainTextOnly')}</p>`}
        <button id="emailInspectionRun" type="button" class="btn btn-secondary" aria-disabled="${tools.checking}" onclick="runEmailInspection()">${t(tools.checking ? 'loading' : 'inspectRun')}</button>
        <div id="emailInspectionStatus" role="status" aria-live="polite">
        ${renderEmailInspectionReport(tools)}
        </div></section>`;
}
function renderEmailInspectionReport(tools) {
    const report = tools.report;
    return `        ${tools.error ? `<p class="error">${escapeHtml(tools.error)}</p>` : ''}
        ${report ? `<h3>${t(report.passed ? 'inspectPass' : 'inspectFail')}</h3>
            <ul>${report.checks.map(check => `<li>${check.passed ? '✓' : '✗'} ${escapeHtml(check.name.startsWith('subject_contains') ? t('inspectSubject') : check.name.startsWith('text_contains') ? t('inspectText') : check.name === 'allowed_link_hosts' ? 'Link hosts' : check.name)}: ${escapeHtml(check.message)}</li>`).join('')}</ul>
            ${report.links_truncated || report.extractions_truncated ? `<p class="error">${t('inspectTruncated')}</p>` : ''}
            <h4>Links</h4><ul>${report.links.map(link => `<li>${escapeHtml(link.url)}</li>`).join('')}</ul>
            ${Object.entries(report.extractions).map(([name, values]) => `<h4>${escapeHtml(name)}</h4><ul>${values.map(value => `<li><code>${escapeHtml(value)}</code></li>`).join('')}</ul>`).join('')}` : ''}`;
}
function updateEmailInspectionDisplay(tools) {
    const button = document.getElementById('emailInspectionRun');
    if (button) { button.setAttribute('aria-disabled', String(tools.checking)); button.textContent = t(tools.checking ? 'loading' : 'inspectRun'); }
    const status = document.getElementById('emailInspectionStatus');
    if (status) status.innerHTML = renderEmailInspectionReport(tools);
}
async function emailToolRequest(path, options) {
    const response = await fetch(`${API_BASE}${path}`, options);
    if (!response.ok) throw new Error(parseAPIError(await response.text()));
    return response.json();
}
async function runEmailInspection() {
    if (!state.currentEmail) return;
    const tools = emailToolsFor(state.currentEmail);
    if (tools.checking) return;
    const assertions = state.currentEmail.text ? [tools.subject, tools.text] : [tools.subject];
    if (assertions.some(value => new TextEncoder().encode(value).length > 1024)) {
        tools.report = null; tools.error = t('inspectAssertionTooLarge');
        updateEmailInspectionDisplay(tools);
        return;
    }
    const revision = tools.revision;
    tools.checking = true; tools.error = ''; tools.report = null;
    const request = { subject_contains: tools.subject ? [tools.subject] : [], text_contains: state.currentEmail.text && tools.text ? [tools.text] : [] };
    if (tools.hosts.trim()) request.allowed_link_hosts = tools.hosts.split(',').map(host => host.trim()).filter(Boolean);
    if (state.currentEmail.text && tools.codes) request.extract = [{ name: 'codes', pattern: '\\b([0-9]{6})\\b' }];
    updateEmailInspectionDisplay(tools);
    try {
        const report = await emailToolRequest(`/emails/${encodeURIComponent(tools.id)}/check`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(request) });
        if (tools.revision === revision) tools.report = report;
    }
    catch (error) { if (tools.revision === revision) tools.error = String(error.message || error); }
    finally {
        tools.checking = false;
        if (emailTools === tools && state.currentEmail?.id === tools.id && emailContentTab === 'inspect') {
            updateEmailInspectionDisplay(tools);
        }
    }
}
function renderEmailDeliveries(email) {
    const tools = emailToolsFor(email);
    const renderGroup = (kind, result, field) => {
        if (!result) return '';
        if (result.error) return `<p class="error">${escapeHtml(result.error)}</p>`;
        const rows = result[field] || [];
        return `<h3>${kind === 'relay' ? 'Relay' : 'Webhook'}</h3>
            ${result.persistence_healthy === false ? `<p class="error">${t('workflowHealth')}</p>` : ''}
            ${result.enabled === false ? `<p>${t('workflowDisabled')}</p>` : ''}
            ${rows.length ? `<ul class="email-delivery-list">${rows.map((job, index) => `<li><code>${escapeHtml(job.id)}</code> — ${escapeHtml(job.status)}
                ${kind === 'relay' ? `<p>Attempts: ${escapeHtml(String(job.attempts))}; ${escapeHtml(job.errorCategory || '')}</p>
                <p>${escapeHtml((job.confirmedRecipients || []).join(', '))}</p>
                ${job.nextAttemptAt ? `<p>Next attempt: ${escapeHtml(job.nextAttemptAt)}</p>` : ''}` : `<ul>${(job.outcomes || []).map(outcome => `<li>${escapeHtml(outcome.target || 'Queue')}: ${escapeHtml(outcome.error_category || 'Succeeded')}; attempts: ${escapeHtml(String(outcome.attempts))}; HTTP: ${escapeHtml(String(outcome.status_code))}</li>`).join('')}</ul>`}
                ${emailWorkflowRetryable(kind, job) ? `<button id="${escapeHtml(emailWorkflowActionID(kind, job.id))}" type="button" class="btn btn-secondary" aria-disabled="${tools.busy || mailboxBatchBusy}" onclick="retryEmailWorkflow('${kind}', ${index})">${t('workflowRetry')}</button>` : ''}</li>`).join('')}</ul>` : `<p>${t('workflowEmpty')}</p>`}
            ${result.more_possible ? `<p>${t('workflowMore')}</p>` : ''}`;
    };
    return `<section id="emailDeliveriesPanel" class="email-tools"><button id="emailDeliveriesRefresh" type="button" class="btn btn-secondary" aria-disabled="${tools.busy}" onclick="refreshEmailDeliveries()">${t(tools.busy ? 'loading' : 'refresh')}</button>
        <p role="status" aria-live="polite">${escapeHtml(tools.notice)}</p>
        ${renderGroup('relay', tools.relay, 'jobs')}${renderGroup('webhook', tools.webhook, 'deliveries')}</section>`;
}
function emailWorkflowActionID(kind, jobID) {
    return `email-workflow-retry-${kind}-${encodeURIComponent(jobID)}`;
}
function emailWorkflowRetryable(kind, job) {
    return job?.status === 'failed' && (kind === 'relay' || (job.outcomes || []).some(outcome => outcome.target && outcome.error_category));
}
function renderEmailDetailKeepingTabFocus() {
    const active = document.activeElement;
    const tab = active && ['html', 'text', 'headers', 'source', 'inspect', 'deliveries'].find(name => document.getElementById(`email-content-tab-${name}`) === active);
    const actionID = active?.id === 'emailDeliveriesRefresh' || active?.id?.startsWith('email-workflow-retry-') ? active.id : null;
    const panel = document.getElementById('emailDeliveriesPanel');
    if (!panel || !state.currentEmail || emailContentTab !== 'deliveries') return;
    panel.outerHTML = renderEmailDeliveries(state.currentEmail);
    if (tab) document.getElementById(`email-content-tab-${tab}`)?.focus?.();
    else if (actionID) (document.getElementById(actionID) || document.getElementById('emailDeliveriesRefresh'))?.focus?.();
}
async function refreshEmailDeliveries() {
    if (!state.currentEmail) return;
    const tools = emailToolsFor(state.currentEmail);
    if (tools.busy) return;
    tools.busy = true; tools.notice = ''; renderEmailDetailKeepingTabFocus();
    const query = `?email_id=${encodeURIComponent(tools.id)}&limit=100`;
    deliveryRefreshesInFlight.add(tools);
    const results = await Promise.allSettled([emailToolRequest(`/relay-jobs${query}`), emailToolRequest(`/webhook-deliveries${query}`)]);
    tools.relay = results[0].status === 'fulfilled' ? results[0].value : { error: String(results[0].reason.message || results[0].reason) };
    if (Array.isArray(tools.relay?.jobs)) {
        for (const { refreshes, job } of completedRelayJobs.values()) {
            if (!refreshes.has(tools)) continue;
            const cached = tools.relay.jobs.find(candidate => candidate.id === job.id);
            if (cached) Object.assign(cached, job);
            else tools.relay.jobs.unshift({ ...job });
        }
    }
    const tracking = relayTracking.get(tools.id);
    const terminal = tracking?.jobID && tools.relay?.jobs?.find(job => job.id === tracking.jobID && ['succeeded', 'failed'].includes(job.status));
    if (terminal) {
        tracking.onCompleted?.(terminal);
        cacheRelayCompletion(tools.id, terminal);
        relayTracking.delete(tools.id);
        relayPending.delete(tools.id);
        const waiter = relayWaitControllers.get(tools.id);
        relayWaitControllers.delete(tools.id);
        waiter?.abort('RELAY_WAIT_RECONCILED');
        tools.notice = '';
        syncRelayMutationControls();
        if (state.currentEmail?.id === tools.id) updateEmailRelayControls();
    }
    deliveryRefreshesInFlight.delete(tools);
    for (const [id, completion] of completedRelayJobs) {
        completion.refreshes.delete(tools);
        if (!completion.refreshes.size) completedRelayJobs.delete(id);
    }
    tools.webhook = results[1].status === 'fulfilled' ? results[1].value : { error: String(results[1].reason.message || results[1].reason) };
    tools.busy = false; tools.loaded = true;
    if (emailTools === tools && state.currentEmail?.id === tools.id && emailContentTab === 'deliveries') renderEmailDetailKeepingTabFocus();
}
async function retryEmailWorkflow(kind, index) {
    if (!state.currentEmail || !['relay', 'webhook'].includes(kind)) return;
    const tools = emailToolsFor(state.currentEmail);
    const field = kind === 'relay' ? 'jobs' : 'deliveries';
    const job = tools[kind]?.[field]?.[index];
    if (mailboxBatchBusy || tools.busy || kind === 'relay' && relayPending.has(tools.id) || !job || !emailWorkflowRetryable(kind, job) || !confirm(t('workflowRetryConfirm'))) return;
    tools.busy = true; tools.notice = ''; renderEmailDetailKeepingTabFocus();
    if (kind === 'relay') {
        relayPending.add(tools.id);
        syncRelayMutationControls();
        updateEmailRelayControls();
    }
    try {
        const result = await emailToolRequest(`/${kind === 'relay' ? 'relay-jobs' : 'webhook-deliveries'}/${encodeURIComponent(job.id)}/retry`, { method: 'POST' });
        tools.notice = t('workflowQueued');
        if (kind === 'relay') {
            const data = result?.data;
            if (!data?.job || !data.statusUrl) throw new Error('Relay retry status is missing');
            tools.relay.jobs.unshift(data.job);
            if (emailContentTab === 'deliveries' && state.currentEmail?.id === tools.id) renderEmailDetailKeepingTabFocus();
            relayTracking.set(tools.id, { jobID: data.job.id, statusUrl: data.statusUrl, paused: false, onCompleted: completed => {
                Object.assign(data.job, completed);
                const cachedJob = tools.relay?.jobs?.find(job => job.id === data.job.id);
                if (cachedJob) Object.assign(cachedJob, completed);
                else tools.loaded = false;
                tools.notice = '';
            } });
            await resumeRelayWaiting(tools.id);
            tools.notice = relayTracking.has(tools.id) ? t('relayPaused') : '';
        } else { job.status = 'queued'; }
    }
    catch (error) { tools.notice = String(error.message || error); }
    finally {
        if (kind === 'relay') {
            if (!relayTracking.has(tools.id)) relayPending.delete(tools.id);
            syncRelayMutationControls();
            if (state.currentEmail?.id === tools.id) updateEmailRelayControls();
        }
        tools.busy = false;
        if (emailTools === tools && state.currentEmail?.id === tools.id && emailContentTab === 'deliveries') renderEmailDetailKeepingTabFocus();
    }
}


Object.assign(i18n.en, { filterSortOrder:'Sort direction', filterDateRangeError:'The through date must be on or after the from date.' });
Object.assign(i18n['zh-CN'], { filterSortOrder:'排序方向', filterDateRangeError:'截止日期不能早于起始日期。' });
let mailboxFilters = {};
const selectedMailIDs = new Set();
let mailboxBatchBusy = false;
const MAILBOX_FILTER_FIELDS = ['from', 'to', 'dateFrom', 'dateTo', 'read', 'sortBy', 'sortOrder'];
function renderMailboxTools({ preserveDraft = true } = {}) {
    const root = document.getElementById('mailboxTools');
    if (!root) return;
    const draft = { ...mailboxFilters };
    if (preserveDraft && document.getElementById('mailboxFilterForm')) {
        for (const field of MAILBOX_FILTER_FIELDS) {
            const control = document.getElementById('mail-filter-' + field);
            if (control) draft[field] = control.value;
        }
    }
    const expanded = root.querySelector?.('.mailbox-filters')?.open || false;
    const input = (field, label, type = 'text') => `<label>${t(label)}<input id="mail-filter-${field}" type="${type}" value="${escapeHtmlAttribute(draft[field] || '')}"></label>`;
    const select = (field, label, options) => `<label>${t(label)}<select id="mail-filter-${field}">${options.map(([value, key]) => `<option value="${value}" ${String(draft[field] || '') === value ? 'selected' : ''}>${t(key)}</option>`).join('')}</select></label>`;
    root.innerHTML = `<details class="mailbox-filters" ${expanded ? 'open' : ''}><summary>${t('mailFilters')}</summary><form id="mailboxFilterForm" class="mailbox-filter-grid">
        ${input('from','filterFrom')}${input('to','filterTo')}${input('dateFrom','filterDateFrom','date')}${input('dateTo','filterDateTo','date')}
        ${select('read','filterRead',[['','filterAny'],['false','filterUnread'],['true','filterReadOnly']])}
        ${select('sortBy','filterSort',[['time','sortTime'],['store','sortStore'],['subject','sortSubject'],['from','sortFrom'],['size','sortSize']])}
        ${select('sortOrder','filterSortOrder',[['','sortDesc'],['asc','sortAsc']])}
        <div><button type="submit" class="btn btn-primary">${t('applyFilters')}</button><button type="button" class="btn btn-secondary" onclick="resetMailboxFilters()">${t('resetFilters')}</button></div>
    </form></details>
    <div class="mailbox-batch-actions"><label><input id="mailSelectPage" type="checkbox" onchange="selectMailboxPage(this.checked)"> ${t('selectPage')}</label>
        <span id="mailSelectionCount" role="status"></span>
        <button id="mailBatchRead" type="button" class="btn btn-secondary" onclick="runMailboxBatch('read')">${t('batchRead')}</button>
        <button id="mailBatchDelete" type="button" class="btn btn-danger" onclick="runMailboxBatch('delete')">${t('batchDelete')}</button>
        <button id="mailExportSelected" type="button" class="btn btn-secondary" onclick="exportMailbox(true)">${t('exportSelected')}</button>
        <button id="mailExportFiltered" type="button" class="btn btn-secondary" onclick="exportMailbox(false)">${t('exportFiltered')}</button>
        <span id="mailBatchResult" role="status"></span>
    </div>`;
    document.getElementById('mailboxFilterForm')?.addEventListener('submit', event => { event.preventDefault(); void applyMailboxFilters(); });
    document.getElementById('mail-filter-dateTo')?.addEventListener('input', event => event.target.setCustomValidity(''));
    document.getElementById('mail-filter-dateFrom')?.addEventListener('input', () => document.getElementById('mail-filter-dateTo')?.setCustomValidity(''));
    updateMailboxSelectionControls();
}
async function applyMailboxFilters() {
    const form = document.getElementById('mailboxFilterForm');
    if (form && !form.reportValidity()) return;
    const filters = {};
    for (const field of MAILBOX_FILTER_FIELDS) {
        const value = document.getElementById('mail-filter-' + field)?.value?.trim() || '';
        if (value) filters[field] = value;
    }
    const through = document.getElementById('mail-filter-dateTo');
    through?.setCustomValidity('');
    if (filters.dateFrom && filters.dateTo && filters.dateFrom > filters.dateTo) {
        through?.setCustomValidity(t('filterDateRangeError')); form?.reportValidity(); return;
    }
    mailboxFilters = filters;
    state.searchQuery = document.getElementById('searchInput')?.value?.trim() || '';
    state.currentPage = 0;
    clearMailboxQueryRows(true);
    await loadEmails();
}
async function resetMailboxFilters() {
    mailboxFilters = {}; state.currentPage = 0;
    clearMailboxQueryRows(true);
    renderMailboxTools({ preserveDraft: false }); await loadEmails();
}
function clearMailboxQueryRows(resetTotal = false) {
    selectedMailIDs.clear();
    state.emails = [];
    if (resetTotal) state.total = 0;
    renderEmailList(); updateEmailCount(); updatePagination();
}
function mailboxControlsBusy() { return mailboxBatchBusy || emailListActiveSequence !== null; }
function toggleMailSelection(id, checked) {
    if (mailboxControlsBusy() || !state.emails.some(email => email.id === id)) return;
    if (checked) selectedMailIDs.add(id); else selectedMailIDs.delete(id);
    updateMailboxSelectionControls();
}
function selectMailboxPage(checked) {
    if (mailboxControlsBusy()) return;
    for (const email of state.emails) { if (checked) selectedMailIDs.add(email.id); else selectedMailIDs.delete(email.id); }
    renderEmailList(); updateMailboxSelectionControls();
}
function updateMailboxSelectionControls() {
    const ids = Array.from(selectedMailIDs);
    const count = document.getElementById('mailSelectionCount');
    if (count) count.textContent = t('selectionCount', { count: ids.length });
    const select = document.getElementById('mailSelectPage');
    if (select) {
        select.checked = state.emails.length > 0 && state.emails.every(email => selectedMailIDs.has(email.id));
        select.indeterminate = ids.length > 0 && !select.checked;
        select.disabled = mailboxControlsBusy() || state.emails.length === 0;
    }
    for (const id of ['mailBatchRead','mailBatchDelete','mailExportSelected']) {
        const button = document.getElementById(id);
        if (button) button.disabled = mailboxControlsBusy() || ids.length === 0 || id === 'mailBatchDelete' && ids.some(mailID => relayPending.has(mailID));
    }
    const filtered = document.getElementById('mailExportFiltered');
    if (filtered) filtered.disabled = mailboxControlsBusy() || state.total === 0;
}
async function runMailboxBatch(action) {
    if (!['read','delete'].includes(action) || mailboxControlsBusy() || loadingCounts.has('global')) return;
    const ids = Array.from(selectedMailIDs);
    if (!ids.length || action === 'delete' && ids.some(id => relayPending.has(id))) return;
    if (action === 'delete' && !confirm(t('batchDeleteConfirm', { count: ids.length }))) return;
    mailboxBatchBusy = true; syncRelayMutationControls(); updateEmailRelayControls(); updateMailboxSelectionControls(); renderEmailList();
    try {
        const response = await fetch(`${API_BASE}/emails/${action === 'read' ? 'batch/read' : 'batch'}`, {
            method: action === 'read' ? 'PATCH' : 'DELETE', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ids })
        });
        const result = await handleAPIResponse(response);
        const failed = new Set(result.failedIDs || []);
        for (const id of ids) if (!failed.has(id)) selectedMailIDs.delete(id);
        const succeeded = new Set(ids.filter(id => !failed.has(id)));
        if (action === 'delete') removeKnownDeletedEmail([...succeeded]);
        else {
            for (const email of state.emails) if (succeeded.has(email.id)) email.read = true;
            if (state.currentEmail && succeeded.has(state.currentEmail.id)) state.currentEmail.read = true;
            if (mailboxFilters.read === 'false') {
                const remaining = state.emails.filter(email => !succeeded.has(email.id));
                state.total = Math.max(0, state.total - (state.emails.length - remaining.length));
                state.emails = remaining;
                const lastPage = Math.max(0, Math.ceil(state.total / state.pageSize) - 1);
                if (state.currentPage > lastPage) { state.currentPage = lastPage; state.emails = []; }
            }
            renderEmailList(); updateEmailCount(); updatePagination(); updateMailboxSelectionControls();
        }
        if (action === 'delete' && state.currentEmail && ids.includes(state.currentEmail.id) && !failed.has(state.currentEmail.id)) clearEmailSelection('replace');
        await loadEmails();
        const message = document.getElementById('mailBatchResult');
        const completed = action === 'read' ? Math.max(0, (result.total ?? ids.length) - (result.failed || 0)) : result.success || 0;
        if (message) message.textContent = t('batchResult', {success:completed,failed:result.failed || 0});
    } catch (error) { alert(parseAPIError(error)); }
    finally { mailboxBatchBusy = false; syncRelayMutationControls(); updateEmailRelayControls(); updateMailboxSelectionControls(); renderEmailList(); }
}
function mailboxExportURL(selected) {
    const params = new URLSearchParams();
    if (selected) { if (!selectedMailIDs.size) return ''; params.set('ids', Array.from(selectedMailIDs).join(',')); }
    else {
        if (state.searchQuery) params.set('q',state.searchQuery);
        for (const [key,value] of Object.entries(mailboxFilters)) if (!key.startsWith('sort') && value !== '') params.set(key,value);
    }
    return `${API_BASE}/emails/export?${params}`;
}
function exportMailbox(selected) {
    if (mailboxControlsBusy()) return;
    const url = mailboxExportURL(selected);
    if (url) window.open(url, '_blank', 'noopener');
}
