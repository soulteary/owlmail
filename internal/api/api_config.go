package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/version-kit/v2"
)

// getConfig handles GET /api/v1/settings
func (api *API) getConfig(c fiber.Ctx) error {
	config := fiber.Map{
		"version": version.Default().Version,
		"smtp": fiber.Map{
			"host": api.mailServer.GetHost(),
			"port": api.mailServer.GetPort(),
		},
		"web": fiber.Map{
			"host":         api.host,
			"port":         api.port,
			"basePathname": api.basePathname,
		},
		"mailDir": api.mailServer.GetMailDir(),
	}

	outgoingConfig := api.mailServer.GetOutgoingConfig()
	if outgoingConfig != nil && outgoingConfig.Host != "" {
		config["outgoing"] = outgoingConfigResponse(outgoingConfig)
	} else {
		config["outgoing"] = nil
	}

	authConfig := api.mailServer.GetAuthConfig()
	if authConfig != nil {
		config["smtpAuth"] = fiber.Map{
			"enabled":  authConfig.Enabled,
			"username": authConfig.Username,
		}
	} else {
		config["smtpAuth"] = nil
	}

	tlsConfig := api.mailServer.GetTLSConfig()
	if tlsConfig != nil {
		config["tls"] = fiber.Map{
			"enabled":  tlsConfig.Enabled,
			"certFile": tlsConfig.CertFile,
			"keyFile":  tlsConfig.KeyFile,
		}
	} else {
		config["tls"] = nil
	}

	return c.JSON(config)
}

// getOutgoingConfig handles GET /api/v1/settings/outgoing
func (api *API) getOutgoingConfig(c fiber.Ctx) error {
	outgoingConfig := api.mailServer.GetOutgoingConfig()
	if outgoingConfig == nil || outgoingConfig.Host == "" {
		return c.JSON(fiber.Map{
			"enabled": false,
			"message": "Outgoing mail not configured",
		})
	}

	response := outgoingConfigResponse(outgoingConfig)
	response["enabled"] = true
	return c.JSON(response)
}

// updateOutgoingConfig handles PUT /api/v1/settings/outgoing
func (api *API) updateOutgoingConfig(c fiber.Ctx) error {
	// PUT is a complete replacement: omitting or clearing password disables
	// authentication for relay tasks accepted after this update.
	var config outgoing.OutgoingConfig
	if err := c.Bind().Body(&config); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Invalid request: "+err.Error()))
	}
	if config.Password == "" {
		config.User = ""
	}

	if config.Host == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeHostRequired, "Host is required"))
	}

	if config.Port <= 0 || config.Port > 65535 {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodePortOutOfRange, "Port must be between 1 and 65535"))
	}
	if err := config.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeInvalidRequest, err.Error()))
	}

	if err := api.mailServer.SetOutgoingConfig(&config); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(ErrorResponse(ErrorCodeConfigUpdateFailed, err.Error()))
	}
	updatedConfig := api.mailServer.GetOutgoingConfig()

	return c.JSON(fiber.Map{
		"code":    SuccessCodeConfigUpdated,
		"message": "Outgoing mail configuration updated",
		"config":  outgoingConfigResponse(updatedConfig),
	})
}

