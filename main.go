/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"k8s.io/apimachinery/pkg/api/errors"
	"os"
	"strconv"

	valid "github.com/asaskevich/govalidator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	opv1 "github.com/kubesaw/snapshot-operator/api/v1alpha1"
	"github.com/kubesaw/snapshot-operator/controllers"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))
	utilruntime.Must(opv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// setting privileged pod security labels to operator ns
	err := addPodSecurityPrivilegedLabels()
	if err != nil {
		setupLog.Error(err, "error setting privileged pod security labels to operator namespace")
		os.Exit(1)
	}

	setupLog.Info("Labels added")

	newCacheFunc := cache.BuilderWithOptions(cache.Options{
		Scheme: scheme,
		SelectorsByObject: cache.SelectorsByObject{
			&batchv1.Job{}: {
				Label: labels.SelectorFromSet(labels.Set{"openshift.snapshot.operator": "true"}),
			},
			&rbacv1.ClusterRole{}: {
				Label: labels.SelectorFromSet(labels.Set{"openshift.snapshot.operator": "true"}),
			},
			&rbacv1.ClusterRoleBinding{}: {
				Label: labels.SelectorFromSet(labels.Set{"openshift.snapshot.operator": "true"}),
			},
			&corev1.ServiceAccount{}: {
				Label: labels.SelectorFromSet(labels.Set{"openshift.snapshot.operator": "true"}),
			},
			&corev1.PersistentVolumeClaim{}: {
				Label: labels.SelectorFromSet(labels.Set{"openshift.snapshot.operator": "true"}),
			},
			&corev1.Service{}: {
				Label: labels.SelectorFromSet(labels.Set{"openshift.snapshot.operator": "true"}),
			},
			&corev1.Secret{}: {
				Label: labels.SelectorFromSet(labels.Set{"openshift.snapshot.operator": "true"}),
			},
			&appsv1.Deployment{}: {
				Label: labels.SelectorFromSet(labels.Set{"openshift.snapshot.operator": "true"}),
			},
			&routev1.Route{}: {
				Label: labels.SelectorFromSet(labels.Set{"openshift.snapshot.operator": "true"}),
			},
			&networkingv1.NetworkPolicy{}: {
				Label: labels.SelectorFromSet(labels.Set{"openshift.snapshot.operator": "true"}),
			},
		},
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		NewCache:               newCacheFunc,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.ExportReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Export")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// setting privileged pod security labels to operator namespace
func addPodSecurityPrivilegedLabels() error {
	setupLog.Info("patching namespace with PSA labels")
	kubeconf := ctrl.GetConfigOrDie()
	clientset, err := kubernetes.NewForConfig(kubeconf)
	if err != nil {
		setupLog.Error(err, "problem getting client")
		return err
	}

	version, err := clientset.ServerVersion()
	if err != nil {
		setupLog.Error(err, "problem getting server version")
		return err
	}
	minVer := version.Minor
	if !valid.IsInt(minVer) {
		minVer = minVer[:len(minVer)-1]
	}
	minor, err := strconv.Atoi(minVer)
	if err != nil {
		setupLog.Error(err, "problem getting minor version")
		return err
	}

	if minor < 24 {
		return nil
	}

	operatorNamespace, err := clientset.CoreV1().Namespaces().Get(context.TODO(), "snapshot-operator-system", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			setupLog.Info("snapshot-operator-system namespace not found. If installing operator in any other namespace, please add appropriate namespace labels to make sure primer works in OpenShift environment 4.11+")
			return nil
		}
		setupLog.Error(err, "problem getting operator namespace")
		return err
	}

	privilegedLabels := map[string]string{
		"pod-security.kubernetes.io/enforce": "privileged",
		"pod-security.kubernetes.io/audit":   "privileged",
		"pod-security.kubernetes.io/warn":    "privileged",
	}

	operatorNamespace.SetLabels(privilegedLabels)

	_, err = clientset.CoreV1().Namespaces().Update(context.TODO(), operatorNamespace, metav1.UpdateOptions{})
	if err != nil {
		setupLog.Error(err, "problem patching operator namespace for privileged pod security labels")
		return err
	}
	return nil
}
