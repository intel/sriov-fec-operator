// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2026 Intel Corporation

package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
 * What to do with a PF is decided from the requested configuration, a record
 * of the configuration last applied to the PF, and the state of the device:
 *
 *   - everything matches                      -> leave the PF alone
 *   - everything matches, pf_bb_config is dead -> start pf_bb_config again
 *   - anything else                           -> reconfigure the PF
 *
 * The record is kept in an annotation of the node config CR, so that it
 * survives a restart of the daemon. Before, the last applied configuration
 * only lived in memory: after a restart every PF looked new and was torn
 * down and reconfigured, under the workloads using its VFs; and within one
 * daemon run, a failed attempt left the old record in place, so going back
 * to that configuration skipped a half torn down PF.
 */

const (
	fecAppliedConfigAnnotation = "sriovfec.intel.com/applied-config"
	vrbAppliedConfigAnnotation = "sriovvrb.intel.com/applied-config"
)

// appliedConfigs maps the PCI address of a PF to a hash of the configuration
// last applied to it successfully.
type appliedConfigs map[string]string

func configHash(cfg interface{}) string {
	data, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// readAppliedConfigs returns the records stored on obj, and whether the
// annotation exists at all. A malformed annotation counts as present and
// empty, so that every PF is reconfigured.
func readAppliedConfigs(obj metav1.Object, key string) (appliedConfigs, bool) {
	value, ok := obj.GetAnnotations()[key]
	if !ok {
		return appliedConfigs{}, false
	}
	applied := appliedConfigs{}
	if err := json.Unmarshal([]byte(value), &applied); err != nil {
		return appliedConfigs{}, true
	}
	return applied, true
}

// storeAppliedConfigs saves the records on obj with a merge patch; obj is
// updated in place. Changing an annotation does not change the generation,
// so it does not trigger a reconcile.
func storeAppliedConfigs(ctx context.Context, c client.Client, obj client.Object, key string, applied appliedConfigs) error {
	data, err := json.Marshal(applied)
	if err != nil {
		return err
	}
	if current, ok := obj.GetAnnotations()[key]; ok && current == string(data) {
		return nil
	}
	base := obj.DeepCopyObject().(client.Object)
	annotations := map[string]string{}
	for k, v := range obj.GetAnnotations() {
		annotations[k] = v
	}
	annotations[key] = string(data)
	obj.SetAnnotations(annotations)
	if err := c.Patch(ctx, obj, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("failed to record the applied configuration: %w", err)
	}
	return nil
}

type pfAction int

const (
	pfNoAction pfAction = iota
	pfRestartPfBBConfig
	pfReconfigure
)

func (a pfAction) String() string {
	switch a {
	case pfRestartPfBBConfig:
		return "restart pf_bb_config"
	case pfReconfigure:
		return "reconfigure"
	}
	return "none"
}

// pfRequest is the requested configuration of a PF.
type pfRequest struct {
	pfDriver string
	vfDriver string
	vfAmount int
	hash     string
}

// pfDeviceState is what is found on the node for a PF.
type pfDeviceState struct {
	pfDriver  string
	vfDrivers []string
}

func sameDriver(a, b string) bool {
	norm := func(s string) string { return strings.ReplaceAll(strings.ToLower(s), "_", "-") }
	return norm(a) == norm(b)
}

// planPFAction decides what to do with one PF. req is nil for a PF that is
// not in the spec. pfBBConfigState is only called when the answer depends on
// it.
func planPFAction(req *pfRequest, applied string, state pfDeviceState, staleWrite bool,
	pfBBConfigState func() procState) (pfAction, string) {

	if staleWrite {
		return pfReconfigure, "a sysfs write was left unfinished by a previous daemon instance"
	}

	if req == nil {
		if len(state.vfDrivers) > 0 {
			return pfReconfigure, fmt.Sprintf("not in the spec but has %d VFs", len(state.vfDrivers))
		}
		return pfNoAction, ""
	}

	switch {
	case applied == "":
		return pfReconfigure, "no record of a configuration applied to this PF"
	case applied != req.hash:
		return pfReconfigure, "requested configuration differs from the applied one"
	case !sameDriver(state.pfDriver, req.pfDriver):
		return pfReconfigure, fmt.Sprintf("PF driver is %q, requested %q", state.pfDriver, req.pfDriver)
	case len(state.vfDrivers) != req.vfAmount:
		return pfReconfigure, fmt.Sprintf("PF has %d VFs, requested %d", len(state.vfDrivers), req.vfAmount)
	}
	for _, driver := range state.vfDrivers {
		if !sameDriver(driver, req.vfDriver) {
			return pfReconfigure, fmt.Sprintf("a VF is bound to %q, requested %q", driver, req.vfDriver)
		}
	}

	// pf_bb_config only keeps running for a PF bound to vfio-pci
	if sameDriver(req.pfDriver, utils.VfioPci) && pfBBConfigState != nil && pfBBConfigState() == procDead {
		return pfRestartPfBBConfig, "pf_bb_config is not running; the PF and its VFs are configured"
	}
	return pfNoAction, ""
}

// adoptCurrentConfiguration tells whether, without any record of an applied
// configuration (first run after an upgrade from a version without
// records), the current configuration can be taken as applied: the node
// config reports the current generation as successfully configured.
func adoptCurrentConfiguration(recordPresent bool, condition metav1.Condition, generation int64) bool {
	return !recordPresent &&
		condition.Reason == string(ConfigurationSucceeded) &&
		condition.ObservedGeneration == generation
}

// refusedBeforeTeardown tells whether an error from applying a PF's
// configuration means that nothing was changed on the PF yet.
func refusedBeforeTeardown(err error) bool {
	var inUse *DeviceInUseError
	return errors.As(err, &inUse)
}
