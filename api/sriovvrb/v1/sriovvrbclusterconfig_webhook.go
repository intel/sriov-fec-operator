// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2023 Intel Corporation

package v1

import (
	"fmt"

	"github.com/smart-edge-open/sriov-fec-operator/pkg/common/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var vrbclusterconfiglog = utils.NewLogger()

func (r *SriovVrbClusterConfig) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-sriovvrb-intel-com-v1-sriovvrbclusterconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=sriovvrb.intel.com,resources=sriovvrbclusterconfigs,verbs=create;update,versions=v1,name=vsriovvrbclusterconfig.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &SriovVrbClusterConfig{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *SriovVrbClusterConfig) ValidateCreate() error {
	vrbclusterconfiglog.WithField("name", r.Name).Info("validate create")
	if errs := validate(r.Spec); len(errs) != 0 {
		return apierrors.NewInvalid(schema.GroupKind{Group: "sriovvrb.intel.com", Kind: "SriovVrbClusterConfig"}, r.Name, errs)
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *SriovVrbClusterConfig) ValidateUpdate(_ runtime.Object) error {
	vrbclusterconfiglog.WithField("name", r.Name).Info("validate update")
	if errs := validate(r.Spec); len(errs) != 0 {
		return apierrors.NewInvalid(schema.GroupKind{Group: "sriovvrb.intel.com", Kind: "SriovVrbClusterConfig"}, r.Name, errs)
	}
	return nil
}

func validate(spec SriovVrbClusterConfigSpec) (errs field.ErrorList) {

	validators := []func(spec SriovVrbClusterConfigSpec) field.ErrorList{
		ambiguousBBDevConfigValidator,
		vrb1VfAmountValidator,
		vrb1NumQueueGroupsValidator,
		vrb1NumAqsPerGroupsValidator,
		vrb2VfAmountValidator,
		vrb2NumQueueGroupsValidator,
	}

	for _, validate := range validators {
		errs = append(errs, validate(spec)...)
	}

	return errs
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *SriovVrbClusterConfig) ValidateDelete() error {
	vrbclusterconfiglog.WithField("name", r.Name).Info("validate delete")

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func hasAmbiguousBBDevConfigs(bbDevConfig BBDevConfig) *field.Error {

	var found interface{}
	for _, config := range []interface{}{bbDevConfig.VRB1, bbDevConfig.VRB2} {
		if !isNil(config) && !isNil(found) {
			return field.Forbidden(
				field.NewPath("spec").Child("physicalFunction").Child("bbDevConfig"),
				"specified bbDevConfig cannot contain multiple configurations")
		}
		found = config
	}
	return nil
}

func ambiguousBBDevConfigValidator(spec SriovVrbClusterConfigSpec) (errs field.ErrorList) {
	if err := hasAmbiguousBBDevConfigs(spec.PhysicalFunction.BBDevConfig); err != nil {
		errs = append(errs, err)
		return
	}

	if spec.PhysicalFunction.BBDevConfig.VRB1 == nil &&
		spec.PhysicalFunction.BBDevConfig.VRB2 == nil {

		err := field.Forbidden(
			field.NewPath("spec").Child("physicalFunction").Child("bbDevConfig"),
			"bbDevConfig section cannot be empty")
		errs = append(errs, err)
		return
	}
	return
}

func vrb1VfAmountValidator(spec SriovVrbClusterConfigSpec) (errs field.ErrorList) {

	validate := func(accConfig *VRB1BBDevConfig, vfAmount int, path *field.Path) *field.Error {
		if accConfig == nil {
			return nil
		}

		if accConfig.NumVfBundles > vrb1maxVfNums {
			return field.Invalid(
				path,
				accConfig.NumVfBundles,
				fmt.Sprintf("value should not be greater than %d", vrb1maxVfNums),
			)
		}

		if vfAmount != accConfig.NumVfBundles {
			return field.Invalid(
				path,
				accConfig.NumVfBundles,
				"value should be the same as physicalFunction.vfAmount")
		}
		return nil
	}

	if err := validate(
		spec.PhysicalFunction.BBDevConfig.VRB1,
		spec.PhysicalFunction.VFAmount,
		field.NewPath("spec").Child("physicalFunction").Child("bbDevConfig", "vrb1", "numVfBundles"),
	); err != nil {
		errs = append(errs, err)
	}

	return
}

func vrb1NumQueueGroupsValidator(spec SriovVrbClusterConfigSpec) (errs field.ErrorList) {

	validate := func(accConfig *VRB1BBDevConfig, path *field.Path) *field.Error {
		if accConfig == nil {
			return nil
		}

		downlink4g := accConfig.Downlink4G.NumQueueGroups
		uplink4g := accConfig.Uplink4G.NumQueueGroups
		downlink5g := accConfig.Downlink5G.NumQueueGroups
		uplink5g := accConfig.Uplink5G.NumQueueGroups
		qfft := accConfig.QFFT.NumQueueGroups

		if sum := downlink5g + uplink5g + downlink4g + uplink4g + qfft; sum > vrb1maxQueueGroups {
			return field.Invalid(
				field.NewPath("spec", "physicalFunction", "bbDevConfig", "vrb1", "[downlink4G|uplink4G|downlink5G|uplink5G|qfft]", "numQueueGroups"),
				sum,
				fmt.Sprintf("sum of all numQueueGroups should not be greater than %d", vrb1maxQueueGroups),
			)
		}
		return nil
	}

	if err := validate(spec.PhysicalFunction.BBDevConfig.VRB1, field.NewPath("spec", "physicalFunction", "bbDevConfig", "vrb1", "[downlink4G|uplink4G|downlink5G|uplink5G|qfft]", "numQueueGroups")); err != nil {
		errs = append(errs, err)
	}

	return
}

func vrb1NumAqsPerGroupsValidator(spec SriovVrbClusterConfigSpec) (errs field.ErrorList) {

	validate := func(accConfig *VRB1BBDevConfig, path *field.Path) *field.Error {
		if accConfig == nil {
			return nil
		}

		downlink4g := accConfig.Downlink4G.NumAqsPerGroups
		uplink4g := accConfig.Uplink4G.NumAqsPerGroups
		downlink5g := accConfig.Downlink5G.NumAqsPerGroups
		uplink5g := accConfig.Uplink5G.NumAqsPerGroups
		qfft := accConfig.QFFT.NumAqsPerGroups

		if downlink4g > vrb1maxQueueGroups || uplink4g > vrb1maxQueueGroups || downlink5g > vrb1maxQueueGroups ||
			uplink5g > vrb1maxQueueGroups || qfft > vrb1maxQueueGroups {
			return field.Invalid(
				field.NewPath("spec", "physicalFunction", "bbDevConfig", "vrb1", "[downlink4G|uplink4G|downlink5G|uplink5G|qfft]", "NumAqsPerGroups"),
				"incorrect",
				fmt.Sprintf("NumAqsPerGroups should not be greater than %d", vrb1maxQueueGroups),
			)
		}
		return nil
	}

	if err := validate(spec.PhysicalFunction.BBDevConfig.VRB1, field.NewPath("spec", "physicalFunction", "bbDevConfig", "vrb1", "[downlink4G|uplink4G|downlink5G|uplink5G|qfft]", "NumAqsPerGroups")); err != nil {
		errs = append(errs, err)
	}

	return

}

func vrb2VfAmountValidator(spec SriovVrbClusterConfigSpec) (errs field.ErrorList) {

	validate := func(accConfig *VRB2BBDevConfig, vfAmount int, path *field.Path) *field.Error {
		if accConfig == nil {
			return nil
		}

		if vfAmount != accConfig.NumVfBundles {
			return field.Invalid(
				path,
				accConfig.NumVfBundles,
				"value should be the same as physicalFunction.vfAmount")
		}
		return nil
	}

	if err := validate(
		spec.PhysicalFunction.BBDevConfig.VRB2,
		spec.PhysicalFunction.VFAmount,
		field.NewPath("spec").Child("physicalFunction").Child("bbDevConfig", "vrb2", "numVfBundles"),
	); err != nil {
		errs = append(errs, err)
	}

	return
}

func vrb2NumQueueGroupsValidator(spec SriovVrbClusterConfigSpec) (errs field.ErrorList) {

	validate := func(accConfig *VRB2BBDevConfig, path *field.Path) *field.Error {
		if accConfig == nil {
			return nil
		}

		downlink4g := accConfig.Downlink4G.NumQueueGroups
		uplink4g := accConfig.Uplink4G.NumQueueGroups
		downlink5g := accConfig.Downlink5G.NumQueueGroups
		uplink5g := accConfig.Uplink5G.NumQueueGroups
		qfft := accConfig.QFFT.NumQueueGroups
		qmld := accConfig.QMLD.NumQueueGroups

		if sum := downlink5g + uplink5g + downlink4g + uplink4g + qfft + qmld; sum > vrb2maxQueueGroups {
			return field.Invalid(
				field.NewPath("spec", "physicalFunction", "bbDevConfig", "vrb2", "[downlink4G|uplink4G|downlink5G|uplink5G|qfft|qmld]", "numQueueGroups"),
				sum,
				fmt.Sprintf("sum of all numQueueGroups should not be greater than %d", vrb2maxQueueGroups),
			)
		}
		return nil
	}

	if err := validate(spec.PhysicalFunction.BBDevConfig.VRB2, field.NewPath("spec", "physicalFunction", "bbDevConfig", "vrb2", "[downlink4G|uplink4G|downlink5G|uplink5G|qfft|qmld]", "numQueueGroups")); err != nil {
		errs = append(errs, err)
	}

	return
}
