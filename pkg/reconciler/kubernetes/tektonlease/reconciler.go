package tektonlease

import (
	"context"

	"k8s.io/client-go/kubernetes"

	leasereconciler "github.com/tektoncd/operator/pkg/client/injection/kube/reconciler/coordination/v1/lease"
	coordinationv1 "k8s.io/api/coordination/v1"

	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

type Reconciler struct {
	kubeclient kubernetes.Interface
}

// Check that our Reconciler implements Interface
var _ leasereconciler.Interface = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, lease *coordinationv1.Lease) reconciler.Event {
	logger := logging.FromContext(ctx)
	logger.Infow("received a Lease event",
		"namespace", lease.Namespace, "name", lease.Name,
		"spec", lease.Spec.String(),
	)

	// add logic to handle leases

	return nil
}
