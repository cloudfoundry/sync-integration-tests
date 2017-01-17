package sync_integration_test

import (
	"flag"
	"fmt"
	"log"
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

	"testing"
)

var (
	bbsClient  bbs.Client
	logger     lager.Logger
	testConfig sync_integration.Config
	testSetup  *workflowhelpers.ReproducibleTestSuiteSetup

	environmentPath string
	deploymentName  string
	instanceName    string
	boshBinary      string

	bbsAddress    string
	bbsClientCert string
	bbsClientKey  string

	session *Session
)

const (
	ShortTimeout = 10 * time.Second
	Timeout      = 60 * time.Second
	PushTimeout  = 2 * time.Minute
)

func init() {
	flag.StringVar(&environmentPath, "environment-path", "", "absolute path to private credentials directory. leave blank for bosh-lite")
	flag.StringVar(&deploymentName, "deployment-name", "", "name of the bosh deployment containing an instance to be used as gateway")
	flag.StringVar(&instanceName, "instance-name", "", "name of the instance to be used as gateway")
	flag.StringVar(&boshBinary, "bosh-binary", "bosh", "path or executable name of the bosh binary. this has to be the cli-v2")

	flag.StringVar(&bbsAddress, "bbs-address", "https://10.244.16.2:8889", "http address for the bbs (required)")
	flag.StringVar(&bbsClientCert, "bbs-client-cert", "", "bbs client ssl certificate")
	flag.StringVar(&bbsClientKey, "bbs-client-key", "", "bbs client ssl key")
	flag.Parse()

	if bbsAddress == "" {
		log.Fatal("i need a bbs address to talk to Diego...")
	}

	if environmentPath != "" {
		if deploymentName == "" {
			log.Fatal("deployment-name is required if using a gateway")
		}

		if instanceName == "" {
			log.Fatal("instance-name is required if using a gateway")
		}
	}
}

func TestSITSTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SITS Suite")
}

var _ = BeforeSuite(func() {
	var err error
	logger = lagertest.NewTestLogger("sits")
	Expect(err).NotTo(HaveOccurred())

	logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

	if environmentPath != "" {
		bbsAddress = "https://127.0.0.1:8889"
		cmd := fmt.Sprintf("source .envrc && %s -d %s ssh %s --opts='-N' --opts='-L 8889:bbs.service.cf.internal:8889'", boshBinary, deploymentName, instanceName)
		command := exec.Command("bash", "-c", cmd)
		command.Dir = environmentPath
		session, err = Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
	}

	bbsClient, err = bbs.NewSecureSkipVerifyClient(bbsAddress, bbsClientCert, bbsClientKey, 0, 0)
	Expect(err).NotTo(HaveOccurred())
	Eventually(func() bool {
		return bbsClient.Ping(logger)
	}, ShortTimeout).Should(BeTrue())

	testConfig, err = sync_integration.NewConfig(os.Getenv("CONFIG"))
	Expect(err).NotTo(HaveOccurred())

	testSetup = workflowhelpers.NewTestSuiteSetup(testConfig)
	testSetup.Setup()
})

var _ = AfterSuite(func() {
	if testSetup != nil {
		testSetup.Teardown()
	}
	if session != nil {
		Eventually(session.Interrupt()).Should(Exit())
	}
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
