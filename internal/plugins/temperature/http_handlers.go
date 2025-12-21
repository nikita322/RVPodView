package temperature

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"podmanview/internal/plugins"
)

// PluginSettings represents plugin configuration
type PluginSettings struct {
	UpdateInterval int `json:"updateInterval"` // Update interval in seconds
}

// MQTTStatus represents MQTT status
type MQTTStatus struct {
	Enabled     bool   `json:"enabled"`     // MQTT publishing enabled
	Connected   bool   `json:"connected"`   // MQTT client connected
	Configured  bool   `json:"configured"`  // MQTT broker configured
	BrokerURL   string `json:"brokerUrl"`   // MQTT broker URL (for display)
	TopicPrefix string `json:"topicPrefix"` // MQTT topic prefix
}

// MQTTToggleRequest represents request to toggle MQTT
type MQTTToggleRequest struct {
	Enabled bool `json:"enabled"` // Enable or disable MQTT
}

// handleGetTemperatures returns current temperature data
func (p *TemperaturePlugin) handleGetTemperatures(w http.ResponseWriter, r *http.Request) {
	data := p.GetTemperatureData()
	plugins.WriteJSON(w, http.StatusOK, data)
}

// handleGetSettings returns current plugin settings
func (p *TemperaturePlugin) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	interval := int(p.updatePeriod.Seconds())
	p.mu.RUnlock()

	settings := PluginSettings{
		UpdateInterval: interval,
	}

	plugins.WriteJSON(w, http.StatusOK, settings)
}

// handleUpdateSettings updates plugin settings
func (p *TemperaturePlugin) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings PluginSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		plugins.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	// Validate interval (5-60 seconds)
	if settings.UpdateInterval < 5 || settings.UpdateInterval > 60 {
		plugins.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Update interval must be between 5 and 60 seconds"})
		return
	}

	// Update in-memory interval
	p.mu.Lock()
	p.updatePeriod = time.Duration(settings.UpdateInterval) * time.Second
	p.mu.Unlock()

	// Save to storage
	if p.Deps() != nil && p.Deps().Storage != nil {
		if err := p.Deps().Storage.SetInt(p.Name(), "updateInterval", settings.UpdateInterval); err != nil {
			if p.Logger() != nil {
				p.Logger().Printf("[%s] Failed to save update interval to storage: %v", p.Name(), err)
			}
			plugins.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save settings"})
			return
		}
	}

	// Restart background task with new interval
	if err := p.RestartBackgroundTasks(); err != nil {
		if p.Logger() != nil {
			p.Logger().Printf("[%s] Failed to restart background tasks: %v", p.Name(), err)
		}
		plugins.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to restart background tasks"})
		return
	}

	if p.Logger() != nil {
		p.Logger().Printf("[%s] Update interval changed to %d seconds and background task restarted", p.Name(), settings.UpdateInterval)
	}

	plugins.WriteJSON(w, http.StatusOK, map[string]string{"status": "Settings updated successfully"})
}

// handleGetMQTTStatus returns MQTT connection status
func (p *TemperaturePlugin) handleGetMQTTStatus(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	enabled := p.mqttEnabled
	p.mu.RUnlock()

	deps := p.Deps()
	mqttClient := deps.MQTTClient

	status := MQTTStatus{
		Enabled:    enabled,
		Connected:  mqttClient != nil && mqttClient.IsConnected(),
		Configured: mqttClient != nil,
	}

	// Add broker info if configured
	if mqttClient != nil {
		cfg := mqttClient.GetConfig()
		status.BrokerURL = cfg.Broker
		status.TopicPrefix = cfg.Prefix
	}

	plugins.WriteJSON(w, http.StatusOK, status)
}

// handleToggleMQTT enables or disables MQTT publishing
func (p *TemperaturePlugin) handleToggleMQTT(w http.ResponseWriter, r *http.Request) {
	var req MQTTToggleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		plugins.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	deps := p.Deps()
	mqttClient := deps.MQTTClient

	// Check if MQTT client is configured
	if mqttClient == nil {
		plugins.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "MQTT is not configured. Please set MQTT broker in .env file"})
		return
	}

	// Update MQTT enabled state
	p.mu.Lock()
	p.mqttEnabled = req.Enabled
	p.mu.Unlock()

	// Save to storage
	if deps.Storage != nil {
		if err := deps.Storage.SetBool(p.Name(), "mqttEnabled", req.Enabled); err != nil {
			if p.Logger() != nil {
				p.Logger().Printf("[%s] Failed to save MQTT enabled state: %v", p.Name(), err)
			}
			plugins.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save settings"})
			return
		}
	}

	// Connect or disconnect based on the enabled state
	if req.Enabled {
		if !mqttClient.IsConnected() {
			if err := mqttClient.Connect(); err != nil {
				if p.Logger() != nil {
					p.Logger().Printf("[%s] Failed to connect to MQTT broker: %v", p.Name(), err)
				}
				plugins.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to connect to MQTT broker"})
				return
			}
		}

		// Publish online status
		if mqttClient.IsConnected() {
			mqttClient.Publish("sensor/temperature/availability", []byte("online"))
		}

		if p.Logger() != nil {
			p.Logger().Printf("[%s] MQTT publishing enabled", p.Name())
		}
	} else {
		// Publish offline status before disconnecting
		if mqttClient.IsConnected() {
			mqttClient.Publish("sensor/temperature/availability", []byte("offline"))
			time.Sleep(100 * time.Millisecond) // Wait for publish
		}

		mqttClient.Disconnect()
		if p.Logger() != nil {
			p.Logger().Printf("[%s] MQTT publishing disabled", p.Name())
		}
	}

	status := "enabled"
	if !req.Enabled {
		status = "disabled"
	}

	plugins.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "MQTT " + status + " successfully",
		"enabled": strconv.FormatBool(req.Enabled),
	})
}
