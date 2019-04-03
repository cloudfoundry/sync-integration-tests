package helpers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

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
	Eventually(session).Should(Exit(0))

	var sidecarData struct {
		Guid string `json:"guid"`
	}
	err = json.Unmarshal(session.Out.Contents(), &sidecarData)
	Expect(err).NotTo(HaveOccurred())
	return sidecarData.Guid
}

func GetAppGuid(appName string) string {
	cfApp := cf.Cf("app", appName, "--guid")
	Eventually(cfApp).Should(Exit(0))

	appGuid := strings.TrimSpace(string(cfApp.Out.Contents()))
	Expect(appGuid).NotTo(Equal(""))
	return appGuid
}
