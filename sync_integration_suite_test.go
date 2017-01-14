package sync_integration_test

import (
	"flag"
	"log"
	"os"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/cf-acceptance-tests/helpers/config"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry-incubator/cf-test-helpers/workflowhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var (
	bbsClient  bbs.Client
	logger     lager.Logger
	testConfig config.CatsConfig
	testSetup  *workflowhelpers.ReproducibleTestSuiteSetup

	bbsAddress    string
	bbsClientCert string
	bbsClientKey  string
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

	testConfig, err = config.NewCatsConfig(os.Getenv("CONFIG"))
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
