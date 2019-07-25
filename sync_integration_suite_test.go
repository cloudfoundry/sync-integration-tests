package sync_integration_test

import (
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry-incubator/cf-test-helpers/workflowhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"

	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"testing"

	"code.cloudfoundry.org/copilot"
	"code.cloudfoundry.org/sync-integration-tests/config"
)

type relationship struct {
	App   map[string]string `json:"app"`
	Route map[string]string `json:"route"`
}

type routeList struct {
	Resources []struct {
		Metadata struct {
			Guid string `json:"guid"`
		} `json:"metadata"`
	} `json:"resources"`
}

var (
	bbsClient         bbs.Client
	copilotClient     copilot.CloudControllerClient
	runRouteTests     bool
	runRevisionsTests bool
	runSidecarTests   bool
	logger            lager.Logger
	testConfig        config.Config
	testSetup         *workflowhelpers.ReproducibleTestSuiteSetup

	portForwardingSession *Session
)

const (
	BBSAddress      = "https://127.0.0.1:8889"
	CopilotAddress  = "127.0.0.1:9001"
	ShortTimeout    = 10 * time.Second
	Timeout         = 60 * time.Second
	PushTimeout     = 2 * time.Minute
	PollingInterval = 5 * time.Second
)

func loadConfigAndSetVariables() {
	var err error
	configPath := os.Getenv("CONFIG")
	if configPath == "" {
		log.Fatal("Missing required environment variable CONFIG.")
	}
	testConfig, err = config.NewConfig(configPath)
	Expect(err).NotTo(HaveOccurred())

	err = testConfig.Validate()
	Expect(err).NotTo(HaveOccurred())

	if testConfig.PortForwardingScript == "" {
		os.Setenv("BOSH_CA_CERT", testConfig.BoshCACert)
		os.Setenv("BOSH_CLIENT", testConfig.BoshClient)
		os.Setenv("BOSH_CLIENT_SECRET", testConfig.BoshClientSecret)
		os.Setenv("BOSH_ENVIRONMENT", testConfig.BoshEnvironment)
		os.Setenv("BOSH_GW_USER", testConfig.BoshGWUser)
		os.Setenv("BOSH_GW_HOST", testConfig.BoshGWHost)
		os.Setenv("BOSH_GW_PRIVATE_KEY", testConfig.BoshGWPrivateKey)
	}
}

func TestSITSTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SITS Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	loadConfigAndSetVariables()

	var command *exec.Cmd
	if testConfig.PortForwardingScript == "" {
		command = exec.Command(testConfig.BoshBinary,
			"-d",
			testConfig.BoshDeploymentName,
			"ssh",
			testConfig.APIInstance,
			"--opts=-N",
			"--opts=-L 8889:bbs.service.cf.internal:8889",
			"--opts=-L 9001:copilot.service.cf.internal:9001",
		)
	} else {
		command = exec.Command(testConfig.PortForwardingScript)
	}
	var err error
	portForwardingSession, err = Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return nil
}, func(_ []byte) {
	loadConfigAndSetVariables()

	logger = lagertest.NewTestLogger("sits")

	logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

	var err error
	bbsClient, err = bbs.NewSecureSkipVerifyClient(BBSAddress, testConfig.BBSClientCert, testConfig.BBSClientKey, 0, 0)
	Expect(err).NotTo(HaveOccurred())
	Eventually(func() bool {
		return bbsClient.Ping(logger)
	}, ShortTimeout, 5*time.Second).Should(BeTrue(), "Unable to reach BBS at %s", BBSAddress)

	runRouteTests = testConfig.CopilotClientCert != "" && testConfig.CopilotClientKey != ""

	if runRouteTests {
		copilotClientCert, err := tls.LoadX509KeyPair(testConfig.CopilotClientCert, testConfig.CopilotClientKey)
		copilotTLSConfig := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
			InsecureSkipVerify: true,
			Certificates:       []tls.Certificate{copilotClientCert},
		}

		copilotClient, err = copilot.NewCloudControllerClient(CopilotAddress, copilotTLSConfig)
		Expect(err).NotTo(HaveOccurred())
	}

	testSetup = workflowhelpers.NewTestSuiteSetup(testConfig)
	testSetup.Setup()
})

