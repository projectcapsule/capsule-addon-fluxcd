// Copyright 2020-2024 Project Capsule Authors.
// SPDX-License-Identifier: Apache-2.0

package serviceaccount

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	capsulev1beta2 "github.com/projectcapsule/capsule/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//nolint:revive
type ServiceAccountReconciler struct {
	proxyURL string
	proxyCA  string

	Client client.Client
	Log    logr.Logger
}

type Option func(r *ServiceAccountReconciler)

func WithClient(c client.Client) Option {
	return func(r *ServiceAccountReconciler) {
		r.Client = c
	}
}

func WithLogger(log logr.Logger) Option {
	return func(r *ServiceAccountReconciler) {
		r.Log = log
	}
}

func WithProxyCA(proxyCA string) Option {
	return func(r *ServiceAccountReconciler) {
		r.proxyCA = proxyCA
	}
}

func WithProxyURL(proxyURL string) Option {
	return func(r *ServiceAccountReconciler) {
		r.proxyURL = proxyURL
	}
}

func NewServiceAccountReconciler(opts ...Option) *ServiceAccountReconciler {
	reconciler := new(ServiceAccountReconciler)

	for _, f := range opts {
		f(reconciler)
	}

	return reconciler
}

func (r *ServiceAccountReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ServiceAccount{}, r.forOption(ctx)).
		Owns(&capsulev1beta2.GlobalTenantResource{}).
		Complete(r)
}

func (r *ServiceAccountReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	r.Log = r.Log.WithValues("Request.NamespacedName", request.NamespacedName)

	// Unmarshal ServiceAccount object.
	sa := new(corev1.ServiceAccount)
	if err := r.Client.Get(ctx, request.NamespacedName, sa); err != nil {
		if apierrors.IsNotFound(err) {
			r.Log.Info("Request object not found, could have been deleted after reconcile request")

			return reconcile.Result{}, nil
		}

		r.Log.Error(err, "Error reading the object")

		return ctrl.Result{}, err
	}

	// Ensure the (Cluster)RoleBindings for the ServiceAccount.
	if err := r.ensureRoles(ctx, sa.Name, sa.Namespace); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "error ensuring the role bindings for the service account")
	}

	// Ensure ServiceAccount token.
	if err := r.ensureSATokenSecret(ctx, sa.Name, sa.Namespace); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "error ensuring token of the service account")
	}

	tokenSecret, err := r.getSATokenSecret(ctx, sa.Name, sa.Namespace)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "error getting token of the service account")
	}

	if tokenSecret.Data == nil {
		r.Log.Info("ServiceAccount token data is missing. Requeueing.")

		return reconcile.Result{Requeue: true}, nil
	}

	// Build the kubeConfig for the ServiceAccount Tenant Owner.
	config := r.buildKubeconfig(r.proxyURL, string(tokenSecret.Data[corev1.ServiceAccountTokenKey]))

	configRaw, err := clientcmd.Write(*config)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "error building the tenant owner config")
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s%s", sa.Name, SecretNameSuffixKubeconfig),
			Namespace: sa.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			SecretKeyKubeconfig: configRaw,
		},
	}
	if _, err = controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
		return nil
	}); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "error ensuring the kubeConfig secret")
	}

	// Get the Tenant owned by the ServiceAccount.
	ownerName := fmt.Sprintf("system:serviceaccount:%s:%s", sa.GetNamespace(), sa.GetName())

	tenantList, err := r.listTenantsOwned(ctx, string(capsulev1beta2.ServiceAccountOwner), ownerName)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "error listing Tenants for owner")
	}

	if tenantList.Items == nil {
		return reconcile.Result{}, errors.New("Tenant list for owner is empty")
	}

	// Get the ServiceAccount's Namespace.
	ns := new(corev1.Namespace)
	if err = r.Client.Get(ctx, types.NamespacedName{Namespace: "", Name: sa.Namespace}, ns); err != nil {
		if apierrors.IsNotFound(err) {
			r.Log.Info("ServiceAccount Namespace is missing. Requeueing.")

			return reconcile.Result{Requeue: true}, nil
		}

		r.Log.Error(err, "Error reading the object")

		return reconcile.Result{}, err
	}
	// And set the first Tenant owned by the SA as Namespace owner.
	if err = r.setNamespaceOwnerRef(ctx, ns, tenantList.Items[0].DeepCopy()); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "error setting the owner reference on the namespace")
	}
	// If the option for distributing the kubeConfig to Tenant globally.
	if sa.GetAnnotations()[ServiceAccountGlobalAnnotationKey] == ServiceAccountGlobalAnnotationValue {
		for _, tenant := range tenantList.Items {
			// Ensure the GlobalTenantResource to distribute the kubeConfig Secret.
			name := fmt.Sprintf("%s-%s%s", tenant.Name, sa.Name, GlobalTenantResourceSuffix)
			if err = r.ensureGlobalTenantResource(ctx, name, tenant.Name, secret); err != nil {
				return reconcile.Result{}, errors.Wrap(err, "error ensuring the kubeConfig globaltenantresource")
			}
		}
	}

	r.Log.Info("ServiceAccount reconciliation completed")

	return reconcile.Result{}, nil
}

