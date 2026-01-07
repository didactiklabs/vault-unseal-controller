/*
Copyright 2026.

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

package controller

import (
	"context"
	"strconv"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/didactiklabs/vault-unseal-controller/api/v1alpha1"
	"github.com/didactiklabs/vault-unseal-controller/pkg/vault"
)

const (
	keysPath = "/secrets/keys"
)

// UnsealReconciler reconciles a Unseal object
type UnsealReconciler struct {
	client.Client
	log    logr.Logger
	Scheme *runtime.Scheme
	// UnsealerImage holds the container image used for unsealing jobs, configured via the UNSEALER_IMAGE environment variable via kustomize.
	UnsealerImage string
}

// +kubebuilder:rbac:groups=platform.didactiklabs.io,resources=unseals,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.didactiklabs.io,resources=unseals/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.didactiklabs.io,resources=unseals/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Unseal object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *UnsealReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = log.FromContext(ctx)
	r.log.Info("reconcile", "req", req)

	// Create job object
	job := &batchv1.Job{}

	// Fetch the IPCidr resource
	unseal := &platformv1alpha1.Unseal{}
	if err := r.Get(ctx, req.NamespacedName, unseal); err != nil { // check if resource is available and stored in err
		if apierrors.IsNotFound(err) { // check for "not found error"
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion with finalizer
	if !unseal.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(unseal, unsealFinalizer) {
			// Delete all jobs before removing finalizer
			jobNamespace := unseal.Spec.UnsealKeysSecretRef.Namespace
			for i := range unseal.Status.SealedNodes {
				num := strconv.Itoa(i)
				jobName := unseal.Name + "-" + num
				job := &batchv1.Job{}
				err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: jobNamespace}, job)
				if err == nil {
					err = r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground))
					if err != nil {
						r.log.Error(err, "Failed to delete job during cleanup", "Job Name", jobName)
						return ctrl.Result{}, err
					}
					r.log.Info("Job deleted during cleanup", "Job Name", jobName)
				} else if !apierrors.IsNotFound(err) {
					r.log.Error(err, "Failed to check job existence during cleanup", "Job Name", jobName)
					return ctrl.Result{}, err
				}
			}

			// Remove finalizer
			r.log.Info("Removing finalizer from the object", "Unseal", unseal.Name)
			controllerutil.RemoveFinalizer(unseal, unsealFinalizer)
			if err := r.Update(ctx, unseal); err != nil {
				r.log.Error(err, "Error removing finalizer")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizers
	if unseal.DeletionTimestamp.IsZero() &&
		!controllerutil.ContainsFinalizer(unseal, unsealFinalizer) {
		r.log.Info("Adding finalizer to the object", "Unseal", unseal.Name)
		controllerutil.AddFinalizer(unseal, unsealFinalizer)
		if err := r.Update(ctx, unseal); err != nil {
			r.log.Error(err, "Error adding finalizer")
			return ctrl.Result{}, err
		}
	}

	// Get the actual status and populate field
	if unseal.Status.VaultStatus == "" {
		unseal.Status.VaultStatus = platformv1alpha1.StatusUnsealed
	}

	// Get CA certificate value if needed
	var caCert string
	if !unseal.Spec.TlsSkipVerify && unseal.Spec.CaCertSecret != "" {
		secret := corev1.Secret{}
		sName := types.NamespacedName{
			Namespace: unseal.Spec.UnsealKeysSecretRef.Namespace,
			Name:      unseal.Spec.CaCertSecret,
		}
		err := r.Get(ctx, sName, &secret)
		if err != nil {
			r.log.Error(err, "Failed to get CA certificate secret")
			return ctrl.Result{}, err
		}
		caCert = string(secret.Data["ca.crt"])
	}

	// Switch implementing vault state logic
	switch unseal.Status.VaultStatus {
	case platformv1alpha1.StatusUnsealed:
		var err error
		// Get Vault status and make decision based on number of sealed nodes
		if unseal.Spec.TlsSkipVerify {
			unseal.Status.SealedNodes, err = vault.GetVaultStatus(vault.VSParams{VaultNodes: unseal.Spec.VaultNodes, Insecure: true})
			if err != nil {
				r.log.Error(err, "Vault Status error")
			}
		} else {
			unseal.Status.SealedNodes, err = vault.GetVaultStatus(vault.VSParams{VaultNodes: unseal.Spec.VaultNodes, Insecure: false, CaCert: caCert})
			if err != nil {
				r.log.Error(err, "Vault Status error")
			}
		}
		if len(unseal.Status.SealedNodes) > 0 {
			// Set VaultStatus to unsealing and update the status of resources in the cluster
			unseal.Status.VaultStatus = platformv1alpha1.StatusUnsealing
			err := r.Status().Update(context.TODO(), unseal)
			if err != nil {
				r.log.Error(err, "failed to update unseal status")
				return ctrl.Result{}, err
			} else {
				return ctrl.Result{RequeueAfter: requeueTime}, nil
			}
		} else {
			// Set VaultStatus to unseal and update the status of resources in the cluster
			err := r.Status().Update(context.TODO(), unseal)
			if err != nil {
				r.log.Error(err, "failed to update unseal status")
				return ctrl.Result{}, err
			} else {
				return ctrl.Result{RequeueAfter: requeueTime}, nil
			}
		}

	case platformv1alpha1.StatusUnsealing:
		// Create job for each sealed node
		for i, nodeName := range unseal.Status.SealedNodes {
			// Check if the job already exists, if not create a new one
			num := strconv.Itoa(i)
			jobName := unseal.Name + "-" + num
			// Use the namespace from the UnsealKeysSecretRef
			jobNamespace := unseal.Spec.UnsealKeysSecretRef.Namespace
			err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: jobNamespace}, job)
			if err != nil && apierrors.IsNotFound(err) {
				// Define a new job
				job := r.createJob(
					unseal,
					jobName,
					nodeName,
				)
				// Establish the parent-child relationship between unseal resource and the job
				if err = controllerutil.SetControllerReference(unseal, job, r.Scheme); err != nil {
					r.log.Error(err, "Failed to set deployment controller reference")
					return ctrl.Result{}, err
				}
				// Create the job
				r.log.Info("Creating a new Job", "Job.Namespace", job.Namespace, "Job.Name", jobName)
				err = r.Create(ctx, job)
				if err != nil {
					r.log.Error(err, "Failed to create new Job", "Job Namespace", job.Namespace, "Job Name", jobName)
					return ctrl.Result{}, err
				}
				// Job created successfully - return and requeue
				return ctrl.Result{RequeueAfter: requeueTime}, nil
			} else if err != nil {
				r.log.Error(err, "Failed to get Job")
				return ctrl.Result{}, err
			}

			// Check Job status
			err = r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: jobNamespace}, job)
			if err != nil {
				r.log.Error(err, "Failed to get Job")
				return ctrl.Result{}, err
			}
			succeeded := job.Status.Succeeded
			failed := job.Status.Failed
			if succeeded == 1 {
				r.log.Info("Job Succeeded", "Node Name", nodeName, "Job Name", jobName)
				unseal.Status.VaultStatus = platformv1alpha1.StatusCleaning
				err := r.Status().Update(context.TODO(), unseal)
				if err != nil {
					r.log.Error(err, "failed to update unseal status")
					return ctrl.Result{}, err
				} else {
					return ctrl.Result{RequeueAfter: requeueTime}, nil
				}
			} else if failed == 1 {
				r.log.Info("Job Failed, please check your configuration", "Node Name", nodeName, "Job Name", jobName, "Backoff Limit", unseal.Spec.RetryCount)
				// Set VaultStatus to cleaning and update the status of resources in the cluster
				unseal.Status.VaultStatus = platformv1alpha1.StatusCleaning
				err := r.Status().Update(context.TODO(), unseal)
				if err != nil {
					r.log.Error(err, "failed to update unseal status")
					return ctrl.Result{}, err
				} else {
					return ctrl.Result{RequeueAfter: requeueTime}, nil
				}
			}
		}

	case platformv1alpha1.StatusCleaning:
		// Remove job if status is cleaning
		jobNamespace := unseal.Spec.UnsealKeysSecretRef.Namespace
		for i := range unseal.Status.SealedNodes {
			num := strconv.Itoa(i)
			jobName := unseal.Name + "-" + num
			err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: jobNamespace}, job)
			if err == nil && unseal.DeletionTimestamp.IsZero() {
				err = r.Delete(context.TODO(), job, client.PropagationPolicy(metav1.DeletePropagationBackground))
				if err != nil {
					r.log.Error(err, "Failed to remove old job", "Job Name", jobName)
					return ctrl.Result{}, err
				} else {
					r.log.Info("Old job removed", "Job Name", jobName)
					// Set VaultStatus to cleaning and update the status of resources in the cluster
					unseal.Status.VaultStatus = platformv1alpha1.StatusUnsealed
					err := r.Status().Update(context.TODO(), unseal)
					if err != nil {
						r.log.Error(err, "failed to update unseal status")
						return ctrl.Result{}, err
					} else {
						return ctrl.Result{RequeueAfter: requeueTime}, nil
					}
				}
			}
		}
	default:
	}

	return ctrl.Result{RequeueAfter: requeueTime}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UnsealReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.Unseal{}).
		Named("unseal").
		Complete(r)
}

// getLabels return labels depends on unseal resource name
func getLabels(unseal *platformv1alpha1.Unseal, jobName string) map[string]string {
	return map[string]string{
		"app":     jobName,
		"part-of": unseal.Name,
	}
}

// createJob return a k8s job object depends on unseal resource name and namespace
func (r *UnsealReconciler) createJob(
	unseal *platformv1alpha1.Unseal,
	jobName string,
	nodeName string,
) *batchv1.Job {
	// Job Metadata
	jobMeta := metav1.ObjectMeta{
		Name:      jobName,
		Namespace: unseal.Spec.UnsealKeysSecretRef.Namespace,
		Labels:    getLabels(unseal, jobName),
	}

	// Pod Volume Mounts
	podMounts := []corev1.VolumeMount{
		{
			Name:      "unseal-keys",
			MountPath: keysPath,
		},
	}

	// Pod Volumes
	podVolumes := []corev1.Volume{
		{
			Name: "unseal-keys",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: unseal.Spec.UnsealKeysSecretRef.Name,
				},
			},
		},
	}

	// Pod Env Vars
	podEnv := []corev1.EnvVar{
		{
			Name:  "VAULT_ADDR",
			Value: nodeName,
		},
		{
			Name:  "UNSEALER_SECRET_PATH",
			Value: keysPath,
		},
	}

	// Condition for insecure mode or CA Cert usage
	if unseal.Spec.TlsSkipVerify {
		podEnv = []corev1.EnvVar{
			{
				Name:  "VAULT_ADDR",
				Value: nodeName,
			},
			{
				Name:  "UNSEALER_SECRET_PATH",
				Value: keysPath,
			},
			{
				Name:  "VAULT_SKIP_VERIFY",
				Value: "true",
			},
		}
	} else if unseal.Spec.CaCertSecret != "" {
		podEnv = []corev1.EnvVar{
			{
				Name:  "VAULT_ADDR",
				Value: nodeName,
			},
			{
				Name:  "UNSEALER_SECRET_PATH",
				Value: keysPath,
			},
			{
				Name:  "VAULT_CAPATH",
				Value: "/secrets/cacerts/ca.crt",
			},
		}

		podMounts = []corev1.VolumeMount{
			{
				Name:      "unseal-keys",
				MountPath: keysPath,
			},
			{
				Name:      "cacert",
				MountPath: "/secrets/cacerts",
			},
		}

		podVolumes = []corev1.Volume{
			{
				Name: "unseal-keys",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: unseal.Spec.UnsealKeysSecretRef.Name,
					},
				},
			},
			{
				Name: "cacert",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: unseal.Spec.CaCertSecret,
					},
				},
			},
		}
	}

	// Pod Specs
	podSpec := corev1.PodSpec{
		RestartPolicy: "OnFailure",
		Containers: []corev1.Container{
			{
				Name:            "unsealer",
				Image:           r.UnsealerImage,
				ImagePullPolicy: "Always",
				Env:             podEnv,
				VolumeMounts:    podMounts,
			},
		},
		Volumes: podVolumes,
	}

	// Job Spec
	jobSpec := batchv1.JobSpec{
		BackoffLimit: &unseal.Spec.RetryCount,
		Template: corev1.PodTemplateSpec{
			Spec: podSpec,
		},
	}

	// Return job object
	return &batchv1.Job{
		ObjectMeta: jobMeta,
		Spec:       jobSpec,
	}
}
