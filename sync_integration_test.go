package sync_integration_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/copilot/api"
	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry-incubator/cf-test-helpers/generator"
	"github.com/cloudfoundry-incubator/cf-test-helpers/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Syncing", func() {
	Describe("Reconciling state between cc and diego", func() {
		Describe("LRP Syncing", func() {
			It("restarts processes missing from diego", func() {
				appName := generator.PrefixedRandomName("SITS", "APP")
				Expect(cf.Cf("push", appName, "--no-start", "-d", testConfig.GetAppsDomain(), "-p", "fixtures/dora", "-b", "ruby_buildpack").Wait(Timeout)).To(Exit(0))
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
				Expect(cf.Cf("push", appName, "--no-start", "-d", testConfig.GetAppsDomain(), "-p", "fixtures/dora", "-b", "ruby_buildpack").Wait(Timeout)).To(Exit(0))
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
				Expect(cf.Cf("push", appName, "--no-start", "-d", testConfig.GetAppsDomain(), "-p", "fixtures/dora", "-b", "ruby_buildpack").Wait(Timeout)).To(Exit(0))
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

				if !runRouteTests {
					Eventually(func() string {
						return helpers.CurlAppRoot(testConfig, appName)
					}, Timeout).Should(ContainSubstring("404"))
				} else {
					Eventually(func() (int, error) {
						resp, err := http.Get(fmt.Sprintf("http://%s.%s", appName, testConfig.GetAppsDomain()))
						return resp.StatusCode, err
					}, Timeout).Should(Equal(http.StatusNotFound))
				}

				Expect(bbsClient.DesireLRP(logger, &desiredLRP)).To(Succeed())

				Eventually(func() error {
					_, err := bbsClient.DesiredLRPByProcessGuid(logger, desiredLRP.ProcessGuid)
					return err
				}, Timeout).Should(Equal(models.ErrResourceNotFound))
			})
		})
	})

	Describe("Reconciling state between cc and copilot", func() {
		BeforeEach(func() {
			if !runRouteTests {
				Skip("skipping routing tests")
			}
		})

		Describe("Route syncing", func() {
			It("Adds missing routes to copilot", func() {
				appName := generator.PrefixedRandomName("SITS", "APP")
				Expect(cf.Cf("push", appName, "--no-start", "-d", testConfig.GetAppsDomain(), "-p", "fixtures/dora", "-b", "ruby_buildpack", "--hostname", appName).Wait(Timeout)).To(Exit(0))
				Expect(cf.Cf("start", appName).Wait(PushTimeout)).To(Exit(0))

				Eventually(func() string {
					return helpers.CurlAppRoot(testConfig, appName)
				}, Timeout).Should(ContainSubstring("Hi, I'm Dora!"))

				routeGuid := GetRouteGuid(appName)
				Expect(routeGuid).NotTo(BeEmpty())

				_, err := copilotClient.DeleteRoute(context.Background(), &api.DeleteRouteRequest{
					Guid: routeGuid,
				})
				Expect(err).NotTo(HaveOccurred())

				desiredRoute := fmt.Sprintf("%s.%s", strings.ToLower(appName), testConfig.GetAppsDomain())
				Eventually(func() string {
					response, err := copilotClient.ListCfRoutes(context.Background(), &api.ListCfRoutesRequest{})
					Expect(err).NotTo(HaveOccurred())
					return response.Routes[routeGuid]
				}, Timeout).Should(Equal(desiredRoute))
			})

			It("Removes extraneous routes from copilot", func() {
				extraneousRouteGuid := generator.PrefixedRandomName("SITS", "GUID")
				_, err := copilotClient.UpsertRoute(context.Background(), &api.UpsertRouteRequest{
					Route: &api.Route{
						Guid: extraneousRouteGuid,
						Host: "some-host.example.com",
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() string {
					response, err := copilotClient.ListCfRoutes(context.Background(), &api.ListCfRoutesRequest{})
					Expect(err).NotTo(HaveOccurred())
					return response.Routes[extraneousRouteGuid]
				}, Timeout).Should(BeEmpty())
			})
		})

		Describe("RouteMappings syncing", func() {
			It("Adds missing route mappings to copilot", func() {
				appName := generator.PrefixedRandomName("SITS", "APP")
				Expect(cf.Cf("push", appName, "--no-start", "-d", testConfig.GetAppsDomain(), "-p", "fixtures/dora", "-b", "ruby_buildpack").Wait(Timeout)).To(Exit(0))
				Expect(cf.Cf("start", appName).Wait(PushTimeout)).To(Exit(0))

				Eventually(func() string {
					return helpers.CurlAppRoot(testConfig, appName)
				}, Timeout).Should(ContainSubstring("Hi, I'm Dora!"))

				appGuid := GetAppGuid(appName)
				routeGuid := GetRouteGuid(appName)

				routeMapping := &api.RouteMapping{
					RouteGuid:       routeGuid,
					CapiProcessGuid: appGuid,
				}
				_, err := copilotClient.UnmapRoute(context.Background(), &api.UnmapRouteRequest{
					RouteMapping: routeMapping,
				})
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() *api.RouteMapping {
					response, err := copilotClient.ListCfRouteMappings(context.Background(), &api.ListCfRouteMappingsRequest{})
					Expect(err).NotTo(HaveOccurred())
					return response.RouteMappings[fmt.Sprintf("%s-%s", routeGuid, appGuid)]
				}, Timeout).Should(Equal(routeMapping))
			})

			It("Removes extraneous route mappings from copilot", func() {
				extraneousRouteGuid := generator.PrefixedRandomName("SITS", "GUID")
				extraneousAppGuid := generator.PrefixedRandomName("SITS", "GUID")
				routeMapping := &api.RouteMapping{
					RouteGuid:       extraneousRouteGuid,
					CapiProcessGuid: extraneousAppGuid,
				}
				_, err := copilotClient.MapRoute(context.Background(), &api.MapRouteRequest{
					RouteMapping: routeMapping,
				})
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() *api.RouteMapping {
					response, err := copilotClient.ListCfRouteMappings(context.Background(), &api.ListCfRouteMappingsRequest{})
					Expect(err).NotTo(HaveOccurred())
					return response.RouteMappings[fmt.Sprintf("%s-%s", extraneousRouteGuid, extraneousAppGuid)]
				}, Timeout).Should(BeNil())
			})

			Describe("CAPIDiegoProcessAssociation syncing", func() {
				It("Adds missing CAPI Diego Process Associations to copilot", func() {
					appName := generator.PrefixedRandomName("SITS", "APP")
					Expect(cf.Cf("push", appName, "--no-start", "-d", testConfig.GetAppsDomain(), "-p", "fixtures/dora", "-b", "ruby_buildpack").Wait(Timeout)).To(Exit(0))
					Expect(cf.Cf("start", appName).Wait(PushTimeout)).To(Exit(0))

					Eventually(func() string {
						return helpers.CurlAppRoot(testConfig, appName)
					}, Timeout).Should(ContainSubstring("Hi, I'm Dora!"))

					guid := cf.Cf("app", appName, "--guid").Wait(Timeout).Out.Contents()
					appGuid := strings.TrimSpace(string(guid))
					Expect(appGuid).NotTo(BeEmpty())

					appInfo := cf.Cf("curl", fmt.Sprintf("/v2/apps/%s", appGuid)).Wait(Timeout).Out.Contents()
					Expect(appInfo).NotTo(BeEmpty())

					type appResponse struct {
						Metadata struct {
							Guid string
						}
						Entity struct {
							Version string
						}
					}
					a := &appResponse{}

					json.Unmarshal(appInfo, a)
					capiProcessVersion := a.Entity.Version
					Expect(capiProcessVersion).NotTo(BeEmpty())

					diegoProcessGuids := &api.DiegoProcessGuids{
						DiegoProcessGuids: []string{fmt.Sprintf("%s-%s", appGuid, capiProcessVersion)},
					}

					_, err := copilotClient.DeleteCapiDiegoProcessAssociation(context.Background(), &api.DeleteCapiDiegoProcessAssociationRequest{
						CapiProcessGuid: appGuid,
					})
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() *api.DiegoProcessGuids {
						response, err := copilotClient.ListCapiDiegoProcessAssociations(context.Background(), &api.ListCapiDiegoProcessAssociationsRequest{})
						Expect(err).NotTo(HaveOccurred())
						return response.CapiDiegoProcessAssociations[appGuid]
					}, Timeout).Should(Equal(diegoProcessGuids))
				})

				It("Removes extraneous CAPI Diego Process Associations from copilot", func() {
					extraneousAppVersion := generator.PrefixedRandomName("SITS", "V")
					extraneousAppGuid := generator.PrefixedRandomName("SITS", "GUID")
					capiDiegoProcessAssociation := &api.CapiDiegoProcessAssociation{
						CapiProcessGuid:   extraneousAppGuid,
						DiegoProcessGuids: []string{fmt.Sprintf("%s-%s", extraneousAppGuid, extraneousAppVersion)},
					}
					_, err := copilotClient.UpsertCapiDiegoProcessAssociation(context.Background(), &api.UpsertCapiDiegoProcessAssociationRequest{
						CapiDiegoProcessAssociation: capiDiegoProcessAssociation,
					})
					Expect(err).NotTo(HaveOccurred())
					Eventually(func() *api.DiegoProcessGuids {
						response, err := copilotClient.ListCapiDiegoProcessAssociations(context.Background(), &api.ListCapiDiegoProcessAssociationsRequest{})
						Expect(err).NotTo(HaveOccurred())
						return response.CapiDiegoProcessAssociations[extraneousAppGuid]
					}, Timeout).Should(BeNil())
				})
			})
		})
	})
})
