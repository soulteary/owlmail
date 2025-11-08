package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// relayEmail handles POST /api/v1/emails/:id/actions/relay
func (api *API) relayEmail(c *gin.Context) {
	id := c.Param("id")

	// Get optional relayTo parameter from query or body
	relayTo := c.Query("relayTo")
	if relayTo == "" {
		var body struct {
			RelayTo string `json:"relayTo"`
		}
		if err := c.ShouldBindJSON(&body); err == nil {
			relayTo = body.RelayTo
		}
	}

	// Get email
	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}

	// Relay email
	var relayErr error
	if relayTo != "" {
		// Relay to specific address
		relayErr = api.mailServer.RelayMailTo(email, relayTo, func(err error) {
			if err != nil {
				Error("Error relaying email %s to %s: %v", id, relayTo, err)
			}
		})
	} else {
		// Relay to configured SMTP server
		relayErr = api.mailServer.RelayMail(email, false, func(err error) {
			if err != nil {
				Error("Error relaying email %s: %v", id, err)
			}
		})
	}

	if relayErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": relayErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Email relayed successfully",
		"relayTo": relayTo,
	})
}

// relayEmailWithParam handles POST /api/v1/emails/:id/actions/relay/:relayTo
func (api *API) relayEmailWithParam(c *gin.Context) {
	id := c.Param("id")
	relayTo := c.Param("relayTo")

	// Validate email address format (simple check)
	if relayTo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email address provided"})
		return
	}

	// Get email
	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}

	// Relay email to specific address
	relayErr := api.mailServer.RelayMailTo(email, relayTo, func(err error) {
		if err != nil {
			Error("Error relaying email %s to %s: %v", id, relayTo, err)
		}
	})

	if relayErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": relayErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Email relayed successfully",
		"relayTo": relayTo,
	})
}
