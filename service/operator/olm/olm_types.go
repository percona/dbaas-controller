// dbaas-controller
// Copyright (C) 2020 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// Package operator contains logic related to kubernetes operators.
package olm

import "time"

type APIVersion string

const (
	APIVersionV1             APIVersion = "v1"
	APIVersionCoreosV1       APIVersion = "operators.coreos.com/v1"
	APIVersionCoreosV1Alpha1 APIVersion = "operators.coreos.com/v1alpha1"
)

type ObjectKind string

const (
	ObjectKindOperatorGroup ObjectKind = "OperatorGroup"
	ObjectKindSubscription  ObjectKind = "Subscription"
)

// Approval is the user approval policy for an InstallPlan.
// It must be one of "Automatic" or "Manual".
type Approval string

const (
	ApprovalAutomatic Approval = "Automatic"
	ApprovalManual    Approval = "Manual"
)

type OperatorGroupList struct {
	APIVersion APIVersion `json:"apiVersion,omitempty"`
	Kind       ObjectKind `json:"kind,omitempty"`
	Metadata   struct {
		ResourceVersion string `json:"resourceVersion,omitempty" yaml:"resourceVersion"`
	} `json:"metadata,omitempty" yaml:"metadata"`
	Items []OperatorGroup `json:"items,omitempty" yaml:"items"`
}

type OperatorGroup struct {
	APIVersion APIVersion            `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       ObjectKind            `json:"kind,omitempty" yaml:"kind,omitempty"`
	Metadata   OperatorGroupMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec       OperatorGroupSpec     `json:"spec,omitempty" yaml:"spec,omitempty"`
}

type OperatorGroupMetadata struct {
	Annotations       map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	CreationTimestamp time.Time         `json:"creationTimestamp,omitempty" yaml:"creationTimestamp,omitempty"`
	Generation        int               `json:"generation,omitempty" yaml:"generation,omitempty"`
	Name              string            `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace         string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	ResourceVersion   string            `json:"resourceVersion,omitempty" yaml:"resourceVersion,omitempty"`
	UID               string            `json:"uid,omitempty" yaml:"uid,omitempty"`
}

type OperatorGroupSpec struct {
	TargetNamespaces []string `json:"targetNamespaces,omitempty" yaml:"targetNamespaces,omitempty"`
	UpgradeStrategy  string   `json:"upgradeStrategy,omitempty" yaml:"upgradeStrategy,omitempty"`
}

type Subscription struct {
	APIVersion APIVersion           `yaml:"apiVersion" json:"apiVersion"`
	Kind       ObjectKind           `yaml:"kind" json:"kind"`
	Metadata   SubscriptionMetadata `yaml:"metadata" json:"metadata"`
	Spec       SubscriptionSpec     `yaml:"spec" json:"spec"`
}

type SubscriptionMetadata struct {
	Name      string `yaml:"name" json:"name"`
	Namespace string `yaml:"namespace" json:"namespace"`
}

type SubscriptionSpec struct {
	Channel             string   `yaml:"channel" json:"channel"`
	Name                string   `yaml:"name" json:"name"`
	Source              string   `yaml:"source" json:"source"`
	SourceNamespace     string   `yaml:"sourceNamespace" json:"sourceNamespace"`
	InstallPlanApproval Approval `yaml:"installPlanApproval" json:"installPlanApproval"`
	StartingCSV         string   `yaml:"startingCSV" json:"startingCSV"`
}
