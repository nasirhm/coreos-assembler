// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package etcd

import (
	"fmt"
	"time"

	"github.com/coreos/mantle/Godeps/_workspace/src/github.com/coreos/pkg/capnslog"
	"github.com/coreos/mantle/platform"
	"github.com/coreos/mantle/util"
)

var plog = capnslog.NewPackageLogger("github.com/coreos/mantle", "kola/tests/etcd")

func DiscoveryV2(c platform.TestCluster) error {
	return discovery(c, 2)
}

func DiscoveryV1(c platform.TestCluster) error {
	return discovery(c, 1)
}

func discovery(cluster platform.Cluster, version int) error {
	csize := len(cluster.Machines())

	if plog.LevelAt(capnslog.DEBUG) {
		// get journalctl -f from all machines before starting
		for _, m := range cluster.Machines() {
			if err := m.StartJournal(); err != nil {
				return fmt.Errorf("failed to start journal: %v", err)
			}
		}
	}

	// point etcd on each machine to discovery
	for i, m := range cluster.Machines() {
		// start etcd instance
		var etcdStart string
		if version == 1 {
			etcdStart = "sudo systemctl start etcd.service"
		} else if version == 2 {
			etcdStart = "sudo systemctl start etcd2.service"
		} else {
			return fmt.Errorf("etcd version unspecified")
		}

		_, err := m.SSH(etcdStart)
		if err != nil {
			return fmt.Errorf("SSH cmd to %v failed: %s", m.IP(), err)
		}
		plog.Infof("etcd instance%d started", i)
	}

	if version == 2 {
		err := getClusterHealth(cluster.Machines()[0], csize)
		if err != nil {
			return fmt.Errorf("discovery failed health check: %v", err)
		}
	} else if version == 1 {
		var keyMap map[string]string
		var retryFuncs []func() error

		retryFuncs = append(retryFuncs, func() error {
			var err error
			keyMap, err = setKeys(cluster, 5)
			if err != nil {
				return err
			}
			return nil
		})
		retryFuncs = append(retryFuncs, func() error {
			if err := checkKeys(cluster, keyMap, false); err != nil {
				return err
			}
			return nil
		})
		for _, retry := range retryFuncs {
			if err := util.Retry(5, 5*time.Second, retry); err != nil {
				return fmt.Errorf("discovery failed health check: %v", err)
			}
			//NOTE(pb): etcd1 seems to fail in odd ways when I quorum
			//read, instead just sleep between setting and getting.
			time.Sleep(2 * time.Second)
		}
	} else {
		return fmt.Errorf("etcd version unspecified")
	}

	return nil
}
