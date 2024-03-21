package client

import (
	"context"
	"fmt"
	"k8s.io/klog/v2"
	"time"

	"github.com/kubernetes-csi/csi-lib-utils/connection"
	"github.com/kubernetes-csi/csi-lib-utils/metrics"
	"github.com/kubernetes-csi/csi-lib-utils/rpc"
	"google.golang.org/grpc"

	lmodifyrpc "github.com/vngcloud/vngcloud-csi-volume-modifier/pkg/rpc"
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

func (s *client) SupportsVolumeModification(pctx context.Context) error {
	cc := lmodifyrpc.NewModifyClient(s.conn)
	req := &lmodifyrpc.GetCSIDriverModificationCapabilityRequest{}
	_, err := cc.GetCSIDriverModificationCapability(pctx, req)
	return err
}

func (s *client) Modify(pctx context.Context, pvolumeID string, pparams, preqContext map[string]string) error {
	cc := lmodifyrpc.NewModifyClient(s.conn)
	req := &lmodifyrpc.ModifyVolumePropertiesRequest{
		Name:       pvolumeID,
		Parameters: pparams,
		Context:    preqContext,
	}
	_, err := cc.ModifyVolumeProperties(pctx, req)
	if err == nil {
		klog.V(4).InfoS("Volume modification completed", "volumeID", pvolumeID)
	}
	return err
}

func (s *client) CloseConnection() {
	_ = s.conn.Close()
}
