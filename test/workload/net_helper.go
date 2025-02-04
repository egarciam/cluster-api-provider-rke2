package workload_tests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func execPing(config *rest.Config /*clientset *kubernetes.Clientset,*/, namespace, podName, targetIP string) error {
	// config, err := rest.InClusterConfig()
	// if err != nil {
	// 	return fmt.Errorf("failed to get in-cluster config: %w", err)
	// }

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"ping", "-c", "4", targetIP},
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, metav1.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create SPDY executor: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	return nil
}

// ExecToPodThroughAPI -Exec command in Pod
func ExecToPodThroughAPI(config *rest.Config, clientPod, serverPod *corev1.Pod, stdin io.Reader) error {
	cmd := []string{"ping", "-c", "4", serverPod.Status.PodIP}
	// config, err := GetClientConfig()
	// if err != nil {
	// 	return "", "", err
	// }

	// clientset, err := GetClientsetFromConfig(config)
	// if err != nil {
	// 	return "", "", err
	// }
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(clientPod.Name).
		Namespace(clientPod.Namespace).
		SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("error adding to scheme: %v", err)
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command: cmd, //strings.Fields(command),
		//Container: containerName,
		Stdin:  stdin != nil,
		Stdout: true,
		Stderr: true,
		TTY:    false,
	}, parameterCodec)

	if debug {
		fmt.Println("Request URL:", req.URL().String())
	}

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("error while creating Executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return fmt.Errorf("error in Stream: %v", err)
	}
	fmt.Println("Output: ", stdout.String())

	return nil
}