// patchOutgoingConfig handles PATCH /api/v1/settings/outgoing
func (api *API) patchOutgoingConfig(c fiber.Ctx) error {
	currentConfig := api.mailServer.GetOutgoingConfig()
	if currentConfig == nil {
		currentConfig = &outgoing.OutgoingConfig{}
	}

	var updates map[string]interface{}
	if err := c.Bind().Body(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Invalid request: "+err.Error()))
	}

	if host, ok := updates["host"].(string); ok {
		currentConfig.Host = host
	}
	if port, ok := updates["port"].(float64); ok {
		currentConfig.Port = int(port)
	}
	if user, ok := updates["user"].(string); ok {
		currentConfig.User = user
	}
	// PATCH preserves the current password unless the field is present. An
	// explicit empty string clears both credentials so the resulting snapshot
	// disables authentication while remaining a valid configuration.
	if password, ok := updates["password"].(string); ok {
		currentConfig.Password = password
		if password == "" {
			currentConfig.User = ""
		}
	}
	if secure, ok := updates["secure"].(bool); ok {
		currentConfig.Secure = secure
		if secure {
			currentConfig.TLSMode = outgoing.TLSModeSMTPS
		} else {
			currentConfig.TLSMode = outgoing.TLSModePlain
		}
	}
	if tlsMode, ok := updates["tlsMode"].(string); ok {
		currentConfig.TLSMode = outgoing.TLSMode(tlsMode)
	}
	if insecureSkipVerify, ok := updates["insecureSkipVerify"].(bool); ok {
		currentConfig.InsecureSkipVerify = insecureSkipVerify
	}
	if connectTimeout, ok := updates["connectTimeout"].(string); ok {
		currentConfig.ConnectTimeout = connectTimeout
	}
	if tlsHandshakeTimeout, ok := updates["tlsHandshakeTimeout"].(string); ok {
		currentConfig.TLSHandshakeTimeout = tlsHandshakeTimeout
	}
	if authTimeout, ok := updates["authTimeout"].(string); ok {
		currentConfig.AuthTimeout = authTimeout
	}
	if envelopeTimeout, ok := updates["envelopeTimeout"].(string); ok {
		currentConfig.EnvelopeTimeout = envelopeTimeout
	}
	if dataTimeout, ok := updates["dataTimeout"].(string); ok {
		currentConfig.DataTimeout = dataTimeout
	}
	if quitTimeout, ok := updates["quitTimeout"].(string); ok {
		currentConfig.QuitTimeout = quitTimeout
	}
	if autoRelay, ok := updates["autoRelay"].(bool); ok {
		currentConfig.AutoRelay = autoRelay
	}
	if autoRelayAddr, ok := updates["autoRelayAddr"].(string); ok {
		currentConfig.AutoRelayAddr = autoRelayAddr
	}
	if allowRules, ok := updates["allowRules"].([]interface{}); ok {
		currentConfig.AllowRules = make([]string, 0, len(allowRules))
		for _, rule := range allowRules {
			if ruleStr, ok := rule.(string); ok {
				currentConfig.AllowRules = append(currentConfig.AllowRules, ruleStr)
			}
		}
	}
	if denyRules, ok := updates["denyRules"].([]interface{}); ok {
		currentConfig.DenyRules = make([]string, 0, len(denyRules))
		for _, rule := range denyRules {
			if ruleStr, ok := rule.(string); ok {
				currentConfig.DenyRules = append(currentConfig.DenyRules, ruleStr)
			}
		}
	}

	if currentConfig.Host == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeHostRequired, "Host is required"))
	}

	if currentConfig.Port <= 0 || currentConfig.Port > 65535 {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodePortOutOfRange, "Port must be between 1 and 65535"))
	}
	if err := currentConfig.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeInvalidRequest, err.Error()))
	}

	if err := api.mailServer.SetOutgoingConfig(currentConfig); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(ErrorResponse(ErrorCodeConfigUpdateFailed, err.Error()))
	}
	updatedConfig := api.mailServer.GetOutgoingConfig()

	return c.JSON(fiber.Map{
		"code":    SuccessCodeConfigUpdated,
		"message": "Outgoing mail configuration updated",
		"config":  outgoingConfigResponse(updatedConfig),
	})
}

func outgoingConfigResponse(config *outgoing.OutgoingConfig) fiber.Map {
	if config == nil {
		return fiber.Map{}
	}
	return fiber.Map{
		"host":                config.Host,
		"port":                config.Port,
		"user":                config.User,
		"secure":              config.Secure,
		"tlsMode":             config.TLSMode,
		"insecureSkipVerify":  config.InsecureSkipVerify,
		"connectTimeout":      config.ConnectTimeout,
		"tlsHandshakeTimeout": config.TLSHandshakeTimeout,
		"authTimeout":         config.AuthTimeout,
		"envelopeTimeout":     config.EnvelopeTimeout,
		"dataTimeout":         config.DataTimeout,
		"quitTimeout":         config.QuitTimeout,
		"autoRelay":           config.AutoRelay,
		"autoRelayAddr":       config.AutoRelayAddr,
		"allowRules":          config.AllowRules,
		"denyRules":           config.DenyRules,
	}
}
