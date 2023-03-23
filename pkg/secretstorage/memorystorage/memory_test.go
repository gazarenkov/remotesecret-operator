package memorystorage

import (
	"context"
	"fmt"
	"github.com/redhat-appstudio/remotesecret-operator/pkg/secretstorage"
	"testing"
)

func TestP(t *testing.T) {

	ctx := context.TODO()
	storage := MemorySecretStorage{}
	storage.Initialize(ctx)
	uid := secretstorage.SecretUid{Cluster: "cluster", Namespace: "ns", RemoteSecret: "name1"}

	storage.Store(ctx, uid, []byte("my_secret"))

	res, _ := storage.Get(ctx, uid)
	fmt.Printf("%v=%s\n", uid, string(res))
}
