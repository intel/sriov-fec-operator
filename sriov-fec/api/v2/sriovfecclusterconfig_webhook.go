// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package v2

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var sriovfecclusterconfiglog = logf.Log.WithName("sriovfecclusterconfig-resource")

func (r *SriovFecClusterConfig) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-sriovfec-intel-com-v2-sriovfecclusterconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=sriovfec.intel.com,resources=sriovfecclusterconfigs,verbs=create;update,versions=v2,name=vsriovfecclusterconfig.kb.io,admissionReviewVersions={v1}

var _ webhook.Validator = &SriovFecClusterConfig{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *SriovFecClusterConfig) ValidateCreate() error {
	sriovfecclusterconfiglog.Info("validate create", "name", r.Name)
	if errs := r.validate(); len(errs) != 0 {
		return apierrors.NewInvalid(schema.GroupKind{Group: "sriovfec.intel.com", Kind: "SriovFecClusterConfig"}, r.Name, errs)
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *SriovFecClusterConfig) ValidateUpdate(_ runtime.Object) error {
	sriovfecclusterconfiglog.Info("validate update", "name", r.Name)
	if errs := r.validate(); len(errs) != 0 {
		return apierrors.NewInvalid(schema.GroupKind{Group: "sriovfec.intel.com", Kind: "SriovFecClusterConfig"}, r.Name, errs)
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *SriovFecClusterConfig) ValidateDelete() error {
	sriovfecclusterconfiglog.Info("validate delete", "name", r.Name)
	//do nothing
	return nil
}

func (r *SriovFecClusterConfig) validate() (errs field.ErrorList) {
	if r.Spec.PhysicalFunction.BBDevConfig.N3000 != nil && r.Spec.PhysicalFunction.BBDevConfig.ACC100 != nil {
		err := field.Forbidden(
			field.NewPath("spec").Child("physicalFunction").Child("bbDevConfig"),
			"specified bbDevConfig cannot contain acc100 and n3000 configuration in the same time")

		errs = append(errs, err)
	}

	n3000BBDevConfig := r.Spec.PhysicalFunction.BBDevConfig.N3000
	if n3000BBDevConfig == nil {
		return
	}

	queuePath := field.NewPath("spec").
		Child("physicalFunction").
		Child("bbDevConfig", "n3000", "uplink", "queues")

	if err := validateN3000Queues(queuePath, n3000BBDevConfig.Uplink.Queues); err != nil {
		errs = append(errs, err)
	}

	queuePath = field.NewPath("spec").
		Child("physicalFunction").
		Child("bbDevConfig", "n3000", "downlink", "queues")

	if err := validateN3000Queues(queuePath, n3000BBDevConfig.Downlink.Queues); err != nil {
		errs = append(errs, err)
	}

	return
}

func validateN3000Queues(qID *field.Path, queues UplinkDownlinkQueues) *field.Error {
	total := queues.VF0 + queues.VF1 + queues.VF2 + queues.VF3 + queues.VF4 + queues.VF5 + queues.VF5 + queues.VF6 + queues.VF7
	if total > 32 {
		return field.Invalid(qID, total, "sum of all specified queues must be no more than 32")
	}
	return nil
}
