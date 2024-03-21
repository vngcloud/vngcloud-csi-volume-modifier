package modifier

import (
	lcoreV1 "k8s.io/api/core/v1"
)

type IModifier interface {
	Name() string
	Modify(*lcoreV1.PersistentVolume, map[string]string, map[string]string) error
}
