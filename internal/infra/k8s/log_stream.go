package k8s

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	podWaitTimeout    = 45 * time.Second
	streamOpenTimeout = 45 * time.Second
)

type logStreamOptions struct {
	Namespace     string
	LabelSelector string
}

type logStreamer struct {
	clientset kubernetes.Interface
}

func NewLogStreamer(cs kubernetes.Interface) *logStreamer {
	return &logStreamer{clientset: cs}
}

func (s *logStreamer) Stream(ctx context.Context, opts logStreamOptions) (io.ReadCloser, error) {
	podName, err := s.waitForPod(ctx, opts)
	if err != nil {
		return nil, err
	}
	return s.openLogStream(ctx, opts.Namespace, podName)
}

func (s *logStreamer) EachLine(stream io.ReadCloser, fn func(string) string) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		defer func() { _ = pw.Close() }()
		defer func() { _ = stream.Close() }()
		reader := bufio.NewReader(stream)
		for {
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				if _, writeErr := pw.Write([]byte(fn(line))); writeErr != nil {
					return
				}
			}
			if err == nil {
				continue
			}
			if errors.Is(err, io.EOF) {
				return
			}
			_ = pw.CloseWithError(err)
			return
		}
	}()
	return &eachLineReader{pr: pr, upstream: stream}
}

type eachLineReader struct {
	pr       *io.PipeReader
	upstream io.ReadCloser
}

func (e *eachLineReader) Read(p []byte) (int, error) { return e.pr.Read(p) }

func (e *eachLineReader) Close() error {
	upErr := e.upstream.Close()
	prErr := e.pr.Close()
	if upErr != nil {
		return upErr
	}
	return prErr
}

func (s *logStreamer) waitForPod(ctx context.Context, opts logStreamOptions) (string, error) {
	waitCtx, cancel := context.WithTimeout(ctx, podWaitTimeout)
	defer cancel()
	for {
		select {
		case <-waitCtx.Done():
			return "", fmt.Errorf("timed out waiting for pod (ns=%s selector=%q): %w", opts.Namespace, opts.LabelSelector, waitCtx.Err())
		default:
		}
		pods, err := s.clientset.CoreV1().Pods(opts.Namespace).List(waitCtx, metav1.ListOptions{
			LabelSelector: opts.LabelSelector,
		})
		if err == nil && len(pods.Items) > 0 {
			return pods.Items[0].Name, nil
		}
		time.Sleep(1 * time.Second)
	}
}

func (s *logStreamer) openLogStream(ctx context.Context, namespace, podName string) (io.ReadCloser, error) {
	deadline := time.Now().Add(streamOpenTimeout)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out opening log stream for pod %s/%s", namespace, podName)
		}
		req := s.clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{Follow: true})
		stream, err := req.Stream(ctx)
		if err == nil {
			return stream, nil
		}
		time.Sleep(1 * time.Second)
	}
}
