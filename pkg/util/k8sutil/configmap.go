package k8sutil

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateConfigmap create a configmap
func CreateConfigmap(name string, data map[string]string) error {
	configMap := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: Namespace,
		},
		Data: data,
	}
	_, err := kubecli.CoreV1().ConfigMaps(Namespace).Create(&configMap)
	return err
}

// GetConfigmap ...
func GetConfigmap(name string) (map[string]string, error) {
	configMap, err := kubecli.CoreV1().ConfigMaps(Namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return configMap.Data, nil
}

// UpdateConfigMap ...
func UpdateConfigMap(name string, data map[string]string) error {
	configMap, err := kubecli.CoreV1().ConfigMaps(Namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	configMap.Data = data
	if err != nil {
		return err
	}
	_, err = kubecli.CoreV1().ConfigMaps(Namespace).Update(configMap)
	return err
}
