package workload_tests

import (
	"bytes"
	"context"
	"fmt"
	"time"

	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"

	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	managementClient ctrlclient.Client
	workloadClient   *kubernetes.Clientset
)

func execCommandInPod(clientset *kubernetes.Clientset, config *rest.Config, namespace, podName string, command []string) (string, error) {
	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Command: command,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, metav1.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", err
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return stderr.String(), err
	}

	return stdout.String(), nil
}

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

// testNetworkConnectivity runs "nc -z -w 5 <targetIP> 80" inside the source pod to test network connectivity.
func testNetworkConnectivity(clientset *kubernetes.Clientset, config *rest.Config, sourcePod *v1.Pod, targetIP string) error {
	cmd := []string{"nc", "-z", "-w", "5", targetIP, "80"}
	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(sourcePod.Name).
		Namespace(sourcePod.Namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Command: cmd,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, metav1.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create SPDY executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	// Log the output for debugging
	fmt.Printf("Netcat Output: %s\n", stdout.String())
	fmt.Printf("Netcat Error: %s\n", stderr.String())

	if err != nil {
		return fmt.Errorf("network test failed: %s, %w", stderr.String(), err)
	}

	return nil
}

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
	createdPod, err := workloadClient.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	Eventually(func() corev1.PodPhase {
		p, _ := workloadClient.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		return p.Status.Phase
	}, 2*time.Minute, 5*time.Second).Should(Equal(corev1.PodRunning))
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
