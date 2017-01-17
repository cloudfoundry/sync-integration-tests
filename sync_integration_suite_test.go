package sync_integration_test

import (
	"flag"
	"log"
	"os"
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

	"testing"
)

var (
	bbsClient  bbs.Client
	logger     lager.Logger
	testConfig sync_integration.Config
	testSetup  *workflowhelpers.ReproducibleTestSuiteSetup

	bbsAddress    string
	bbsClientCert string
	bbsClientKey  string
)

const (
	Timeout     = 60 * time.Second
	PushTimeout = 2 * time.Minute
)

func init() {
	flag.StringVar(&bbsAddress, "bbs-address", "http://10.244.16.2:8889", "http address for the bbs (required)")
	flag.StringVar(&bbsClientCert, "bbs-client-cert", "", "bbs client ssl certificate")
	flag.StringVar(&bbsClientKey, "bbs-client-key", "", "bbs client ssl key")
	flag.Parse()

	if bbsAddress == "" {
		log.Fatal("i need a bbs address to talk to Diego...")
	}
}

func TestSITSTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SITS Suite")
}

var _ = BeforeSuite(func() {
	var err error
	bbsClient, err = bbs.NewSecureSkipVerifyClient(bbsAddress, bbsClientCert, bbsClientKey, 0, 0)
	Expect(err).NotTo(HaveOccurred())

	testConfig, err = sync_integration.NewConfig(os.Getenv("CONFIG"))
	Expect(err).NotTo(HaveOccurred())

	testSetup = workflowhelpers.NewTestSuiteSetup(testConfig)
	testSetup.Setup()

	logger = lagertest.NewTestLogger("sits")
	Expect(err).NotTo(HaveOccurred())

	logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))
})

var _ = AfterSuite(func() {
	testSetup.Teardown()
})

func GetAppGuid(appName string) string {
	cfApp := cf.Cf("app", appName, "--guid")
	Eventually(cfApp, Timeout).Should(Exit(0))

	appGuid := strings.TrimSpace(string(cfApp.Out.Contents()))
	Expect(appGuid).NotTo(Equal(""))
	return appGuid
}

func EnableDiego(appName string) {
	guid := GetAppGuid(appName)
	Eventually(cf.Cf("curl", "/v2/apps/"+guid, "-X", "PUT", "-d", `{"diego": true}`), Timeout).Should(Exit(0))
}
