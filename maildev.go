package main

import (
	"os"
	"strconv"
)

// MailDev 环境变量兼容层
//
// 此文件提供 MailDev 环境变量到 OwlMail 环境变量的兼容性映射
// 优先使用 MailDev 环境变量，如果不存在则使用 OwlMail 环境变量
//
// 支持的 MailDev 环境变量映射：
//   - MAILDEV_SMTP_PORT → OWLMAIL_SMTP_PORT
//   - MAILDEV_IP → OWLMAIL_SMTP_HOST
//   - MAILDEV_MAIL_DIRECTORY → OWLMAIL_MAIL_DIR
//   - MAILDEV_WEB_PORT → OWLMAIL_WEB_PORT
//   - MAILDEV_WEB_IP → OWLMAIL_WEB_HOST
//   - MAILDEV_WEB_USER → OWLMAIL_WEB_USER
//   - MAILDEV_WEB_PASS → OWLMAIL_WEB_PASSWORD
//   - MAILDEV_HTTPS → OWLMAIL_HTTPS_ENABLED
//   - MAILDEV_HTTPS_CERT → OWLMAIL_HTTPS_CERT
//   - MAILDEV_HTTPS_KEY → OWLMAIL_HTTPS_KEY
//   - MAILDEV_OUTGOING_HOST → OWLMAIL_OUTGOING_HOST
//   - MAILDEV_OUTGOING_PORT → OWLMAIL_OUTGOING_PORT
//   - MAILDEV_OUTGOING_USER → OWLMAIL_OUTGOING_USER
//   - MAILDEV_OUTGOING_PASS → OWLMAIL_OUTGOING_PASSWORD
//   - MAILDEV_OUTGOING_SECURE → OWLMAIL_OUTGOING_SECURE
//   - MAILDEV_AUTO_RELAY → OWLMAIL_AUTO_RELAY
//   - MAILDEV_AUTO_RELAY_ADDR → OWLMAIL_AUTO_RELAY_ADDR
//   - MAILDEV_AUTO_RELAY_RULES → OWLMAIL_AUTO_RELAY_RULES
//   - MAILDEV_INCOMING_USER → OWLMAIL_SMTP_USER
//   - MAILDEV_INCOMING_PASS → OWLMAIL_SMTP_PASSWORD
//   - MAILDEV_INCOMING_SECURE → OWLMAIL_TLS_ENABLED
//   - MAILDEV_INCOMING_CERT → OWLMAIL_TLS_CERT
//   - MAILDEV_INCOMING_KEY → OWLMAIL_TLS_KEY
//
// 日志级别支持：
//   - MAILDEV_VERBOSE → verbose
//   - MAILDEV_SILENT → silent
//   - OWLMAIL_LOG_LEVEL → normal/verbose/silent

