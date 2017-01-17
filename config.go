package sync_integration

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	ApiEndpoint       string `json:"api"`
	AdminUser         string `json:"admin_user"`
	AdminPassword     string `json:"admin_password"`
	SkipSSLValidation bool   `json:"skip_ssl_validation"`
	AppsDomain        string `json:"apps_domain"`
}

func NewConfig(path string) (Config, error) {
	config := Config{}

	configFile, err := os.Open(path)
	if err != nil {
		return config, err
	}
	defer configFile.Close()

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&config)
	return config, err
}

func (c Config) GetAdminPassword() string { return c.AdminPassword }

func (c Config) GetAdminUser() string { return c.AdminUser }

func (c Config) GetApiEndpoint() string { return c.ApiEndpoint }

func (c Config) GetSkipSSLValidation() bool { return c.SkipSSLValidation }

func (c Config) GetAppsDomain() string { return c.AppsDomain }

func (c Config) GetNamePrefix() string { return "SITS" }

func (c Config) Protocol() string { return "http://" }

func (c Config) GetScaledTimeout(timeout time.Duration) time.Duration {
	return timeout
}

// the following are not used but need to be implemented to satisfy the required interface
func (c Config) GetConfigurableTestPassword() string { return "" }
func (c Config) GetExistingUser() string             { return "" }
func (c Config) GetExistingUserPassword() string     { return "" }
func (c Config) GetPersistentAppOrg() string         { return "" }
func (c Config) GetPersistentAppQuotaName() string   { return "" }
func (c Config) GetPersistentAppSpace() string       { return "" }
func (c Config) GetShouldKeepUser() bool             { return false }
func (c Config) GetUseExistingUser() bool            { return false }
