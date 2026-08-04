package auth

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// metav1Options returns a GetOptions for use in dynamic client calls.
// context.TODO() is used since gin context is not threaded here.
func metav1Options() metav1.GetOptions {
	return metav1.GetOptions{}
}

// contextTODO provides a background context.
func contextTODO() context.Context {
	return context.TODO()
}