// getEnvStringWithMailDevCompat 获取环境变量值，优先使用 MailDev 环境变量，如果不存在则使用 OwlMail 环境变量
func getEnvStringWithMailDevCompat(maildevKey, owlmailKey, defaultValue string) string {
	// 优先检查 MailDev 环境变量
	if value := os.Getenv(maildevKey); value != "" {
		return value
	}
	// 如果 MailDev 环境变量不存在，使用 OwlMail 环境变量
	if value := os.Getenv(owlmailKey); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntWithMailDevCompat 获取环境变量整数值，优先使用 MailDev 环境变量
func getEnvIntWithMailDevCompat(maildevKey, owlmailKey string, defaultValue int) int {
	// 优先检查 MailDev 环境变量
	if value := os.Getenv(maildevKey); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	// 如果 MailDev 环境变量不存在，使用 OwlMail 环境变量
	if value := os.Getenv(owlmailKey); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBoolWithMailDevCompat 获取环境变量布尔值，优先使用 MailDev 环境变量
func getEnvBoolWithMailDevCompat(maildevKey, owlmailKey string, defaultValue bool) bool {
	// 优先检查 MailDev 环境变量
	if value := os.Getenv(maildevKey); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	// 如果 MailDev 环境变量不存在，使用 OwlMail 环境变量
	if value := os.Getenv(owlmailKey); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// MailDev 环境变量映射表
// 映射关系：MAILDEV_* → OWLMAIL_*
var maildevEnvMapping = map[string]string{
	// SMTP 服务器配置
	"MAILDEV_SMTP_PORT":      "OWLMAIL_SMTP_PORT",
	"MAILDEV_IP":             "OWLMAIL_SMTP_HOST",
	"MAILDEV_MAIL_DIRECTORY": "OWLMAIL_MAIL_DIR",

	// Web API 配置
	"MAILDEV_WEB_PORT": "OWLMAIL_WEB_PORT",
	"MAILDEV_WEB_IP":   "OWLMAIL_WEB_HOST",
	"MAILDEV_WEB_USER": "OWLMAIL_WEB_USER",
	"MAILDEV_WEB_PASS": "OWLMAIL_WEB_PASSWORD",

	// HTTPS 配置
	"MAILDEV_HTTPS":      "OWLMAIL_HTTPS_ENABLED",
	"MAILDEV_HTTPS_CERT": "OWLMAIL_HTTPS_CERT",
	"MAILDEV_HTTPS_KEY":  "OWLMAIL_HTTPS_KEY",

	// 出站邮件配置
	"MAILDEV_OUTGOING_HOST":   "OWLMAIL_OUTGOING_HOST",
	"MAILDEV_OUTGOING_PORT":   "OWLMAIL_OUTGOING_PORT",
	"MAILDEV_OUTGOING_USER":   "OWLMAIL_OUTGOING_USER",
	"MAILDEV_OUTGOING_PASS":   "OWLMAIL_OUTGOING_PASSWORD",
	"MAILDEV_OUTGOING_SECURE": "OWLMAIL_OUTGOING_SECURE",

	// 自动中继配置
	"MAILDEV_AUTO_RELAY":       "OWLMAIL_AUTO_RELAY",
	"MAILDEV_AUTO_RELAY_ADDR":  "OWLMAIL_AUTO_RELAY_ADDR",
	"MAILDEV_AUTO_RELAY_RULES": "OWLMAIL_AUTO_RELAY_RULES",

	// SMTP 认证配置
	"MAILDEV_INCOMING_USER": "OWLMAIL_SMTP_USER",
	"MAILDEV_INCOMING_PASS": "OWLMAIL_SMTP_PASSWORD",

	// TLS 配置
	"MAILDEV_INCOMING_SECURE": "OWLMAIL_TLS_ENABLED",
	"MAILDEV_INCOMING_CERT":   "OWLMAIL_TLS_CERT",
	"MAILDEV_INCOMING_KEY":    "OWLMAIL_TLS_KEY",
}

// getMailDevEnvString 获取环境变量值，支持 MailDev 兼容
// 优先使用 MailDev 环境变量，如果不存在则使用 OwlMail 环境变量
func getMailDevEnvString(owlmailKey string, defaultValue string) string {
	// 查找对应的 MailDev 环境变量名
	for maildevKey, mappedKey := range maildevEnvMapping {
		if mappedKey == owlmailKey {
			return getEnvStringWithMailDevCompat(maildevKey, owlmailKey, defaultValue)
		}
	}
	// 如果没有找到映射，直接使用 OwlMail 环境变量
	return getEnvString(owlmailKey, defaultValue)
}

// getMailDevEnvInt 获取环境变量整数值，支持 MailDev 兼容
func getMailDevEnvInt(owlmailKey string, defaultValue int) int {
	// 查找对应的 MailDev 环境变量名
	for maildevKey, mappedKey := range maildevEnvMapping {
		if mappedKey == owlmailKey {
			return getEnvIntWithMailDevCompat(maildevKey, owlmailKey, defaultValue)
		}
	}
	// 如果没有找到映射，直接使用 OwlMail 环境变量
	return getEnvInt(owlmailKey, defaultValue)
}

// getMailDevEnvBool 获取环境变量布尔值，支持 MailDev 兼容
func getMailDevEnvBool(owlmailKey string, defaultValue bool) bool {
	// 查找对应的 MailDev 环境变量名
	for maildevKey, mappedKey := range maildevEnvMapping {
		if mappedKey == owlmailKey {
			return getEnvBoolWithMailDevCompat(maildevKey, owlmailKey, defaultValue)
		}
	}
	// 如果没有找到映射，直接使用 OwlMail 环境变量
	return getEnvBool(owlmailKey, defaultValue)
}

// getMailDevLogLevel 获取日志级别，支持 MailDev 兼容
// MailDev 使用 --verbose 和 --silent 参数，这里通过环境变量兼容
func getMailDevLogLevel(defaultValue string) string {
	// MailDev 使用 --verbose 和 --silent 参数，没有对应的环境变量
	// 但我们可以检查是否有 MAILDEV_VERBOSE 或 MAILDEV_SILENT 环境变量
	if os.Getenv("MAILDEV_VERBOSE") != "" {
		return "verbose"
	}
	if os.Getenv("MAILDEV_SILENT") != "" {
		return "silent"
	}
	// 如果没有设置，使用 OwlMail 的日志级别环境变量
	return getEnvString("OWLMAIL_LOG_LEVEL", defaultValue)
}

// ============================================================================
// MailDev API 兼容层
// ============================================================================
//
// 此部分提供与 MailDev 完全兼容的 API 路由，保持向后兼容性
// 新的 API 设计在 api.go 中实现，使用更合理的 RESTful 设计
//
// MailDev 原始 API 端点（保持兼容）：
//   - GET    /email                    - 获取所有邮件
//   - GET    /email/:id                - 获取单个邮件
//   - GET    /email/:id/html           - 获取邮件 HTML
//   - GET    /email/:id/attachment/:filename - 下载附件
//   - GET    /email/:id/download        - 下载原始 .eml 文件
//   - GET    /email/:id/source         - 获取邮件原始源码
//   - DELETE /email/:id                - 删除单个邮件
//   - DELETE /email/all                 - 删除所有邮件
//   - PATCH  /email/read-all            - 标记所有邮件为已读
//   - POST   /email/:id/relay/:relayTo? - 转发邮件
//   - GET    /config                    - 获取配置
//   - GET    /healthz                   - 健康检查
//   - GET    /reloadMailsFromDirectory  - 重新加载邮件
//   - GET    /socket.io                 - WebSocket 连接
//
// 新的 API 设计（更合理）：
//   - GET    /api/v1/emails             - 获取所有邮件（复数资源）
//   - GET    /api/v1/emails/:id         - 获取单个邮件
//   - GET    /api/v1/emails/:id/html    - 获取邮件 HTML
//   - GET    /api/v1/emails/:id/attachments/:filename - 下载附件（复数）
//   - GET    /api/v1/emails/:id/raw     - 获取原始邮件（更清晰的命名）
//   - GET    /api/v1/emails/:id/source  - 获取邮件源码
//   - DELETE /api/v1/emails/:id         - 删除单个邮件
//   - DELETE /api/v1/emails              - 删除所有邮件（批量操作）
//   - PATCH  /api/v1/emails/read         - 标记所有邮件为已读（更清晰）
//   - PATCH  /api/v1/emails/:id/read    - 标记单个邮件为已读
//   - POST   /api/v1/emails/:id/actions/relay - 转发邮件（动作更清晰）
//   - GET    /api/v1/emails/stats       - 邮件统计
//   - GET    /api/v1/emails/preview     - 邮件预览
//   - DELETE /api/v1/emails/batch      - 批量删除（更 RESTful）
//   - PATCH  /api/v1/emails/batch/read  - 批量标记已读
//   - GET    /api/v1/emails/export      - 导出邮件
//   - GET    /api/v1/settings           - 获取所有设置
//   - GET    /api/v1/settings/outgoing - 获取出站配置
//   - PUT    /api/v1/settings/outgoing - 更新出站配置
//   - PATCH  /api/v1/settings/outgoing - 部分更新出站配置
//   - GET    /api/v1/health             - 健康检查（更标准）
//   - POST   /api/v1/emails/reload     - 重新加载邮件（POST 更合理）
//   - GET    /api/v1/ws                 - WebSocket 连接（更清晰）
//
// API 设计改进说明：
// 1. 资源命名统一使用复数：/emails 而不是 /email
// 2. RESTful 设计更规范：DELETE /emails 表示批量删除
// 3. 动作命名更清晰：/actions/relay 明确表示这是一个动作
// 4. 子资源命名更规范：/attachments 使用复数
// 5. 配置 API 更清晰：/settings 比 /config 更语义化
// 6. 健康检查更标准：/health 比 /healthz 更常见
// 7. 重新加载使用 POST：POST /emails/reload 比 GET 更合理
// 8. WebSocket 路径更清晰：/ws 比 /socket.io 更简洁
// 9. API 版本化：/api/v1/ 提供版本控制
// 10. 批量操作更 RESTful：DELETE /emails/batch 而不是 POST /email/batch/delete
