package capi_experimental

import (
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/cloudfoundry/cf-acceptance-tests/cats_suite_helpers"
	"github.com/cloudfoundry/cf-acceptance-tests/helpers/random_name"
	. "github.com/cloudfoundry/cf-acceptance-tests/helpers/v3_helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = CapiExperimentalDescribe("labels", func() {

	var (
		appName   string
		appGuid   string
		spaceName string
		spaceGuid string
		token     string
	)

	BeforeEach(func() {
		appName = random_name.CATSRandomName("APP")
		spaceName = TestSetup.RegularUserContext().Space
		spaceGuid = GetSpaceGuidFromName(spaceName)
		appGuid = CreateApp(appName, spaceGuid, `{"foo":"bar"}`)
		token = GetAuthToken()
	})

	AfterEach(func() {
		FetchRecentLogs(appGuid, token, Config)
		DeleteApp(appGuid)
	})

	It("can set labels", func() {
		labels_body := "{\"metadata\":{\"labels\":{\"cat\": \"calico\", \"dog\": \"chihuahua\"}}}"
		session := cf.Cf("curl", fmt.Sprintf("/v3/apps/%s", appGuid), "-X", "PATCH", "-d", labels_body).Wait()

		var app struct {
			Metadata struct {
				Labels struct {
					ExpectedLabelCat string `json:"cat"`
					ExpectedLabelDog string `json:"dog"`
				} `json:"labels"`
			} `json:"metadata"`
		}

		bytes := session.Out.Contents()
		json.Unmarshal(bytes, &app)

		Expect(app.Metadata.Labels.ExpectedLabelCat).To(Equal("calico"))
		Expect(app.Metadata.Labels.ExpectedLabelDog).To(Equal("chihuahua"))
		Expect(session).To(Exit(0))
	})

})
