/*
Copyright 2020.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetZkQuorumPodNames(t *testing.T) {
	s := `
server.3=zookeepercluster-sample-2.zookeepercluster-sample.default.svc.cluster.local:2888:3888:participant
server.1=zookeepercluster-sample-0.zookeepercluster-sample.default.svc.cluster.local:2888:3888:participant
server.2=zookeepercluster-sample-1.zookeepercluster-sample.default.svc.cluster.local:2888:3888:participant
`
	names := getZkQuorumPodNames(s)
	assert.Equal(t, []string{
		"zookeepercluster-sample-0",
		"zookeepercluster-sample-1",
		"zookeepercluster-sample-2"}, names)
}
