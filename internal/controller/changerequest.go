package controller

import (
	"context"
	"fmt"

	synv1 "github.com/ops-vanir/syn/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (s *SupervisedGitReconciler) EnsureCHG(ctx context.Context, obj *synv1.SupervisedGit) (chg synv1.ChangeRequest, err error) {
	log := log.FromContext(ctx)

	chgLookupKey := types.NamespacedName{Name: fmt.Sprint(obj.Name, "-", obj.GetLastSeenRevision()), Namespace: obj.Namespace}

	if err := s.Get(ctx, chgLookupKey, &chg); err != nil {

		// If the CHG doesn't exist, we'll create it
		if apierrors.IsNotFound(err) {
			chg = synv1.ChangeRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      chgLookupKey.Name,
					Namespace: chgLookupKey.Namespace,
				},
				Spec: synv1.ChangeRequestSpec{
					TargetRevision: obj.GetLastSeenRevision(),
				},
			}
			if err := controllerutil.SetControllerReference(obj, &chg, s.Scheme); err != nil {
				log.Error(err, "unable to set controller reference")
				return chg, err
			}
			if err := s.Create(ctx, &chg); err != nil {
				log.Error(err, "unable to create CHG")
				return chg, err
			}
			// return the newly created CHG
			return chg, nil
		}
		return chg, err
	}

	return chg, nil
}

func (s *SupervisedGitReconciler) CloseCHG(ctx context.Context, obj *synv1.SupervisedGit, closecode string) (err error) {
	log := log.FromContext(ctx)

	chg := synv1.ChangeRequest{}

	chgLookupKey := types.NamespacedName{Name: fmt.Sprint(obj.Name, "-", obj.GetLastApprovedRevision()), Namespace: obj.Namespace}

	if err := s.Get(ctx, chgLookupKey, &chg); err != nil {
		return err
	}

	if chg.Spec.CloseCode != closecode {
		chg.Spec.CloseCode = closecode
		if err := s.Update(ctx, &chg); err != nil {
			log.Error(err, "unable to update CHG")
			return err
		}
	}

	return nil

}
