package k8s

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	podWaitTimeout    = 45 * time.Second
	streamOpenTimeout = 10 * time.Minute
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

func (s *logStreamer) StreamLines(ctx context.Context, opts logStreamOptions) (<-chan string, <-chan error, error) {
	raw, err := s.Stream(ctx, opts)
	if err != nil {
		return nil, nil, err
	}

	lines := make(chan string)
	errs := make(chan error, 1)

	context.AfterFunc(ctx, func() { _ = raw.Close() })

	go func() {
		defer func() { _ = raw.Close() }()
		defer close(lines)
		defer close(errs)

		scanner := bufio.NewScanner(raw)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case lines <- scanner.Text():
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			errs <- err
		}
	}()

	return lines, errs, nil
}

func (s *logStreamer) waitForPod(ctx context.Context, opts logStreamOptions) (string, error) {
	waitCtx, cancel := context.WithTimeout(ctx, podWaitTimeout)
	defer cancel()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		pods, err := s.clientset.CoreV1().Pods(opts.Namespace).List(waitCtx, metav1.ListOptions{
			LabelSelector: opts.LabelSelector,
		})
		if err == nil && len(pods.Items) > 0 {
			return pods.Items[0].Name, nil
		}
		select {
		case <-waitCtx.Done():
			return "", fmt.Errorf("timed out waiting for pod (ns=%s selector=%q): %w", opts.Namespace, opts.LabelSelector, waitCtx.Err())
		case <-ticker.C:
		}
	}
}

func (s *logStreamer) openLogStream(ctx context.Context, namespace, podName string) (io.ReadCloser, error) {
	deadline := time.Now().Add(streamOpenTimeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		// ストリームのbodyは親ctxに束縛する。タイムアウトはリトライの予算としてのみ使い、
		// 子ctxを req.Stream に渡すと defer cancel で開いた直後にストリームが閉じてしまう。
		req := s.clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{Follow: true})
		if stream, err := req.Stream(ctx); err == nil {
			return stream, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out opening log stream for pod %s/%s", namespace, podName)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
