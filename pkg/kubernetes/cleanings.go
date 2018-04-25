package kubernetes

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/simonswine/rocklet/pkg/apis/vacuum/v1alpha1"
)

// ensureCleaning makes sure the cleaning has been posted upstream
func (k *Kubernetes) ensureCleaning(c *v1alpha1.Cleaning) error {
	cl := k.vacuumClient.Cleanings(metav1.NamespaceDefault)
	_, err := cl.Get(c.Name, metav1.GetOptions{})

	// is already existing
	if err == nil {
		return nil
	}

	// other error than existing
	if !apierrors.IsNotFound(err) {
		return err
	}

	// create cleaning
	k.logger.Debug().Msgf("creating cleaning %s", c.Name)
	_, err = cl.Create(c)
	return err
}
