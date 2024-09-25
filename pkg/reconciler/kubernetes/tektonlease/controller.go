package tektonlease

import (
	"context"

	leaseclient "github.com/tektoncd/operator/pkg/client/injection/kube/client"
	leaseinformer "github.com/tektoncd/operator/pkg/client/injection/kube/informers/coordination/v1/lease"
	leasereconciler "github.com/tektoncd/operator/pkg/client/injection/kube/reconciler/coordination/v1/lease"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
)

// NewController creates a Reconciler and returns the result of NewImpl.
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// Obtain an informer to both the main and child resources. These will be started by
	// the injection framework automatically. They'll keep a cached representation of the
	// cluster's state of the respective resource at all times.
	leaseInformer := leaseinformer.Get(ctx)

	logger := logging.FromContext(ctx)

	leaseclient.Get(ctx)

	r := &Reconciler{
		// The client will be needed to create/delete Pods via the API.
		kubeclient: kubeclient.Get(ctx),
	}

	// number of works to process the events
	concurrentWorkers := 5
	ctrlOptions := controller.Options{
		Concurrency: concurrentWorkers,
	}

	impl := leasereconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options { return ctrlOptions })

	// Listen for events on the main resource and enqueue themselves.
	leaseInformer.Informer().AddEventHandler(controller.HandleAll(filterLease(logger, impl)))

	return impl
}

// filters the leases which is part of the tekton ecosystem
func filterLease(logger *zap.SugaredLogger, impl *controller.Impl) func(obj interface{}) {
	return func(obj interface{}) {
		lease, err := kmeta.DeletionHandlingAccessor(obj)
		if err != nil {
			logger.Errorw("error on getting object as Accessor", zap.Error(err))
			return
		}

		if hasMatchingLeaseData(lease.GetName()) {
			impl.EnqueueKey(types.NamespacedName{Namespace: lease.GetNamespace(), Name: lease.GetName()})
		}

	}
}
