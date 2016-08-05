package routing

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	. "github.com/cloudfoundry-incubator/cf-routing-test-helpers/helpers"
	"github.com/cloudfoundry-incubator/cf-test-helpers/generator"
	"github.com/cloudfoundry-incubator/cf-test-helpers/helpers"
	"github.com/cloudfoundry-incubator/cf-test-helpers/runner"
	"github.com/cloudfoundry/cf-acceptance-tests/helpers/assets"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const (
	VCAP_ID = "__VCAP_ID__"
)

var _ = Describe("Session Affinity", func() {
	var stickyAsset = assets.NewAssets().HelloRouting

	Context("when an app sets a JSESSIONID cookie", func() {
		var (
			appName         string
			cookieStorePath string
		)
		BeforeEach(func() {
			appName = generator.RandomNameForResource("APP")
			PushApp(appName, stickyAsset, config.RubyBuildpackName, config.AppsDomain, CF_PUSH_TIMEOUT)

			cookieStore, err := ioutil.TempFile("", "cats-sticky-session")
			Expect(err).ToNot(HaveOccurred())
			cookieStorePath = cookieStore.Name()
			cookieStore.Close()
		})

		AfterEach(func() {
			AppReport(appName, DEFAULT_TIMEOUT)

			DeleteApp(appName, DEFAULT_TIMEOUT)

			err := os.Remove(cookieStorePath)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when an app has multiple instances", func() {
			BeforeEach(func() {
				ScaleAppInstances(appName, 3, DEFAULT_TIMEOUT)
			})

			Context("when the client sends VCAP_ID and JSESSION cookies", func() {
				It("routes every request to the same instance", func() {
					var body string

					Eventually(func() string {
						body = curlAppWithCookies(appName, "/", cookieStorePath)
						return body
					}, DEFAULT_TIMEOUT).Should(ContainSubstring(fmt.Sprintf("Hello, %s", appName)))

					index := parseInstanceIndex(body)

					Consistently(func() string {
						return curlAppWithCookies(appName, "/", cookieStorePath)
					}, 3*time.Second).Should(ContainSubstring(fmt.Sprintf("Hello, %s at index: %d", appName, index)))
				})
			})
		})
	})

	Context("when an app does not set a JSESSIONID cookie", func() {
		var (
			helloWorldAsset = assets.NewAssets().HelloRouting

			appName string
		)

		BeforeEach(func() {
			appName = generator.RandomNameForResource("APP")
			PushApp(appName, helloWorldAsset, config.RubyBuildpackName, config.AppsDomain, CF_PUSH_TIMEOUT)
		})

		AfterEach(func() {
			AppReport(appName, DEFAULT_TIMEOUT)
			DeleteApp(appName, DEFAULT_TIMEOUT)
		})

		Context("when an app has multiple instances", func() {
			BeforeEach(func() {
				ScaleAppInstances(appName, 3, CF_PUSH_TIMEOUT)
			})

			Context("when the client does not send VCAP_ID and JSESSION cookies", func() {
				It("routes requests round robin to all instances", func() {
					var body string

					Eventually(func() string {
						body = helpers.CurlAppRoot(appName)
						return body
					}, DEFAULT_TIMEOUT).Should(ContainSubstring(fmt.Sprintf("Hello, %s", appName)))

					indexPre := parseInstanceIndex(body)

					Eventually(func() int {
						body := helpers.CurlAppRoot(appName)
						index := parseInstanceIndex(body)
						return index
					}, DEFAULT_TIMEOUT).ShouldNot(Equal(indexPre))
				})
			})
		})
	})

	Context("when two apps have different context paths", func() {
		var (
			app1Path        = "/app1"
			app2Path        = "/app2"
			app1            string
			app2            string
			hostname        string
			cookieStorePath string
		)

		BeforeEach(func() {
			domain := config.AppsDomain

			app1 = generator.RandomNameForResource("APP")
			PushApp(app1, stickyAsset, config.RubyBuildpackName, config.AppsDomain, CF_PUSH_TIMEOUT)
			app2 = generator.RandomNameForResource("APP")
			PushApp(app2, stickyAsset, config.RubyBuildpackName, config.AppsDomain, CF_PUSH_TIMEOUT)

			ScaleAppInstances(app1, 2, DEFAULT_TIMEOUT)
			ScaleAppInstances(app2, 2, DEFAULT_TIMEOUT)
			hostname = generator.RandomNameForResource("ROUTE")

			MapRouteToApp(app1, domain, hostname, app1Path, DEFAULT_TIMEOUT)
			MapRouteToApp(app2, domain, hostname, app2Path, DEFAULT_TIMEOUT)

			cookieStore, err := ioutil.TempFile("", "cats-sticky-session")
			Expect(err).ToNot(HaveOccurred())
			cookieStorePath = cookieStore.Name()
			cookieStore.Close()
		})

		AfterEach(func() {
			AppReport(app1, DEFAULT_TIMEOUT)
			AppReport(app2, DEFAULT_TIMEOUT)
			DeleteApp(app1, DEFAULT_TIMEOUT)
			DeleteApp(app2, DEFAULT_TIMEOUT)

			err := os.Remove(cookieStorePath)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Sticky session should work", func() {
			var body string

			// Hit the APP1
			Eventually(func() string {
				body = curlAppWithCookies(hostname, app1Path, cookieStorePath)
				return body
			}, DEFAULT_TIMEOUT).Should(ContainSubstring(fmt.Sprintf("Hello, %s", app1)))

			index1 := parseInstanceIndex(body)

			// Hit the APP2
			Eventually(func() string {
				body = curlAppWithCookies(hostname, app2Path, cookieStorePath)
				return body
			}, DEFAULT_TIMEOUT).Should(ContainSubstring(fmt.Sprintf("Hello, %s", app2)))

			index2 := parseInstanceIndex(body)

			// Hit the APP1 again to verify that the session is stick to the right instance.
			Eventually(func() string {
				return curlAppWithCookies(hostname, app1Path, cookieStorePath)
			}, DEFAULT_TIMEOUT).Should(ContainSubstring(fmt.Sprintf("Hello, %s at index: %d", app1, index1)))

			// Hit the APP2 again to verify that the session is stick to the right instance.
			Eventually(func() string {
				return curlAppWithCookies(hostname, app2Path, cookieStorePath)
			}, DEFAULT_TIMEOUT).Should(ContainSubstring(fmt.Sprintf("Hello, %s at index: %d", app2, index2)))
		})
	})

	Context("when one app has a root path and another with a context path", func() {
		var (
			app2Path        = "/app2"
			app1            string
			app2            string
			hostname        string
			cookieStorePath string
		)

		BeforeEach(func() {
			domain := config.AppsDomain

			app1 = generator.RandomNameForResource("APP")
			PushApp(app1, stickyAsset, config.RubyBuildpackName, config.AppsDomain, CF_PUSH_TIMEOUT)
			app2 = generator.RandomNameForResource("APP")
			PushApp(app2, stickyAsset, config.RubyBuildpackName, config.AppsDomain, CF_PUSH_TIMEOUT)

			ScaleAppInstances(app1, 2, DEFAULT_TIMEOUT)
			ScaleAppInstances(app2, 2, DEFAULT_TIMEOUT)
			hostname = app1

			MapRouteToApp(app2, domain, hostname, app2Path, DEFAULT_TIMEOUT)

			cookieStore, err := ioutil.TempFile("", "cats-sticky-session")
			Expect(err).ToNot(HaveOccurred())
			cookieStorePath = cookieStore.Name()
			cookieStore.Close()
		})

		AfterEach(func() {
			AppReport(app1, DEFAULT_TIMEOUT)
			AppReport(app2, DEFAULT_TIMEOUT)

			DeleteApp(app1, DEFAULT_TIMEOUT)
			DeleteApp(app2, DEFAULT_TIMEOUT)

			err := os.Remove(cookieStorePath)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Sticky session should work", func() {
			var body string

			// 1: Hit the APP1: the root app. We can set the cookie of the root path.
			// Path: /
			Eventually(func() string {
				body = curlAppWithCookies(hostname, "/", cookieStorePath)
				return body
			}, DEFAULT_TIMEOUT).Should(ContainSubstring(fmt.Sprintf("Hello, %s", app1)))

			index1 := parseInstanceIndex(body)

			// 2: Hit the APP2. App2 has a path. We can set the cookie of the APP2 path.
			// Path: /app2
			Eventually(func() string {
				body = curlAppWithCookies(hostname, app2Path, cookieStorePath)
				return body
			}, DEFAULT_TIMEOUT).Should(ContainSubstring(fmt.Sprintf("Hello, %s", app2)))

			index2 := parseInstanceIndex(body)

			// To do list:
			// 3. Hit the APP1 (root APP) again, to ensure that the instance ID is
			// stick correctly. Only send the first session ID.
			Eventually(func() string {
				return curlAppWithCookies(hostname, "/", cookieStorePath)
			}, DEFAULT_TIMEOUT).Should(ContainSubstring(fmt.Sprintf("Hello, %s at index: %d", app1, index1)))

			// 4. Hit the APP2 (path APP) again, to ensure that the instance ID is
			// stick correctly. In this case, both the two cookies will be sent to
			// the server. The curl would store them.
			Eventually(func() string {
				return curlAppWithCookies(hostname, app2Path, cookieStorePath)
			}, DEFAULT_TIMEOUT).Should(ContainSubstring(fmt.Sprintf("Hello, %s at index: %d", app2, index2)))
		})
	})
})

func parseInstanceIndex(body string) int {
	strs := strings.SplitN(body, "index: ", -1)
	indexStr := strings.SplitN(strs[len(strs)-1], "!", -1)
	index, err := strconv.ParseInt(indexStr[0], 10, 0)
	Expect(err).ToNot(HaveOccurred())
	return int(index)
}

func curlAppWithCookies(appName, path string, cookieStorePath string) string {
	uri := helpers.AppUri(appName, path)
	curlCmd := runner.Curl(uri, "-b", cookieStorePath, "-c", cookieStorePath).Wait(helpers.CURL_TIMEOUT)
	Expect(curlCmd).To(gexec.Exit(0))
	return string(curlCmd.Out.Contents())
}
