// Copyright 2018 Prometheus Team
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/client_golang/prometheus"
)

func TestJoinLeave(t *testing.T) {
	logger := log.NewNopLogger()
	p, err := Join(
		logger,
		prometheus.NewRegistry(),
		"0.0.0.0:0",
		"",
		[]string{},
		true,
		DefaultPushPullInterval,
		DefaultGossipInterval,
		DefaultTcpTimeout,
		DefaultProbeTimeout,
		DefaultProbeInterval,
		DefaultReconnectInterval,
		DefaultReconnectTimeout,
	)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.False(t, p.Ready())
	require.Equal(t, p.Status(), "settling")
	go p.Settle(context.Background(), 0*time.Second)
	p.WaitReady()
	require.Equal(t, p.Status(), "ready")

	// Create the peer who joins the first.
	p2, err := Join(
		logger,
		prometheus.NewRegistry(),
		"0.0.0.0:0",
		"",
		[]string{p.Self().Address()},
		true,
		DefaultPushPullInterval,
		DefaultGossipInterval,
		DefaultTcpTimeout,
		DefaultProbeTimeout,
		DefaultProbeInterval,
		DefaultReconnectInterval,
		DefaultReconnectTimeout,
	)
	require.NoError(t, err)
	require.NotNil(t, p2)
	go p2.Settle(context.Background(), 0*time.Second)

	require.Equal(t, 2, p.ClusterSize())
	p2.Leave(0 * time.Second)
	require.Equal(t, 1, p.ClusterSize())
	require.Equal(t, 1, len(p.failedPeers))
	require.Equal(t, p2.Self().Address(), p.peers[p2.Self().Address()].Node.Address())
	require.Equal(t, p2.Name(), p.failedPeers[0].Name)
}

func TestReconnect(t *testing.T) {
	logger := log.NewNopLogger()
	p, err := Join(
		logger,
		prometheus.NewRegistry(),
		"0.0.0.0:0",
		"",
		[]string{},
		true,
		DefaultPushPullInterval,
		DefaultGossipInterval,
		DefaultTcpTimeout,
		DefaultProbeTimeout,
		DefaultProbeInterval,
		DefaultReconnectInterval,
		DefaultReconnectTimeout,
	)
	require.NoError(t, err)
	require.NotNil(t, p)
	go p.Settle(context.Background(), 0*time.Second)
	p.WaitReady()

	p2, err := Join(
		logger,
		prometheus.NewRegistry(),
		"0.0.0.0:0",
		"",
		[]string{},
		true,
		DefaultPushPullInterval,
		DefaultGossipInterval,
		DefaultTcpTimeout,
		DefaultProbeTimeout,
		DefaultProbeInterval,
		DefaultReconnectInterval,
		DefaultReconnectTimeout,
	)
	require.NoError(t, err)
	require.NotNil(t, p2)
	go p2.Settle(context.Background(), 0*time.Second)
	p2.WaitReady()

	p.peerJoin(p2.Self())
	p.peerLeave(p2.Self())

	require.Equal(t, 1, p.ClusterSize())
	require.Equal(t, 1, len(p.failedPeers))

	p.reconnect()

	require.Equal(t, 2, p.ClusterSize())
	require.Equal(t, 0, len(p.failedPeers))
	require.Equal(t, StatusAlive, p.peers[p2.Self().Address()].status)
}

func TestRemoveFailedPeers(t *testing.T) {
	logger := log.NewNopLogger()
	p, err := Join(
		logger,
		prometheus.NewRegistry(),
		"0.0.0.0:0",
		"",
		[]string{},
		true,
		DefaultPushPullInterval,
		DefaultGossipInterval,
		DefaultTcpTimeout,
		DefaultProbeTimeout,
		DefaultProbeInterval,
		DefaultReconnectInterval,
		DefaultReconnectTimeout,
	)
	require.NoError(t, err)
	require.NotNil(t, p)
	n := p.Self()

	now := time.Now()
	p1 := peer{
		status:    StatusFailed,
		leaveTime: now,
		Node:      n,
	}
	p2 := peer{
		status:    StatusFailed,
		leaveTime: now.Add(-time.Hour),
		Node:      n,
	}
	p3 := peer{
		status:    StatusFailed,
		leaveTime: now.Add(30 * -time.Minute),
		Node:      n,
	}
	p.failedPeers = []peer{p1, p2, p3}

	p.removeFailedPeers(30 * time.Minute)
	require.Equal(t, 1, len(p.failedPeers))
	require.Equal(t, p1, p.failedPeers[0])
}

func TestInitiallyFailingPeers(t *testing.T) {
	logger := log.NewNopLogger()
	peerAddrs := []string{"1.2.3.4:5000", "2.3.4.5:5000", "3.4.5.6:5000"}
	p, err := Join(
		logger,
		prometheus.NewRegistry(),
		"0.0.0.0:0",
		"",
		[]string{},
		true,
		DefaultPushPullInterval,
		DefaultGossipInterval,
		DefaultTcpTimeout,
		DefaultProbeTimeout,
		DefaultProbeInterval,
		DefaultReconnectInterval,
		DefaultReconnectTimeout,
	)
	require.NoError(t, err)
	require.NotNil(t, p)

	p.setInitialFailed(peerAddrs)

	require.Equal(t, len(peerAddrs), len(p.failedPeers))
	for _, addr := range peerAddrs {
		pr, ok := p.peers[addr]
		require.True(t, ok)
		require.Equal(t, StatusNone, pr.status)
	}
}
