/*
Copyright 2022.

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
	"encoding/json"
	"fmt"
	api "github.com/redhat-appstudio/remotesecret-operator/api/v1alpha1"
	"github.com/redhat-appstudio/remotesecret-operator/pkg/logs"
	"github.com/redhat-appstudio/remotesecret-operator/pkg/secretstorage"
	"strconv"
	"time"

	kuberrors "k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	remoteSecretLabel              = "spi.appstudio.redhat.com/upload-secret"      //#nosec G101 -- false positive, this is not a token
	remoteSecretNameAnno           = "spi.appstudio.redhat.com/remote-secret.name" //#nosec G101 -- false positive, this is not a token
	remoteSecretExposeOnCreateAnno = "spi.appstudio.redhat.com/remote-secret.expose-on-create"
)

//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=remotesecrets,verbs=get;list;watch;create;update;patch;delete

// SecretUploadReconciler reconciles a Secret object
type SecretUploadReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	SecretStorage secretstorage.SecretStorage
	ClusterName   string
}

func (r *SecretUploadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	lg := log.FromContext(ctx)
	uploadSecretList := &corev1.SecretList{}

	// uploadSecretList secrets labeled with "spi.appstudio.redhat.com/upload-secret"
	err := r.List(ctx, uploadSecretList, client.HasLabels{remoteSecretLabel})
	if err != nil {
		lg.Error(err, "can not get uploadSecretList of secrets")
		return ctrl.Result{}, fmt.Errorf("can not get uploadSecretList of secrets: %w", err)
	}

	for i, uploadSecret := range uploadSecretList.Items {

		// we immediately delete the Secret
		err := r.Delete(ctx, &uploadSecretList.Items[i])
		if err != nil {
			r.logError(ctx, uploadSecret, fmt.Errorf("can not delete the Secret: %w", err), lg)
			continue
		}

		// try to find
		remoteSecret, err := r.findRemoteSecret(ctx, uploadSecret, lg)
		if err != nil {
			r.logError(ctx, uploadSecret, fmt.Errorf("can not find RemoteSecret: %w ", err), lg)
			continue
		} else if remoteSecret == nil {
			// SPIAccessToken does not exist, so create it
			remoteSecret, err = r.createRemoteSecret(ctx, uploadSecret, lg)
			if err != nil {
				r.logError(ctx, uploadSecret, fmt.Errorf("can not create RemoteSecret: %w ", err), lg)
				continue
			}
		}

		logs.AuditLog(ctx).Info("manual token upload initiated", uploadSecret.Namespace, remoteSecret.Name)
		// Upload Token, it will cause update SPIAccessToken Phase as well
		secretId := secretstorage.SecretUid{
			Cluster:      r.ClusterName,
			Namespace:    remoteSecret.Namespace,
			RemoteSecret: remoteSecret.Name,
		}

		secretData, err := json.Marshal(uploadSecret.Data)
		if err != nil {
			r.logError(ctx, uploadSecret, fmt.Errorf("failed to serialize token data: %w", err), lg)
		}

		//err = r.SecretStorage.Store(ctx, secretId, uploadSecret.Data["data"])
		err = r.SecretStorage.Store(ctx, secretId, secretData)
		if err != nil {
			r.logError(ctx, uploadSecret, fmt.Errorf("failed to store the token: %w", err), lg)
			logs.AuditLog(ctx).Error(err, "manual token upload failed", uploadSecret.Namespace, remoteSecret.Name)
			continue
		}
		/////

		//remoteSecret.Status.Conditions = append(remoteSecret.Status.Conditions, metav1.Condition{
		//	Type:               "Ready",
		//	Status:             "True",
		//	LastTransitionTime: metav1.NewTime(time.Now()),
		//	Reason:             "dataStored",
		//	Message:            "SecretData stored",
		//})
		//remoteSecret.Status.Phase = api.RemoteSecretDataStoredState
		//err = r.Client.Status().Update(ctx, remoteSecret)
		//if err != nil {
		//	return ctrl.Result{}, fmt.Errorf("error updating RemoteSecret Status %w", err)
		//}
		/////
		logs.AuditLog(ctx).Info("manual token upload completed", uploadSecret.Namespace, remoteSecret.Name)

		r.tryDeleteEvent(ctx, uploadSecret.Name, req.Namespace, lg)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretUploadReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(createSecretPredicate()).
		Complete(r); err != nil {
		err = fmt.Errorf("failed to build the controller manager: %w", err)
		return err
	}
	return nil
}

func createSecretPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			_, ok := e.Object.GetLabels()[remoteSecretLabel]
			return ok
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			_, newLabelExists := e.ObjectNew.GetLabels()[remoteSecretLabel]
			_, oldLabelExists := e.ObjectOld.GetLabels()[remoteSecretLabel]
			if newLabelExists && !oldLabelExists {
				return true
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
}

// reports error in both Log and Event in current Namespace
func (r *SecretUploadReconciler) logError(ctx context.Context, secret corev1.Secret, err error, lg logr.Logger) {

	lg.Error(err, "Secret upload failed:")

	r.tryDeleteEvent(ctx, secret.Name, secret.Namespace, lg)

	secretErrEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
		Message:        err.Error(),
		Reason:         "can not upload secret",
		InvolvedObject: corev1.ObjectReference{Namespace: secret.Namespace, Name: secret.Name, Kind: secret.Kind, APIVersion: secret.APIVersion},
		Type:           "Error",
		LastTimestamp:  metav1.NewTime(time.Now()),
	}

	err = r.Create(ctx, secretErrEvent)
	if err != nil {
		lg.Error(err, "Event creation failed for Secret: ", "secret.name", secret.Name)
	}

}

// Contract: having at only one event if upload failed and no events if uploaded.
// For this need to delete the event every attempt
func (r *SecretUploadReconciler) tryDeleteEvent(ctx context.Context, secretName string, ns string, lg logr.Logger) {
	stored := &corev1.Event{}
	err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: ns}, stored)

	if err == nil {
		lg.V(logs.DebugLevel).Info("event Found and will be deleted: ", "event.name", stored.Name)
		err = r.Delete(ctx, stored)
		if err != nil {
			lg.Error(err, " can not delete Event ")
		}
	}
}

func (r *SecretUploadReconciler) findRemoteSecret(ctx context.Context, uploadSecret corev1.Secret, lg logr.Logger) (*api.RemoteSecret, error) {
	remoteSecretName := uploadSecret.Annotations[remoteSecretNameAnno]
	//string(uploadSecret.Data[remoteSecretNameField])
	if remoteSecretName == "" {
		lg.V(logs.DebugLevel).Info("No RemoteSecret found, will try to create with generated ", "RemoteSecret", remoteSecretName)
		return nil, nil
	}

	secretRef := api.RemoteSecret{}
	err := r.Get(ctx, types.NamespacedName{Name: remoteSecretName, Namespace: uploadSecret.Namespace}, &secretRef)

	if err != nil {
		if kuberrors.IsNotFound(err) {
			lg.V(logs.DebugLevel).Info("RemoteSecret NOT found, will try to create  ", "RemoteSecret.name", secretRef.Name)
			return nil, nil
		} else {
			return nil, fmt.Errorf("can not find SecretRef %s: %w ", remoteSecretName, err)
		}
	} else {
		lg.V(logs.DebugLevel).Info("RemoteSecret found : ", "RemoteSecret.name", secretRef.Name)
		return &secretRef, nil
	}
}

func (r *SecretUploadReconciler) createRemoteSecret(ctx context.Context, uploadSecret corev1.Secret, lg logr.Logger) (*api.RemoteSecret, error) {
	remoteSecret := api.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: uploadSecret.Namespace,
		},
		Spec: api.RemoteSecretSpec{
			ClusterName: r.ClusterName,
			Type:        uploadSecret.Labels[remoteSecretLabel],
			ExposedAs:   exposeOnCreate(uploadSecret.Annotations[remoteSecretExposeOnCreateAnno]),
			Stored:      true,
		},
		Status: api.RemoteSecretStatus{
			Phase: api.RemoteSecretInitialState,
		},
	}
	remoteSecretName := uploadSecret.Annotations[remoteSecretNameAnno]

	if remoteSecretName == "" {
		remoteSecret.GenerateName = "rs-"
	} else {
		remoteSecret.Name = remoteSecretName
	}

	err := r.Create(ctx, &remoteSecret)
	if err == nil {
		lg.V(logs.DebugLevel).Info("RemoteSecret created : ", "RemoteSecret.name", remoteSecret.Name)
		return &remoteSecret, nil
	} else {
		return nil, fmt.Errorf("can not create SPIAccessToken %s. Reason: %w", remoteSecret.Name, err)
	}
}

func exposeOnCreate(anno string) bool {
	boolValue, err := strconv.ParseBool(anno)
	if err != nil {
		return false
	}
	return boolValue
}
