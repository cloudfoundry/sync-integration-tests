package sync_integration_test

import (
	"os"
	"os/exec"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	sync_integration "code.cloudfoundry.org/sync-integration-tests"
	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry-incubator/cf-test-helpers/workflowhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"

	"encoding/json"
	"fmt"
	"log"
	"testing"
)

var (
	bbsClient  bbs.Client
	logger     lager.Logger
	testConfig sync_integration.Config
	testSetup  *workflowhelpers.ReproducibleTestSuiteSetup

	session *Session
)

const (
	BBSAddress      = "https://127.0.0.1:8889"
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
	testConfig, err = sync_integration.NewConfig(configPath)
	Expect(err).NotTo(HaveOccurred())

	err = testConfig.Validate()
	Expect(err).NotTo(HaveOccurred())

	os.Setenv("BOSH_CA_CERT", testConfig.BoshCACert)
	os.Setenv("BOSH_CLIENT", testConfig.BoshClient)
	os.Setenv("BOSH_CLIENT_SECRET", testConfig.BoshClientSecret)
	os.Setenv("BOSH_ENVIRONMENT", testConfig.BoshEnvironment)
	os.Setenv("BOSH_GW_USER", testConfig.BoshGWUser)
	os.Setenv("BOSH_GW_HOST", testConfig.BoshGWHost)
	os.Setenv("BOSH_GW_PRIVATE_KEY", testConfig.BoshGWPrivateKey)
}

func TestSITSTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SITS Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	loadConfigAndSetVariables()

	command := exec.Command(testConfig.BoshBinary,
		"-d",
		testConfig.BoshDeploymentName,
		"ssh",
		testConfig.APIInstance,
		"--opts=-N",
		"--opts=-L 8889:bbs.service.cf.internal:8889",
	)
	var err error
	session, err = Start(command, GinkgoWriter, GinkgoWriter)
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

	testSetup = workflowhelpers.NewTestSuiteSetup(testConfig)
	testSetup.Setup()
})

var _ = SynchronizedAfterSuite(func() {
	if testSetup != nil {
		testSetup.Teardown()
	}
}, func() {
	if session != nil {
		session.Kill()
	}
})

func GetAppGuid(appName string) string {
	cfApp := cf.Cf("app", appName, "--guid")
	Eventually(cfApp, Timeout).Should(Exit(0))

	appGuid := strings.TrimSpace(string(cfApp.Out.Contents()))
	Expect(appGuid).NotTo(Equal(""))
	return appGuid
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

func EnableDiego(appName string) {
	guid := GetAppGuid(appName)
	Eventually(cf.Cf("curl", "/v2/apps/"+guid, "-X", "PUT", "-d", `{"diego": true}`), Timeout).Should(Exit(0))
}
