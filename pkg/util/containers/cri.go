// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2018 Datadog, Inc.

// +build linux

package containers

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/pkg/util/log"
	"github.com/DataDog/datadog-agent/pkg/util/retry"
	"google.golang.org/grpc"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

var (
	globalCRIUtil *CRIUtil
	once          sync.Once
)

// CRIUtil wraps interactions with the CRI
// see https://github.com/kubernetes/kubernetes/blob/release-1.12/pkg/kubelet/apis/cri/runtime/v1alpha2/api.proto
type CRIUtil struct {
	// used to setup the CRIUtil
	initRetry retry.Retrier

	sync.Mutex
	client         pb.RuntimeServiceClient
	Runtime        string
	RuntimeVersion string
	queryTimeout   time.Duration
}

// init makes an empty CRIUtil bootstrap itself.
// This is not exposed as public API but is called by the retrier embed.
func (c *CRIUtil) init() error {
	// TODO config?
	c.queryTimeout = 5 * time.Second

	addr := "/var/run/containerd/containerd.sock"
	dialer := func(addr string, timeout time.Duration) (net.Conn, error) {
		return net.DialTimeout("unix", addr, timeout)
	}

	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithTimeout(c.queryTimeout), grpc.WithDialer(dialer))
	if err != nil {
		return fmt.Errorf("failed to dial: %v", err)
	}

	c.client = pb.NewRuntimeServiceClient(conn)
	// validating the connection fetching the version
	request := &pb.VersionRequest{}
	r, err := c.client.Version(context.Background(), request)
	if err != nil {
		return err
	}
	c.Runtime = r.RuntimeName
	c.RuntimeVersion = r.RuntimeVersion

	return nil
}

// GetCRIUtil returns a ready to use CRIUtil. It is backed by a shared singleton.
func GetCRIUtil() (*CRIUtil, error) {
	once.Do(func() {
		globalCRIUtil = &CRIUtil{}
		globalCRIUtil.initRetry.SetupRetrier(&retry.Config{
			Name:          "criutil",
			AttemptMethod: globalCRIUtil.init,
			Strategy:      retry.RetryCount,
			RetryCount:    10,
			RetryDelay:    30 * time.Second,
		})
	})

	if err := globalCRIUtil.initRetry.TriggerRetry(); err != nil {
		log.Debugf("CRI init error: %s", err)
		return nil, err
	}
	return globalCRIUtil, nil
}

// Version sends a VersionRequest to the server, and parses the returned VersionResponse.
func (c *CRIUtil) ListContainerStats() (map[string]*pb.ContainerStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.queryTimeout)
	defer cancel()
	filter := &pb.ContainerStatsFilter{}
	request := &pb.ListContainerStatsRequest{Filter: filter}
	r, err := c.client.ListContainerStats(ctx, request)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]*pb.ContainerStats)
	for _, s := range r.GetStats() {
		stats[s.Attributes.Id] = s
	}
	return stats, nil
}
