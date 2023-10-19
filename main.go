package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
)

const (
	leaderLabelName = "alpha.k8s.io/role-active"
)

type Configuration struct {
	MemberID        string `envconfig:"MEMBER_ID" required:"true"`
	ElectionGroup   string `envconfig:"ELECTION_GROUP" required:"true"`
	PodName         string `envconfig:"POD_NAME" required:"true"`
	Namespace       string `envconfig:"NAMESPACE" required:"true"`
	LeaseDuration   int    `envconfig:"LEASE_DURATION" default:"15"`
	RenewalDeadline int    `envconfig:"RENEWAL_DEADLINE" default:"10"`
	RetryPeriod     int    `envconfig:"RETRY_PERIOD" default:"5"`
}

var (
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	conf   Configuration
)

func main() {
	if err := run(); err != nil {
		logger.Error("Exiting", "error", err)
		os.Exit(1)
	}
}

func run() error {
	err := envconfig.Process("", &conf)
	if err != nil {
		return fmt.Errorf("failed to process env variables: %w", err)
	}

	clientset, err := getKubeClient()
	if err != nil {
		return fmt.Errorf("failed to get k8s clientset")
	}

	leaderElectionConfig := leaderelection.LeaderElectionConfig{
		Lock: &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      conf.ElectionGroup,
				Namespace: conf.Namespace,
			},
			Client: clientset.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity: conf.MemberID,
				EventRecorder: record.NewBroadcaster().NewRecorder(runtime.NewScheme(), v1.EventSource{
					Component: conf.MemberID,
				}),
			},
		},
		LeaseDuration: time.Duration(conf.LeaseDuration) * time.Second,
		RenewDeadline: time.Duration(conf.RenewalDeadline) * time.Second,
		RetryPeriod:   time.Duration(conf.RetryPeriod) * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: onStartedLeading,
			OnStoppedLeading: onStoppedLeading,
			OnNewLeader:      onNewLeader,
		},
		ReleaseOnCancel: true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	leaderelection.RunOrDie(ctx, leaderElectionConfig)
	return nil
}

func onStartedLeading(ctx context.Context) {
	logger := logger.With("leaderID", conf.MemberID, "electionGroup", conf.ElectionGroup, "memberID", conf.MemberID)

	logger.Info("Became leader")
	clientset, err := getKubeClient()
	if err != nil {
		logger.Error("Failed to get k8s clientset", "error", err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopped leader loop")
			return
		default:
			err := setCurrentPodLabel(ctx, clientset, leaderLabelName, "true")
			if err != nil {
				logger.Error("failed to set pod label", "error", err)
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func onNewLeader(identity string) {
	logger := logger.With("leaderID", identity, "electionGroup", conf.ElectionGroup, "memberID", conf.MemberID)

	logger.Info("Another leader elected")
	clientset, err := getKubeClient()
	if err != nil {
		logger.Error("Failed to get k8s clientset", "error", err)
		return
	}
	err = setCurrentPodLabel(context.Background(), clientset, leaderLabelName, "false")
	if err != nil {
		logger.Error("failed to set pod label", "error", err)
	}
}

func onStoppedLeading() {
	logger := logger.With("leaderID", conf.ElectionGroup, "memberID", conf.MemberID)

	logger.Info("Stopped being leader")
	clientset, err := getKubeClient()
	if err != nil {
		logger.Error("Failed to get k8s clientset", "error", err)
		return
	}
	err = setCurrentPodLabel(context.Background(), clientset, leaderLabelName, "false")
	if err != nil {
		logger.Error("failed to set pod label", "error", err)
	}
}

func setCurrentPodLabel(ctx context.Context, clientset *kubernetes.Clientset, labelname, labelvalue string) error {
	pod, err := clientset.CoreV1().Pods(conf.Namespace).Get(ctx, conf.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if value, ok := pod.ObjectMeta.Labels[labelname]; ok && value == labelvalue {
		return nil
	}
	pod.ObjectMeta.Labels[labelname] = labelvalue

	_, err = clientset.CoreV1().Pods(conf.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	logger.Info("Successfully set pod label", "label", labelname, "value", labelvalue)
	return nil
}

func getKubeClient() (*kubernetes.Clientset, error) {
	// Create a Kubernetes client using the current context
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}
