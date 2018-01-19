package check_test

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/fakes"

	. "github.com/concourse/s3-resource/check"
	"github.com/aws/aws-sdk-go/service/s3"
	"time"
)

var _ = Describe("Check Command", func() {
	Describe("running the command", func() {
		var (
			tmpPath string
			request CheckRequest

			s3client *fakes.FakeS3Client
			command  *CheckCommand
		)

		BeforeEach(func() {
			var err error
			tmpPath, err = ioutil.TempDir("", "check_command")
			Ω(err).ShouldNot(HaveOccurred())

			request = CheckRequest{
				Source: s3resource.Source{
					Bucket: "bucket-name",
				},
			}

			s3client = &fakes.FakeS3Client{}
			command = NewCheckCommand(s3client)

			var path1 = "files/abc-0.0.1.tgz"
			var path2 = "files/abc-2.33.333.tgz"
			var path3 = "files/abc-2.4.3.tgz"
			var path4 = "files/abc-3.53.1493664d.tgz"
			var path5 = "files/abc-3.53.b487a80f.tgz"

			var now = time.Now()
			var yesterday = time.Now().AddDate(0,0,-1)

			s3client.GetBucketContentsReturns(map[string]*s3.Object{
				path1: {
					Key: &path1,
					LastModified: &now,
				},
				path2: {
					Key: &path2,
					LastModified: &now,
				},
				path3: {
					Key: &path3,
					LastModified: &now,
				},
				path4: {
					Key: &path4,
					LastModified: &now,
				},
				path5: {
					Key: &path5,
					LastModified: &yesterday,
				},
			}, nil)
		})

		AfterEach(func() {
			err := os.RemoveAll(tmpPath)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when there is no previous version", func() {
			It("includes the latest version only", func() {
				request.Version.Path = ""
				request.Source.Regexp = "files/abc-(.*)(\\.\\w{8}).tgz"

				response, err := command.Run(request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response).Should(HaveLen(1))
				Ω(response).Should(ConsistOf(
					s3resource.Version{
						Path: "files/abc-3.53.1493664d.tgz",
					},
				))
			})

			Context("when the regexp does not match anything", func() {
				It("does not explode", func() {
					request.Source.Regexp = "no-files/missing-(.*).tgz"
					response, err := command.Run(request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(HaveLen(0))
				})
			})

			Context("when the regex does not match the previous version", func() {
				It("returns the latest version that matches the regex", func() {
					request.Version.Path = "files/abc-0.0.1.tgz"
					request.Source.Regexp = `files/abc-(2\.33.*).tgz`
					response, err := command.Run(request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(HaveLen(1))
					Expect(response).To(ConsistOf(s3resource.Version{Path: "files/abc-2.33.333.tgz"}))
				})
			})
		})

		Context("when there is a previous version", func() {
			Context("when using regex that matches the provided version", func() {
				It("includes all versions from the previous one and the current one", func() {
					request.Version.Path = "files/abc-2.4.3.tgz"
					request.Source.Regexp = "files/abc-(.*).tgz"

					response, err := command.Run(request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(HaveLen(4))
					Ω(response).Should(ConsistOf(
						s3resource.Version{
							Path: "files/abc-2.4.3.tgz",
						},
						s3resource.Version{
							Path: "files/abc-2.33.333.tgz",
						},
						s3resource.Version{
							Path: "files/abc-3.53.1493664d.tgz",
						},
						s3resource.Version{
							Path: "files/abc-3.53.b487a80f.tgz",
						},
					))
				})
			})

			Context("when using versioned file", func() {
				BeforeEach(func() {
					s3client.BucketFileVersionsReturns([]string{
						"file-version-3",
						"file-version-2",
						"file-version-1",
					}, nil)
				})

				It("includes all versions from the previous one and the current one", func() {
					request.Version.VersionID = "file-version-2"
					request.Source.VersionedFile = "files/(.*).tgz"

					response, err := command.Run(request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(HaveLen(2))
					Ω(response).Should(ConsistOf(
						s3resource.Version{
							VersionID: "file-version-2",
						},
						s3resource.Version{
							VersionID: "file-version-3",
						},
					))
				})
			})
		})
	})
})

