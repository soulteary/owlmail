package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/owlmail/internal/outgoing"
)

// getConfig handles GET /api/v1/settings
func (api *API) getConfig(c *gin.Context) {
	config := gin.H{
		"version": "1.0.0",
		"smtp": gin.H{
			"host": api.mailServer.GetHost(),
			"port": api.mailServer.GetPort(),
		},
		"web": gin.H{
			"host": api.host,
			"port": api.port,
		},
		"mailDir": api.mailServer.GetMailDir(),
	}

	// Add outgoing mail configuration if available
	outgoingConfig := api.mailServer.GetOutgoingConfig()
	if outgoingConfig != nil {
		config["outgoing"] = gin.H{
			"host":          outgoingConfig.Host,
			"port":          outgoingConfig.Port,
			"user":          outgoingConfig.User,
			"secure":        outgoingConfig.Secure,
			"autoRelay":     outgoingConfig.AutoRelay,
			"autoRelayAddr": outgoingConfig.AutoRelayAddr,
			"allowRules":    outgoingConfig.AllowRules,
			"denyRules":     outgoingConfig.DenyRules,
		}
	} else {
		config["outgoing"] = nil
	}

	// Add SMTP authentication configuration if available
	authConfig := api.mailServer.GetAuthConfig()
	if authConfig != nil {
		config["smtpAuth"] = gin.H{
			"enabled":  authConfig.Enabled,
			"username": authConfig.Username,
		}
	} else {
		config["smtpAuth"] = nil
	}

	// Add TLS configuration if available
	tlsConfig := api.mailServer.GetTLSConfig()
	if tlsConfig != nil {
		config["tls"] = gin.H{
			"enabled":  tlsConfig.Enabled,
			"certFile": tlsConfig.CertFile,
			"keyFile":  tlsConfig.KeyFile,
		}
	} else {
		config["tls"] = nil
	}

	c.JSON(http.StatusOK, config)
}

// getOutgoingConfig handles GET /api/v1/settings/outgoing
func (api *API) getOutgoingConfig(c *gin.Context) {
	outgoingConfig := api.mailServer.GetOutgoingConfig()
	if outgoingConfig == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
			"message": "Outgoing mail not configured",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled":       true,
		"host":          outgoingConfig.Host,
		"port":          outgoingConfig.Port,
		"user":          outgoingConfig.User,
		"secure":        outgoingConfig.Secure,
		"autoRelay":     outgoingConfig.AutoRelay,
		"autoRelayAddr": outgoingConfig.AutoRelayAddr,
		"allowRules":    outgoingConfig.AllowRules,
		"denyRules":     outgoingConfig.DenyRules,
	})
}

// updateOutgoingConfig handles PUT /api/v1/settings/outgoing
func (api *API) updateOutgoingConfig(c *gin.Context) {
	var config outgoing.OutgoingConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Validate required fields
	if config.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Host is required"})
		return
	}

	if config.Port <= 0 || config.Port > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Port must be between 1 and 65535"})
		return
	}

	// Update configuration
	api.mailServer.SetOutgoingConfig(&config)

	c.JSON(http.StatusOK, gin.H{
		"message": "Outgoing mail configuration updated",
		"config": gin.H{
			"host":          config.Host,
			"port":          config.Port,
			"user":          config.User,
			"secure":        config.Secure,
			"autoRelay":     config.AutoRelay,
			"autoRelayAddr": config.AutoRelayAddr,
			"allowRules":    config.AllowRules,
			"denyRules":     config.DenyRules,
		},
	})
}

// patchOutgoingConfig handles PATCH /api/v1/settings/outgoing
func (api *API) patchOutgoingConfig(c *gin.Context) {
	// Get current configuration
	currentConfig := api.mailServer.GetOutgoingConfig()
	if currentConfig == nil {
		// Create new config if none exists
		currentConfig = &outgoing.OutgoingConfig{}
	}

	// Parse partial update
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Apply updates
	if host, ok := updates["host"].(string); ok {
		currentConfig.Host = host
	}
	if port, ok := updates["port"].(float64); ok {
		currentConfig.Port = int(port)
	}
	if user, ok := updates["user"].(string); ok {
		currentConfig.User = user
	}
	if password, ok := updates["password"].(string); ok {
		currentConfig.Password = password
	}
	if secure, ok := updates["secure"].(bool); ok {
		currentConfig.Secure = secure
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

	// Validate if host is set
	if currentConfig.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Host is required"})
		return
	}

	if currentConfig.Port <= 0 || currentConfig.Port > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Port must be between 1 and 65535"})
		return
	}

	// Update configuration
	api.mailServer.SetOutgoingConfig(currentConfig)

	c.JSON(http.StatusOK, gin.H{
		"message": "Outgoing mail configuration updated",
		"config": gin.H{
			"host":          currentConfig.Host,
			"port":          currentConfig.Port,
			"user":          currentConfig.User,
			"secure":        currentConfig.Secure,
			"autoRelay":     currentConfig.AutoRelay,
			"autoRelayAddr": currentConfig.AutoRelayAddr,
			"allowRules":    currentConfig.AllowRules,
			"denyRules":     currentConfig.DenyRules,
		},
	})
}

// healthCheck handles GET /api/v1/health
func (api *API) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
