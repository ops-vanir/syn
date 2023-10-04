package controller

import (
	"context"
	"time"

	fluxkustomize "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	fluxsource "github.com/fluxcd/source-controller/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	synv1 "github.com/ops-vanir/syn/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func makeFluxRevision(revision string) (fluxrev string) {
	return "branch@sha1:" + revision
}

var _ = Describe("SupervisedGit Controller", func() {
	const (
		SupervisedGitName      = "supervisedgittest"
		SupervisedGitNamespace = "default"
		// Adopted gitrepo definitions
		TargetRepoName      = "myrepo"
		TargetRepoNamespace = "default"
		TargetRepoUrl       = "https://myrepo.com/repo.git"
		TargetRepoBranch    = "prod"

		Revision        = "4882e30a235a7d00a21be7e6b1a15cf501dec0fe"
		FailingRevision = "failingrev"
		NewRevision     = "averynewrevision"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Second * 5
	)

	var (
		sourceRepoLookupKey = &types.NamespacedName{Name: SupervisedGitName + "-source", Namespace: SupervisedGitNamespace}

		// targetRepo represents the existing gitrepo that we want to adopt
		targetRepo = &fluxsource.GitRepository{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "source.toolkit.fluxcd.io/v1",
				Kind:       "GitRepository",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      TargetRepoName,
				Namespace: TargetRepoNamespace,
				Labels: map[string]string{
					"original": "label",
				},
			},
			Spec: fluxsource.GitRepositorySpec{
				URL: TargetRepoUrl,
				Reference: &fluxsource.GitRepositoryRef{
					Branch: TargetRepoBranch,
				},
			},
			Status: fluxsource.GitRepositoryStatus{
				ObservedGeneration: 1,
				Artifact: &fluxsource.Artifact{
					Path:     "path",
					URL:      "url",
					Revision: makeFluxRevision(Revision),
					LastUpdateTime: metav1.Time{
						Time: time.Now(),
					},
				},
			},
		}

		fluxKustomization = &fluxkustomize.Kustomization{
			TypeMeta: metav1.TypeMeta{
				APIVersion: fluxkustomize.GroupVersion.Version,
				Kind:       "Kustomization",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mykustomization",
				Namespace: TargetRepoNamespace,
				Labels: map[string]string{
					"original":                         "label",
					"kustomize.toolkit.fluxcd.io/name": "foobar",
				},
			},
			Spec: fluxkustomize.KustomizationSpec{
				Path: "path",
				SourceRef: fluxkustomize.CrossNamespaceSourceReference{
					Kind:      "GitRepository",
					Name:      TargetRepoName,
					Namespace: TargetRepoNamespace,
				},
			},
			Status: fluxkustomize.KustomizationStatus{
				ObservedGeneration:    1,
				LastAttemptedRevision: makeFluxRevision(FailingRevision),
				LastAppliedRevision:   makeFluxRevision(Revision),
			},
		}

		// TODO REMOVE
		// changeRequest represents the change request for a given revision
		// chg = &synv1.ChangeRequest{
		// 	TypeMeta: metav1.TypeMeta{
		// 		APIVersion: "syn.servicenow.com/v1alpha1",
		// 		Kind:       "ChangeRequest",
		// 	},
		// 	ObjectMeta: metav1.ObjectMeta{
		// 		Name:      fmt.Sprintf("%s-%s", SupervisedGitName, Revision),
		// 		Namespace: SupervisedGitNamespace,
		// 	},
		// 	Spec: synv1.ChangeRequestSpec{
		// 		TargetRevision: Revision,
		// 	},
		// 	Status: synv1.ChangeRequestStatus{
		// 		RequestPhase:     synv1.ChangeRequestApproved,
		// 		ApprovedRevision: Revision,
		// 	},
		// }

		supervisedgit = &synv1.SupervisedGit{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "syn.servicenow.com/v1alpha1",
				Kind:       "SupervisedGit",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      SupervisedGitName,
				Namespace: SupervisedGitNamespace,
			},
			Spec: synv1.SupervisedGitSpec{
				AdoptTargetRef: synv1.CrossNamespaceObjectReference{
					Kind:      "GitRepository",
					Name:      TargetRepoName,
					Namespace: TargetRepoNamespace,
					ChildLabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kustomize.toolkit.fluxcd.io/name": "foobar",
						},
					},
				},
				ReconcileTimeLimit: &metav1.Duration{
					Duration: time.Second * 5,
				},
				Interval: &metav1.Duration{
					Duration: time.Second * 2,
				},
			},
		}
	)
	ctx := context.Background()
	// When we create a supervisedgit object with sourceref.kind gitrepository,
	// we expect the controller to create a gitrepository object with the same
	// name and spec adding -source to the name so it can monitor the revisions on the
	// git repository
	// We also expect the controller to set the .spec.ref.commit on the original repo
	// to the last approved revision
	Context("The supervisedgit controller can adopt flux gitrepositories", Label("supervisedgit"), func() {
		It("should manage the lifecycle of the gitrepo", func() {

			By("Having an existing gitrepo")
			Eventually(func() error {
				return k8sClient.Create(ctx, targetRepo)
			}, timeout, interval).Should(BeNil())

			By("Creating the supervisedgit")
			Expect(k8sClient.Create(ctx, supervisedgit)).Should(Succeed())

			By("Expecting the target gitrepo to be updated with labels")
			Eventually(func() (map[string]string, error) {
				found := fluxsource.GitRepository{}

				err := k8sClient.Get(ctx, types.NamespacedName{Name: TargetRepoName, Namespace: TargetRepoNamespace}, &found)
				if err != nil {
					return map[string]string{}, err
				}
				return found.GetLabels(), nil
			}, timeout, interval).Should(HaveKeyWithValue(synv1.SynManagedNameLabel, SupervisedGitName))

			By("Not overwriting original labels")
			Eventually(func() (map[string]string, error) {
				found := fluxsource.GitRepository{}

				err := k8sClient.Get(ctx, types.NamespacedName{Name: TargetRepoName, Namespace: TargetRepoNamespace}, &found)
				if err != nil {
					return map[string]string{}, err
				}
				return found.GetLabels(), nil

			}, timeout, interval).Should(HaveKeyWithValue("original", "label"))

			By("Expecting the source gitrepo to be created with the same spec but not pinned")
			sourceRepo := fluxsource.GitRepository{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, *sourceRepoLookupKey, &sourceRepo)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(sourceRepo.Spec.URL).Should(Equal(TargetRepoUrl))
			Expect(sourceRepo.Spec.Reference.Branch).Should(Equal(TargetRepoBranch))
			Expect(sourceRepo.Spec.Reference.Tag).Should(Equal(targetRepo.Spec.Reference.Tag))
			Expect(sourceRepo.Spec.Reference.SemVer).Should(Equal(targetRepo.Spec.Reference.SemVer))
			Expect(sourceRepo.Spec.Reference.Commit).Should(Equal(""))

			By("Allowing the source to pick up new revisions")
			sourceRepo.Status.Artifact = &fluxsource.Artifact{
				Path:     "path",
				URL:      "url",
				Revision: FailingRevision,
				LastUpdateTime: metav1.Time{
					Time: time.Now(),
				},
			}
			Expect(k8sClient.Status().Update(ctx, &sourceRepo)).Should(Succeed())

			By("Expecting the supervisedgit to be updated with the last seen revision")
			Eventually(func() string {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: SupervisedGitName, Namespace: SupervisedGitNamespace}, supervisedgit)
				if err != nil {
					return ""
				}
				return supervisedgit.GetLastSeenRevision()
			}, timeout, interval).Should(BeIdenticalTo(FailingRevision))

			By("Expecting Ready condition to be Unknown")
			desiredReadyCondition := metav1.Condition{
				Type:   meta.ReadyCondition,
				Status: metav1.ConditionUnknown,
			}
			Eventually(func() []metav1.Condition {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: SupervisedGitName, Namespace: SupervisedGitNamespace}, supervisedgit)
				if err != nil {
					return nil
				}
				return supervisedgit.GetConditions()
			}, timeout, interval).Should(
				ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(desiredReadyCondition.Type),
					"Status": Equal(desiredReadyCondition.Status)})))
		})
	})

	// The supervisedgit controller uses CHG to track when new revisions from the
	// source flux repository are enabled on the target flux repository.
	Context("The supervisedgit should manage the approved revision", Label("supervisedgit"), func() {
		It("should pin the last approved revision on the target repository", func() {
			foundchg := synv1.ChangeRequest{}

			By("Creating a CHG for last seen revision")
			// Ensure CHG for last seen revision has been created by controller
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: SupervisedGitName + "-" + FailingRevision, Namespace: SupervisedGitNamespace}, &foundchg)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(foundchg.Spec.TargetRevision).Should(Equal(FailingRevision))

			By("When a CHG is approved")
			foundchg.Status.RequestPhase = synv1.ChangeRequestApproved
			foundchg.Status.ApprovedRevision = FailingRevision
			Expect(k8sClient.Status().Update(ctx, &foundchg)).Should(Succeed())

			By("Setting the supervisedgit approved revision status")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: SupervisedGitName, Namespace: SupervisedGitNamespace}, supervisedgit)
				if err != nil {
					return false
				}
				return supervisedgit.GetLastApprovedRevision() == FailingRevision
			}, timeout, interval).Should(BeTrue())

			By("Ensuring that the target repository is pinned to the last approved revision")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: TargetRepoName, Namespace: TargetRepoNamespace}, targetRepo)
				if err != nil {
					return false
				}
				return targetRepo.Spec.Reference.Commit == FailingRevision
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("The supervisedgit should manage the lifecycle of a change", Label("supervisedgit"), func() {

		It("should should detect when children of adopted resources fail to reconcile new revisions", func() {
			By("Creating a child resource")
			Expect(k8sClient.Create(ctx, fluxKustomization)).Should(Succeed())

			By("Not changing last applied revision when children have not reconciled")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: SupervisedGitName, Namespace: SupervisedGitNamespace}, supervisedgit)
				if err != nil {
					return false
				}
				return supervisedgit.GetLastAppliedRevision() == FailingRevision
			}, timeout, interval).Should(BeFalse())

			By("Setting the Stalled Condition")
			desiredStalledCondition := metav1.Condition{
				Type:   meta.StalledCondition,
				Status: metav1.ConditionTrue,
			}

			Eventually(func() []metav1.Condition {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: SupervisedGitName, Namespace: SupervisedGitNamespace}, supervisedgit)
				if err != nil {
					return nil
				}
				return supervisedgit.GetConditions()
			}, timeout, interval).Should(
				ContainElements(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(desiredStalledCondition.Type),
					"Status": Equal(desiredStalledCondition.Status)})))

			By("Closing the CHG as failed after failed reconciliation")
			foundchg := synv1.ChangeRequest{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: SupervisedGitName + "-" + FailingRevision, Namespace: SupervisedGitNamespace}, &foundchg)
				if err != nil {
					return false
				}
				return foundchg.Spec.CloseCode == synv1.CloseFail
			}, timeout*4, interval).Should(BeTrue())
		})

		It("Should detect when children of adopted resources successfully reconciles", Label("supervisedgit"), func() {

			By("Creating a new approved revision")
			sourceRepo := fluxsource.GitRepository{}
			Expect(k8sClient.Get(ctx, *sourceRepoLookupKey, &sourceRepo)).Should(Succeed())
			sourceRepo.Status.Artifact.Revision = makeFluxRevision(NewRevision)
			Expect(k8sClient.Status().Update(ctx, &sourceRepo)).Should(Succeed())
			newChg := synv1.ChangeRequest{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: SupervisedGitName + "-" + NewRevision, Namespace: SupervisedGitNamespace}, &newChg)
				if err != nil {
					return false
				}
				return true
			}, timeout*3, interval).Should(BeTrue())
			newChg.Status.RequestPhase = synv1.ChangeRequestApproved
			newChg.Status.ApprovedRevision = NewRevision
			Expect(k8sClient.Status().Update(ctx, &newChg)).Should(Succeed())

			By("Setting the child resource to a reconciled state")
			fluxKustomization.Status.LastAppliedRevision = makeFluxRevision(NewRevision)
			Expect(k8sClient.Status().Update(ctx, fluxKustomization)).Should(Succeed())

			By("Setting the supervisedgit last applied revision status")
			Eventually(func() string {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: SupervisedGitName, Namespace: SupervisedGitNamespace}, supervisedgit)
				if err != nil {
					return ""
				}
				return supervisedgit.GetLastAppliedRevision()
			}, timeout*3, interval).Should(BeIdenticalTo(NewRevision))

			By("Closing the CHG as successful")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: SupervisedGitName + "-" + NewRevision, Namespace: SupervisedGitNamespace}, &newChg)
				if err != nil {
					return false
				}
				return newChg.Spec.CloseCode == synv1.CloseSuccess
			}, timeout*3, interval).Should(BeTrue())
		})
	})
})
