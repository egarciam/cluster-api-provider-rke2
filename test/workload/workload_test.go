package workload_tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/klog/v2"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout     = 10 * time.Second // Maximum time to wait for a condition
	interval    = 10 * time.Second // Interval between checks
	clusterName = "bm-osp"
	namespace   = "capi-test-infra-docker"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	ctrl.SetLogger(klog.Background())

	RunSpecs(t, "caprke2-e2e-new")
}

var _ = BeforeSuite(func() {
	scheme := runtime.NewScheme()

	err := clientgoscheme.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred(), "Failed to add core k8s types to scheme")
	// Add the Cluster API types to the scheme
	err = clusterv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred(), "Failed to add Cluster API types to scheme")

	// Load kubeconfig for the management cluster
	cfg, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	Expect(err).NotTo(HaveOccurred(), "Failed to load management cluster kubeconfig")

	// Create the management client
	managementClient, err = ctrlclient.New(cfg, ctrlclient.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred(), "Failed to create management cluster client")

	// Optionally, initialize the workload client
	workloadClient, err = getWorkloadKubeconfig(managementClient, clusterName, namespace)
	Expect(err).NotTo(HaveOccurred(), "Failed to create workload cluster client")
})

var _ = AfterSuite(func() {
	//Hay que limpiar
	Expect(true)
})

// Test 1: Cluster API Endpoint Validation
// var _ = Describe("Cluster API Operations", func() {
// 	It("Should provision cluster with correct API endpoint", func() {
// 		var cluster clusterv1.Cluster
// 		Eventually(func() error {
// 			return managementClient.Get(
// 				context.TODO(),
// 				ctrlclient.ObjectKey{Name: clusterName, Namespace: namespace},
// 				&cluster,
// 			)
// 		}, timeout, interval).Should(Succeed())

// 		// Verify API server endpoint
// 		Eventually(func() error {
// 			// cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
// 			// if err != nil {
// 			// 	return err
// 			// // }
// 			// client, err := kubernetes.NewForConfig(	workloadClientet
// 			// )
// 			// if err != nil {
// 			// 	return err
// 			// }
// 			_, err := workloadClient.Discovery().ServerVersion()
// 			return err
// 		}, 5*time.Minute, 10*time.Second).ShouldNot(Succeed())

// 		// Verify certificate validity
// 		// Eventually(func() error {
// 		// 	secret := &corev1.Secret{}
// 		// 	err := managementClient.Get(
// 		// 		context.TODO(),
// 		// 		ctrlclient.ObjectKey{
// 		// 			Name:      fmt.Sprintf("%s-apiserver-cert", clusterName),
// 		// 			Namespace: namespace,
// 		// 		},
// 		// 		secret,
// 		// 	)
// 		// 	if err != nil {
// 		// 		return err
// 		// 	}
// 		// 	// Add certificate validation logic here
// 		// 	return nil
// 		// }, timeout, interval).Should(Succeed())
// 	})
// })

// Test 4: Network Policy Enforcement
var _ = Describe("Network Connectivity", func() {
	It("Should enforce pod-to-pod network connectivity", func() {
		// Create test namespaces
		testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "network-test"}}

		// Ensure workloadClient is initialized
		if workloadClient == nil {
			panic("workloadClient is nil")
		}

		// Delete namespace if it already exists
		err := workloadClient.CoreV1().Namespaces().Delete(context.TODO(), "network-test", metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			panic(fmt.Sprintf("Failed to delete namespace: %v", err))
		}

		// Create namespace and handle errors
		err = func() error {
			_, err := workloadClient.CoreV1().Namespaces().Create(context.TODO(), testNS, metav1.CreateOptions{})
			if apierrors.IsAlreadyExists(err) {
				return nil // Ignore "already exists" error
			}
			return err // Return other errors
		}()
		Expect(err).To(Succeed())
		defer workloadClient.CoreV1().Namespaces().Delete(context.TODO(), "network-test", metav1.DeleteOptions{})
		// Expect(workloadClient.CoreV1().Namespaces().Create(context.TODO(), testNS, metav1.CreateOptions{})).To(Succeed())

		// Deploy test pods
		serverPod := createNetworkTestPod("server", "network-test")
		By(fmt.Sprintf("Creating [%s]client pod in [%s] namespace with ip: [%s]", serverPod.Name, serverPod.Namespace, serverPod.Status.PodIP))
		defer workloadClient.CoreV1().Pods("network-test").Delete(context.TODO(), "server", metav1.DeleteOptions{})
		clientPod := createNetworkTestPod("client", "default")
		By(fmt.Sprintf("And creating [%s]client pod in [%s] namespace with ip: [%s]", clientPod.Name, clientPod.Namespace, clientPod.Status.PodIP))
		defer workloadClient.CoreV1().Pods("default").Delete(context.TODO(), "client", metav1.DeleteOptions{})
		By("And creating net pol that blocks ingress to 'network-test' ns")
		// Create restrictive network policy
		policy := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "deny-cross-ns", Namespace: "network-test"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From: []networkingv1.NetworkPolicyPeer{{
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"kubernetes.io/metadata.name": "network-test"},
						},
					}},
				}},
			},
		}
		_, err = workloadClient.NetworkingV1().NetworkPolicies("network-test").Create(context.TODO(), policy, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		kubeconfig, err := getWorkloadRestKubeconfig(managementClient, clusterName, namespace)
		Expect(err).NotTo(HaveOccurred())

		//Verify connectivity
		// Eventually(func() error {
		// 	return testNetworkConnectivity(kubeconfig, clientPod, serverPod.Status.PodIP /*sourcePod*/)
		// }, 15*time.Second, 5*time.Second).ShouldNot(HaveOccurred(), "Network connectivity test failed")
		By("pinging is not allowed")
		//Test ping
		Eventually(func() error {
			//return execPing(kubeconfig, clientPod.Namespace, clientPod.Name, serverPod.Status.PodIP)
			return ExecToPodThroughAPI(kubeconfig, clientPod, serverPod, nil)

			//}, 15*time.Second, 5*time.Second).ShouldNot(HaveOccurred(), "Network ExecPing connectivity test failed")
		}, 60*time.Second, 5*time.Second).ShouldNot(Succeed(), "Network ExecPing connectivity test failed")

		// Eventually(func() error {
		// 	return testNetworkConnectivity(clientPod, kubeconfig, serverPod /*.Status.PodIP*/)
		// }, 2*time.Minute, 5*time.Second).Should(HaveOccurred())

		By("deleting netpol")
		// Cleanup policy and verify restored connectivity
		Expect(workloadClient.NetworkingV1().NetworkPolicies("network-test").Delete(context.TODO(), "deny-cross-ns", metav1.DeleteOptions{})).To(Succeed())
		// Eventually(func() error {
		// 	return testNetworkConnectivity(clientPod, serverPod)
		// }, 2*time.Minute, 5*time.Second).Should(Succeed())
		//Test ping again
		By("ping is allowed again")
		Eventually(func() error {
			//return execPing(kubeconfig, clientPod.Namespace, clientPod.Name, serverPod.Status.PodIP)
			return ExecToPodThroughAPI(kubeconfig, clientPod, serverPod, nil)

			//}, 15*time.Second, 5*time.Second).ShouldNot(HaveOccurred(), "Network ExecPing connectivity test failed")
		}, 60*time.Second, 5*time.Second).Should(Succeed(), "Network ExecPing connectivity test succeded")
	})
})
