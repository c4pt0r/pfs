package config

import (
	"fmt"
	"strconv"
	"strings"
)

// GetStringConfig retrieves a string value from config with a default fallback
func GetStringConfig(config map[string]interface{}, key, defaultValue string) string {
	if val, ok := config[key].(string); ok && val != "" {
		return val
	}
	return defaultValue
}

// GetBoolConfig retrieves a boolean value from config with a default fallback
func GetBoolConfig(config map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := config[key].(bool); ok {
		return val
	}
	return defaultValue
}

// GetIntConfig retrieves an integer value from config with a default fallback
// Supports int, int64, and float64 types
func GetIntConfig(config map[string]interface{}, key string, defaultValue int) int {
	if val, ok := config[key].(int); ok {
		return val
	}
	if val, ok := config[key].(int64); ok {
		return int(val)
	}
	if val, ok := config[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}

// GetFloat64Config retrieves a float64 value from config with a default fallback
// Supports float64 and int types
func GetFloat64Config(config map[string]interface{}, key string, defaultValue float64) float64 {
	if val, ok := config[key].(float64); ok {
		return val
	}
	if val, ok := config[key].(int); ok {
		return float64(val)
	}
	return defaultValue
}

// RequireString validates that a required string config value is present and non-empty
func RequireString(config map[string]interface{}, key string) (string, error) {
	val, ok := config[key].(string)
	if !ok || val == "" {
		return "", fmt.Errorf("%s is required in configuration", key)
	}
	return val, nil
}

// RequireInt validates that a required integer config value is present
// Supports int, int64, and float64 types
func RequireInt(config map[string]interface{}, key string) (int, error) {
	if val, ok := config[key].(int); ok {
		return val, nil
	}
	if val, ok := config[key].(int64); ok {
		return int(val), nil
	}
	if val, ok := config[key].(float64); ok {
		return int(val), nil
	}
	return 0, fmt.Errorf("%s is required in configuration and must be an integer", key)
}

// ValidateStringType checks if a config value is a string type (if present)
func ValidateStringType(config map[string]interface{}, key string) error {
	if val, exists := config[key]; exists {
		if _, ok := val.(string); !ok {
			return fmt.Errorf("%s must be a string", key)
		}
	}
	return nil
}

// ValidateBoolType checks if a config value is a boolean type (if present)
func ValidateBoolType(config map[string]interface{}, key string) error {
	if val, exists := config[key]; exists {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", key)
		}
	}
	return nil
}

// ValidateIntType checks if a config value is an integer type (if present)
// Accepts int, int64, and float64 types
func ValidateIntType(config map[string]interface{}, key string) error {
	if val, exists := config[key]; exists {
		switch val.(type) {
		case int, int64, float64:
			return nil
		default:
			return fmt.Errorf("%s must be an integer", key)
		}
	}
	return nil
}

// ValidateMapType checks if a config value is a map type (if present)
func ValidateMapType(config map[string]interface{}, key string) error {
	if val, exists := config[key]; exists {
		if _, ok := val.(map[string]interface{}); !ok {
			return fmt.Errorf("%s must be a map", key)
		}
	}
	return nil
}

// ValidateArrayType checks if a config value is an array/slice type (if present)
func ValidateArrayType(config map[string]interface{}, key string) error {
	if val, exists := config[key]; exists {
		if _, ok := val.([]interface{}); !ok {
			return fmt.Errorf("%s must be an array", key)
		}
	}
	return nil
}

// ParseSize parses a size string with units (e.g., "512KB", "1MB", "2GB") or a plain number
// Returns size in bytes
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))

	// Handle pure numbers (bytes)
	if val, err := strconv.ParseInt(s, 10, 64); err == nil {
		return val, nil
	}

	// Parse with unit suffix
	units := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
	}

	for suffix, multiplier := range units {
		if strings.HasSuffix(s, suffix) {
			numStr := strings.TrimSuffix(s, suffix)
			numStr = strings.TrimSpace(numStr)

			// Try parsing as integer first
			if val, err := strconv.ParseInt(numStr, 10, 64); err == nil {
				return val * multiplier, nil
			}

			// Try parsing as float
			if val, err := strconv.ParseFloat(numStr, 64); err == nil {
				return int64(val * float64(multiplier)), nil
			}
		}
	}

	return 0, fmt.Errorf("invalid size format: %s (expected format: number with optional unit B/KB/MB/GB/TB)", s)
}

// GetSizeConfig retrieves a size value from config with a default fallback
// Supports string with units (e.g., "512KB"), int, and float64
func GetSizeConfig(config map[string]interface{}, key string, defaultBytes int64) (int64, error) {
	val, exists := config[key]
	if !exists {
		return defaultBytes, nil
	}

	switch v := val.(type) {
	case string:
		return ParseSize(v)
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("%s must be a size string (e.g., '512KB') or number", key)
	}
}

// GetPortConfig retrieves a port value from config with a default fallback
// Supports string, int, and float64 types
func GetPortConfig(config map[string]interface{}, key, defaultPort string) string {
	if port, ok := config[key].(string); ok && port != "" {
		return port
	}
	if portInt, ok := config[key].(int); ok {
		return fmt.Sprintf("%d", portInt)
	}
	if portFloat, ok := config[key].(float64); ok {
		return fmt.Sprintf("%d", int(portFloat))
	}
	return defaultPort
}

// ValidateOnlyKnownKeys checks that config only contains keys from the allowedKeys list
// Returns an error if any unknown keys are found
func ValidateOnlyKnownKeys(config map[string]interface{}, allowedKeys []string) error {
	// Create a map for fast lookup
	allowed := make(map[string]bool)
	for _, key := range allowedKeys {
		allowed[key] = true
	}

	// Check for unknown keys
	var unknownKeys []string
	for key := range config {
		if !allowed[key] {
			unknownKeys = append(unknownKeys, key)
		}
	}

	if len(unknownKeys) > 0 {
		return fmt.Errorf("unknown configuration parameter(s) '%s' - allowed parameters are: %s",
			strings.Join(unknownKeys, "', '"),
			strings.Join(allowedKeys, "', '"))
	}

	return nil
}
