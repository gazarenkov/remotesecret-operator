package secretstorage

type SecretUid struct {
	Cluster      string `json:"cluster,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
	RemoteSecret string `json:"remote-secret,omitempty"`
}

func NewSecretUid(cluster string, namespace string, name string) SecretUid {
	return SecretUid{
		Cluster:      cluster,
		Namespace:    namespace,
		RemoteSecret: name,
	}
}
