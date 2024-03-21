package main

import (
	"context"
	"time"

	lcsi "github.com/vngcloud/vngcloud-csi-volume-modifier/pkg/client"
)

// _____________________________________________________________________________________________________________________PRIVATE METHODS

func getDriverName(pclient lcsi.IClient, ptimeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ptimeout)
	defer cancel()
	return pclient.GetDriverName(ctx)
}