// forOption is the option used to make reconciliation only of ServiceAccounts that both:
// - have the required addon annotation, and
// - are Tenant owners.
func (r *ServiceAccountReconciler) forOption(ctx context.Context) builder.ForOption {
	return builder.WithPredicates(
		predicate.And(
			predicate.NewPredicateFuncs(func(object client.Object) bool {
				return object.GetAnnotations()[ServiceAccountAddonAnnotationKey] == ServiceAccountAddonAnnotationValue
			}),
			predicate.NewPredicateFuncs(func(object client.Object) bool {
				ownerName := fmt.Sprintf("system:serviceaccount:%s:%s", object.GetNamespace(), object.GetName())
				tntList, err := r.listTenantsOwned(ctx, string(capsulev1beta2.ServiceAccountOwner), ownerName)

				return err == nil && tntList.Items != nil && len(tntList.Items) != 0
			}),
		),
	)
}

// listTenantsOwned returns the first Tenant object owned by the ServiceAccount of which the name and the
// namespace are specified as arguments.
func (r *ServiceAccountReconciler) listTenantsOwned(ctx context.Context, ownerKind, ownerName string) (*capsulev1beta2.TenantList, error) {
	tntList := &capsulev1beta2.TenantList{}
	fields := client.MatchingFields{
		".spec.owner.ownerkind": fmt.Sprintf("%s:%s", ownerKind, ownerName),
	}
	err := r.Client.List(ctx, tntList, fields)

	return tntList, err
}

// buildKubeconfig returns a client-go/clientcmd/api.Config with a token and server URL specified as arguments.
// The server set is be the proxy configured at ServiceAccountReconciler-level.
func (r *ServiceAccountReconciler) buildKubeconfig(server, token string) *clientcmdapi.Config {
	// Build the client API Config.
	config := clientcmdapi.NewConfig()
	config.APIVersion = clientcmdlatest.Version
	config.Kind = "Config"

	// Build the client Config cluster.
	cluster := clientcmdapi.NewCluster()
	cluster.Server = server

	cluster.CertificateAuthorityData = []byte(r.proxyCA)
	config.Clusters = map[string]*clientcmdapi.Cluster{
		"default": cluster,
	}

	// Build the client Config AuthInfo.
	authInfo := clientcmdapi.NewAuthInfo()
	authInfo.Token = token
	config.AuthInfos = map[string]*clientcmdapi.AuthInfo{
		"default": authInfo,
	}

	// Build the client Config context.
	contexts := make(map[string]*clientcmdapi.Context)
	kctx := clientcmdapi.NewContext()
	kctx.Cluster = KubeconfigClusterName
	kctx.AuthInfo = KubeconfigUserName
	kctx.Namespace = corev1.NamespaceDefault
	contexts[KubeconfigContextName] = kctx
	config.Contexts = contexts
	config.CurrentContext = KubeconfigContextName

	return config
}
