package controllers

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	. "github.com/project-codeflare/codeflare-common/support"
	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	mcadv1beta1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	mc "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/clientset/versioned"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
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

func TestReconciler(t *testing.T) {
	test := With(t)
	//specify testEnv configuration
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}
	cfg, err = testEnv.Start()

	err = mcadv1beta1.AddToScheme(scheme.Scheme)
	test.Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	test.Expect(err).NotTo(HaveOccurred())
	test.Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	test.Expect(err).ToNot(HaveOccurred())

	mcadClient, err := mc.NewForConfig(cfg)

	instaScaleController := &AppWrapperReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		// Config: cfg.InstaScale.InstaScaleConfiguration,
	}
	instaScaleController.SetupWithManager(context.Background(), k8sManager)

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	time.Sleep(5 * time.Second)
	nsSpec := &apiv1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	k8sClient.Create(test.Ctx(), nsSpec)
	app := &arbv1.AppWrapper{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"orderedinstance": "test.instance1_test.instance2",
			},
			Name: "default",
		},
		Spec: arbv1.AppWrapperSpec{
			AggrResources: arbv1.AppWrapperResourceList{
				GenericItems: []arbv1.AppWrapperGenericResource{
					{
						CustomPodResources: []arbv1.CustomPodResourceTemplate{
							{
								Replicas: 1,
							},
							{
								Replicas: 2,
							},
						},
					},
				},
			},
		},
	}
	_, err = mcadClient.WorkloadV1beta1().AppWrappers("default").Create(test.Ctx(), app, metav1.CreateOptions{})

	time.Sleep(3 * time.Second)
	test.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Stop()
	test.Expect(err).NotTo(HaveOccurred())
}
