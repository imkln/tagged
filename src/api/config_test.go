package api

import "testing"

func TestConfigRejectsInvalidSettings(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Config)
	}{
		{
			name: "missing owner",
			change: func(config *Config) {
				config.OwnerEmail = ""
			},
		},
		{
			name: "insecure public origin",
			change: func(config *Config) {
				config.Origin = "http://tagged.example.test"
			},
		},
		{
			name: "missing client ID",
			change: func(config *Config) {
				config.ClientID = ""
			},
		},
		{
			name: "missing issuer",
			change: func(config *Config) {
				config.Issuer = ""
			},
		},
		{
			name: "missing authentication domain",
			change: func(config *Config) {
				config.AuthDomain = ""
			},
		},
		{
			name: "invalid cookie key",
			change: func(config *Config) {
				config.CookieKey = "invalid"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := validConfig()
			test.change(&config)

			if err := config.Validate(); err == nil {
				t.Fatal("Validate() accepted invalid configuration")
			}
		})
	}
}

func TestConfigAllowsLocalDevelopment(t *testing.T) {
	config := validConfig()
	config.Origin = "http://localhost:5173"

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() rejected a localhost origin: %v", err)
	}
}
