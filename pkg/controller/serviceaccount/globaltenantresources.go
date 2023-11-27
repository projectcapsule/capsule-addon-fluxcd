package serviceaccount

import (
	"context"
	capsulev1beta2 "github.com/projectcapsule/capsule/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ensureGlobalTenantResource ensures the GlobalTenantResource to distribute an object.
func (r *ServiceAccountReconciler) ensureGlobalTenantResource(ctx context.Context, name, tenantName string, object runtime.Object) error {
	gtr := &capsulev1beta2.GlobalTenantResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"app.kubernetes.io/managed-by": ManagerName},
		},
		Spec: capsulev1beta2.GlobalTenantResourceSpec{
			TenantSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"kubernetes.io/metadata.name": tenantName},
			},
			TenantResourceSpec: capsulev1beta2.TenantResourceSpec{
				Resources: []capsulev1beta2.ResourceSpec{{
					RawItems: []capsulev1beta2.RawExtension{{
						RawExtension: runtime.RawExtension{
							Object: object,
						},
					}},
				}},
			},
		},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, gtr, func() error {
		if gtr.ObjectMeta.Labels == nil {
			gtr.ObjectMeta.Labels = make(map[string]string, 1)
		}
		gtr.ObjectMeta.Labels["app.kubernetes.io/managed-by"] = ManagerName

		return nil
	}); err != nil {
		return err
	}

	return nil
}
