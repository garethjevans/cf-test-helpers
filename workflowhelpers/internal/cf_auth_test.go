package internal_test

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"bytes"

	"github.com/cloudfoundry-incubator/cf-test-helpers/internal"
	"github.com/cloudfoundry-incubator/cf-test-helpers/internal/fakes"
	. "github.com/cloudfoundry-incubator/cf-test-helpers/workflowhelpers/internal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CfAuth", func() {
	var (
		password string

		cmdStarter *fakes.FakeCmdStarter

		redactor          internal.Redactor
		reporterOutput    *bytes.Buffer
		redactingReporter internal.Reporter
	)

	BeforeEach(func() {
		password = "foobar"
		cmdStarter = fakes.NewFakeCmdStarter()
		redactor = internal.NewRedactor(password)
		reporterOutput = bytes.NewBuffer([]byte{})
		redactingReporter = internal.NewRedactingReporter(reporterOutput, redactor)
	})

	It("runs the cf auth command", func() {
		err := CfAuth(cmdStarter, redactingReporter, "user", password, 5*time.Second)

		Expect(err).NotTo(HaveOccurred())
		Expect(cmdStarter.CalledWith[0].Executable).To(Equal("cf"))
		Expect(cmdStarter.CalledWith[0].Args).To(Equal([]string{"auth", "user", "foobar"}))
	})

	It("does not reveal the password", func() {
		err := CfAuth(cmdStarter, redactingReporter, "user", password, 5*time.Second)

		Expect(err).NotTo(HaveOccurred())
		Expect(reporterOutput.String()).To(ContainSubstring("REDACTED"))
		Expect(reporterOutput.String()).NotTo(ContainSubstring("foobar"))
	})

	It("errors if cf auth takes longer than timeout", func() {
		timeout := rand.Intn(10)
		cmdStarter.ToReturn[0].SleepTime = timeout
		cmdStarter.ToReturn[1].SleepTime = timeout

		err := CfAuth(cmdStarter, redactingReporter, "user", password, time.Duration(timeout)*time.Second)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Timed out after %d", timeout)))
	})

	Context("when the starter returns error", func() {
		BeforeEach(func() {
			cmdStarter.ToReturn[0].Err = fmt.Errorf("something went wrong")
		})

		It("errors", func() {
			err := CfAuth(cmdStarter, redactingReporter, "user", password, 5*time.Second)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("something went wrong"))
		})
	})

	Context("when the secret debug environment variable is set", func() {
		BeforeEach(func() {
			os.Setenv(VerboseAuth, "true")
		})

		AfterEach(func() {
			os.Unsetenv(VerboseAuth)
		})

		It("does not reveal the password", func() {
			err := CfAuth(cmdStarter, redactingReporter, "user", password, 5*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(cmdStarter.CalledWith[0].Executable).To(Equal("cf"))
			Expect(cmdStarter.CalledWith[0].Args).To(Equal([]string{"auth", "user", "foobar", "-v"}))
		})
	})

	Context("retries", func() {
		It("does not retry and succeeds when cf auth is successful on the first try", func() {
			err := CfAuth(cmdStarter, redactingReporter, "user", password, 5*time.Second)

			Expect(err).NotTo(HaveOccurred())
			Expect(cmdStarter.TotalCallsToStart).To(Equal(1))
			Expect(cmdStarter.CalledWith[0].Executable).To(Equal("cf"))
			Expect(cmdStarter.CalledWith[0].Args).To(Equal([]string{"auth", "user", password}))
		})

		Context("when the first command fails", func() {
			AfterEach(func() {
				Expect(cmdStarter.TotalCallsToStart).To(Equal(2))
				Expect(cmdStarter.CalledWith[0].Executable).To(Equal("cf"))
				Expect(cmdStarter.CalledWith[0].Args).To(Equal([]string{"auth", "user", password}))
				Expect(cmdStarter.CalledWith[1].Executable).To(Equal("cf"))
				Expect(cmdStarter.CalledWith[1].Args).To(Equal([]string{"auth", "user", password}))
			})

			It("retries once and succeeds when cf auth times out first, and succeeds on the second try", func() {
				cmdStarter.ToReturn[0].SleepTime = 6

				err := CfAuth(cmdStarter, redactingReporter, "user", password, 5*time.Second)

				Expect(err).NotTo(HaveOccurred())
			})

			It("retries once and succeeds when cf auth exists with a non-zero exit code first, and succeeds on the second try", func() {
				cmdStarter.ToReturn[0].ExitCode = 1

				err := CfAuth(cmdStarter, redactingReporter, "user", password, 5*time.Second)

				Expect(err).NotTo(HaveOccurred())
			})

			It("retries once and errors when cf auth times out on the second attempt", func() {
				cmdStarter.ToReturn[0].ExitCode = 1
				cmdStarter.ToReturn[1].SleepTime = 6

				err := CfAuth(cmdStarter, redactingReporter, "user", password, 5*time.Second)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cf auth command timed out"))
			})

			It("retries once and errors when cf auth exits non-zero on the second attempt", func() {
				cmdStarter.ToReturn[0].ExitCode = 1
				cmdStarter.ToReturn[1].ExitCode = 5

				err := CfAuth(cmdStarter, redactingReporter, "user", password, 5*time.Second)

				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("cf auth command exited with 5"))
			})
		})
	})
})
