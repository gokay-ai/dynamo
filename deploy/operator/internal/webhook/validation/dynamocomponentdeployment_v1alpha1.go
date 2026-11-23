/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package validation

import (
	"strings"

	nvidiacomv1alpha1 "github.com/ai-dynamo/dynamo/deploy/operator/api/v1alpha1"
	"github.com/ai-dynamo/dynamo/deploy/operator/internal/consts"
	componentdeployment "github.com/ai-dynamo/dynamo/deploy/operator/internal/controller/component"
	"github.com/ai-dynamo/dynamo/deploy/operator/internal/dynamo"
	networkingv1 "k8s.io/api/networking/v1"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// validateDynamoComponentDeploymentV1alpha1 validates newDCD; oldDCD is nil on create.
// newDCD must be non-nil.
func (v *dynamoComponentDeploymentValidation) validateDynamoComponentDeploymentV1alpha1(
	newDCD, oldDCD *nvidiacomv1alpha1.DynamoComponentDeployment,
) field.ErrorList {
	if !hasDynamoComponentDeploymentV1alpha1CompatibilityFields(newDCD) &&
		!v.hasRuntimeVersionSource(runtimeVersionSourceV1Alpha1) {
		return nil
	}
	allErrs := v.validateDynamoComponentDeploymentSpecV1alpha1(
		&newDCD.Spec,
		field.NewPath("spec"),
		newDCD.GetDynamoNamespace(),
	)
	if operatorOwnedDCDNameExemption(v.ctx, newDCD, v.operatorPrincipal) {
		return allErrs
	}
	if oldDCD != nil && !sameDynamoGraphDeploymentController(newDCD, oldDCD) {
		oldDCD = nil
	}

	// The original alpha service name becomes the component label without lowercasing.
	if oldDCD == nil || newDCD.Spec.ServiceName != oldDCD.Spec.ServiceName {
		if messages := k8svalidation.IsValidLabelValue(newDCD.Spec.ServiceName); len(messages) > 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "serviceName"), newDCD.Spec.ServiceName, "generated "+consts.KubeLabelDynamoComponent+" label value is invalid: "+strings.Join(messages, "; ")))
		}
	}
	return append(allErrs, v.validateDynamoComponentDeploymentIngressV1alpha1(newDCD, oldDCD, field.NewPath("spec", "ingress"))...)
}

// validateDynamoComponentDeploymentSpecV1alpha1 validates newSpec.
// newSpec and fldPath must not be nil.
func (v *dynamoComponentDeploymentValidation) validateDynamoComponentDeploymentSpecV1alpha1(
	newSpec *nvidiacomv1alpha1.DynamoComponentDeploymentSpec,
	fldPath *field.Path,
	dynamoNamespace string,
) field.ErrorList {
	allErrs := v.validateDynamoComponentDeploymentSharedSpecV1alpha1(
		&newSpec.DynamoComponentDeploymentSharedSpec,
		fldPath,
		dynamoNamespace,
	)

	return allErrs
}

// validateDynamoComponentDeploymentIngressV1alpha1 checks rendered legacy Ingress backend names at the source ingress field.
// newDCD and fldPath must be non-nil; oldDCD is nil when no matching old DCD existed.
func (v *sharedValidation) validateDynamoComponentDeploymentIngressV1alpha1(newDCD, oldDCD *nvidiacomv1alpha1.DynamoComponentDeployment, fldPath *field.Path) field.ErrorList {
	ingress := newDCD.Spec.Ingress
	if ingress == nil || !componentdeployment.IngressEnabled(ingress.Enabled, ingress.IngressControllerClassName) {
		return nil
	}
	rendered := dynamo.GenerateComponentIngress(v.ctx, newDCD.Name, newDCD.Namespace, dynamo.IngressSpec{IngressControllerClassName: ingress.IngressControllerClassName})
	backend := &rendered.Spec.Rules[0].HTTP.Paths[0].Backend
	var oldBackend *networkingv1.IngressBackend
	if oldDCD != nil && oldDCD.Spec.Ingress != nil && componentdeployment.IngressEnabled(oldDCD.Spec.Ingress.Enabled, oldDCD.Spec.Ingress.IngressControllerClassName) {
		oldIngress := dynamo.GenerateComponentIngress(v.ctx, oldDCD.Name, oldDCD.Namespace, dynamo.IngressSpec{IngressControllerClassName: oldDCD.Spec.Ingress.IngressControllerClassName})
		oldBackend = &oldIngress.Spec.Rules[0].HTTP.Paths[0].Backend
	}
	return v.validateIngressBackend(backend, oldBackend, fldPath)
}
