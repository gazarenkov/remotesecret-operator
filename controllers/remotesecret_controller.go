/*
Copyright 2023.

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

package controllers

import (
	"context"
	"fmt"
	"github.com/redhat-appstudio/remotesecret-operator/pkg/logs"
	"github.com/redhat-appstudio/remotesecret-operator/pkg/secretstorage"
	"k8s.io/apimachinery/pkg/api/errors"
	//"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	api "github.com/redhat-appstudio/remotesecret-operator/api/v1alpha1"
	//"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const secretStorageFinalizerName = "spi.appstudio.redhat.com/secret-storage" //#nosec G101 -- false positive, we're not storing any sensitive data using this

// RemoteSecretReconciler reconciles a RemoteSecret object
type RemoteSecretReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	SecretStorage secretstorage.SecretStorage
	finalizers    finalizer.Finalizers
}

type secretStorageFinalizer struct {
	storage secretstorage.SecretStorage
}

//var _ finalizer.Finalizer = (*secretStorageFinalizer)(nil)

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=remotesecrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=remotesecrets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=remotesecrets/finalizers,verbs=update

func (r *RemoteSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	lg := log.FromContext(ctx)
	lg.V(logs.DebugLevel).Info("starting reconciliation")
	//defer logs.TimeTrackWithLazyLogger(func() logr.Logger { return lg }, time.Now(), "Reconcile RemoteSecret")

	rs := api.RemoteSecret{}
	if err := r.Get(ctx, req.NamespacedName, &rs); err != nil {
		if errors.IsNotFound(err) {
			lg.V(logs.DebugLevel).Info("remote secret already gone from the cluster. skipping reconciliation")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to load remote secret from the cluster: %w", err)
	} else {
		// make sure Conditions initialized
		if rs.Status.Conditions == nil {
			rs.Status.Conditions = make([]metav1.Condition, 0)
			lg.V(logs.DebugLevel).Info("RemoteSecret.Status.Conditions initialized")
		}
	}

	finalizationResult, err := r.finalizers.Finalize(ctx, &rs)
	if err != nil {
		// if the finalization fails, the finalizer stays in place, and so we don't want any repeated attempts until
		// we get another reconciliation due to cluster state change
		return ctrl.Result{Requeue: false}, fmt.Errorf("failed to finalize: %w", err)
	}

	if finalizationResult.Updated {
		if err = r.Client.Update(ctx, &rs); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update based on finalization result: %w", err)
		}
	}
	if finalizationResult.StatusUpdated {
		if err = r.Client.Status().Update(ctx, &rs); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update the status based on finalization result: %w", err)
		}
	}

	if rs.DeletionTimestamp != nil {
		lg.V(logs.DebugLevel).Info("RemoteSecret being deleted, no other changes required after completed finalization", "timestamp", rs.DeletionTimestamp)
		return ctrl.Result{}, nil
	}

	//if rs.ObjectMeta.DeletionTimestamp.IsZero() {
	//	if !controllerutil.ContainsFinalizer(&rs, secretStorageFinalizerName) {
	//		controllerutil.AddFinalizer(&rs, secretStorageFinalizerName)
	//		if err := r.Update(ctx, &rs); err != nil {
	//			return ctrl.Result{}, err
	//		}
	//	}
	//} else {
	//
	//	lg.V(logs.DebugLevel).Info("remote secret being deleted, no other changes required after completed finalization")
	//	finalizationResult, err := r.finalizers.Finalize(ctx, &rs)
	//	if err != nil {
	//		// if the finalization fails, the finalizer stays in place, and so we don't want any repeated attempts until
	//		// we get another reconciliation due to cluster state change
	//		return ctrl.Result{Requeue: false}, fmt.Errorf("failed to finalize: %w", err)
	//	} else {
	//		lg.V(logs.DebugLevel).Info("RemoteSecret finalized ", "res ", finalizationResult, "name", rs.Name)
	//	}
	//
	//	if controllerutil.ContainsFinalizer(&rs, secretStorageFinalizerName) {
	//		// our finalizer is present, so lets handle any external dependency
	//		if _, err := r.finalizers.Finalize(ctx, &rs); err != nil {
	//			// if fail to delete the external dependency here, return with error
	//			// so that it can be retried
	//			return ctrl.Result{}, err
	//		}
	//
	//		// remove our finalizer from the list and update it.
	//		controllerutil.RemoveFinalizer(&rs, secretStorageFinalizerName)
	//		if err := r.Update(ctx, &rs); err != nil {
	//			return ctrl.Result{}, err
	//		}
	//	}
	//
	//	return ctrl.Result{}, nil
	//}

	//if rs.Spec.Exposed {
	//	c := r.condition("exposed")
	//	if c.Type == "Ready" && c.Status == "True" {
	//
	//	}
	//}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RemoteSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {

	r.finalizers = finalizer.NewFinalizers()
	if err := r.finalizers.Register(secretStorageFinalizerName, &secretStorageFinalizer{storage: r.SecretStorage}); err != nil {
		return fmt.Errorf("failed to register the secret storage finalizer: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&api.RemoteSecret{}).
		Complete(r)
}

func (f *secretStorageFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	secretId := secretstorage.NewSecretUid(obj.(*api.RemoteSecret).Spec.ClusterName, obj.GetNamespace(), obj.GetName())
	err := f.storage.Delete(ctx, secretId)
	if err != nil {
		err = fmt.Errorf("failed to delete Secret Data during finalization of %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	return finalizer.Result{}, err
}
