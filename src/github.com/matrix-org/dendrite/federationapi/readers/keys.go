// Copyright 2017 Vector Creations Ltd
//
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

package readers

import (
	"encoding/json"
	"github.com/matrix-org/dendrite/federationapi/config"
	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/util"
	"net/http"
	"time"
)

func localKeys(cfg config.FederationAPI, validUntil time.Time) (*gomatrixserverlib.ServerKeys, error) {
	var keys gomatrixserverlib.ServerKeys

	keys.ServerName = cfg.ServerName
	keys.FromServer = cfg.ServerName

	publicKey := cfg.PrivateKey.Public().([]byte)

	keys.VerifyKeys = map[gomatrixserverlib.KeyID]gomatrixserverlib.VerifyKey{
		cfg.KeyID: gomatrixserverlib.VerifyKey{
			gomatrixserverlib.Base64String(publicKey),
		},
	}

	for _, fingerprint := range cfg.TLSFingerPrints {
		keys.TLSFingerprints = append(
			keys.TLSFingerprints, gomatrixserverlib.TLSFingerprint{
				gomatrixserverlib.Base64String(fingerprint),
			},
		)
	}

	keys.OldVerifyKeys = map[gomatrixserverlib.KeyID]gomatrixserverlib.OldVerifyKey{}
	keys.ValidUntilTS = gomatrixserverlib.AsTimestamp(validUntil)

	toSign, err := json.Marshal(keys.ServerKeyFields)
	if err != nil {
		return nil, err
	}

	keys.Raw, err = gomatrixserverlib.SignJSON(string(cfg.ServerName), cfg.KeyID, cfg.PrivateKey, toSign)
	if err != nil {
		return nil, err
	}

	return &keys, nil
}

// LocalKeys returns the local keys for the server.
func LocalKeys(req *http.Request, cfg config.FederationAPI) util.JSONResponse {
	keys, err := localKeys(cfg, time.Now().Add(cfg.ValidityPeriod))
	if err != nil {
		return util.ErrorResponse(err)
	}
	return util.JSONResponse{JSON: keys}
}
