package workload_tests

import (
	"bytes"
	"context"
	"fmt"
	"time"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	managementClient ctrlclient.Client
	workloadClient   *kubernetes.Clientset
	debug            = false
)

// func execCommandInPod(clientset *kubernetes.Clientset, config *rest.Config, namespace, podName string, command []string) (string, error) {
// 	cmd := []string{"ping", "-c", "2"}
// 	command = cmd
// 	req := clientset.CoreV1().RESTClient().
// 		Post().
// 		Resource("pods").
// 		Name(podName).
// 		Namespace(namespace).
// 		SubResource("exec")
// 	scheme := runtime.NewScheme()
// 	parameterCodec := runtime.NewParameterCodec(scheme)
// 	req.VersionedParams(&v1.PodExecOptions{
// 		Command: command,
// 		Stdin:   false,
// 		Stdout:  true,
// 		Stderr:  true,
// 		TTY:     false,
// 	}, parameterCodec)

// 	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
// 	if err != nil {
// 		return "", err
// 	}

// 	var stdout, stderr bytes.Buffer
// 	err = exec.Stream(remotecommand.StreamOptions{
// 		Stdout: &stdout,
// 		Stderr: &stderr,
// 	})
// 	if err != nil {
// 		return stderr.String(), err
// 	}

// 	return stdout.String(), nil
// }

func getWorkloadKubeconfig(managementClient ctrlclient.Client, clusterName, namespace string) (*kubernetes.Clientset, error) {
	// Fetch the kubeconfig secret from the management cluster
	secret := &corev1.Secret{}
	err := managementClient.Get(
		context.TODO(),
		ctrlclient.ObjectKey{
			Name:      fmt.Sprintf("%s-kubeconfig", clusterName),
			Namespace: namespace,
		},
		secret,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch workload cluster kubeconfig: %v", err)
	}

	// Load the kubeconfig from the secret
	config, err := clientcmd.RESTConfigFromKubeConfig(secret.Data["value"])
	if err != nil {
		return nil, fmt.Errorf("failed to parse workload cluster kubeconfig: %v", err)
	}

	// Create a Kubernetes client for the workload cluster
	return kubernetes.NewForConfig(config)
}

func createNamespaceIfNotExists(clientset *kubernetes.Clientset, namespaceName string) error {
	// Define the namespace object
	testNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}

	// Attempt to create the namespace
	_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), testNS, metav1.CreateOptions{})
	if err != nil {
		// If the namespace already exists, ignore the error
		if errors.IsAlreadyExists(err) {
			fmt.Printf("Namespace %q already exists, skipping creation\n", namespaceName)
			return nil
		}
		// For any other error, return it
		return fmt.Errorf("failed to create namespace %q: %v", namespaceName, err)
	}

	fmt.Printf("Namespace %q created successfully\n", namespaceName)
	return nil
}

func getWorkloadRestKubeconfig(managementClient ctrlclient.Client, clusterName, namespace string) (*rest.Config, error) {
	// Fetch the kubeconfig secret from the management cluster
	secret := &corev1.Secret{}
	err := managementClient.Get(
		context.TODO(),
		ctrlclient.ObjectKey{
			Name:      fmt.Sprintf("%s-kubeconfig", clusterName),
			Namespace: namespace,
		},
		secret,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch workload cluster kubeconfig: %v", err)
	}

	// Load the kubeconfig from the secret
	config, err := clientcmd.RESTConfigFromKubeConfig(secret.Data["value"])
	if err != nil {
		return nil, fmt.Errorf("failed to parse workload cluster kubeconfig: %v", err)
	}

	// Create a Kubernetes client for the workload cluster
	return config, nil
}

