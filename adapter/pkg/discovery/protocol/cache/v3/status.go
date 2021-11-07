// Copyright (c) 2021, WSO2 Inc. (http://www.wso2.org) All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package cache

import (
	"sync"
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/stream/v3"
)

// NodeHash computes string identifiers for Envoy nodes.
type NodeHash interface {
	// ID function defines a unique string identifier for the remote Envoy node.
	ID(node *core.Node) string
}

// IDHash uses ID field as the node hash.
type IDHash struct{}

// ID uses the node ID field
func (IDHash) ID(node *core.Node) string {
	if node == nil {
		return ""
	}
	return node.Id
}

var _ NodeHash = IDHash{}

// StatusInfo tracks the server state for the remote Envoy node.
// Not all fields are used by all cache implementations.
type StatusInfo interface {
	// GetNode returns the node metadata.
	GetNode() *core.Node

	// GetNumWatches returns the number of open watches.
	GetNumWatches() int

	// GetNumDeltaWatches returns the number of open delta watches.
	GetNumDeltaWatches() int

	// GetLastWatchRequestTime returns the timestamp of the last discovery watch request.
	GetLastWatchRequestTime() time.Time

	// GetLastDeltaWatchRequestTime returns the timestamp of the last delta discovery watch request.
	GetLastDeltaWatchRequestTime() time.Time

	// SetLastDeltaWatchRequestTime will set the current time of the last delta discovery watch request
	SetLastDeltaWatchRequestTime(time.Time)

	// SetDeltaResponseWatch will set the provided delta response watch to the associate watch ID
	SetDeltaResponseWatch(int64, envoy_cache.DeltaResponseWatch)
}

type statusInfo struct {
	// node is the constant Envoy node metadata.
	node *core.Node

	// watches are indexed channels for the response watches and the original requests.
	watches map[int64]envoy_cache.ResponseWatch

	// deltaWatches are indexed channels for the delta response watches and the original requests
	deltaWatches map[int64]envoy_cache.DeltaResponseWatch

	// the timestamp of the last watch request
	lastWatchRequestTime time.Time

	// the timestamp of the last delta watch request
	lastDeltaWatchRequestTime time.Time

	// mutex to protect the status fields.
	// should not acquire mutex of the parent cache after acquiring this mutex.
	mu sync.RWMutex
}

// newStatusInfo initializes a status info data structure.
func newStatusInfo(node *core.Node) *statusInfo {
	out := statusInfo{
		node:         node,
		watches:      make(map[int64]envoy_cache.ResponseWatch),
		deltaWatches: make(map[int64]envoy_cache.DeltaResponseWatch),
	}
	return &out
}

func (info *statusInfo) GetNode() *core.Node {
	info.mu.RLock()
	defer info.mu.RUnlock()
	return info.node
}

func (info *statusInfo) GetNumWatches() int {
	info.mu.RLock()
	defer info.mu.RUnlock()
	return len(info.watches)
}

func (info *statusInfo) GetNumDeltaWatches() int {
	info.mu.RLock()
	defer info.mu.RUnlock()
	return len(info.deltaWatches)
}

func (info *statusInfo) GetLastWatchRequestTime() time.Time {
	info.mu.RLock()
	defer info.mu.RUnlock()
	return info.lastWatchRequestTime
}

func (info *statusInfo) GetLastDeltaWatchRequestTime() time.Time {
	info.mu.RLock()
	defer info.mu.RUnlock()
	return info.lastDeltaWatchRequestTime
}

// GetDeltaStreamState will pull the stream state with the version map out of a specific watch
func (info *statusInfo) GetDeltaStreamState(watchID int64) stream.StreamState {
	info.mu.RLock()
	defer info.mu.RUnlock()
	return info.deltaWatches[watchID].StreamState
}

func (info *statusInfo) SetLastDeltaWatchRequestTime(t time.Time) {
	info.mu.Lock()
	defer info.mu.Unlock()
	info.lastDeltaWatchRequestTime = t
}

func (info *statusInfo) SetDeltaResponseWatch(id int64, drw envoy_cache.DeltaResponseWatch) {
	info.mu.Lock()
	defer info.mu.Unlock()
	info.deltaWatches[id] = drw
}
