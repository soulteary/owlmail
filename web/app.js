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
    localStorage.setItem('language', lang);
    document.documentElement.lang = lang;
    updateUI(renderDynamic);
    if (browserNotificationsInitialized) updateBrowserNotificationButton();
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
let relayEnabled = false;
const relayPending = new Set();

function syncRelayMutationControls() {
    const deleteAllBtn = document.getElementById('deleteAllBtn');
    if (deleteAllBtn) deleteAllBtn.disabled = relayPending.size > 0;
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
    if (!event || event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey || isEditableKeyboardTarget(event.target)) {
        return;
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
let emailViewportPreset = '100%';
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
    async getEmailPreviews(offset = 0, limit = 50, query = '') {
        const params = new URLSearchParams({
            offset: offset.toString(),
            limit: limit.toString()
        });
        if (query) {
            params.append('q', query);
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

    async getRelayJob(statusURL) {
        const response = await fetch(new URL(statusURL, window.location.origin).toString());
        return await handleAPIResponse(response);
    }
};

// WebSocket Connection - 使用新的 API v1 WebSocket 端点
function connectWebSocket() {
    try {
        // Use ws:// or wss:// based on current protocol
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}${BASE_PATHNAME}/api/v1/ws`;
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
        notifyBrowserForEmail(data.email);
    } else if (data.type === 'delete') {
        // Remove deleted email from the list
        state.emails = state.emails.filter(e => e.id !== data.id);
        state.total--;
        renderEmailList();
        updateEmailCount();
        if (state.currentEmail && state.currentEmail.id === data.id) {
            clearEmailSelection('replace');
        }
    }
}

// Update UI with current language
function updateUI(renderDynamic = true) {
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
        deleteAllBtn.disabled = relayPending.size > 0;
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
    const container = document.getElementById('emailList');
    if (!container) return;

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
            <div class="email-item ${unreadClass} ${selectedClass}" data-id="${email.id}" tabindex="0" role="button">
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
            ${relayEnabled ? `
                <button class="btn btn-secondary" ${relayPending.has(email.id) ? 'disabled' : ''} onclick="relayCurrentEmail('${email.id}')">${t('relayOriginal')}</button>
                <button class="btn btn-secondary" ${relayPending.has(email.id) ? 'disabled' : ''} onclick="relayCurrentEmail('${email.id}', true)">${t('relayOverride')}</button>
            ` : ''}
            <button class="btn btn-danger" ${relayPending.has(email.id) ? 'disabled' : ''} onclick="deleteEmail('${email.id}')">${t('delete')}</button>
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
        ${renderEmailContentTabs(email)}
        ${attachments}
    `;
}

function renderEmailContentTabs(email) {
    const availableTabs = ['html', 'text', 'headers', 'source'];
    if (!availableTabs.includes(emailContentTab) || (emailContentTab === 'html' && !email.html)) {
        emailContentTab = email.html ? 'html' : 'text';
    }
    const labels = { html: 'contentHTML', text: 'contentText', headers: 'contentHeaders', source: 'contentSource' };
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
    if (!email || !['html', 'text', 'headers', 'source'].includes(tab)) return;
    if (tab === 'html' && !email.html) return;
    emailContentTab = tab;
    const shouldLoadSource = tab === 'source' && !emailSourceCache.has(email.id) && !emailSourceOversized.has(email.id);
    if (shouldLoadSource) emailSourceErrors.delete(email.id);
    renderEmailDetail();
    if (restoreFocus) {
        document.getElementById(`email-content-tab-${tab}`)?.focus?.();
    }
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
    const tabs = ['html', 'text', 'headers', 'source'].filter((candidate) => candidate !== 'html' || email.html);
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
        + '<meta name="referrer" content="no-referrer">';
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
async function loadEmails() {
    try {
        showLoading();
        const data = await API.getEmailPreviews(
            state.currentPage * state.pageSize,
            state.pageSize,
            state.searchQuery
        );
        state.emails = data.previews || [];
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

async function loadRelayAvailability() {
    try {
        const outgoing = await API.getOutgoingConfig();
        relayEnabled = outgoing && outgoing.enabled === true;
        if (state.currentEmail) renderEmailDetail();
    } catch (error) {
        relayEnabled = false;
        console.warn('Unable to inspect outgoing relay configuration:', error);
    }
}

async function relayCurrentEmail(id, askForRecipient = false) {
    if (!relayEnabled || relayPending.has(id)) return;
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
        if (!state.currentEmail || state.currentEmail.id !== id) return;
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
    renderEmailDetail();
    let releasePending = true;
    try {
        const result = await API.relayEmail(id, relayTo, confirmedRecipients);
        const data = result && result.data;
        const job = data && data.job;
        alert(t('relayQueued', { id: job && job.id ? job.id : t('unknown') }));
        if (!data || !data.statusUrl) {
            throw new Error('Relay status URL is missing');
        }
        releasePending = false;
        while (true) {
            let statusResult;
            try {
                statusResult = await API.getRelayJob(data.statusUrl);
            } catch (error) {
                if (!relayStatusErrorIsTransient(error)) {
                    releasePending = true;
                    throw error;
                }
                console.warn('Relay status check failed; retrying:', error);
                await new Promise((resolve) => setTimeout(resolve, RELAY_STATUS_POLL_INTERVAL_MS));
                continue;
            }
            const current = statusResult && statusResult.data;
            if (current && (current.status === 'succeeded' || current.status === 'failed')) {
                releasePending = true;
                if (current.status === 'failed') {
                    throw new Error(current.errorCategory || 'delivery failed');
                }
                break;
            }
            await new Promise((resolve) => setTimeout(resolve, RELAY_STATUS_POLL_INTERVAL_MS));
        }
    } catch (error) {
        alert(t('relayError', { error: parseAPIError(error) }));
    } finally {
        if (releasePending) relayPending.delete(id);
        syncRelayMutationControls();
        if (state.currentEmail && state.currentEmail.id === id) renderEmailDetail();
    }
}

async function loadEmailDetail(id, { historyMode = 'push' } = {}) {
    if (!id) {
        clearEmailSelection(historyMode);
        return;
    }
    const requestSequence = ++emailDetailRequestSequence;
    try {
        showLoading();
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
        hideLoading();
    }
}

async function deleteEmail(id) {
    if (relayPending.has(id)) return;
    if (!confirm(t('deleteConfirm'))) return;

    try {
        showLoading();
        await API.deleteEmail(id);
        // Remove from list
        state.emails = state.emails.filter(e => e.id !== id);
        state.total--;
        if (state.currentEmail && state.currentEmail.id === id) {
            clearEmailSelection('replace');
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
    if (relayPending.size > 0) return;
    if (!confirm(t('deleteAllConfirm'))) return;

    try {
        showLoading();
        await API.deleteAllEmails();
        state.emails = [];
        state.total = 0;
        clearEmailSelection('replace');
        renderEmailList();
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
