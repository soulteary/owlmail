(() => {
    'use strict';

    const MAX_CONFIG_BYTES = 1024 * 1024;
    const MAX_TARGETS = 32;
    const MAX_RETRIES = 5;
    const MAX_TIMEOUT_MS = 60 * 1000;
    const DEFAULT_TARGET = {
        name: 'primary',
        url: 'https://example.com/hooks/owlmail',
        method: 'POST',
        contentType: 'application/json',
        timeout: '5s',
        retries: 0
    };
    const EXAMPLE_CONFIG = {
        version: 1,
        targets: [
            {
                name: 'build-alerts',
                url: 'https://example.com/hooks/owlmail',
                headers: {
                    Authorization: 'Bearer ' + '$' + '{OWLMAIL_WEBHOOK_TOKEN}'
                },
                timeout: '10s',
                retries: 2,
                match: {
                    to: ['alerts@example.com'],
                    subject: ['[staging]*', '[production]*']
                }
            }
        ]
    };

    const translations = {
        en: {
            language: 'Language',
            toggleTheme: 'Toggle theme',
            help: 'Help',
            backToInbox: 'Back to inbox',
            eyebrow: 'WEBHOOK CONFIGURATION',
            pageTitle: 'Build a webhook configuration',
            pageDescription: 'Create targets visually or import an existing JSON file, inspect every rule, and download a configuration ready for OwlMail.',
            localOnlyTitle: 'Local-only editor.',
            localOnlyBody: 'Configuration stays in this browser and is never sent to OwlMail. Download the JSON, mount it into the server, and restart OwlMail to activate it.',
            builderKicker: 'CONFIGURATION BUILDER',
            targets: 'Targets',
            loadExample: 'Load example',
            addTarget: 'Add target',
            importKicker: 'EXISTING CONFIGURATION',
            importTitle: 'Import and inspect',
            importLabel: 'Paste JSON',
            importPlaceholder: '{"version":1,"targets":[...]}',
            dropTitle: 'Drop a JSON file here',
            dropBody: 'or choose a file (maximum 1 MiB)',
            parseImport: 'Parse and replace targets',
            replaceWarning: 'A successful import replaces the targets currently shown in the builder.',
            outputKicker: 'LIVE OUTPUT',
            outputTitle: 'Generated JSON',
            valid: 'Valid',
            warning: 'Valid with notes',
            invalid: 'Needs attention',
            copy: 'Copy JSON',
            download: 'Download webhooks.json',
            target: 'Target',
            removeTarget: 'Remove',
            name: 'Name',
            url: 'Destination URL',
            envHint: 'Environment variables such as ' + '$' + '{OWLMAIL_WEBHOOK_HOST} are supported.',
            method: 'HTTP method',
            timeout: 'Timeout',
            timeoutHint: 'Go duration, greater than 0 and at most 1m.',
            retries: 'Retries',
            contentType: 'Content type',
            advanced: 'Authentication, headers, filters, and body template',
            secret: 'HMAC secret',
            secretHint: 'Use an environment variable instead of committing a real secret.',
            headers: 'Request headers',
            headersHint: 'Header values may reference environment variables.',
            addHeader: 'Add header',
            filters: 'Match filters',
            filtersHint: 'Fields are combined with AND. Put one case-insensitive * or ? wildcard pattern on each line.',
            matchFrom: 'From',
            matchTo: 'To',
            matchSubject: 'Subject',
            matchText: 'Text body',
            bodyTemplate: 'Body template',
            templateHint: "Leave empty for OwlMail's default JSON payload. Go template syntax is compiled when OwlMail starts.",
            headerName: 'Header name',
            headerValue: 'Header value',
            removeHeader: 'Remove header',
            configReady: 'The configuration is ready to download.',
            errorCount: '{count} error(s) must be fixed.',
            warningCount: '{count} runtime note(s).',
            importSuccess: 'Imported {count} target(s).',
            importSuccessWithWarnings: 'Imported {count} target(s) with {warnings} runtime note(s).',
            copied: 'Configuration copied.',
            copyFailed: 'Could not copy automatically. Select the JSON and copy it manually.',
            downloaded: 'Downloaded webhooks.json.',
            fileTooLarge: 'The selected file exceeds 1 MiB.',
            fileReadFailed: 'The selected file could not be read.',
            configTooLarge: 'The configuration exceeds OwlMail’s 1 MiB limit.',
            maxTargets: 'A configuration can contain at most 32 targets.',
            importedVersionNormalized: 'Version 0 or an omitted version is exported as version 1.',
            invalidJSON: 'JSON could not be parsed: {detail}',
            importEmpty: 'Paste JSON or choose a file first.',
            invalidRoot: 'The top-level value must be a JSON object.',
            unknownField: 'Unknown field "{field}".',
            invalidVersion: 'Version must be 0, 1, or omitted.',
            targetsRequired: 'At least one target is required.',
            targetsArray: 'Targets must be an array.',
            tooManyTargets: 'No more than 32 targets are allowed.',
            targetObject: 'Each target must be an object.',
            fieldString: 'This field must be a string.',
            nameRequired: 'A target name is required.',
            nameInvalid: 'The name must be at most 100 UTF-8 bytes and contain no newlines.',
            duplicateName: 'Target name "{name}" is duplicated.',
            urlRequired: 'A destination URL is required.',
            urlInvalid: 'The destination URL is invalid.',
            urlScheme: 'The URL scheme must be http or https.',
            urlHost: 'The URL must contain a host.',
            urlCredentials: 'URL credentials are not allowed; use request headers.',
            urlFragment: 'URL fragments are not allowed.',
            unsupportedMethod: 'Only POST, PUT, and PATCH are supported.',
            headersObject: 'Headers must be a JSON object.',
            headerValueString: 'Every header value must be a string.',
            headerNameInvalid: 'Header name "{name}" is invalid.',
            managedHeader: 'Header "{name}" is managed by the HTTP client.',
            headerNewline: 'Header values cannot contain newlines.',
            duplicateHeader: 'Header "{name}" is entered more than once.',
            contentTypeNewline: 'Content type cannot contain newlines.',
            timeoutInvalid: 'Use a Go duration such as 500ms, 5s, or 1m.',
            timeoutRange: 'Timeout must be greater than 0 and no more than 1m.',
            retriesInvalid: 'Retries must be an integer from 0 to 5.',
            matchObject: 'Match must be a JSON object.',
            matchArray: 'Each match field must be an array of strings.',
            matchPattern: 'Match patterns cannot be empty.',
            matchGlob: 'Wildcard pattern "{pattern}" is malformed.',
            envRuntime: 'Environment placeholders are resolved and checked when OwlMail starts.',
            templateRuntime: 'The Go body template is compiled and checked when OwlMail starts.'
        },
        'zh-CN': {
            language: '语言',
            toggleTheme: '切换主题',
            help: '帮助',
            backToInbox: '返回收件箱',
            eyebrow: 'WEBHOOK 配置',
            pageTitle: '生成 Webhook 配置',
            pageDescription: '可视化创建目标，或导入已有 JSON 文件，检查每条规则，并下载可供 OwlMail 使用的配置。',
            localOnlyTitle: '仅在本地编辑。',
            localOnlyBody: '配置只在当前浏览器中处理，不会发送给 OwlMail。下载 JSON、挂载到服务器并重启 OwlMail 后才会生效。',
            builderKicker: '配置生成器',
            targets: '转发目标',
            loadExample: '载入示例',
            addTarget: '增加目标',
            importKicker: '已有配置',
            importTitle: '导入并解析',
            importLabel: '粘贴 JSON',
            importPlaceholder: '{"version":1,"targets":[...]}',
            dropTitle: '拖入 JSON 文件',
            dropBody: '或点击选择文件（最大 1 MiB）',
            parseImport: '解析并替换当前目标',
            replaceWarning: '导入成功后，将替换生成器中当前显示的全部目标。',
            outputKicker: '实时输出',
            outputTitle: '生成的 JSON',
            valid: '配置有效',
            warning: '有效，但有提示',
            invalid: '需要修正',
            copy: '复制 JSON',
            download: '下载 webhooks.json',
            target: '目标',
            removeTarget: '移除',
            name: '名称',
            url: '目标 URL',
            envHint: '支持 ' + '$' + '{OWLMAIL_WEBHOOK_HOST} 形式的环境变量。',
            method: 'HTTP 方法',
            timeout: '超时时间',
            timeoutHint: '使用 Go duration 格式，必须大于 0 且不超过 1m。',
            retries: '重试次数',
            contentType: '内容类型',
            advanced: '鉴权、请求头、过滤规则和正文模板',
            secret: 'HMAC 密钥',
            secretHint: '建议引用环境变量，不要把真实密钥提交到配置仓库。',
            headers: '请求头',
            headersHint: '请求头的值可以引用环境变量。',
            addHeader: '增加请求头',
            filters: '匹配过滤器',
            filtersHint: '不同字段之间为 AND；每行一个不区分大小写的 * 或 ? 通配模式。',
            matchFrom: '发件人',
            matchTo: '收件人',
            matchSubject: '主题',
            matchText: '文本正文',
            bodyTemplate: '正文模板',
            templateHint: '留空将使用 OwlMail 默认 JSON；Go 模板语法会在 OwlMail 启动时编译检查。',
            headerName: '请求头名称',
            headerValue: '请求头值',
            removeHeader: '移除请求头',
            configReady: '配置有效，可以复制或下载。',
            errorCount: '需要修正 {count} 个错误。',
            warningCount: '有 {count} 条运行时提示。',
            importSuccess: '已导入 {count} 个目标。',
            importSuccessWithWarnings: '已导入 {count} 个目标，并有 {warnings} 条运行时提示。',
            copied: '配置已复制。',
            copyFailed: '无法自动复制，请选中 JSON 后手动复制。',
            downloaded: '已下载 webhooks.json。',
            fileTooLarge: '所选文件超过 1 MiB。',
            fileReadFailed: '无法读取所选文件。',
            configTooLarge: '配置超过 OwlMail 的 1 MiB 限制。',
            maxTargets: '一份配置最多包含 32 个目标。',
            importedVersionNormalized: '导入的版本为 0 或未填写时，导出结果会规范为版本 1。',
            invalidJSON: '无法解析 JSON：{detail}',
            importEmpty: '请先粘贴 JSON 或选择文件。',
            invalidRoot: '顶层内容必须是 JSON 对象。',
            unknownField: '存在未知字段“{field}”。',
            invalidVersion: 'version 只能是 0、1，或省略。',
            targetsRequired: '至少需要一个目标。',
            targetsArray: 'targets 必须是数组。',
            tooManyTargets: '最多允许 32 个目标。',
            targetObject: '每个目标必须是对象。',
            fieldString: '该字段必须是字符串。',
            nameRequired: '必须填写目标名称。',
            nameInvalid: '名称不能超过 100 个 UTF-8 字节，也不能包含换行。',
            duplicateName: '目标名称“{name}”重复。',
            urlRequired: '必须填写目标 URL。',
            urlInvalid: '目标 URL 无效。',
            urlScheme: 'URL 协议必须是 http 或 https。',
            urlHost: 'URL 必须包含主机名。',
            urlCredentials: 'URL 中不能包含用户名或密码，请改用请求头。',
            urlFragment: 'URL 不能包含 fragment。',
            unsupportedMethod: '仅支持 POST、PUT 和 PATCH。',
            headersObject: 'headers 必须是 JSON 对象。',
            headerValueString: '每个请求头的值都必须是字符串。',
            headerNameInvalid: '请求头名称“{name}”无效。',
            managedHeader: '请求头“{name}”由 HTTP 客户端管理。',
            headerNewline: '请求头的值不能包含换行。',
            duplicateHeader: '请求头“{name}”被重复填写。',
            contentTypeNewline: '内容类型不能包含换行。',
            timeoutInvalid: '请使用 500ms、5s 或 1m 这样的 Go duration。',
            timeoutRange: '超时时间必须大于 0 且不超过 1m。',
            retriesInvalid: '重试次数必须是 0 到 5 的整数。',
            matchObject: 'match 必须是 JSON 对象。',
            matchArray: '每个匹配字段都必须是字符串数组。',
            matchPattern: '匹配模式不能为空。',
            matchGlob: '通配模式“{pattern}”格式错误。',
            envRuntime: '环境变量占位符会在 OwlMail 启动时解析并检查。',
            templateRuntime: 'Go 正文模板会在 OwlMail 启动时编译并检查。'
        }
    };

    const rootKeys = new Set(['version', 'targets']);
    const targetKeys = new Set(['name', 'url', 'method', 'headers', 'contentType', 'bodyTemplate', 'secret', 'timeout', 'retries', 'match']);
    const matchKeys = new Set(['from', 'to', 'subject', 'text']);
    const headerNamePattern = /^[!#$%&'*+\-.^_\`|~0-9A-Za-z]+$/;
    const environmentPattern = /\$\{[A-Za-z_][A-Za-z0-9_]*\}/g;
    const exactEnvironmentPattern = /^\$\{[A-Za-z_][A-Za-z0-9_]*\}$/;

    let currentLanguage = 'en';
    let lastGeneratedJSON = '';
    let actionStatusTimer = null;
    let elements = {};
    const preservedMatchPatterns = new WeakMap();
    const preservedControlValues = new WeakMap();

    function tr(key, params) {
        const table = translations[currentLanguage] || translations.en;
        let value = table[key] || translations.en[key] || key;
        Object.entries(params || {}).forEach(([name, replacement]) => {
            value = value.replaceAll('{' + name + '}', String(replacement));
        });
        return value;
    }

    function issue(code, path, params) {
        return { code, path: path || '', params: params || {} };
    }

    function isPlainObject(value) {
        return value !== null && typeof value === 'object' && !Array.isArray(value);
    }

    function addUnknownFieldIssues(value, allowed, path, errors) {
        Object.keys(value).forEach((key) => {
            if (!allowed.has(key)) {
                errors.push(issue('unknownField', path ? path + '.' + key : key, { field: key }));
            }
        });
    }

    function parseDurationMilliseconds(value) {
        if (typeof value !== 'string' || value === '') return null;
        const unitNanoseconds = {
            ns: 1,
            us: 1000,
            'µs': 1000,
            'μs': 1000,
            ms: 1000000,
            s: 1000000000,
            m: 60000000000,
            h: 3600000000000
        };
        let sign = 1;
        let duration = value;
        if (duration[0] === '+' || duration[0] === '-') {
            sign = duration[0] === '-' ? -1 : 1;
            duration = duration.slice(1);
        }
        const partPattern = /(\d+(?:\.\d*)?|\.\d+)(ns|us|µs|μs|ms|s|m|h)/gy;
        let totalNanoseconds = 0;
        let position = 0;
        let match;
        while ((match = partPattern.exec(duration)) !== null) {
            if (match.index !== position) return Number.NaN;
            const partNanoseconds = Math.trunc(Number(match[1]) * unitNanoseconds[match[2]]);
            if (!Number.isFinite(partNanoseconds)) return sign * Number.POSITIVE_INFINITY;
            totalNanoseconds += partNanoseconds;
            position = partPattern.lastIndex;
        }
        return position === duration.length && position > 0
            ? sign * totalNanoseconds / 1000000
            : Number.NaN;
    }

    function nextCodePointIndex(value, index) {
        const codePoint = value.codePointAt(index);
        return index + (codePoint > 0xFFFF ? 2 : 1);
    }

    function readClassCharacter(pattern, index) {
        if (index >= pattern.length || pattern[index] === '-' || pattern[index] === ']') return -1;
        if (pattern[index] === '\\') {
            index++;
            if (index >= pattern.length) return -1;
        }
        return nextCodePointIndex(pattern, index);
    }

    function validGlobPattern(pattern) {
        for (let index = 0; index < pattern.length;) {
            const character = pattern[index];
            if (character === '\\') {
                if (index + 1 >= pattern.length) return false;
                index = nextCodePointIndex(pattern, index + 1);
                continue;
            }
            if (character !== '[') {
                index = nextCodePointIndex(pattern, index);
                continue;
            }

            index++;
            if (pattern[index] === '^') index++;
            let ranges = 0;
            let closed = false;
            while (index < pattern.length) {
                if (pattern[index] === ']' && ranges > 0) {
                    index++;
                    closed = true;
                    break;
                }
                index = readClassCharacter(pattern, index);
                if (index < 0) return false;
                if (pattern[index] === '-') {
                    index = readClassCharacter(pattern, index + 1);
                    if (index < 0) return false;
                }
                ranges++;
            }
            if (!closed) return false;
        }
        return true;
    }

    function validateOptionalString(value, path, errors) {
        if (value !== undefined && value !== null && typeof value !== 'string') {
            errors.push(issue('fieldString', path));
            return false;
        }
        return true;
    }

    function validateURL(value, path, errors, warnings) {
        if (typeof value !== 'string') {
            if (value === undefined || value === null || value === '') errors.push(issue('urlRequired', path));
            else errors.push(issue('fieldString', path));
            return;
        }
        if (value === '') {
            errors.push(issue('urlRequired', path));
            return;
        }
        if (value !== value.trim() || /[\u0000-\u001F\u007F]/.test(value)) {
            errors.push(issue('urlInvalid', path));
            return;
        }

        const environmentMatches = value.match(environmentPattern);
        if (environmentMatches) {
            warnings.push(issue('envRuntime', path));
            if (exactEnvironmentPattern.test(value)) return;
        }
        const percentValidationValue = value.replace(environmentPattern, '00');
        if (/%(?![0-9A-Fa-f]{2})/.test(percentValidationValue)) {
            errors.push(issue('urlInvalid', path));
            return;
        }

        const schemeEnd = value.indexOf('://');
        let placeholderInScheme = false;
        if (environmentMatches && schemeEnd >= 0) {
            const scheme = value.slice(0, schemeEnd);
            placeholderInScheme = environmentPattern.test(scheme);
            environmentPattern.lastIndex = 0;
        }

        const authorityStart = schemeEnd >= 0 ? schemeEnd + 3 : -1;
        const authorityEndOffset = authorityStart >= 0 ? value.slice(authorityStart).search(/[/?#]/) : -1;
        const authorityEnd = authorityStart < 0
            ? -1
            : (authorityEndOffset < 0 ? value.length : authorityStart + authorityEndOffset);
        const authority = authorityStart < 0 ? '' : value.slice(authorityStart, authorityEnd);
        const authorityHasUserInfo = authority.includes('@');
        const authorityIsStaticallyEmpty = authorityStart >= 0 && authorityStart === authorityEnd;
        if (authorityHasUserInfo) errors.push(issue('urlCredentials', path));
        if (authority.includes('\\')) {
            errors.push(issue('urlInvalid', path));
            return;
        }
        if (authorityIsStaticallyEmpty) {
            errors.push(issue('urlHost', path));
        }
        const parseableValue = value.replace(environmentPattern, (token, offset) => {
            if (placeholderInScheme && offset < schemeEnd) return 'https';
            if (offset < authorityStart || offset >= authorityEnd) return 'value';
            const authorityPrefix = value.slice(authorityStart, offset);
            const afterCredentials = authorityPrefix.slice(authorityPrefix.lastIndexOf('@') + 1);
            const openingBracket = afterCredentials.lastIndexOf('[');
            const closingBracket = afterCredentials.lastIndexOf(']');
            if (openingBracket > closingBracket) {
                const authoritySuffix = value.slice(offset + token.length, authorityEnd);
                const bracketEnd = authoritySuffix.indexOf(']');
                if (bracketEnd >= 0) {
                    const beforePlaceholder = afterCredentials.slice(openingBracket + 1);
                    const afterPlaceholder = authoritySuffix.slice(0, bracketEnd);
                    return beforePlaceholder === '' && afterPlaceholder === '' ? '::1' : '1';
                }
            }
            const lastColon = afterCredentials.lastIndexOf(':');
            if (lastColon > closingBracket) return '443';
            return 'placeholder.invalid';
        });
        let parsed;
        try {
            parsed = new URL(parseableValue);
        } catch (_) {
            errors.push(issue('urlInvalid', path));
            return;
        }
        if (!placeholderInScheme && parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
            errors.push(issue('urlScheme', path));
        }
        if (schemeEnd < 0 && (parsed.protocol === 'http:' || parsed.protocol === 'https:')) {
            errors.push(issue('urlHost', path));
        }
        if (!parsed.hostname && !authorityIsStaticallyEmpty) errors.push(issue('urlHost', path));
        if ((parsed.username || parsed.password) && !authorityHasUserInfo) {
            errors.push(issue('urlCredentials', path));
        }
        if (parsed.hash) errors.push(issue('urlFragment', path));
    }

    function validateMatch(match, path, errors) {
        if (match === undefined || match === null) return;
        if (!isPlainObject(match)) {
            errors.push(issue('matchObject', path));
            return;
        }
        addUnknownFieldIssues(match, matchKeys, path, errors);
        matchKeys.forEach((field) => {
            const patterns = match[field];
            if (patterns === undefined || patterns === null) return;
            if (!Array.isArray(patterns) || patterns.some((pattern) => typeof pattern !== 'string')) {
                errors.push(issue('matchArray', path + '.' + field));
                return;
            }
            patterns.forEach((pattern, index) => {
                const patternPath = path + '.' + field + '[' + index + ']';
                if (pattern.trim() === '') errors.push(issue('matchPattern', patternPath));
                else if (!validGlobPattern(pattern.toLowerCase())) errors.push(issue('matchGlob', patternPath, { pattern }));
            });
        });
    }

    function validateTarget(target, index, names, errors, warnings) {
        const path = 'targets[' + index + ']';
        if (!isPlainObject(target)) {
            errors.push(issue('targetObject', path));
            return;
        }
        addUnknownFieldIssues(target, targetKeys, path, errors);

        if (typeof target.name !== 'string') {
            if (target.name === undefined || target.name === null || target.name === '') errors.push(issue('nameRequired', path + '.name'));
            else errors.push(issue('fieldString', path + '.name'));
        } else {
            const name = target.name.trim();
            if (!name) {
                errors.push(issue('nameRequired', path + '.name'));
            } else if (utf8ByteLength(name) > 100 || /[\r\n]/.test(name)) {
                errors.push(issue('nameInvalid', path + '.name'));
            } else {
                if (names.has(name)) errors.push(issue('duplicateName', path + '.name', { name }));
                names.add(name);
            }
        }

        validateURL(target.url, path + '.url', errors, warnings);

        if (validateOptionalString(target.method, path + '.method', errors) && target.method) {
            const method = target.method.trim().toUpperCase();
            if (!['POST', 'PUT', 'PATCH'].includes(method)) errors.push(issue('unsupportedMethod', path + '.method'));
        }

        if (target.headers !== undefined && target.headers !== null) {
            if (!isPlainObject(target.headers)) {
                errors.push(issue('headersObject', path + '.headers'));
            } else {
                const canonicalNames = new Set();
                Object.entries(target.headers).forEach(([name, value]) => {
                    const headerPath = path + '.headers.' + name;
                    const canonicalName = name.trim().toLowerCase();
                    if (!headerNamePattern.test(name)) errors.push(issue('headerNameInvalid', headerPath, { name }));
                    if (canonicalName === 'host' || canonicalName === 'content-length') {
                        errors.push(issue('managedHeader', headerPath, { name }));
                    }
                    if (canonicalNames.has(canonicalName)) {
                        errors.push(issue('duplicateHeader', headerPath, { name }));
                    }
                    canonicalNames.add(canonicalName);
                    if (typeof value !== 'string') errors.push(issue('headerValueString', headerPath));
                    else {
                        if (/[\r\n]/.test(value)) errors.push(issue('headerNewline', headerPath));
                        if (environmentPattern.test(value)) warnings.push(issue('envRuntime', headerPath));
                        environmentPattern.lastIndex = 0;
                    }
                });
            }
        }

        if (validateOptionalString(target.contentType, path + '.contentType', errors) &&
            typeof target.contentType === 'string' && /[\r\n]/.test(target.contentType)) {
            errors.push(issue('contentTypeNewline', path + '.contentType'));
        }

        if (validateOptionalString(target.secret, path + '.secret', errors) &&
            typeof target.secret === 'string' && environmentPattern.test(target.secret)) {
            warnings.push(issue('envRuntime', path + '.secret'));
        }
        environmentPattern.lastIndex = 0;

        if (validateOptionalString(target.bodyTemplate, path + '.bodyTemplate', errors) && target.bodyTemplate) {
            warnings.push(issue('templateRuntime', path + '.bodyTemplate'));
        }

        if (validateOptionalString(target.timeout, path + '.timeout', errors) && target.timeout) {
            const milliseconds = parseDurationMilliseconds(target.timeout);
            if (Number.isNaN(milliseconds)) errors.push(issue('timeoutInvalid', path + '.timeout'));
            else if (milliseconds <= 0 || milliseconds > MAX_TIMEOUT_MS) errors.push(issue('timeoutRange', path + '.timeout'));
        }

        if (target.retries !== undefined && target.retries !== null) {
            if (!Number.isInteger(target.retries) || target.retries < 0 || target.retries > MAX_RETRIES) {
                errors.push(issue('retriesInvalid', path + '.retries'));
            }
        }

        validateMatch(target.match, path + '.match', errors);
    }

    function validateConfig(config) {
        const errors = [];
        const warnings = [];
        if (!isPlainObject(config)) {
            errors.push(issue('invalidRoot'));
            return { errors, warnings };
        }
        addUnknownFieldIssues(config, rootKeys, '', errors);

        if (config.version !== undefined && config.version !== null &&
            config.version !== 0 && config.version !== 1) {
            errors.push(issue('invalidVersion', 'version'));
        } else if (config.version === undefined || config.version === 0) {
            warnings.push(issue('importedVersionNormalized', 'version'));
        }

        if (!Array.isArray(config.targets)) {
            if (config.targets === undefined || config.targets === null) errors.push(issue('targetsRequired', 'targets'));
            else errors.push(issue('targetsArray', 'targets'));
            return { errors, warnings };
        }
        if (config.targets.length === 0) errors.push(issue('targetsRequired', 'targets'));
        if (config.targets.length > MAX_TARGETS) errors.push(issue('tooManyTargets', 'targets'));

        const names = new Set();
        config.targets.forEach((target, index) => validateTarget(target, index, names, errors, warnings));
        return { errors, warnings };
    }

    function normalizedPatterns(value) {
        if (!Array.isArray(value)) return [];
        return value.slice();
    }

    function normalizeConfig(config) {
        return {
            version: 1,
            targets: config.targets.map((source) => {
                const target = {
                    name: source.name.trim(),
                    url: source.url
                };
                const method = typeof source.method === 'string' ? source.method.trim().toUpperCase() : '';
                const contentType = typeof source.contentType === 'string' ? source.contentType.trim() : '';
                if (method && method !== 'POST') target.method = method;
                if (source.headers && Object.keys(source.headers).length) target.headers = { ...source.headers };
                if (contentType && contentType !== 'application/json') target.contentType = contentType;
                if (source.bodyTemplate) target.bodyTemplate = source.bodyTemplate;
                if (source.secret) target.secret = source.secret;
                if (source.timeout && source.timeout !== '5s') target.timeout = source.timeout;
                if (source.retries) target.retries = source.retries;
                if (source.match) {
                    const match = {};
                    matchKeys.forEach((field) => {
                        const patterns = normalizedPatterns(source.match[field]);
                        if (patterns.length) match[field] = patterns;
                    });
                    if (Object.keys(match).length) target.match = match;
                }
                return target;
            })
        };
    }

    function parseConfigText(text) {
        if (typeof text !== 'string' || text.trim() === '') {
            return { config: null, errors: [issue('importEmpty')], warnings: [] };
        }
        if (utf8ByteLength(text) > MAX_CONFIG_BYTES) {
            return { config: null, errors: [issue('configTooLarge')], warnings: [] };
        }
        let parsed;
        try {
            parsed = JSON.parse(text);
        } catch (error) {
            return {
                config: null,
                errors: [issue('invalidJSON', '', { detail: error && error.message ? error.message : String(error) })],
                warnings: []
            };
        }
        const validation = validateConfig(parsed);
        if (validation.errors.length) return { config: null, ...validation };
        return { config: normalizeConfig(parsed), ...validation };
    }

    function splitPatternLines(value) {
        return value.split(/\r?\n/).filter((line) => line.trim() !== '');
    }

    function patternsFromEditorValue(value, preserved) {
        if (preserved && value === preserved.displayValue) return preserved.patterns.slice();
        return splitPatternLines(value);
    }

    function editorValueFromPreserved(value, preserved) {
        if (preserved && value === preserved.displayValue) return preserved.value;
        return value;
    }

    function setPreservedControlValue(control, value) {
        control.value = value;
        preservedControlValues.set(control, { displayValue: control.value, value });
        control.addEventListener('input', () => preservedControlValues.delete(control), { once: true });
    }

    function createHeaderMap() {
        return Object.create(null);
    }

    function utf8ByteLength(value) {
        return typeof TextEncoder === 'function'
            ? new TextEncoder().encode(value).byteLength
            : unescape(encodeURIComponent(value)).length;
    }

    function validateGeneratedConfig(config) {
        const validation = validateConfig(config);
        const json = JSON.stringify(config, null, 2) + '\n';
        if (utf8ByteLength(json) > MAX_CONFIG_BYTES) {
            validation.errors.push(issue('configTooLarge'));
        }
        return { ...validation, json };
    }

    function applyTranslations(root) {
        const scope = root || document;
        scope.querySelectorAll('[data-i18n]').forEach((node) => {
            node.textContent = tr(node.dataset.i18n);
        });
        scope.querySelectorAll('[data-i18n-title]').forEach((node) => {
            const value = tr(node.dataset.i18nTitle);
            node.title = value;
            if (node.hasAttribute('aria-label')) node.setAttribute('aria-label', value);
        });
        scope.querySelectorAll('[data-i18n-placeholder]').forEach((node) => {
            node.placeholder = tr(node.dataset.i18nPlaceholder);
        });
    }

    function detectLanguage() {
        try {
            const saved = localStorage.getItem('language');
            if (translations[saved]) return saved;
        } catch (_) {
            // Continue with browser language when storage is unavailable.
        }
        return (navigator.language || '').toLowerCase().startsWith('zh') ? 'zh-CN' : 'en';
    }

    function setLanguage(language) {
        currentLanguage = translations[language] ? language : 'en';
        document.documentElement.lang = currentLanguage;
        document.documentElement.dataset.language = currentLanguage;
        document.title = currentLanguage === 'zh-CN' ? 'OwlMail Webhook 配置生成器' : 'OwlMail Webhook Configurator';
        elements.language.value = currentLanguage;
        applyTranslations(document);
        try {
            localStorage.setItem('language', currentLanguage);
        } catch (_) {
            // Language still applies when storage is unavailable.
        }
        refreshTargetLabels();
        renderOutput();
    }

    function applyTheme(theme) {
        const dark = theme === 'dark';
        document.body.classList.toggle('dark-theme', dark);
        document.body.classList.toggle('light-theme', !dark);
        elements.theme.textContent = dark ? '☀️' : '🌙';
        const label = tr('toggleTheme');
        elements.theme.title = label;
        elements.theme.setAttribute('aria-label', label);
        try {
            localStorage.setItem('theme', dark ? 'dark' : 'light');
        } catch (_) {
            // Theme still applies when storage is unavailable.
        }
    }

    function initialTheme() {
        try {
            return localStorage.getItem('theme') === 'dark' ? 'dark' : 'light';
        } catch (_) {
            return 'light';
        }
    }

    function addHeaderRow(container, name, value) {
        const row = elements.headerTemplate.content.firstElementChild.cloneNode(true);
        applyTranslations(row);
        row.querySelector('[data-header="name"]').value = name || '';
        row.querySelector('[data-header="value"]').value = value || '';
        row.querySelectorAll('input').forEach((input) => input.addEventListener('input', renderOutput));
        row.querySelector('.remove-header').addEventListener('click', () => {
            row.remove();
            renderOutput();
        });
        container.appendChild(row);
        return row;
    }

    function targetHasAdvancedFields(target) {
        return Boolean(
            target.secret ||
            target.bodyTemplate ||
            (target.headers && Object.keys(target.headers).length) ||
            (target.match && Object.values(target.match).some((patterns) => Array.isArray(patterns) && patterns.length))
        );
    }

    function addTarget(target, options) {
        if (elements.targetList.children.length >= MAX_TARGETS) {
            setActionStatus(tr('maxTargets'), true);
            return null;
        }
        const source = { ...DEFAULT_TARGET, ...(target || {}) };
        const card = elements.targetTemplate.content.firstElementChild.cloneNode(true);
        applyTranslations(card);
        card.querySelector('[data-field="name"]').value = source.name || '';
        card.querySelector('[data-field="url"]').value = source.url || '';
        card.querySelector('[data-field="method"]').value = (source.method || 'POST').toUpperCase();
        card.querySelector('[data-field="timeout"]').value = source.timeout || '5s';
        card.querySelector('[data-field="retries"]').value = source.retries === undefined || source.retries === null ? '0' : String(source.retries);
        card.querySelector('[data-field="contentType"]').value = source.contentType || 'application/json';
        setPreservedControlValue(card.querySelector('[data-field="secret"]'), source.secret || '');
        setPreservedControlValue(card.querySelector('[data-field="bodyTemplate"]'), source.bodyTemplate || '');

        matchKeys.forEach((field) => {
            const patterns = source.match && Array.isArray(source.match[field]) ? source.match[field] : [];
            const control = card.querySelector('[data-match="' + field + '"]');
            control.value = patterns.join('\n');
            preservedMatchPatterns.set(control, {
                displayValue: control.value,
                patterns: patterns.slice()
            });
            control.addEventListener('input', () => preservedMatchPatterns.delete(control), { once: true });
        });

        const headerContainer = card.querySelector('[data-role="headers"]');
        Object.entries(source.headers || {}).forEach(([name, value]) => addHeaderRow(headerContainer, name, value));

        card.querySelector('.add-header').addEventListener('click', () => {
            const row = addHeaderRow(headerContainer, '', '');
            row.querySelector('[data-header="name"]').focus();
            renderOutput();
        });
        card.querySelector('.remove-target').addEventListener('click', () => {
            card.remove();
            refreshTargetLabels();
            renderOutput();
        });
        card.querySelectorAll('input, select, textarea').forEach((control) => {
            control.addEventListener('input', renderOutput);
            control.addEventListener('change', renderOutput);
        });
        if (targetHasAdvancedFields(source)) card.querySelector('.advanced-options').open = true;

        elements.targetList.appendChild(card);
        refreshTargetLabels();
        if (!(options && options.silent)) renderOutput();
        return card;
    }

    function refreshTargetLabels() {
        if (!elements.targetList) return;
        const cards = Array.from(elements.targetList.querySelectorAll('.target-card'));
        cards.forEach((card, index) => {
            card.querySelector('[data-role="target-number"]').textContent = String(index + 1);
            card.querySelector('.remove-target').disabled = cards.length === 1;
        });
        elements.targetCount.textContent = cards.length + ' / ' + MAX_TARGETS;
        elements.addTarget.disabled = cards.length >= MAX_TARGETS;
    }

    function readTargetCard(card, index, formErrors) {
        const value = (selector) => card.querySelector(selector).value;
        const target = {
            name: value('[data-field="name"]').trim(),
            url: value('[data-field="url"]').trim()
        };
        const method = value('[data-field="method"]').trim().toUpperCase();
        const timeout = value('[data-field="timeout"]').trim();
        const retriesText = value('[data-field="retries"]').trim();
        const contentType = value('[data-field="contentType"]').trim();
        const secretControl = card.querySelector('[data-field="secret"]');
        const bodyTemplateControl = card.querySelector('[data-field="bodyTemplate"]');
        const secret = editorValueFromPreserved(secretControl.value, preservedControlValues.get(secretControl));
        const bodyTemplate = editorValueFromPreserved(
            bodyTemplateControl.value,
            preservedControlValues.get(bodyTemplateControl)
        );

        if (method && method !== 'POST') target.method = method;
        if (timeout && timeout !== '5s') target.timeout = timeout;
        if (retriesText !== '' && Number(retriesText) !== 0) target.retries = Number(retriesText);
        if (contentType && contentType !== 'application/json') target.contentType = contentType;
        if (secret) target.secret = secret;
        if (bodyTemplate) target.bodyTemplate = bodyTemplate;

        const headers = createHeaderMap();
        const headerNames = new Set();
        card.querySelectorAll('.header-row').forEach((row) => {
            const name = row.querySelector('[data-header="name"]').value.trim();
            const headerValue = row.querySelector('[data-header="value"]').value;
            if (!name && !headerValue) return;
            const canonical = name.toLowerCase();
            if (headerNames.has(canonical)) {
                formErrors.push(issue('duplicateHeader', 'targets[' + index + '].headers.' + name, { name }));
            }
            headerNames.add(canonical);
            headers[name] = headerValue;
        });
        if (Object.keys(headers).length) target.headers = headers;

        const match = {};
        matchKeys.forEach((field) => {
            const control = card.querySelector('[data-match="' + field + '"]');
            const patterns = patternsFromEditorValue(control.value, preservedMatchPatterns.get(control));
            if (patterns.length) match[field] = patterns;
        });
        if (Object.keys(match).length) target.match = match;
        return target;
    }

    function collectConfig() {
        const errors = [];
        const targets = Array.from(elements.targetList.querySelectorAll('.target-card'))
            .map((card, index) => readTargetCard(card, index, errors));
        return { config: { version: 1, targets }, errors };
    }

    function formatIssue(item) {
        const message = tr(item.code, item.params);
        return item.path ? item.path + ': ' + message : message;
    }

    function renderIssueList(container, heading, issues, className) {
        container.replaceChildren();
        container.className = className || '';
        const strong = document.createElement('strong');
        strong.textContent = heading;
        container.appendChild(strong);
        if (issues.length) {
            const list = document.createElement('ul');
            issues.forEach((item) => {
                const row = document.createElement('li');
                row.textContent = formatIssue(item);
                list.appendChild(row);
            });
            container.appendChild(list);
        }
    }

    function renderValidation(errors, warnings) {
        const badge = elements.validationBadge;
        badge.classList.toggle('is-valid', errors.length === 0 && warnings.length === 0);
        badge.classList.toggle('is-warning', errors.length === 0 && warnings.length > 0);
        badge.classList.toggle('is-invalid', errors.length > 0);

        if (errors.length) {
            badge.textContent = tr('invalid');
            renderIssueList(elements.validationSummary, tr('errorCount', { count: errors.length }), errors, 'validation-summary is-invalid');
        } else if (warnings.length) {
            badge.textContent = tr('warning');
            renderIssueList(elements.validationSummary, tr('warningCount', { count: warnings.length }), warnings, 'validation-summary is-warning');
        } else {
            badge.textContent = tr('valid');
            renderIssueList(elements.validationSummary, tr('configReady'), [], 'validation-summary');
        }
        elements.copy.disabled = errors.length > 0;
        elements.download.disabled = errors.length > 0;
    }

    function renderOutput() {
        if (!elements.targetList) return;
        const collected = collectConfig();
        const validation = validateGeneratedConfig(collected.config);
        const errors = collected.errors.concat(validation.errors);
        lastGeneratedJSON = validation.json;
        elements.output.value = lastGeneratedJSON;
        renderValidation(errors, validation.warnings);
    }

    function replaceTargets(config) {
        elements.targetList.replaceChildren();
        config.targets.forEach((target) => addTarget(target, { silent: true }));
        refreshTargetLabels();
        renderOutput();
    }

    function showImportResult(result) {
        if (result.errors.length) {
            renderIssueList(elements.importStatus, tr('errorCount', { count: result.errors.length }), result.errors, 'import-status is-error');
            return false;
        }
        replaceTargets(result.config);
        const message = result.warnings.length
            ? tr('importSuccessWithWarnings', { count: result.config.targets.length, warnings: result.warnings.length })
            : tr('importSuccess', { count: result.config.targets.length });
        renderIssueList(elements.importStatus, message, result.warnings, 'import-status');
        return true;
    }

    function importCurrentText() {
        return showImportResult(parseConfigText(elements.importInput.value));
    }

    function setActionStatus(message, error) {
        if (!elements.actionStatus) return;
        clearTimeout(actionStatusTimer);
        elements.actionStatus.textContent = message;
        elements.actionStatus.classList.toggle('is-error', Boolean(error));
        actionStatusTimer = setTimeout(() => {
            elements.actionStatus.textContent = '';
            elements.actionStatus.classList.remove('is-error');
        }, 5000);
    }

    async function copyConfiguration() {
        try {
            if (navigator.clipboard && navigator.clipboard.writeText) {
                await navigator.clipboard.writeText(lastGeneratedJSON);
            } else {
                elements.output.focus();
                elements.output.select();
                if (!document.execCommand('copy')) throw new Error('copy command failed');
            }
            setActionStatus(tr('copied'), false);
        } catch (_) {
            elements.output.focus();
            elements.output.select();
            setActionStatus(tr('copyFailed'), true);
        }
    }

    function downloadConfiguration() {
        const blob = new Blob([lastGeneratedJSON], { type: 'application/json;charset=utf-8' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = 'webhooks.json';
        document.body.appendChild(link);
        link.click();
        link.remove();
        URL.revokeObjectURL(url);
        setActionStatus(tr('downloaded'), false);
    }

    function readConfigFile(file) {
        if (!file) return;
        if (file.size > MAX_CONFIG_BYTES) {
            renderIssueList(elements.importStatus, tr('fileTooLarge'), [], 'import-status is-error');
            return;
        }
        const reader = new FileReader();
        reader.addEventListener('load', () => {
            elements.importInput.value = typeof reader.result === 'string' ? reader.result : '';
            importCurrentText();
        });
        reader.addEventListener('error', () => {
            renderIssueList(elements.importStatus, tr('fileReadFailed'), [], 'import-status is-error');
        });
        reader.readAsText(file);
    }

    function initializeDragAndDrop() {
        ['dragenter', 'dragover'].forEach((eventName) => {
            elements.dropZone.addEventListener(eventName, (event) => {
                event.preventDefault();
                elements.dropZone.classList.add('is-dragging');
            });
        });
        ['dragleave', 'drop'].forEach((eventName) => {
            elements.dropZone.addEventListener(eventName, (event) => {
                event.preventDefault();
                elements.dropZone.classList.remove('is-dragging');
            });
        });
        elements.dropZone.addEventListener('drop', (event) => {
            readConfigFile(event.dataTransfer && event.dataTransfer.files ? event.dataTransfer.files[0] : null);
        });
        elements.dropZone.addEventListener('keydown', (event) => {
            if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault();
                elements.file.click();
            }
        });
    }

    function initialize() {
        elements = {
            language: document.getElementById('webhookLanguage'),
            theme: document.getElementById('webhookTheme'),
            targetList: document.getElementById('targetList'),
            targetTemplate: document.getElementById('targetTemplate'),
            headerTemplate: document.getElementById('headerTemplate'),
            targetCount: document.getElementById('targetCount'),
            addTarget: document.getElementById('addTarget'),
            loadExample: document.getElementById('loadExample'),
            importInput: document.getElementById('importInput'),
            importStatus: document.getElementById('importStatus'),
            parseImport: document.getElementById('parseImport'),
            file: document.getElementById('configFile'),
            dropZone: document.getElementById('dropZone'),
            output: document.getElementById('configOutput'),
            validationBadge: document.getElementById('validationBadge'),
            validationSummary: document.getElementById('validationSummary'),
            copy: document.getElementById('copyConfig'),
            download: document.getElementById('downloadConfig'),
            actionStatus: document.getElementById('actionStatus')
        };
        if (!elements.targetList || !elements.targetTemplate || !elements.headerTemplate) return;

        currentLanguage = detectLanguage();
        elements.language.value = currentLanguage;
        document.documentElement.lang = currentLanguage;
        document.documentElement.dataset.language = currentLanguage;
        document.title = currentLanguage === 'zh-CN' ? 'OwlMail Webhook 配置生成器' : 'OwlMail Webhook Configurator';
        applyTranslations(document);
        applyTheme(initialTheme());

        elements.language.addEventListener('change', (event) => setLanguage(event.target.value));
        elements.theme.addEventListener('click', () => {
            applyTheme(document.body.classList.contains('dark-theme') ? 'light' : 'dark');
        });
        elements.addTarget.addEventListener('click', () => {
            const card = addTarget({ name: 'target-' + (elements.targetList.children.length + 1), url: '' });
            if (card) card.querySelector('[data-field="url"]').focus();
        });
        elements.loadExample.addEventListener('click', () => {
            replaceTargets(EXAMPLE_CONFIG);
            elements.importStatus.replaceChildren();
        });
        elements.parseImport.addEventListener('click', importCurrentText);
        elements.file.addEventListener('change', (event) => {
            readConfigFile(event.target.files && event.target.files[0]);
            event.target.value = '';
        });
        elements.copy.addEventListener('click', copyConfiguration);
        elements.download.addEventListener('click', downloadConfiguration);
        initializeDragAndDrop();

        addTarget(DEFAULT_TARGET, { silent: true });
        refreshTargetLabels();
        renderOutput();
    }

    const publicAPI = {
        MAX_CONFIG_BYTES,
        MAX_TARGETS,
        parseConfigText,
        parseDurationMilliseconds,
        validateConfig,
        normalizeConfig,
        validateGeneratedConfig,
        utf8ByteLength,
        validGlobPattern,
        patternsFromEditorValue,
        editorValueFromPreserved,
        createHeaderMap
    };
    if (typeof window !== 'undefined') window.OwlMailWebhookConfigurator = publicAPI;
    if (typeof document !== 'undefined') document.addEventListener('DOMContentLoaded', initialize);
})();
