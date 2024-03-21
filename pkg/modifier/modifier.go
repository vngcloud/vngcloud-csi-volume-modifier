package modifier

import (
	"context"
	"fmt"
	"time"

	lcoreV1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	csitrans "k8s.io/csi-translation-lib"
	"k8s.io/klog/v2"

	lcsi "github.com/vngcloud/vngcloud-csi-volume-modifier/pkg/client"
)

func NewFromClient(
	name string,
	csiClient lcsi.IClient,
	kubeClient kubernetes.Interface,
	timeout time.Duration,
) (IModifier, error) {
	return &csiModifier{
		name:      name,
		client:    csiClient,
		k8sClient: kubeClient,
		timeout:   timeout,
	}, nil
}

// _____________________________________________________________________________________________________________________ENTITIES

type csiModifier struct {
	name      string
	client    lcsi.IClient
	timeout   time.Duration
	k8sClient kubernetes.Interface
}

func (s *csiModifier) Name() string {
	return s.name
}

func (s *csiModifier) Modify(pv *lcoreV1.PersistentVolume, params, reqContext map[string]string) error {
	klog.V(5).InfoS("Received modify request", "pv", pv, "params", params)

	var (
		volumeID string
	)

	if pv.Spec.CSI != nil {
		volumeID = pv.Spec.CSI.VolumeHandle
	} else {
		translator := csitrans.New()
		if translator.IsMigratedCSIDriverByName(s.name) {
			csiPV, err := translator.TranslateInTreePVToCSI(pv)
			if err != nil {
				return fmt.Errorf("failed to translate persistent volume: %w", err)
			}
			volumeID = csiPV.Spec.CSI.VolumeHandle
		} else {
			return fmt.Errorf("volume %v is not migrated to CSI", pv.Name)
		}
	}

	klog.InfoS("Calling modify volume for volume", "volumeID", volumeID)

	ctx, cancel := context.WithTimeout(context.TODO(), s.timeout)
	defer cancel()
	return s.client.Modify(ctx, volumeID, params, reqContext)
}
