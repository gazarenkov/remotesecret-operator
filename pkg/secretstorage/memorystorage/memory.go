//
// Copyright (c) 2021 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package memorystorage

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/remotesecret-operator/pkg/logs"
	"github.com/redhat-appstudio/remotesecret-operator/pkg/secretstorage"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
)

// MemorySecretStorage is an in-memory implementation of the SecretStorage interface intended to be used in tests.
type MemorySecretStorage struct {
	// Secrets is the map of stored Secrets. The keys are object keys of the SPIAccessToken objects.
	Secrets map[secretstorage.SecretUid][]byte
	// ErrorOnInitialize if not nil, the error is thrown when the Initialize method is called.
	ErrorOnInitialize error
	// ErrorOnStore if not nil, the error is thrown when the Store method is called.
	ErrorOnStore error
	// ErrorOnGet if not nil, the error is thrown when the Get method is called.
	ErrorOnGet error
	// ErrorOnDelete if not nil, the error is thrown when the Delete method is called.
	ErrorOnDelete error

	lock sync.RWMutex
	lg   logr.Logger
}

var _ secretstorage.SecretStorage = (*MemorySecretStorage)(nil)

func (m *MemorySecretStorage) Initialize(ctx context.Context) error {
	if m.ErrorOnInitialize != nil {
		return m.ErrorOnInitialize
	}

	m.lg = log.FromContext(ctx)
	m.lock.Lock()
	defer m.lock.Unlock()

	m.Secrets = map[secretstorage.SecretUid][]byte{}
	return nil
}

func (m *MemorySecretStorage) Store(_ context.Context, uid secretstorage.SecretUid, data []byte) error {
	if m.ErrorOnStore != nil {
		return m.ErrorOnStore
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	m.ensureTokens()

	m.Secrets[uid] = data
	m.lg.V(logs.DebugLevel).Info("memory storage stored: ", "id", uid, "len(secretData)", fmt.Sprint(len(data)))

	return nil
}

func (m *MemorySecretStorage) Get(_ context.Context, uid secretstorage.SecretUid) ([]byte, error) {
	if m.ErrorOnGet != nil {
		return nil, m.ErrorOnGet
	}

	m.lock.RLock()
	defer m.lock.RUnlock()

	m.ensureTokens()

	data, ok := m.Secrets[uid]
	if !ok {
		return nil, nil
	}

	return data, nil
}

func (m *MemorySecretStorage) Delete(_ context.Context, uid secretstorage.SecretUid) error {
	if m.ErrorOnDelete != nil {
		return m.ErrorOnDelete
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	m.ensureTokens()

	delete(m.Secrets, uid)
	m.lg.V(logs.DebugLevel).Info("memory storage deletes: ", "id", uid)

	return nil
}

func (m *MemorySecretStorage) ensureTokens() {
	if m.Secrets == nil {
		m.Secrets = map[secretstorage.SecretUid][]byte{}
	}
}