var _ = SynchronizedAfterSuite(func() {
	if testSetup != nil {
		testSetup.Teardown()
	}
}, func() {
	if portForwardingSession != nil {
		portForwardingSession.Kill()
	}
})

func GetAppGuid(appName string) string {
	cfApp := cf.Cf("app", appName, "--guid")
	Eventually(cfApp, Timeout).Should(Exit(0))

	appGuid := strings.TrimSpace(string(cfApp.Out.Contents()))
	Expect(appGuid).NotTo(Equal(""))
	return appGuid
}

func GetProcessGuid(appName string) string {
	desiredLRPs, err := bbsClient.DesiredLRPs(logger, models.DesiredLRPFilter{})
	Expect(err).NotTo(HaveOccurred())

	guid := cf.Cf("app", appName, "--guid").Wait(Timeout).Out.Contents()
	appGuid := strings.TrimSpace(string(guid))

	for _, desiredLRP := range desiredLRPs {
		if strings.Contains(desiredLRP.ProcessGuid, appGuid) {
			return desiredLRP.ProcessGuid
		}
	}
	return ""
}

func DeleteProcessGuidFromDiego(processGuid string) {
	Expect(processGuid).NotTo(BeEmpty())

	Expect(bbsClient.RemoveDesiredLRP(logger, processGuid)).To(Succeed())
}

func GetDropletGuidForApp(appGuid string) string {
	type Resource struct {
		Guid string `json:"guid"`
	}
	type DropletResult struct {
		Resources []Resource `json:"resources"`
	}

	app_droplets := cf.Cf("curl", fmt.Sprintf("/v3/apps/%s/droplets?per_page=1", appGuid)).Wait(Timeout).Out.Contents()
	var dropletResult DropletResult
	err := json.Unmarshal(app_droplets, &dropletResult)
	Expect(err).NotTo(HaveOccurred())

	Expect(dropletResult.Resources).To(HaveLen(1))

	return dropletResult.Resources[0].Guid
}

func GetRouteGuid(appName string) string {
	appGuid := GetAppGuid(appName)

	routes := cf.Cf("curl", fmt.Sprintf("/v2/apps/%s/routes", appGuid)).Wait(Timeout).Out.Contents()
	Expect(routes).NotTo(BeEmpty())

	type routesResponse struct {
		Resources []struct {
			Metadata struct {
				Guid string
			}
			Entity struct {
				Host string
			}
		}
	}
	r := &routesResponse{}

	json.Unmarshal(routes, r)
	routeGuid := r.Resources[0].Metadata.Guid
	Expect(routeGuid).NotTo(BeEmpty())

	return routeGuid
}

func CurlAppRoot(appName string) (string, int) {
	resp, err := http.Get(fmt.Sprintf("http://%s.%s", appName, testConfig.AppsDomain))
	Expect(err).ToNot(HaveOccurred())
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())

	return string(body), resp.StatusCode
}

func CurlApp(appName, path string) (string, int) {
	resp, err := http.Get(fmt.Sprintf("http://%s.%s", appName, filepath.Join(testConfig.AppsDomain, path)))
	Expect(err).ToNot(HaveOccurred())
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())

	return string(body), resp.StatusCode
}

func GetCCProcessGuidsForType(appGuid string, processType string) []string {
	processesPath := fmt.Sprintf("/v3/apps/%s/processes?types=%s", appGuid, processType)
	session := cf.Cf("curl", processesPath).Wait()

	processesJSON := struct {
		Resources []struct {
			Guid string `json:"guid"`
		} `json:"resources"`
	}{}
	bytes := session.Wait().Out.Contents()
	err := json.Unmarshal(bytes, &processesJSON)

	guids := []string{}
	if err != nil || len(processesJSON.Resources) == 0 {
		return guids
	}

	for _, resource := range processesJSON.Resources {
		guids = append(guids, resource.Guid)
	}

	return guids
}
