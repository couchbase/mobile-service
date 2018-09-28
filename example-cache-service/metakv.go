// @author Couchbase <info@couchbase.com>
// @copyright 2015 Couchbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package example_cache_service

import (
	"encoding/json"

	"github.com/couchbase/cbauth/metakv"
)

func MetakvGet(path string, v interface{}) (bool, error) {
	raw, _, err := metakv.Get(path)
	if err != nil {
		return false, err
		// log.Fatalf("Failed to fetch %s from metakv: %s", path, err.Error())
	}

	if raw == nil {
		return false, nil
	}

	err = json.Unmarshal(raw, v)
	if err != nil {
		//log.Fatalf("Failed unmarshalling value for %s: %s\n%s",
		//	path, err.Error(), string(raw))
		return false, err
	}

	return true, nil
}

func MetakvSet(path string, v interface{}) (err error) {
	raw, err := json.Marshal(v)
	if err != nil {
		//log.Fatalf("Failed to marshal value for %s: %s\n%v",
		//	path, err.Error(), v)
		return err
	}

	err = metakv.Set(path, raw, nil)
	if err != nil {
		// log.Fatalf("Failed to set %s: %s", path, err.Error())
		return err
	}

	return nil
}
