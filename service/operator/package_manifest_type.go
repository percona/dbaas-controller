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
package operator

import "time"

// PackageManifests hold the response for package manifests list.
type PackageManifests struct {
	APIVersion string         `json:"apiVersion,omitempty"`
	Items      []PMItem       `json:"items,omitempty"`
	Kind       string         `json:"kind,omitempty"`
	Metadata   PMItemMetadata `json:"metadata,omitempty"`
}

// PMLabels represents the PackageManifest labels.
type PMLabels struct {
	Catalog                      string `json:"catalog,omitempty"`
	CatalogNamespace             string `json:"catalog-namespace,omitempty"`
	OperatorframeworkIoArchAmd64 string `json:"operatorframework.io/arch.amd64,omitempty"`
	OperatorframeworkIoOsLinux   string `json:"operatorframework.io/os.linux,omitempty"`
	Provider                     string `json:"provider,omitempty"`
	ProviderURL                  string `json:"provider-url,omitempty"`
}

// PMItemMetadata holds the metadata for each package manifest.
type PMItemMetadata struct {
	CreationTimestamp time.Time `json:"creationTimestamp,omitempty"`
	Labels            PMLabels  `json:"labels,omitempty"`
	Name              string    `json:"name,omitempty"`
	Namespace         string    `json:"namespace,omitempty"`
}

// PMAnnotations holds package manifests annotations.
type PMAnnotations struct {
	AlmExamples                          string    `json:"alm-examples,omitempty"`
	Capabilities                         string    `json:"capabilities,omitempty"`
	Categories                           string    `json:"categories,omitempty"`
	Certified                            string    `json:"certified,omitempty"`
	ContainerImage                       string    `json:"containerImage,omitempty"`
	CreatedAt                            time.Time `json:"createdAt,omitempty"`
	Description                          string    `json:"description,omitempty"`
	OperatorhubIoUIMetadataMaxK8SVersion string    `json:"operatorhub.io/ui-metadata-max-k8s-version,omitempty"`
	Repository                           string    `json:"repository,omitempty"`
	Support                              string    `json:"support,omitempty"`
}

// PMOwned is the package manifest owned info.
type PMOwned struct {
	Description string `json:"description,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Name        string `json:"name,omitempty"`
	Version     string `json:"version,omitempty"`
}

// PMCustomresourcedefinitions package manifest CRD.
type PMCustomresourcedefinitions struct {
	Owned []PMOwned `json:"owned,omitempty"`
}

// PMInstallModes holds package manifests installation modes.
type PMInstallModes struct {
	Supported bool   `json:"supported,omitempty"`
	Type      string `json:"type,omitempty"`
}

// PMLinks package manifests links.
type PMLinks struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

// PMMaintainers package manifest maintainer.
type PMMaintainers struct {
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

// PMProvider package manifest provider.
type PMProvider struct {
	Name string `json:"name,omitempty"`
}

// PMCurrentCSVDesc description for the current CSV for the package manifest.
type PMCurrentCSVDesc struct {
	Annotations   PMAnnotations    `json:"annotations,omitempty"`
	Description   string           `json:"description,omitempty"`
	DisplayName   string           `json:"displayName,omitempty"`
	InstallModes  []PMInstallModes `json:"installModes,omitempty"`
	Keywords      []string         `json:"keywords,omitempty"`
	Links         []PMLinks        `json:"links,omitempty"`
	Maintainers   []PMMaintainers  `json:"maintainers,omitempty"`
	Maturity      string           `json:"maturity,omitempty"`
	Provider      PMProvider       `json:"provider,omitempty"`
	RelatedImages []string         `json:"relatedImages,omitempty"`
	Version       string           `json:"version,omitempty"`
}

// PMChannels is the list of available channels for the package manifest.
type PMChannels struct {
	CurrentCSV     string           `json:"currentCSV,omitempty"`
	CurrentCSVDesc PMCurrentCSVDesc `json:"currentCSVDesc,omitempty"`
	Name           string           `json:"name,omitempty"`
}

// PMStatus holds the package manifest status information.
type PMStatus struct {
	CatalogSource            string       `json:"catalogSource,omitempty"`
	CatalogSourceDisplayName string       `json:"catalogSourceDisplayName,omitempty"`
	CatalogSourceNamespace   string       `json:"catalogSourceNamespace,omitempty"`
	CatalogSourcePublisher   string       `json:"catalogSourcePublisher,omitempty"`
	Channels                 []PMChannels `json:"channels,omitempty"`
	DefaultChannel           string       `json:"defaultChannel,omitempty"`
	PackageName              string       `json:"packageName,omitempty"`
	Provider                 PMProvider   `json:"provider,omitempty"`
}

// PMItem holds information about each item in tha package manifest list.
type PMItem struct {
	APIVersion string     `json:"apiVersion,omitempty"`
	Kind       string     `json:"kind,omitempty"`
	Metadata   PMMetadata `json:"metadata,omitempty"`
	Status     PMStatus   `json:"status,omitempty"`
}

// PMMetadata holds informations about package manifest metadata.
type PMMetadata struct {
	ResourceVersion string `json:"resourceVersion,omitempty"`
}
