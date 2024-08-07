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
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/cloudfoundry/cf-test-helpers/v2/cf"
	"github.com/cloudfoundry/cf-test-helpers/v2/workflowhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"

	"encoding/json"
	"fmt"
	"log"
	"testing"

	"code.cloudfoundry.org/sync-integration-tests/config"
)

var (
	bbsClient  bbs.Client
	logger     lager.Logger
	testConfig config.Config
	testSetup  *workflowhelpers.ReproducibleTestSuiteSetup

	portForwardingSession *Session
)

const (
	BBSAddress  = "https://127.0.0.1:8889"
	Timeout     = 60 * time.Second
	PushTimeout = 3 * time.Minute
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
		return bbsClient.Ping(logger, "someTraceIDString")
	}, Timeout, 1*time.Second).Should(BeTrue(), "Unable to reach BBS at %s", BBSAddress)

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

func CreateSidecar(name string, processTypes []string, command string, appGuid string) string {
	sidecarEndpoint := fmt.Sprintf("/v3/apps/%s/sidecars", appGuid)
	sidecarOneJSON, err := json.Marshal(
		struct {
			Name         string   `json:"name"`
			Command      string   `json:"command"`
			ProcessTypes []string `json:"process_types"`
		}{
			name,
			command,
			processTypes,
		},
	)
	Expect(err).NotTo(HaveOccurred())
	session := cf.Cf("curl", sidecarEndpoint, "-X", "POST", "-d", string(sidecarOneJSON))
	Eventually(session, Timeout, 1*time.Second).Should(Exit(0))

	var sidecarData struct {
		Guid string `json:"guid"`
	}
	err = json.Unmarshal(session.Out.Contents(), &sidecarData)
	Expect(err).NotTo(HaveOccurred())
	return sidecarData.Guid
}

func GetAppGuid(appName string) string {
	cfApp := cf.Cf("app", appName, "--guid")
	Eventually(cfApp, Timeout, 1*time.Second).Should(Exit(0))

	appGuid := strings.TrimSpace(string(cfApp.Out.Contents()))
	Expect(appGuid).NotTo(Equal(""))
	return appGuid
}

func GetProcessGuid(appName string) string {
	desiredLRPs, err := bbsClient.DesiredLRPs(logger, "someTraceIDString", models.DesiredLRPFilter{})
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

	Expect(bbsClient.RemoveDesiredLRP(logger, "someTraceIDString", processGuid)).To(Succeed())
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
	session := cf.Cf("curl", processesPath).Wait(Timeout)

	processesJSON := struct {
		Resources []struct {
			Guid string `json:"guid"`
		} `json:"resources"`
	}{}
	bytes := session.Wait(Timeout).Out.Contents()
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
