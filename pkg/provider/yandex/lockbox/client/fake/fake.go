/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package fake

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/go-cmp/cmp"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/lockbox/v1"
	"github.com/yandex-cloud/go-sdk/iamkey"

	"github.com/external-secrets/external-secrets/pkg/provider/yandex/lockbox/client"
)

// Fake implementation of LockboxClientCreator.
type LockboxClientCreator struct {
	Backend *LockboxBackend
}

func (lcc *LockboxClientCreator) Create(ctx context.Context, apiEndpoint string, authorizedKey *iamkey.Key) (client.LockboxClient, error) {
	return &LockboxClient{lcc.Backend, authorizedKey}, nil
}

// Fake implementation of LockboxClient.
type LockboxClient struct {
	fakeLockboxBackend *LockboxBackend
	authorizedKey      *iamkey.Key
}

func (lc *LockboxClient) GetPayloadEntries(ctx context.Context, secretID, versionID string) ([]*lockbox.Payload_Entry, error) {
	return lc.fakeLockboxBackend.getEntries(lc.authorizedKey, secretID, versionID)
}

func (lc *LockboxClient) Close(ctx context.Context) error {
	return nil
}

// Fakes Yandex Lockbox service backend.
type LockboxBackend struct {
	lastSecretID  int               // new secret IDs are generated by incrementing lastSecretID
	lastVersionID map[secretKey]int // new version IDs are generated by incrementing lastVersionID[secretKey]

	secretMap  map[secretKey]secretValue   // secret specific data
	versionMap map[versionKey]versionValue // version specific data
}

type secretKey struct {
	secretID string
}

type secretValue struct {
	expectedAuthorizedKey *iamkey.Key // authorized key expected to access the secret
}

type versionKey struct {
	secretID  string
	versionID string
}

type versionValue struct {
	entries []*lockbox.Payload_Entry
}

func NewLockboxBackend() *LockboxBackend {
	return &LockboxBackend{
		lastSecretID:  0,
		lastVersionID: make(map[secretKey]int),
		secretMap:     make(map[secretKey]secretValue),
		versionMap:    make(map[versionKey]versionValue),
	}
}

func (lb *LockboxBackend) CreateSecret(authorizedKey *iamkey.Key, entries ...*lockbox.Payload_Entry) (string, string) {
	secretID := lb.genSecretID()
	versionID := lb.genVersionID(secretID)

	lb.secretMap[secretKey{secretID}] = secretValue{authorizedKey}
	lb.versionMap[versionKey{secretID, ""}] = versionValue{entries} // empty versionID corresponds to the latest version
	lb.versionMap[versionKey{secretID, versionID}] = versionValue{entries}

	return secretID, versionID
}

func (lb *LockboxBackend) AddVersion(secretID string, entries ...*lockbox.Payload_Entry) string {
	versionID := lb.genVersionID(secretID)

	lb.versionMap[versionKey{secretID, ""}] = versionValue{entries} // empty versionID corresponds to the latest version
	lb.versionMap[versionKey{secretID, versionID}] = versionValue{entries}

	return versionID
}

func (lb *LockboxBackend) getEntries(authorizedKey *iamkey.Key, secretID, versionID string) ([]*lockbox.Payload_Entry, error) {
	if _, ok := lb.secretMap[secretKey{secretID}]; !ok {
		return nil, fmt.Errorf("secret not found")
	}
	if _, ok := lb.versionMap[versionKey{secretID, versionID}]; !ok {
		return nil, fmt.Errorf("version not found")
	}
	if !cmp.Equal(authorizedKey, lb.secretMap[secretKey{secretID}].expectedAuthorizedKey) {
		return nil, fmt.Errorf("permission denied")
	}
	return lb.versionMap[versionKey{secretID, versionID}].entries, nil
}

func (lb *LockboxBackend) genSecretID() string {
	lb.lastSecretID++
	return intToString(lb.lastSecretID)
}

func (lb *LockboxBackend) genVersionID(secretID string) string {
	lb.lastVersionID[secretKey{secretID}]++
	return intToString(lb.lastVersionID[secretKey{secretID}])
}

func intToString(i int) string {
	return strconv.FormatInt(int64(i), 10)
}
