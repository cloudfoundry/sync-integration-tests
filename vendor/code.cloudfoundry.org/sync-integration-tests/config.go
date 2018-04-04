package sync_integration

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	ApiEndpoint        string `json:"cf_api"`
	AdminUser          string `json:"cf_admin_user"`
	AdminPassword      string `json:"cf_admin_password"`
	SkipSSLValidation  bool   `json:"cf_skip_ssl_validation"`
	AppsDomain         string `json:"cf_apps_domain"`
	BBSClientCert      string `json:"bbs_client_cert"`
	BBSClientKey       string `json:"bbs_client_key"`
	BoshBinary         string `json:"bosh_binary"`
	APIInstance        string `json:"bosh_api_instance"`
	BoshDeploymentName string `json:"bosh_deployment_name"`
	BoshCACert         string `json:"bosh_ca_cert"`
	BoshClient         string `json:"bosh_client"`
	BoshClientSecret   string `json:"bosh_client_secret"`
	BoshEnvironment    string `json:"bosh_environment"`
	BoshGWUser         string `json:"bosh_gw_user"`
	BoshGWHost         string `json:"bosh_gw_host"`
	BoshGWPrivateKey   string `json:"bosh_gw_private_key"`
}

func NewConfig(path string) (Config, error) {
	config := Config{
		BoshBinary:         "bosh",
		APIInstance:        "api",
		BoshDeploymentName: "cf",
	}

	configFile, err := os.Open(path)
	if err != nil {
		return config, err
	}
	defer configFile.Close()

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&config)
	return config, err
}

func (c Config) Validate() error {
	missingProperties := []string{}
	if c.ApiEndpoint == "" {
		missingProperties = append(missingProperties, "cf_api")
	}
	if c.AdminUser == "" {
		missingProperties = append(missingProperties, "cf_admin_user")
	}
	if c.AdminPassword == "" {
		missingProperties = append(missingProperties, "cf_admin_password")
	}
	if c.AppsDomain == "" {
		missingProperties = append(missingProperties, "cf_apps_domain")
	}
	if c.BBSClientCert == "" {
		missingProperties = append(missingProperties, "bbs_client_cert")
	}
	if c.BBSClientKey == "" {
		missingProperties = append(missingProperties, "bbs_client_key")
	}
	if c.BoshCACert == "" {
		missingProperties = append(missingProperties, "bosh_ca_cert")
	}
	if c.BoshClient == "" {
		missingProperties = append(missingProperties, "bosh_client")
	}
	if c.BoshClientSecret == "" {
		missingProperties = append(missingProperties, "bosh_client_secret")
	}
	if c.BoshEnvironment == "" {
		missingProperties = append(missingProperties, "bosh_environment")
	}
	if c.BoshGWUser == "" {
		missingProperties = append(missingProperties, "bosh_gw_user")
	}
	if c.BoshGWHost == "" {
		missingProperties = append(missingProperties, "bosh_gw_host")
	}
	if c.BoshGWPrivateKey == "" {
		missingProperties = append(missingProperties, "bosh_gw_private_key")
	}

	if len(missingProperties) > 0 {
		return errors.New(fmt.Sprintf("Missing required config properties: %s", strings.Join(missingProperties, ", ")))
	} else {
		return nil
	}
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
