package kubernetes

import (
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/simonswine/rocklet/pkg/apis/vacuum/v1alpha1"
)

func (k *Kubernetes) UpdateVaccumStatus(status *v1alpha1.VacuumStatus) error {
	return k.tryUpdateVaccumStatus(status)
}

func (k *Kubernetes) tryUpdateVaccumStatus(status *v1alpha1.VacuumStatus) error {
	v := k.vacuumObj
	var err error
	newStatus := *status
	client := k.vacuumClient.Vacuums(metav1.NamespaceDefault)

	if v == nil {
		v, err = client.Get(k.nodeName, metav1.GetOptions{})

		// create a new vacuum
		if err != nil && apierrors.IsNotFound(err) {
			v := &v1alpha1.Vacuum{
				ObjectMeta: metav1.ObjectMeta{
					Name: k.nodeName,
				},
				Spec: v1alpha1.VacuumSpec{
					Cloud: false,
				},
				Status: newStatus,
			}

			v, err := client.Create(v)
			if err != nil {
				return err
			}
			k.vacuumObj = v
			return nil

		} else if err != nil {
			return err
		}
	}

	// no need to update existing vacuum if matches
	if reflect.DeepEqual(newStatus, v.Status) {
		k.vacuumObj = v
		return nil
	}

	v.Status = newStatus
	v, err = client.UpdateStatus(v)
	if err != nil {
		k.vacuumObj = nil
		return err
	}
	k.vacuumObj = v
	return nil
}
