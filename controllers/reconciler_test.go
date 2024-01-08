package controllers

import (
	"context"
	"go/build"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	gstruct "github.com/onsi/gomega/gstruct"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machinev1 "github.com/openshift/client-go/machine/clientset/versioned"
	. "github.com/project-codeflare/codeflare-common/support"
	"github.com/project-codeflare/instascale/pkg/config"
	mcadv1beta1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	mc "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/clientset/versioned"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	cfg       *rest.Config
	k8sClient client.Client // You'll be using this client in your tests.
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
	err       error
)

func startEnvTest(t *testing.T) *envtest.Environment {
	test := With(t)
	//specify testEnv configuration
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
			filepath.Join(build.Default.GOROOT, "pkg", "mod", "github.com", "project-codeflare", "multi-cluster-app-dispatcher@v1.38.1", "config", "crd", "bases"),
			filepath.Join(build.Default.GOROOT, "pkg", "mod", "github.com", "openshift", "api@v0.0.0-20220411210816-c3bb724c282a", "machine", "v1beta1"),
		},
	}
	cfg, err = testEnv.Start()
	test.Expect(err).NotTo(HaveOccurred())

	//	defer teardownTestEnv(testEnv)
	establishClient(t)
	return testEnv
}

func establishClient(t *testing.T) {
	test := With(t)
	err = mcadv1beta1.AddToScheme(scheme.Scheme)
	err = machinev1beta1.AddToScheme(scheme.Scheme)
	test.Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	test.Expect(err).NotTo(HaveOccurred())
	test.Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	test.Expect(err).ToNot(HaveOccurred())

	instaScaleController := &AppWrapperReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		Config: config.InstaScaleConfiguration{
			MachineSetsStrategy: "reuse",
			MaxScaleoutAllowed:  5,
		},
	}
	err = instaScaleController.SetupWithManager(context.Background(), k8sManager)
	test.Expect(err).ToNot(HaveOccurred())

	go func() {

		err = k8sManager.Start(ctrl.SetupSignalHandler())
		test.Expect(err).ToNot(HaveOccurred())
	}()

	time.Sleep(5 * time.Second)
}

func teardownTestEnv(testEnv *envtest.Environment) {
	if err := testEnv.Stop(); err != nil {
		klog.Errorf("Error stopping test Environment : %v\n", err)
	}
}

func TestReconciler(t *testing.T) {
	testEnv = startEnvTest(t)
	defer teardownTestEnv(testEnv)

	machineClient, err := machinev1.NewForConfig(cfg)
	With(t).Expect(err).ToNot(HaveOccurred())

	b, err := os.ReadFile("qqq.txt")
	With(t).Expect(err).ToNot(HaveOccurred())

	decode := scheme.Codecs.UniversalDeserializer().Decode
	ms, _, err := decode(b, nil, nil)
	With(t).Expect(err).ToNot(HaveOccurred())
	msa := ms.(*machinev1beta1.MachineSet)

	ms, err = machineClient.MachineV1beta1().MachineSets("default").Create(With(t).Ctx(), msa, metav1.CreateOptions{})
	With(t).Expect(err).ToNot(HaveOccurred())

	app := &mcadv1beta1.AppWrapper{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mnist",
			Labels: map[string]string{
				"orderedinstance": "test.instance1",
			},
		},
		Spec: mcadv1beta1.AppWrapperSpec{
			AggrResources: mcadv1beta1.AppWrapperResourceList{
				GenericItems: []mcadv1beta1.AppWrapperGenericResource{
					{
						DesiredAvailable: 1,
						CustomPodResources: []mcadv1beta1.CustomPodResourceTemplate{
							{
								Replicas: 1,
								Requests: apiv1.ResourceList{
									apiv1.ResourceCPU:    resource.MustParse("250m"),
									apiv1.ResourceMemory: resource.MustParse("1G"),
								},
								Limits: apiv1.ResourceList{
									apiv1.ResourceCPU:    resource.MustParse("1"),
									apiv1.ResourceMemory: resource.MustParse("2G"),
								},
							},
						},
					},
				},
			},
		},
	}

	mcadClient, err := mc.NewForConfig(cfg)
	With(t).Expect(err).ToNot(HaveOccurred())

	_, err = mcadClient.WorkloadV1beta1().AppWrappers("default").Create(With(t).Ctx(), app, metav1.CreateOptions{})
	With(t).Expect(err).ToNot(HaveOccurred())

	time.Sleep(10 * time.Second)

	machineset, err := machineClient.MachineV1beta1().MachineSets("default").Get(With(t).Ctx(), "test-instascale", metav1.GetOptions{})
	With(t).Expect(err).ToNot(HaveOccurred())
	With(t).Expect(machineset.Spec.Replicas).To(gstruct.PointTo(Equal(int32(1))))
}
