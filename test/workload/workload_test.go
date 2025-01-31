package workload_tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout     = 10 * time.Minute // Maximum time to wait for a condition
	interval    = 10 * time.Second // Interval between checks
	clusterName = "rke"
	namespace   = "bm-osp	"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	ctrl.SetLogger(klog.Background())

	RunSpecs(t, "caprke2-e2e-new")
}

var _ = BeforeSuite(func() {
	// Load kubeconfig for the management cluster
	cfg, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	Expect(err).NotTo(HaveOccurred(), "Failed to load management cluster kubeconfig")

	// Create the management client
	managementClient, err = ctrlclient.New(cfg, ctrlclient.Options{})
	Expect(err).NotTo(HaveOccurred(), "Failed to create management cluster client")

	// Optionally, initialize the workload client
	workloadClient, err = getWorkloadKubeconfig(managementClient, clusterName, namespace)
	Expect(err).NotTo(HaveOccurred(), "Failed to create workload cluster client")
})

// Test 1: Cluster API Endpoint Validation
var _ = Describe("Cluster API Operations", func() {
	It("Should provision cluster with correct API endpoint", func() {
		var cluster clusterv1.Cluster
		Eventually(func() error {
			return managementClient.Get(
				context.TODO(),
				ctrlclient.ObjectKey{Name: clusterName, Namespace: namespace},
				&cluster,
			)
		}, timeout, interval).Should(Succeed())

		// Verify API server endpoint
		Eventually(func() error {
			// cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
			// if err != nil {
			// 	return err
			// // }
			// client, err := kubernetes.NewForConfig(	workloadClientet
			// )
			// if err != nil {
			// 	return err
			// }
			_, err := workloadClient.Discovery().ServerVersion()
			return err
		}, 5*time.Minute, 10*time.Second).Should(Succeed())

		// Verify certificate validity
		Eventually(func() error {
			secret := &corev1.Secret{}
			err := managementClient.Get(
				context.TODO(),
				ctrlclient.ObjectKey{
					Name:      fmt.Sprintf("%s-apiserver-cert", clusterName),
					Namespace: namespace,
				},
				secret,
			)
			if err != nil {
				return err
			}
			// Add certificate validation logic here
			return nil
		}, timeout, interval).Should(Succeed())
	})
})

// // Test 4: Network Policy Enforcement
// var _ = Describe("Network Connectivity", func() {
// 	It("Should enforce pod-to-pod network connectivity", func() {
// 		// Create test namespaces
// 		testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "network-test"}}
// 		Expect(workloadClient.CoreV1().Namespaces().Create(context.TODO(), testNS, metav1.CreateOptions{})).To(Succeed())
// 		defer workloadClient.CoreV1().Namespaces().Delete(context.TODO(), "network-test", metav1.DeleteOptions{})

// 		// Deploy test pods
// 		serverPod := createNetworkTestPod("server", "network-test")
// 		clientPod := createNetworkTestPod("client", "default")

// 		// Create restrictive network policy
// 		policy := &networkingv1.NetworkPolicy{
// 			ObjectMeta: metav1.ObjectMeta{Name: "deny-cross-ns", Namespace: "network-test"},
// 			Spec: networkingv1.NetworkPolicySpec{
// 				PodSelector: metav1.LabelSelector{},
// 				Ingress: []networkingv1.NetworkPolicyIngressRule{{
// 					From: []networkingv1.NetworkPolicyPeer{{
// 						NamespaceSelector: &metav1.LabelSelector{
// 							MatchLabels: map[string]string{"kubernetes.io/metadata.name": "network-test"},
// 						},
// 					}},
// 				}},
// 			},
// 		}
// 		_, err := workloadClient.NetworkingV1().NetworkPolicies("network-test").Create(context.TODO(), policy, metav1.CreateOptions{})
// 		Expect(err).NotTo(HaveOccurred())

// 		//Verify connectivity
// 	// 	Eventually(func() error {
// 	// 		return testNetworkConnectivity(workloadClient, kubeConfig, serverPod /*sourcePod*/, "10.0.0.1")
// 	// 	}, 2*time.Minute, 10*time.Second).ShouldNot(HaveOccurred(), "Network connectivity test failed")

// 	// 	// Eventually(func() error {
// 	// 	// 	return testNetworkConnectivity(clientPod, serverPod.Status.PodIP)
// 	// 	// }, 2*time.Minute, 5*time.Second).Should(HaveOccurred())

// 	// 	// Cleanup policy and verify restored connectivity
// 	// 	Expect(workloadClient.NetworkingV1().NetworkPolicies("network-test").Delete(context.TODO(), "deny-cross-ns", metav1.DeleteOptions{})).To(Succeed())
// 	// 	Eventually(func() error {
// 	// 		return testNetworkConnectivity(clientPod, serverPod)
// 	// 	}, 2*time.Minute, 5*time.Second).Should(Succeed())
// 	// })
// })
