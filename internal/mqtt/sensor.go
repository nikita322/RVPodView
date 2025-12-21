package mqtt

// SensorType defines the type of sensor for Home Assistant
type SensorType string

const (
	SensorTypeTemperature SensorType = "temperature"
	SensorTypeHumidity    SensorType = "humidity"
	SensorTypePower       SensorType = "power"
	SensorTypeVoltage     SensorType = "voltage"
	SensorTypeCurrent     SensorType = "current"
	SensorTypeFrequency   SensorType = "frequency"
	SensorTypePercentage  SensorType = "percentage"
	SensorTypeBinary      SensorType = "binary_sensor"
)

// SensorData represents sensor data for MQTT publishing
type SensorData struct {
	ID         string                 // Unique sensor ID (will be sanitized)
	Label      string                 // Human-readable label
	Value      interface{}            // Current value
	Attributes map[string]interface{} // Additional attributes
}

// SensorConfig contains sensor configuration for Home Assistant Discovery
type SensorConfig struct {
	// Basic parameters
	SensorID   string     // Unique sensor ID
	Name       string     // Display name
	SensorType SensorType // Sensor type (temperature, humidity, etc.)

	// Units of measurement
	Unit string // °C, %, W, V, A, Hz, etc.

	// MQTT topics
	StateTopic      string // Topic for value
	AttributesTopic string // Topic for attributes

	// Home Assistant parameters
	DeviceClass string // temperature, humidity, power, voltage, etc.
	StateClass  string // measurement, total, total_increasing

	// Availability
	AvailabilityTopic string // Availability topic

	// Device grouping
	DeviceInfo *DeviceInfo
}

// DeviceInfo contains device information for grouping in Home Assistant
type DeviceInfo struct {
	Identifiers  []string // Unique device identifiers
	Name         string   // Device name
	Model        string   // Model
	Manufacturer string   // Manufacturer
}
