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
