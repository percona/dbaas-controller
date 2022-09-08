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

package k8sclient

import (
	"crypto/rand"
	"math/big"

	"github.com/pkg/errors"
)

const (
	passwordLength = 24
)

func generatePasswords(secrets map[string][]byte) (map[string][]byte, error) {
	for key := range secrets {
		password, err := generatePassword(passwordLength)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate password for  %s", key)
		}
		secrets[key] = []byte(password)
	}
	return secrets, nil
}

func generatePassword(n int) (string, error) {
	// PSMDB do not support all special characters in password https://jira.percona.com/browse/K8SPSMDB-364
	symbols := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	symbolsLen := len(symbols)
	b := make([]rune, n)
	for i := range b {
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(symbolsLen)))
		if err != nil {
			return "", err
		}
		b[i] = symbols[randomIndex.Uint64()]
	}
	return string(b), nil
}

func generatePXCPasswords() (map[string][]byte, error) {
	// secrets represents stringData part of
	// https://github.com/percona/percona-xtradb-cluster-operator/blob/main/deploy/secrets.yaml.
	secrets := map[string][]byte{
		"root":         {},
		"xtrabackup":   {},
		"monitor":      {},
		"clustercheck": {},
		"proxyadmin":   {},
		"operator":     {},
		"replication":  {},
	}

	return generatePasswords(secrets)
}

func generatePSMDBPasswords() (map[string][]byte, error) {
	// secrets represents stringData part of
	// https://github.com/percona/percona-server-mongodb-operator/blob/main/deploy/secrets.yaml.
	secrets := map[string][]byte{
		"MONGODB_BACKUP_USER":          []byte("backup"),
		"MONGODB_CLUSTER_ADMIN_USER":   []byte("clusterAdmin"),
		"MONGODB_CLUSTER_MONITOR_USER": []byte("clusterMonitor"),
		"MONGODB_USER_ADMIN_USER":      []byte("userAdmin"),
	}
	passwords, err := generatePasswords(map[string][]byte{
		"MONGODB_BACKUP_PASSWORD":          {},
		"MONGODB_CLUSTER_ADMIN_PASSWORD":   {},
		"MONGODB_CLUSTER_MONITOR_PASSWORD": {},
		"MONGODB_USER_ADMIN_PASSWORD":      {},
	})
	if err != nil {
		return nil, err
	}
	for k, v := range passwords {
		secrets[k] = v
	}
	return secrets, nil
}
