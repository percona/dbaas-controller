// Package k8sclient provides Kubernetes client.
package k8sclient

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/AlekSi/pointer"
	pxc "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	"gopkg.in/yaml.v3"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var cmd = []string{"minikube", "kubectl", "--"}

func kubectl(args []string, stdin io.Reader) []byte {
	var buf bytes.Buffer
	args = append(cmd, args...)
	log.Println(args)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Print(err)
	}
	return buf.Bytes()
}

func apply(res *pxc.PerconaXtraDBCluster) {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetIndent("", "  ")
	if err := e.Encode(res); err != nil {
		log.Fatal(err)
	}
	log.Printf("apply:\n%s", buf.String())

	b := kubectl([]string{"apply", "-f", "-"}, &buf)
	log.Printf("%s", b)
}

func get(kind, name string) []byte {
	b := kubectl([]string{"get", "-o=yaml", kind, name}, nil)
	log.Printf("%s", b)
	return b
}

func wait(kind, name, condition string, timeout time.Duration) {
	kubectl([]string{"wait", "--for=" + condition, "--timeout=" + timeout.String(), kind, name}, nil)
}

// nolint:funlen
func example() {
	kubectl([]string{"version"}, nil)

	const name = "poc-cluster"
	const size = 1
	// const pxcImage = "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0-debug"
	const pxcImage = "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0"
	const backupImage = "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0-backup"
	const backupStorageName = "test-backup-storage"
	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: meta.TypeMeta{
			APIVersion: "pxc.percona.com/v1-4-0",
			Kind:       "PerconaXtraDBCluster",
		},
		ObjectMeta: meta.ObjectMeta{
			Name: name,
		},
		Spec: pxc.PerconaXtraDBClusterSpec{
			AllowUnsafeConfig: true,
			SecretsName:       "my-cluster-secrets",

			PXC: &pxc.PodSpec{
				Size:  size,
				Image: pxcImage,
				VolumeSpec: &pxc.VolumeSpec{
					PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
				Affinity: &pxc.PodAffinity{
					TopologyKey: pointer.ToString(pxc.AffinityTopologyKeyOff),
				},
			},

			ProxySQL: &pxc.PodSpec{
				Enabled: true,
				Size:    size,
				Image:   "percona/percona-xtradb-cluster-operator:1.4.0-proxysql",
				VolumeSpec: &pxc.VolumeSpec{
					PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
				Affinity: &pxc.PodAffinity{
					TopologyKey: pointer.ToString(pxc.AffinityTopologyKeyOff),
				},
			},

			PMM: &pxc.PMMSpec{
				Enabled: false,
			},

			Backup: &pxc.PXCScheduledBackup{
				Image: backupImage,
				Schedule: []pxc.PXCScheduledBackupSchedule{{
					Name:        "test",
					Schedule:    "*/1 * * * *",
					Keep:        3,
					StorageName: backupStorageName,
				}},
				Storages: map[string]*pxc.BackupStorageSpec{
					backupStorageName: {
						Type: pxc.BackupStorageFilesystem,
						Volume: &pxc.VolumeSpec{
							PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{
								Resources: core.ResourceRequirements{
									Requests: core.ResourceList{
										core.ResourceStorage: resource.MustParse("1Gi"),
									},
								},
							},
						},
					},
				},
				ServiceAccountName: "percona-xtradb-cluster-operator",
			},
		},
	}
	apply(res)

	wait("PerconaXtraDBCluster", name, "condition=Ready", time.Minute)

	for {
		b := get("PerconaXtraDBCluster", name)
		var res struct {
			Status struct {
				State string
			}
		}
		if err := yaml.Unmarshal(b, &res); err != nil {
			log.Fatal(err)
		}
		if res.Status.State == "ready" {
			return
		}
		log.Printf("status.state != 'ready', will wait")
		time.Sleep(30 * time.Second)
	}
}
