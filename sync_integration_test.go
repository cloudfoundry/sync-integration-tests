package sync_integration_test

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/bbs/models"
	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry-incubator/cf-test-helpers/generator"
	"github.com/cloudfoundry-incubator/cf-test-helpers/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	"os/exec"
)

var _ = Describe("Syncing", func() {
	Describe("Reconciling state between cc and diego", func() {
		Describe("LRP Syncing", func() {
			It("restarts processes missing from diego", func() {
				appName := generator.PrefixedRandomName("SITS", "APP")
				Expect(cf.Cf("push", appName, "--no-start", "-p", "fixtures/dora", "-b", "ruby_buildpack").Wait(Timeout)).To(Exit(0))
				EnableDiego(appName)
				Expect(cf.Cf("start", appName).Wait(PushTimeout)).To(Exit(0))

				Eventually(func() string {
					return helpers.CurlAppRoot(testConfig, appName)
				}, Timeout).Should(ContainSubstring("Hi, I'm Dora!"))

				desiredLRPs, err := bbsClient.DesiredLRPs(logger, models.DesiredLRPFilter{})
				Expect(err).NotTo(HaveOccurred())

				guid := cf.Cf("app", appName, "--guid").Wait(Timeout).Out.Contents()
				appGuid := strings.TrimSpace(string(guid))

				processGuid := ""

				for _, desiredLRP := range desiredLRPs {
					if strings.Contains(desiredLRP.ProcessGuid, appGuid) {
						processGuid = desiredLRP.ProcessGuid
						break
					}
				}

				Expect(processGuid).NotTo(BeEmpty())

				Expect(bbsClient.RemoveDesiredLRP(logger, processGuid)).To(Succeed())

				Eventually(func() error {
					_, err := bbsClient.DesiredLRPByProcessGuid(logger, processGuid)
					return err
				}, Timeout).ShouldNot(HaveOccurred())
			})

			It("refreshes stale processes", func() {
				appName := generator.PrefixedRandomName("SITS", "APP")
				Expect(cf.Cf("push", appName, "--no-start", "-p", "fixtures/dora", "-b", "ruby_buildpack").Wait(Timeout)).To(Exit(0))
				EnableDiego(appName)
				Expect(cf.Cf("start", appName).Wait(PushTimeout)).To(Exit(0))

				Eventually(func() string {
					return helpers.CurlAppRoot(testConfig, appName)
				}, Timeout).Should(ContainSubstring("Hi, I'm Dora!"))

				desiredLRPs, err := bbsClient.DesiredLRPs(logger, models.DesiredLRPFilter{})
				Expect(err).NotTo(HaveOccurred())

				guid := cf.Cf("app", appName, "--guid").Wait(Timeout).Out.Contents()
				appGuid := strings.TrimSpace(string(guid))

				processGuid := ""

				for _, desiredLRP := range desiredLRPs {
					if strings.Contains(desiredLRP.ProcessGuid, appGuid) {
						processGuid = desiredLRP.ProcessGuid
						break
					}
				}

				Expect(processGuid).NotTo(BeEmpty())

				instances := int32(2)
				bogusAnnotation := "bogus"
				desiredLRPUpdate := models.DesiredLRPUpdate{
					Instances:  &instances,
					Annotation: &bogusAnnotation,
				}
				Expect(bbsClient.UpdateDesiredLRP(logger, processGuid, &desiredLRPUpdate)).To(Succeed())

				Eventually(func() int32 {
					desiredLRP, err := bbsClient.DesiredLRPByProcessGuid(logger, processGuid)
					Expect(err).NotTo(HaveOccurred())
					return desiredLRP.Instances
				}, Timeout).Should(Equal(int32(1)))
			})

			It("cancels processes that should not be running according to CC", func() {
				appName := generator.PrefixedRandomName("SITS", "APP")
				Expect(cf.Cf("push", appName, "--no-start", "-p", "fixtures/dora", "-b", "ruby_buildpack").Wait(Timeout)).To(Exit(0))
				EnableDiego(appName)
				Expect(cf.Cf("start", appName).Wait(PushTimeout)).To(Exit(0))

				Eventually(func() string {
					return helpers.CurlAppRoot(testConfig, appName)
				}, Timeout).Should(ContainSubstring("Hi, I'm Dora!"))

				desiredLRPs, err := bbsClient.DesiredLRPs(logger, models.DesiredLRPFilter{})
				Expect(err).NotTo(HaveOccurred())

				guid := cf.Cf("app", appName, "--guid").Wait(Timeout).Out.Contents()
				appGuid := strings.TrimSpace(string(guid))

				var desiredLRP models.DesiredLRP

				for _, lrp := range desiredLRPs {
					if strings.Contains(lrp.ProcessGuid, appGuid) {
						desiredLRP = *lrp
						break
					}
				}

				Expect(desiredLRP).NotTo(BeNil())

				Expect(cf.Cf("delete", "-f", appName).Wait(Timeout)).To(Exit(0))

				Eventually(func() string {
					return helpers.CurlAppRoot(testConfig, appName)
				}, Timeout).Should(ContainSubstring("404"))

				Expect(bbsClient.DesireLRP(logger, &desiredLRP)).To(Succeed())

				Eventually(func() error {
					_, err := bbsClient.DesiredLRPByProcessGuid(logger, desiredLRP.ProcessGuid)
					return err
				}, Timeout).Should(Equal(models.ErrResourceNotFound))
			})
		})

		Describe("Task Syncing", func() {
			It("reports tasks unknown to Diego as FAILED", func() {
				appName := generator.PrefixedRandomName("SITS", "APP")
				Expect(cf.Cf("push", appName, "--no-start", "-p", "fixtures/dora", "-b", "ruby_buildpack").Wait(Timeout)).To(Exit(0))
				EnableDiego(appName)
				Expect(cf.Cf("start", appName).Wait(PushTimeout)).To(Exit(0))

				Eventually(func() string {
					return helpers.CurlAppRoot(testConfig, appName)
				}, Timeout, PollingInterval).Should(ContainSubstring("Hi, I'm Dora!"))

				appGuid := GetAppGuid(appName)

				dropletGuid := GetDropletGuidForApp(appGuid)

				// Insert a running task record directly into the CCDB.
				taskName := generator.PrefixedRandomName("SITS", "TASK")
				taskGuid := generator.PrefixedRandomName("SITS", "GUID")

				insertTaskRuby := fmt.Sprintf(`
				  TaskModel.create(
				    guid: "%s",
				    name: "%s",
				    app_guid: "%s",
				    droplet_guid: "%s",
				    command: "read",
				    state: "RUNNING",
				    sequence_id: 1,
				    memory_in_mb: 256,
				    disk_in_mb: 1024
				  )`, taskGuid, taskName, appGuid, dropletGuid)
				insertTaskRuby = strings.Replace(insertTaskRuby, "\n", " ", -1)

				command := exec.Command(testConfig.BoshBinary,
					"-d",
					testConfig.BoshDeploymentName,
					"ssh",
					testConfig.APIInstance,
					"-c",
					fmt.Sprintf("echo '%s' | /var/vcap/jobs/cloud_controller_ng/bin/console", insertTaskRuby),
				)
				session, err := Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, Timeout).Should(Exit(0))

				Eventually(func() *Session {
					return cf.Cf("tasks", appName).Wait(Timeout)
				}, Timeout, PollingInterval).Should(Say(fmt.Sprintf("%s +FAILED", taskName)))
			})
		})
	})
})
