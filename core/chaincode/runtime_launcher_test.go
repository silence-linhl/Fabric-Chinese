
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
/*
版权所有IBM公司。保留所有权利。

SPDX许可证标识符：Apache-2.0
**/


package chaincode_test

import (
	"time"

	"github.com/hyperledger/fabric/common/metrics/metricsfakes"
	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/chaincode/fake"
	"github.com/hyperledger/fabric/core/chaincode/mock"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var _ = Describe("RuntimeLauncher", func() {
	var (
		fakePackageProvider *mock.PackageProvider
		fakeRuntime         *mock.Runtime
		fakeRegistry        *fake.LaunchRegistry
		launchState         *chaincode.LaunchState
		fakeLaunchDuration  *metricsfakes.Histogram
		fakeLaunchFailures  *metricsfakes.Counter
		fakeLaunchTimeouts  *metricsfakes.Counter

		ccci *ccprovider.ChaincodeContainerInfo

		runtimeLauncher *chaincode.RuntimeLauncher
	)

	BeforeEach(func() {
		launchState = chaincode.NewLaunchState()
		fakeRegistry = &fake.LaunchRegistry{}
		fakeRegistry.LaunchingReturns(launchState, false)

		fakeRuntime = &mock.Runtime{}
		fakeRuntime.StartStub = func(*ccprovider.ChaincodeContainerInfo, []byte) error {
			launchState.Notify(nil)
			return nil
		}

		fakePackageProvider = &mock.PackageProvider{}
		fakePackageProvider.GetChaincodeCodePackageReturns([]byte("code-package"), nil)

		fakeLaunchDuration = &metricsfakes.Histogram{}
		fakeLaunchDuration.WithReturns(fakeLaunchDuration)
		fakeLaunchFailures = &metricsfakes.Counter{}
		fakeLaunchFailures.WithReturns(fakeLaunchFailures)
		fakeLaunchTimeouts = &metricsfakes.Counter{}
		fakeLaunchTimeouts.WithReturns(fakeLaunchTimeouts)

		launchMetrics := &chaincode.LaunchMetrics{
			LaunchDuration: fakeLaunchDuration,
			LaunchFailures: fakeLaunchFailures,
			LaunchTimeouts: fakeLaunchTimeouts,
		}
		ccci = &ccprovider.ChaincodeContainerInfo{
			Name:          "chaincode-name",
			Path:          "chaincode-path",
			Version:       "chaincode-version",
			ContainerType: "chaincode-container-type",
			Type:          "chaincode-type",
		}

		runtimeLauncher = &chaincode.RuntimeLauncher{
			Runtime:         fakeRuntime,
			Registry:        fakeRegistry,
			PackageProvider: fakePackageProvider,
			StartupTimeout:  5 * time.Second,
			Metrics:         launchMetrics,
		}
	})

	It("registers the chaincode as launching", func() {
		err := runtimeLauncher.Launch(ccci)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeRegistry.LaunchingCallCount()).To(Equal(1))
		cname := fakeRegistry.LaunchingArgsForCall(0)
		Expect(cname).To(Equal("chaincode-name:chaincode-version"))
	})

	It("starts the runtime for the chaincode", func() {
		err := runtimeLauncher.Launch(ccci)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeRuntime.StartCallCount()).To(Equal(1))
		ccciArg, codePackage := fakeRuntime.StartArgsForCall(0)
		Expect(ccciArg).To(Equal(ccci))
		Expect(codePackage).To(Equal([]byte("code-package")))
	})

	It("waits for the launch to complete", func() {
		fakeRuntime.StartReturns(nil)

		errCh := make(chan error, 1)
		go func() { errCh <- runtimeLauncher.Launch(ccci) }()

		Consistently(errCh).ShouldNot(Receive())
		launchState.Notify(nil)
		Eventually(errCh).Should(Receive(BeNil()))
	})

	It("does not deregister the chaincode", func() {
		err := runtimeLauncher.Launch(ccci)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeRegistry.DeregisterCallCount()).To(Equal(0))
	})

	It("records launch duration", func() {
		err := runtimeLauncher.Launch(ccci)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeLaunchDuration.WithCallCount()).To(Equal(1))
		labelValues := fakeLaunchDuration.WithArgsForCall(0)
		Expect(labelValues).To(Equal([]string{
			"chaincode", "chaincode-name:chaincode-version",
			"success", "true",
		}))
		Expect(fakeLaunchDuration.ObserveArgsForCall(0)).NotTo(BeZero())
		Expect(fakeLaunchDuration.ObserveArgsForCall(0)).To(BeNumerically("<", 1.0))
	})

	Context("when starting the runtime fails", func() {
		BeforeEach(func() {
			fakeRuntime.StartReturns(errors.New("banana"))
		})

		It("returns a wrapped error", func() {
			err := runtimeLauncher.Launch(ccci)
			Expect(err).To(MatchError("error starting container: banana"))
		})

		It("notifies the LaunchState", func() {
			runtimeLauncher.Launch(ccci)
			Eventually(launchState.Done()).Should(BeClosed())
			Expect(launchState.Err()).To(MatchError("error starting container: banana"))
		})

		It("records chaincode launch failures", func() {
			runtimeLauncher.Launch(ccci)
			Expect(fakeLaunchFailures.WithCallCount()).To(Equal(1))
			labelValues := fakeLaunchFailures.WithArgsForCall(0)
			Expect(labelValues).To(Equal([]string{
				"chaincode", "chaincode-name:chaincode-version",
			}))
			Expect(fakeLaunchFailures.AddCallCount()).To(Equal(1))
			Expect(fakeLaunchFailures.AddArgsForCall(0)).To(BeNumerically("~", 1.0))
		})

		It("stops the runtime", func() {
			runtimeLauncher.Launch(ccci)

			Expect(fakeRuntime.StopCallCount()).To(Equal(1))
			ccciArg := fakeRuntime.StopArgsForCall(0)
			Expect(ccciArg).To(Equal(ccci))
		})

		It("deregisters the chaincode", func() {
			runtimeLauncher.Launch(ccci)

			Expect(fakeRegistry.DeregisterCallCount()).To(Equal(1))
			cname := fakeRegistry.DeregisterArgsForCall(0)
			Expect(cname).To(Equal("chaincode-name:chaincode-version"))
		})
	})

	Context("when handler registration fails", func() {
		BeforeEach(func() {
			fakeRuntime.StartStub = func(*ccprovider.ChaincodeContainerInfo, []byte) error {
				launchState.Notify(errors.New("papaya"))
				return nil
			}
		})

		It("returns an error", func() {
			err := runtimeLauncher.Launch(ccci)
			Expect(err).To(MatchError("chaincode registration failed: papaya"))
		})

		It("stops the runtime", func() {
			runtimeLauncher.Launch(ccci)

			Expect(fakeRuntime.StopCallCount()).To(Equal(1))
			ccciArg := fakeRuntime.StopArgsForCall(0)
			Expect(ccciArg).To(Equal(ccci))
		})

		It("deregisters the chaincode", func() {
			runtimeLauncher.Launch(ccci)

			Expect(fakeRegistry.DeregisterCallCount()).To(Equal(1))
			cname := fakeRegistry.DeregisterArgsForCall(0)
			Expect(cname).To(Equal("chaincode-name:chaincode-version"))
		})
	})

	Context("when the runtime startup times out", func() {
		BeforeEach(func() {
			fakeRuntime.StartReturns(nil)
			runtimeLauncher.StartupTimeout = 250 * time.Millisecond
		})

		It("returns a meaningful error", func() {
			err := runtimeLauncher.Launch(ccci)
			Expect(err).To(MatchError("timeout expired while starting chaincode chaincode-name:chaincode-version for transaction"))
		})

		It("notifies the LaunchState", func() {
			runtimeLauncher.Launch(ccci)
			Eventually(launchState.Done()).Should(BeClosed())
			Expect(launchState.Err()).To(MatchError("timeout expired while starting chaincode chaincode-name:chaincode-version for transaction"))
		})

		It("records chaincode launch timeouts", func() {
			runtimeLauncher.Launch(ccci)
			Expect(fakeLaunchTimeouts.WithCallCount()).To(Equal(1))
			labelValues := fakeLaunchTimeouts.WithArgsForCall(0)
			Expect(labelValues).To(Equal([]string{
				"chaincode", "chaincode-name:chaincode-version",
			}))
			Expect(fakeLaunchTimeouts.AddCallCount()).To(Equal(1))
			Expect(fakeLaunchTimeouts.AddArgsForCall(0)).To(BeNumerically("~", 1.0))
		})

		It("stops the runtime", func() {
			runtimeLauncher.Launch(ccci)

			Expect(fakeRuntime.StopCallCount()).To(Equal(1))
			ccciArg := fakeRuntime.StopArgsForCall(0)
			Expect(ccciArg).To(Equal(ccci))
		})

		It("deregisters the chaincode", func() {
			runtimeLauncher.Launch(ccci)

			Expect(fakeRegistry.DeregisterCallCount()).To(Equal(1))
			cname := fakeRegistry.DeregisterArgsForCall(0)
			Expect(cname).To(Equal("chaincode-name:chaincode-version"))
		})
	})

	Context("when the registry indicates the chaincode has already been started", func() {
		BeforeEach(func() {
			fakeRegistry.LaunchingReturns(launchState, true)
		})

		It("does not start the runtime for the chaincode", func() {
			launchState.Notify(nil)

			err := runtimeLauncher.Launch(ccci)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeRuntime.StartCallCount()).To(Equal(0))
		})

		It("waits for the launch to complete", func() {
			fakeRuntime.StartReturns(nil)

			errCh := make(chan error, 1)
			go func() { errCh <- runtimeLauncher.Launch(ccci) }()

			Consistently(errCh).ShouldNot(Receive())
			launchState.Notify(nil)
			Eventually(errCh).Should(Receive(BeNil()))
		})

		Context("when the launch fails", func() {
			BeforeEach(func() {
				launchState.Notify(errors.New("gooey-guac"))
			})

			It("does not deregister the chaincode", func() {
				err := runtimeLauncher.Launch(ccci)
				Expect(err).To(MatchError("chaincode registration failed: gooey-guac"))
				Expect(fakeRegistry.DeregisterCallCount()).To(Equal(0))
			})

			It("does not stop the runtime", func() {
				err := runtimeLauncher.Launch(ccci)
				Expect(err).To(MatchError("chaincode registration failed: gooey-guac"))
				Expect(fakeRuntime.StopCallCount()).To(Equal(0))
			})
		})
	})

	Context("when stopping the runtime fails", func() {
		BeforeEach(func() {
			fakeRuntime.StartReturns(errors.New("whirled-peas"))
			fakeRuntime.StopReturns(errors.New("applesauce"))
		})

		It("preserves the initial error", func() {
			err := runtimeLauncher.Launch(ccci)
			Expect(err).To(MatchError("error starting container: whirled-peas"))
			Expect(fakeRuntime.StopCallCount()).To(Equal(1))
		})
	})
})