// testNetworkConnectivity runs "nc -z -w 5 <targetIP> 80" inside the source pod to test network connectivity.
func testNetworkConnectivity(config *rest.Config, sourcePod *v1.Pod, targetIP string) error {
	// cmd := []string{"nc", "-v", "-z", "-w", "5", targetIP, "80"}
	cmd := []string{"ping", "-c", "2", targetIP}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(sourcePod.Name).
		Namespace(sourcePod.Namespace).
		SubResource("exec")
	scheme := runtime.NewScheme()
	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&v1.PodExecOptions{
		Command: cmd,
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create SPDY executor: %w", err)
	}

	//var stdout, stderr bytes.Buffer
	// stdin := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
		// Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})

	// Log the output for debugging
	fmt.Printf("Executed command: %v\n", cmd)
	fmt.Printf("Command Output: %s\n", stdout.String())
	fmt.Printf("Error Output: %s\n", stderr.String())

	if err != nil {
		return fmt.Errorf("network test failed: %s, %w", stderr.String(), err)
	}

	return nil
}

func testNetworkConnectivity2(config *rest.Config, sourcePod *v1.Pod, targetIP string) error {
	// cmd := []string{"nc", "-v", "-z", "-w", "5", targetIP, "80"}
	cmd := []string{"ping", "-c", "2", targetIP}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(sourcePod.Name).
		Namespace(sourcePod.Namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Command: cmd,
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, metav1.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create SPDY executor: %w", err)
	}

	//var stdout, stderr bytes.Buffer
	stdin := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})

	// Log the output for debugging
	fmt.Printf("Executed command: %v\n", cmd)
	fmt.Printf("Command Output: %s\n", stdout.String())
	fmt.Printf("Error Output: %s\n", stderr.String())

	if err != nil {
		return fmt.Errorf("network test failed: %s, %w", stderr.String(), err)
	}

	return nil
}

// func testNetworkConnectivity(config *rest.Config, sourcePod *corev1.Pod, targetIP string) error {

// 	// cmd := []string{"nc", "-v", "-z", "-w", "5", targetIP, "80"}
// 	// clientset, err := kubernetes.NewForConfig(config)
// 	// if err != nil {
// 	// 	return err
// 	// }
// 	_, err := workloadClient.CoreV1().Pods(sourcePod.Namespace).Exec(
// 		context.TODO(),
// 		sourcePod.Name,
// 		&corev1.PodExecOptions{
// 			Command: []string{"nc", "-z", "-w", "5", targetIP, "80"},
// 		},
// 	)
// 	return err
// }

// Helper functions
func createNetworkTestPod(name, namespace string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "test",
				Image:   "alpine:latest",
				Command: []string{"tail", "-f", "/dev/null"},
			}},
		},
	}
	// createdPod, err := workloadClient.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	// err = func() error {
	createdPod, err := workloadClient.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil // Ignore "already exists" error
	}
	// 	return err // Return other errors
	// // }()
	// if apierrors.IsAlreadyExists(err) {
	// 	err = nil // Ignore "already exists" error
	// }
	Expect(err).NotTo(HaveOccurred())
	// Eventually(func() corev1.PodPhase {
	// 	p, _ := workloadClient.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	// 	return p.Status.Phase
	// }, 2*time.Minute, 5*time.Second).Should(Equal(corev1.PodRunning))

	var podIP string
	Eventually(func() bool {
		p, err := workloadClient.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		// Check if the pod is Running and has an IP address
		if p.Status.Phase == corev1.PodRunning && p.Status.PodIP != "" {
			podIP = p.Status.PodIP
			return true
		}
		return false
	}, 2*time.Minute, 5*time.Second).Should(BeTrue(), "Pod did not reach Running state or get an IP address")

	// Update the createdPod object with the IP address
	createdPod.Status.PodIP = podIP

	return createdPod
}

func createDNSTestPod() *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dns-test",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "dns-test",
				Image:   "alpine:latest",
				Command: []string{"tail", "-f", "/dev/null"},
			}},
		},
	}
	createdPod, err := workloadClient.CoreV1().Pods("default").Create(context.TODO(), pod, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	Eventually(func() corev1.PodPhase {
		p, _ := workloadClient.CoreV1().Pods("default").Get(context.TODO(), "dns-test", metav1.GetOptions{})
		return p.Status.Phase
	}, 2*time.Minute, 5*time.Second).Should(Equal(corev1.PodRunning))
	return createdPod
}
