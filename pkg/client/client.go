package client

import (
	"context"
	"fmt"
	"time"

	"github.com/kubernetes-csi/csi-lib-utils/connection"
	"github.com/kubernetes-csi/csi-lib-utils/metrics"
	"github.com/kubernetes-csi/csi-lib-utils/rpc"
	"google.golang.org/grpc"

	modifyrpc "github.com/vngcloud/vngcloud-csi-volume-modifier/pkg/rpc"
)

// _____________________________________________________________________________________________________________________PUBLIC METHODS

func New(paddr string, ptimeout time.Duration, pmetricsmanager metrics.CSIMetricsManager) (IClient, error) {
	conn, err := connection.Connect(paddr, pmetricsmanager, connection.OnConnectionLoss(connection.ExitOnConnectionLoss()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CSI driver: %w", err)
	}

	err = rpc.ProbeForever(conn, ptimeout)
	if err != nil {
		return nil, fmt.Errorf("failed probing CSI driver: %w", err)
	}

	return &client{
		conn: conn,
	}, nil
}

// _____________________________________________________________________________________________________________________ENTITY

type client struct {
	conn *grpc.ClientConn
}

func (s *client) GetDriverName(pctx context.Context) (string, error) {
	return rpc.GetDriverName(pctx, s.conn)
}

func (s *client) SupportsVolumeModification(ctx context.Context) error {
	cc := modifyrpc.NewModifyClient(s.conn)
	req := &modifyrpc.GetCSIDriverModificationCapabilityRequest{}
	_, err := cc.GetCSIDriverModificationCapability(ctx, req)
	return err
}
