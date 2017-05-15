#!/bin/bash -e
# Copyright 2016-2017 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Run robot integration tests locally without drone.
# Switch environments just by changing GOVC_URL

if ! govc version -require 0.14.0; then
    echo "govc version must be updated"
    exit 1
fi

cd "$(git rev-parse --show-toplevel)"

export TEST_URL_ARRAY=$(govc env -x GOVC_URL_HOST)
export TEST_USERNAME=$(govc env GOVC_USERNAME)
export TEST_PASSWORD=$(govc env GOVC_PASSWORD)
export TEST_DATASTORE=${GOVC_DATASTORE:-$(basename "$(govc ls datastore)")}
export TEST_RESOURCE=${GOVC_RESOURCE_POOL:-$(govc ls host/*/Resources)}
export BRIDGE_NETWORK=$BRIDGE_NETWORK
export PUBLIC_NETWORK=$PUBLIC_NETWORK
export GITHUB_AUTOMATION_API_KEY=$GITHUB_TOKEN
export TEST_TIMEOUT=60s

pybot --exclude skip "$@"
