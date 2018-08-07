package stub

import (
	"context"
	"fmt"

	"github.com/weaveworks/launcher/agent/pkg/apis/agent/v1beta1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
)

func NewHandler() sdk.Handler {
	return &Handler{}
}

type Handler struct {
	// Fill me
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	switch o := event.Object.(type) {
	case *v1beta1.WeaveCloud:
		watch(o)
	}
	return nil
}

func watch(cr *v1beta1.WeaveCloud) {
	fmt.Println("testing...")
}
