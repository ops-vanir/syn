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

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	fluxsource "github.com/fluxcd/source-controller/api/v1"
	synv1 "github.com/ops-vanir/syn/api/v1alpha1"
	synv1alpha1 "github.com/ops-vanir/syn/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SupervisedGitReconciler reconciles a SupervisedGit object
type SupervisedGitReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=syn.servicenow.com,resources=supervisedgits,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=syn.servicenow.com,resources=supervisedgits/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=syn.servicenow.com,resources=supervisedgits/finalizers,verbs=update

// Additional for managing Flux resources
//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=gitrepositories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kustomize.toolkit.fluxcd.io,resources=kustomizations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=helm.toolkit.fluxcd.io,resources=helmreleases,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list

func (r *SupervisedGitReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling SupervisedGit")

	obj := synv1.SupervisedGit{}
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize the runtime patcher with the current version of the object.
	patcher := patch.NewSerialPatcher(&obj, r.Client)

	// Finalise the reconciliation and report the results.
	defer func() {
		// Patch finalizers, status and conditions.
		if err := r.finalizeStatus(ctx, &obj, patcher); err != nil {
			retErr = kerrors.NewAggregate([]error{retErr, err})
		}
	}()

	if obj.Spec.AdoptTargetRef.Kind == "GitRepository" {
		log.Info("Managing adopted GitRepository", "adoptee", obj.Spec.AdoptTargetRef.Name)

		targetRepo, err := r.AdoptGitrepo(ctx, &obj)
		if err != nil {
			if errors.IsNotFound(err) {
				log.Info("Gitrepository not found")
				conditions.MarkFalse(&obj, meta.ReadyCondition, "NotFound", "Repo Not Found")
				// if err := r.Status().Update(ctx, &obj); err != nil {
				if err := r.patch(ctx, &obj, patcher); err != nil {
					return ctrl.Result{}, err
				}
			}
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		// Get the source repo
		sourceRepo, err := r.EnsureSource(ctx, &obj, targetRepo.Spec)
		if err != nil {
			return ctrl.Result{}, err
		}
		if obj.GetLastGitrepoRevisionSHA(sourceRepo) != obj.GetLastSeenRevision() {
			// update last seen revision
			obj.SetLastSeenRevision(obj.GetLastGitrepoRevisionSHA(sourceRepo))
			log.Info("New revision", "revision", obj.GetLastSeenRevision())
			conditions.MarkUnknown(&obj, meta.ReadyCondition, "NewRevision", "New revision seen. Need Chg")
			// if err := r.Status().Update(ctx, &obj); err != nil {
			if err := r.patch(ctx, &obj, patcher); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Ensure last seen revision is approved
	if obj.GetLastSeenRevision() != obj.GetLastApprovedRevision() {
		log.Info("New revision needs CHG", "revision", obj.GetLastSeenRevision())
		chg, err := r.EnsureCHG(ctx, &obj)
		if err != nil {
			return ctrl.Result{}, err
		}

		switch {
		case chg.GetRequestPhase() == synv1.ChangeRequestRejected:
			conditions.MarkTrue(&obj, meta.StalledCondition, "ChgRejected", "Chg Rejected")
			conditions.MarkFalse(&obj, meta.ReadyCondition, "ChgRejected", "Chg Rejected. Will not reconcile the last seen revision")
			// if err := r.Status().Update(ctx, &obj); err != nil {
			if err := r.patch(ctx, &obj, patcher); err != nil {
				return ctrl.Result{}, err
			}

		case chg.GetRequestPhase() == synv1.ChangeRequestApproved:
			obj.SetLastApprovedRevision(chg.GetApprovedRevision())
			obj.SetLastApprovedRevisionStartNow()
			conditions.MarkReconciling(&obj, "ChgApproved", "Chg Approved. Reconciling revision")
			// if err := r.Status().Update(ctx, &obj); err != nil {
			if err := r.patch(ctx, &obj, patcher); err != nil {
				return ctrl.Result{}, err
			}
		case chg.GetRequestPhase() == synv1.ChangeRequestOpened:
			// Still waiting for approval
			// We'll await notification from CHG
			return ctrl.Result{}, nil
		}
	}

	if obj.GetLastAppliedRevision() != obj.GetLastApprovedRevision() {
		// Ensure that the change has applied successfully

		if obj.GetAdopteesChildLabelSelectors() == nil {
			// We require a child label selector to be set
			log.Error(nil, "No child label selector set")
			conditions.MarkFalse(&obj, meta.ReadyCondition, "NoChildLabelSelector", "No child label selector set")
			// if err := r.Status().Update(ctx, &obj); err != nil {
			if err := r.patch(ctx, &obj, patcher); err != nil {
				return ctrl.Result{}, err
			}

		} else {

			log.Info("Evaluating child resources")
			reconciled, err := r.GetChildStatus(ctx, &obj)
			if err != nil {
				return ctrl.Result{}, err
			}

			if reconciled {
				// Set our new status
				obj.SetLastAppliedRevision(obj.GetLastApprovedRevision())
				conditions.MarkTrue(&obj, meta.ReadyCondition, "Applied", fmt.Sprintf("Revision %s applied successfully", obj.GetLastAppliedRevision()))
				conditions.Delete(&obj, meta.StalledCondition)
				conditions.Delete(&obj, meta.ReconcilingCondition)

				// Close the change request
				if err := r.CloseCHG(ctx, &obj, synv1.CloseSuccess); err != nil {
					return ctrl.Result{}, err
				}

				// Update our status
				// if err := r.Status().Update(ctx, &obj); err != nil {
				if err := r.patch(ctx, &obj, patcher); err != nil {
					return ctrl.Result{}, err
				}

			} else {

				// Convert the LastApprovedRevisionStart and ReconcileTimeLimit values to the same time zone as time.Now()
				lastApprovedRevisionStart := obj.Status.LastApprovedRevisionStart.In(time.Now().Location())

				if lastApprovedRevisionStart.Add(obj.GetReconcileTimeLimit()).Before(time.Now()) {
					// The reconciliation has stalled, we need to close the change request
					conditions.MarkStalled(&obj, "ReconcileTimeout", "Reconcile timed out")
					if err := r.CloseCHG(ctx, &obj, synv1.CloseFail); err != nil {
						return ctrl.Result{}, err
					}
					// if err := r.Status().Update(ctx, &obj); err != nil {
					// if err := r.patch(ctx, &obj, patcher); err != nil {
					// 	return ctrl.Result{}, err
					// }
				} else {
					// If we've fallen into this sideways, we'll add a condition which can be removed
					if !conditions.Has(&obj, meta.ReconcilingCondition) {
						conditions.MarkReconciling(&obj, "ChgApproved", fmt.Sprintf("Reconciling revision %s", obj.GetLastApprovedRevision()))
					}
					log.Info("Last transition time", "start time", lastApprovedRevisionStart, "now", time.Now(), "timeout", obj.GetReconcileTimeLimit())
					log.Info("Will wait for child resources to reconcile. Reququeing")
					return ctrl.Result{RequeueAfter: obj.GetInterval()}, nil
				}
			}
		}
		// We can also have a spec for jobs that needs to run
	}

	log.Info("Reconcile complete")
	return ctrl.Result{}, nil
}

func (r *SupervisedGitReconciler) finalizeStatus(ctx context.Context,
	obj *synv1.SupervisedGit,
	patcher *patch.SerialPatcher) error {

	if conditions.IsTrue(obj, meta.ReadyCondition) {
		conditions.Delete(obj, meta.ReconcilingCondition)
		conditions.Delete(obj, meta.StalledCondition)
		obj.Status.ObservedGeneration = obj.Generation
	}

	// Set the Reconciling reason to ProgressingWithRetry if the
	// reconciliation has failed.
	if conditions.IsFalse(obj, meta.ReadyCondition) &&
		conditions.Has(obj, meta.ReconcilingCondition) {
		rc := conditions.Get(obj, meta.ReconcilingCondition)
		rc.Reason = meta.ProgressingWithRetryReason
		conditions.Set(obj, rc)
	}

	// Patch status and conditions.
	return r.patch(ctx, obj, patcher)
	// return r.Status.Update(ctx, obj, patcher)
}

func (r *SupervisedGitReconciler) patch(ctx context.Context,
	obj *synv1.SupervisedGit,
	patcher *patch.SerialPatcher) error {
	// Configure the runtime patcher.
	patchOpts := []patch.Option{}
	ownedConditions := []string{
		meta.ReadyCondition,
		meta.ReconcilingCondition,
		meta.StalledCondition,
	}
	patchOpts = append(patchOpts,
		patch.WithOwnedConditions{Conditions: ownedConditions},
		patch.WithForceOverwriteConditions{},
		patch.WithFieldOwner("supervisedgit-controller"),
		patch.WithStatusObservedGeneration{},
	)

	// Patch status and conditions.
	return patcher.Patch(ctx, obj, patchOpts...)
}

// SetupWithManager sets up the controller with the Manager.
func (r *SupervisedGitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&synv1alpha1.SupervisedGit{}).
		Owns(&fluxsource.GitRepository{}).
		Owns(&synv1alpha1.ChangeRequest{}).
		Complete(r)
}
