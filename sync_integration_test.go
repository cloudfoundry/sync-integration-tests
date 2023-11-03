package sync_integration_test

import (
	"fmt"
	"net/http"
	"strings"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/sync-integration-tests/helpers"
	"github.com/cloudfoundry/cf-test-helpers/v2/cf"
	"github.com/cloudfoundry/cf-test-helpers/v2/generator"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Syncing", func() {
	Describe("Reconciling state between cc and diego", func() {
		Describe("LRP Syncing", func() {
			const BbsAppsDomain = "cf-apps"

			AfterEach(func() {
				domains, err := bbsClient.Domains(logger, "someTraceIDString")
				Expect(err).To(BeNil())
				Expect(domains).To(ContainElement(BbsAppsDomain), "Freshness bump failed!")
			})

			It("restarts processes missing from diego", func() {
				appName := generator.PrefixedRandomName("SITS", "APP")
				Expect(cf.Cf("push", appName, "--no-start", "-p", "fixtures/dora", "-b", "ruby_buildpack").Wait(Timeout)).To(Exit(0))
				Expect(cf.Cf("start", appName).Wait(PushTimeout)).To(Exit(0))

				Eventually(func() string {
					body, _ := CurlAppRoot(appName)
					return body
				}, Timeout).Should(ContainSubstring("Hi, I'm Dora!"))

				processGuid := GetProcessGuid(appName)
				DeleteProcessGuidFromDiego(processGuid)

				Eventually(func() error {
					_, err := bbsClient.DesiredLRPByProcessGuid(logger, "someTraceIDString", processGuid)
					return err
				}, Timeout).ShouldNot(HaveOccurred())
			})

			It("refreshes stale processes", func() {
				appName := generator.PrefixedRandomName("SITS", "APP")
				Expect(cf.Cf("push", appName, "--no-start", "-p", "fixtures/dora", "-b", "ruby_buildpack").Wait(Timeout)).To(Exit(0))
				Expect(cf.Cf("start", appName).Wait(PushTimeout)).To(Exit(0))

				Eventually(func() string {
					body, _ := CurlAppRoot(appName)
					return body
				}, Timeout).Should(ContainSubstring("Hi, I'm Dora!"))

				desiredLRPs, err := bbsClient.DesiredLRPs(logger, "someTraceIDString", models.DesiredLRPFilter{})
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
					OptionalInstances: &models.DesiredLRPUpdate_Instances{
						Instances: instances,
					},
					OptionalAnnotation: &models.DesiredLRPUpdate_Annotation{
						Annotation: bogusAnnotation,
					},
				}
				Expect(bbsClient.UpdateDesiredLRP(logger, "someTraceIDString", processGuid, &desiredLRPUpdate)).To(Succeed())

				Eventually(func() int32 {
					desiredLRP, err := bbsClient.DesiredLRPByProcessGuid(logger, "someTraceIDString", processGuid)
					Expect(err).NotTo(HaveOccurred())
					return desiredLRP.Instances
				}, Timeout).Should(Equal(int32(1)))
			})

			It("cancels processes that should not be running according to CC", func() {
				appName := generator.PrefixedRandomName("SITS", "APP")
				Expect(cf.Cf("push", appName, "--no-start", "-p", "fixtures/dora", "-b", "ruby_buildpack").Wait(Timeout)).To(Exit(0))
				Expect(cf.Cf("start", appName).Wait(PushTimeout)).To(Exit(0))

				Eventually(func() string {
					body, _ := CurlAppRoot(appName)
					return body
				}, Timeout).Should(ContainSubstring("Hi, I'm Dora!"))

				desiredLRPs, err := bbsClient.DesiredLRPs(logger, "someTraceIDString", models.DesiredLRPFilter{})
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

				Eventually(func() int {
					_, statusCode := CurlAppRoot(appName)
					return statusCode
				}, Timeout, "1s").Should(Equal(http.StatusNotFound))

				Expect(bbsClient.DesireLRP(logger, "someTraceIDString", &desiredLRP)).To(Succeed())

				Eventually(func() error {
					_, err := bbsClient.DesiredLRPByProcessGuid(logger, "someTraceIDString", desiredLRP.ProcessGuid)
					return err
				}, Timeout).Should(Equal(models.ErrResourceNotFound))
			})

			Describe("revisions", func() {
				BeforeEach(func() {
					if !testConfig.RunRevisionsTests {
						Skip("skipping revisions tests")
					}
				})

				It("prefers latest_revision to current app state when restarting missing processes", func() {
					appName := generator.PrefixedRandomName("SITS", "APP")
					By("staging OG dora to get a droplet we can set later")
					Expect(cf.Cf("push", appName,
						"-p", "fixtures/dora",
						"-b", "ruby_buildpack",
					).Wait(PushTimeout)).To(Exit(0))
					appGuid := GetAppGuid(appName)
					ogDoraGuid := GetDropletGuidForApp(appGuid)

					Expect(cf.Cf("set-env", appName, "FOO", "og_bar").Wait(ShortTimeout)).To(Exit(0))
					revisionsEnablePath := fmt.Sprintf("/v3/apps/%s/features/revisions", appGuid)
					Expect(cf.Cf("curl", revisionsEnablePath, "-X", "PATCH", "-d", `{"enabled": true}`).Wait(ShortTimeout)).To(Exit(0))

					Expect(cf.Cf("restart", appName).Wait(PushTimeout)).To(Exit(0))

					Eventually(func() string {
						body, _ := CurlAppRoot(appName)
						return body
					}, Timeout).Should(ContainSubstring("Hi, I'm Dora!"))

					Eventually(func() string {
						body, _ := CurlApp(appName, "/env/FOO")
						return body
					}, Timeout).Should(ContainSubstring("og_bar"))

					By("deploying other dora to be the last intentionally started revision")
					webProcessGuid := GetCCProcessGuidsForType(appGuid, "web")[0]
					newCommand := fmt.Sprintf(`{"command": "%s"}`, "TEST_VAR=real bundle exec rackup config.ru -p $PORT")
					cf.Cf("curl", fmt.Sprintf("/v3/processes/%s", webProcessGuid), "-X", "PATCH", "-d", newCommand)
					Expect(cf.Cf("set-env", appName, "FOO", "ng_bar").Wait(ShortTimeout)).To(Exit(0))
					Expect(cf.Cf("push", appName, "-p", "fixtures/other-dora", "-b", "ruby_buildpack").Wait(PushTimeout)).To(Exit(0))

					Eventually(func() string {
						body, _ := CurlAppRoot(appName)
						return body
					}, Timeout).Should(ContainSubstring("Hi, I'm Other Dora!"))

					Eventually(func() string {
						body, _ := CurlApp(appName, "/env/FOO")
						return body
					}, Timeout).Should(ContainSubstring("ng_bar"))

					Eventually(func() string {
						body, _ := CurlApp(appName, "/env/TEST_VAR")
						return body
					}, Timeout).Should(ContainSubstring("real"))

					By("setting droplet back to OG dora and the env var back to og_bar")
					webProcessGuid = GetCCProcessGuidsForType(appGuid, "web")[0]
					newCommand = fmt.Sprintf(`{"command": "%s"}`, "TEST_VAR=fake bundle exec rackup config.ru -p $PORT")
					cf.Cf("curl", fmt.Sprintf("/v3/processes/%s", webProcessGuid), "-X", "PATCH", "-d", newCommand)
					Expect(cf.Cf("set-env", appName, "FOO", "og_bar").Wait(ShortTimeout)).To(Exit(0))
					Expect(cf.Cf("set-droplet", appName, ogDoraGuid).Wait(ShortTimeout)).To(Exit(0))

					processGuid := GetProcessGuid(appName)
					DeleteProcessGuidFromDiego(processGuid)

					Eventually(func() error {
						_, err := bbsClient.DesiredLRPByProcessGuid(logger, "someTraceIDString", processGuid)
						return err
					}, PushTimeout).ShouldNot(HaveOccurred())

					By("when everything has converged, we should be running the last intentionally started revision")
					Eventually(func() string {
						body, _ := CurlAppRoot(appName)
						return body
					}, Timeout).Should(ContainSubstring("Hi, I'm Other Dora!"))

					Eventually(func() string {
						body, _ := CurlApp(appName, "/env/FOO")
						return body
					}, Timeout).Should(ContainSubstring("ng_bar"))

					Eventually(func() string {
						body, _ := CurlApp(appName, "/env/TEST_VAR")
						return body
					}, Timeout).Should(ContainSubstring("real"))
				})
			})

			Describe("sidecars", func() {
				BeforeEach(func() {
					if !testConfig.RunSidecarTests {
						Skip("skipping sidecar tests")
					}
				})
				Context("when a lrp is deleted", func() {
					It("restores its sidecar when it's restarted", func() {
						appName := generator.PrefixedRandomName("SITS", "APP")

						Expect(cf.Cf("push", appName, "--no-start", "-p", "fixtures/dora", "-b", "ruby_buildpack").Wait(Timeout)).To(Exit(0))
						appGUID := helpers.GetAppGuid(appName)
						helpers.CreateSidecar("my_sidecar", []string{"web"}, "sleep 100000", appGUID)
						Expect(cf.Cf("start", appName).Wait(PushTimeout)).To(Exit(0))

						Eventually(func() string {
							body, _ := CurlAppRoot(appName)
							return body
						}, Timeout).Should(ContainSubstring("Hi, I'm Dora!"))

						By("verify the sidecar is running")
						session := cf.Cf("ssh", appName, "-c", "ps aux")
						fmt.Println(session.Out.Contents())
						Expect(cf.Cf("ssh", appName, "-c", "ps aux | grep sleep | grep -v grep").Wait(PushTimeout)).To(Exit(0))

						processGuid := GetProcessGuid(appName)
						DeleteProcessGuidFromDiego(processGuid)

						Eventually(func() error {
							_, err := bbsClient.DesiredLRPByProcessGuid(logger, "someTraceIDString", processGuid)
							return err
						}, PushTimeout).ShouldNot(HaveOccurred())

						By("Verify the LRP is running again")
						Eventually(func() string {
							body, _ := CurlAppRoot(appName)
							return body
						}, Timeout).Should(ContainSubstring("Hi, I'm Dora!"))

						By("verify the sidecar is running after the LRP restarted")
						Expect(cf.Cf("ssh", appName, "-c", "ps aux | grep sleep | grep -v grep").Wait(PushTimeout)).To(Exit(0))
					})
				})
			})
		})
	})
})
