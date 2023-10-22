package internal

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

const (
	leaderLabelName = "alpha.k8s.io/role-active"
)

type App struct {
	logger       *slog.Logger
	conf         *Configuration
	k8sClientSet *kubernetes.Clientset
}

func NewApp(logger *slog.Logger) (*App, error) {
	clientset, err := getKubeClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s clientset")
	}
	disableKlogOutput()

	return &App{
		logger:       logger,
		conf:         &Configuration{},
		k8sClientSet: clientset,
	}, nil
}

func (a *App) Run() error {
	if err := envconfig.Process("", a.conf); err != nil {
		return fmt.Errorf("failed to process env variables: %w", err)
	}

	if err := a.configureMetrics(); err != nil {
		return fmt.Errorf("failed to configure metrics: %w", err)
	}

	leaderElectionConfig := leaderelection.LeaderElectionConfig{
		Name: a.conf.ElectionGroup,
		Lock: &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      a.conf.ElectionGroup,
				Namespace: a.conf.Namespace,
			},
			Client: a.k8sClientSet.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity: a.conf.MemberID,
				EventRecorder: record.NewBroadcaster().NewRecorder(runtime.NewScheme(), v1.EventSource{
					Component: a.conf.MemberID,
				}),
			},
		},
		LeaseDuration: time.Duration(a.conf.LeaseDuration) * time.Second,
		RenewDeadline: time.Duration(a.conf.RenewalDeadline) * time.Second,
		RetryPeriod:   time.Duration(a.conf.RetryPeriod) * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: a.onStartedLeading,
			OnStoppedLeading: a.onStoppedLeading,
			OnNewLeader:      a.onNewLeader,
		},
		ReleaseOnCancel: true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		leaderelection.RunOrDie(ctx, leaderElectionConfig)
	}()

	http.Handle("/metrics", promhttp.Handler())
	a.logger.Info("Listenening on http://localhost:8088/metrics")
	err := http.ListenAndServe(":8088", nil)

	wg.Wait()
	return err
}

func (a *App) onStartedLeading(ctx context.Context) {
	log := a.logger.With("leaderID", a.conf.MemberID, "electionGroup", a.conf.ElectionGroup, "memberID", a.conf.MemberID)

	log.Info("Became leader")
	clientset, err := getKubeClient()
	if err != nil {
		log.Error("Failed to get k8s clientset", "error", err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			log.Info("Stopped leader loop")
			return
		default:
			err := a.setCurrentPodLabel(ctx, clientset, leaderLabelName, "true")
			if err != nil {
				log.Error("failed to set pod label", "error", err)
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func (a *App) onNewLeader(identity string) {
	log := a.logger.With("leaderID", identity, "electionGroup", a.conf.ElectionGroup, "memberID", a.conf.MemberID)

	log.Info("Another leader elected")
	clientset, err := getKubeClient()
	if err != nil {
		log.Error("Failed to get k8s clientset", "error", err)
		return
	}
	err = a.setCurrentPodLabel(context.Background(), clientset, leaderLabelName, "false")
	if err != nil {
		log.Error("failed to set pod label", "error", err)
	}
}

func (a *App) onStoppedLeading() {
	log := a.logger.With("leaderID", a.conf.ElectionGroup, "memberID", a.conf.MemberID)

	log.Info("Stopped being leader")
	clientset, err := getKubeClient()
	if err != nil {
		log.Error("Failed to get k8s clientset", "error", err)
		return
	}
	err = a.setCurrentPodLabel(context.Background(), clientset, leaderLabelName, "false")
	if err != nil {
		log.Error("failed to set pod label", "error", err)
	}
}

func (a *App) setCurrentPodLabel(ctx context.Context, clientset *kubernetes.Clientset, labelname, labelvalue string) error {
	pod, err := clientset.CoreV1().Pods(a.conf.Namespace).Get(ctx, a.conf.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if value, ok := pod.ObjectMeta.Labels[labelname]; ok && value == labelvalue {
		return nil
	}
	pod.ObjectMeta.Labels[labelname] = labelvalue

	_, err = clientset.CoreV1().Pods(a.conf.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	a.logger.Info("Successfully set pod label", "label", labelname, "value", labelvalue)
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

func disableKlogOutput() {
	klog.SetOutput(io.Discard)
	flags := &flag.FlagSet{}
	klog.InitFlags(flags)
	flags.Set("logtostderr", "false")
}
