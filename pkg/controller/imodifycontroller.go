package controller

import (
	"context"
)

type IModifyController interface {
	Run(int, context.Context)
}
