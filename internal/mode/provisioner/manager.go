package provisioner

import (
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctlr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	embeddedfiles "github.com/nginxinc/nginx-kubernetes-gateway"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/framework/controller"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/framework/controller/predicate"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/framework/events"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/framework/status"
)

// Config is configuration for the provisioner mode.
type Config struct {
	Logger           logr.Logger
	GatewayClassName string
	GatewayCtlrName  string
}

// StartManager starts a Manager for the provisioner mode, which provisions
// a Deployment of NKG (static mode) for each Gateway of the provisioner GatewayClass.
//
// The provisioner mode is introduced to allow running Gateway API conformance tests for NKG, which expects
// an independent data plane instance being provisioned for each Gateway.
//
// The provisioner mode is not intended to be used in production (in the short term), as it lacks support for
// many important features. See https://github.com/nginxinc/nginx-kubernetes-gateway/issues/634 for more details.
func StartManager(cfg Config) error {
	scheme := runtime.NewScheme()
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(v1.AddToScheme(scheme))

	options := manager.Options{
		Scheme: scheme,
		Logger: cfg.Logger,
	}
	clusterCfg := ctlr.GetConfigOrDie()

	mgr, err := manager.New(clusterCfg, options)
	if err != nil {
		return fmt.Errorf("cannot build runtime manager: %w", err)
	}

	// Note: for any new object type or a change to the existing one,
	// make sure to also update firstBatchPreparer creation below
	controllerRegCfgs := []struct {
		objectType client.Object
		options    []controller.Option
	}{
		{
			objectType: &gatewayv1beta1.GatewayClass{},
			options: []controller.Option{
				controller.WithK8sPredicate(predicate.GatewayClassPredicate{ControllerName: cfg.GatewayCtlrName}),
			},
		},
		{
			objectType: &gatewayv1beta1.Gateway{},
		},
	}

	ctx := ctlr.SetupSignalHandler()
	eventCh := make(chan interface{})

	for _, regCfg := range controllerRegCfgs {
		err := controller.Register(ctx, regCfg.objectType, mgr, eventCh, regCfg.options...)
		if err != nil {
			return fmt.Errorf("cannot register controller for %T: %w", regCfg.objectType, err)
		}
	}

	firstBatchPreparer := events.NewFirstEventBatchPreparerImpl(
		mgr.GetCache(),
		[]client.Object{
			&gatewayv1beta1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: cfg.GatewayClassName}},
		},
		[]client.ObjectList{
			&gatewayv1beta1.GatewayList{},
		},
	)

	statusUpdater := status.NewUpdater(
		status.UpdaterConfig{
			Client:                   mgr.GetClient(),
			Clock:                    status.NewRealClock(),
			Logger:                   cfg.Logger.WithName("statusUpdater"),
			GatewayClassName:         cfg.GatewayClassName,
			UpdateGatewayClassStatus: true,
		},
	)

	handler := newEventHandler(
		cfg.GatewayClassName,
		statusUpdater,
		mgr.GetClient(),
		cfg.Logger.WithName("eventHandler"),
		embeddedfiles.StaticModeDeploymentYAML,
	)

	eventLoop := events.NewEventLoop(
		eventCh,
		cfg.Logger.WithName("eventLoop"),
		handler,
		firstBatchPreparer,
	)

	err = mgr.Add(eventLoop)
	if err != nil {
		return fmt.Errorf("cannot register event loop: %w", err)
	}

	cfg.Logger.Info("Starting manager")
	return mgr.Start(ctx)
}
