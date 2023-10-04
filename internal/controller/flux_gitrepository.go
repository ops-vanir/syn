package controller

import (
	"context"
	"fmt"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fluxhelm "github.com/fluxcd/helm-controller/api/v2beta1"
	fluxkustomize "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/runtime/conditions"
	fluxsource "github.com/fluxcd/source-controller/api/v1"
	synv1 "github.com/ops-vanir/syn/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *SupervisedGitReconciler) AdoptGitrepo(ctx context.Context, obj *synv1.SupervisedGit) (gitrepo fluxsource.GitRepository, err error) {
	// Make sure the target repo exists and has our labels applied
	targetRepo := fluxsource.GitRepository{}
	target := obj.Spec.AdoptTargetRef

	var targetNs string

	// Some targets might use secrets for credentials. In this case we
	// need to be in the same namespace as the target so we can share the configuration
	// If namespace is not specified, we'll assume the same namespace as the supervisedgit
	if target.Namespace != "" {
		targetNs = target.Namespace
	} else {
		targetNs = obj.Namespace
	}
	if err := r.Get(ctx, types.NamespacedName{Name: target.Name, Namespace: targetNs}, &targetRepo); err != nil {
		return gitrepo, err
	}

	// We'll pass the spec from the target repo
	gitrepo.Spec = targetRepo.Spec

	// Ensure the target repo has our labels
	labels := targetRepo.GetLabels()

	if labels[synv1.SynManagedNameLabel] != obj.Name || labels[synv1.SynManagedNamespaceLabel] != obj.Namespace {
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[synv1.SynManagedNameLabel] = obj.Name
		labels[synv1.SynManagedNamespaceLabel] = obj.Namespace
		targetRepo.SetLabels(labels)

		if err = r.Update(ctx, &targetRepo); err != nil {
			return gitrepo, err
		}

		// If we're just taking this over, we'll need to pin the commit version
		if targetRepo.Spec.Reference.Commit == "" {
			targetRepo.Spec.Reference.Commit = obj.GetLastGitrepoRevisionSHA(targetRepo)
		}

		if err = r.Update(ctx, &targetRepo); err != nil {
			return gitrepo, err
		}
	}

	// If we don't have a last applied revision, we'll assumed that the last revision of target is applied
	if obj.GetLastAppliedRevision() == "" {
		obj.SetLastAppliedRevision(obj.GetLastGitrepoRevisionSHA(targetRepo))
		if err := r.Status().Update(ctx, obj); err != nil {
			return gitrepo, err
		}
	}

	// If we have a new revision approved, we'll update the target repo
	if targetRepo.Spec.Reference.Commit != obj.GetLastApprovedRevision() {
		targetRepo.Spec.Reference.Commit = obj.GetLastApprovedRevision()
		if err := r.Update(ctx, &targetRepo); err != nil {
			return gitrepo, err
		}
	}

	return gitrepo, err
}

func (r *SupervisedGitReconciler) EnsureSource(ctx context.Context, obj *synv1.SupervisedGit, repoSpec fluxsource.GitRepositorySpec) (sourceRepo fluxsource.GitRepository, err error) {
	log := log.FromContext(ctx)

	// Get the source repo
	sourceRepoLookupKey := types.NamespacedName{Name: obj.Name + "-source", Namespace: obj.Namespace}

	if err := r.Get(ctx, sourceRepoLookupKey, &sourceRepo); err != nil {
		// Create if it doesn't exist
		if apierrors.IsNotFound(err) {
			log.Info("Creating source gitrepo")
			sourceRepo = fluxsource.GitRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      obj.Name + "-source",
					Namespace: obj.Namespace,
				},
				Spec: repoSpec,
			}
			if err := controllerutil.SetControllerReference(obj, &sourceRepo, r.Scheme); err != nil {
				return sourceRepo, fmt.Errorf("unable to set controller reference: %w", err)
			}
			if err := r.Create(ctx, &sourceRepo); err != nil {
				log.Error(err, "unable to create source gitrepo")
				return sourceRepo, err
			}
		}
	}

	// Ensure the source repo has the correct spec
	sourceRepo.Spec.Reference.Commit = repoSpec.Reference.Commit
	if !reflect.DeepEqual(sourceRepo.Spec, repoSpec) {
		log.Info("Updating source gitrepo")
		sourceRepo.Spec = repoSpec
		sourceRepo.Spec.Reference.Commit = ""
		if err := r.Update(ctx, &sourceRepo); err != nil {
			log.Error(err, "unable to update source gitrepo")
			return sourceRepo, err
		}
	} else {
		// Set the commit back to empty to reflect the reality we want
		sourceRepo.Spec.Reference.Commit = ""
	}

	return sourceRepo, err
}

func (s *SupervisedGitReconciler) GetChildStatus(ctx context.Context, obj *synv1.SupervisedGit) (finished bool, err error) {
	log := log.FromContext(ctx)

	// Find any flux kustomize object that matches the label
	var fluxKustomizeList fluxkustomize.KustomizationList
	// listOpts := &client.ListOptions{}}
	// listOpts.LabelSelector = client.MatchingLabelsSelector{Selector: metav1.LabelSelectoA{MatchLabels: obj.GetAdopteesChildLabelSelectors()}}

	if err := s.List(ctx, &fluxKustomizeList, client.MatchingLabels(obj.GetAdopteesChildLabelSelectors())); err != nil {
		log.Error(err, "unable to list flux kustomizations")
		return false, err
	}

	// Find any flux helmrelease object that matches the label
	var fluxHelmreleaseList fluxhelm.HelmReleaseList
	if err := s.List(ctx, &fluxHelmreleaseList, client.MatchingLabels(obj.GetAdopteesChildLabelSelectors())); err != nil {
		log.Error(err, "unable to list flux helmreleases")
		return false, err
	}

	log.Info("Checking child kustomizations", "count", len(fluxKustomizeList.Items))
	kustProgressing := 0
	for _, kustomize := range fluxKustomizeList.Items {
		if obj.GetRevisionSHA(kustomize.Status.LastAppliedRevision) != obj.GetLastApprovedRevision() {
			log.Info("Child kustomization not synced", "name", kustomize.Name, "namespace", kustomize.Namespace)
			kustProgressing++
		}
		// We might use the flux isready condition check too
		if !conditions.IsReady(&kustomize) {
			log.Info("Child kustomization not ready", "name", kustomize.Name, "namespace", kustomize.Namespace)
		}
	}

	// Check every child helmrelease
	helmProgressing := 0
	for _, helmrelease := range fluxHelmreleaseList.Items {
		// We don't have the commit hash available on helmreleases.
		// We'll use the Ready condition instead

		if conditions.IsReady(&helmrelease) {
			log.Info("Child helmrelease not synced", "name", helmrelease.Name, "namespace", helmrelease.Namespace)
			helmProgressing++
		}
	}

	// If we do not find any child kustomizations or helmreleases, we'll flag as error, as we expect at least one
	if len(fluxKustomizeList.Items) == 0 && len(fluxHelmreleaseList.Items) == 0 {
		return false, fmt.Errorf("no child kustomizations or helmreleases found")
	}

	return kustProgressing+helmProgressing == 0, nil
}
