# Copyright 2016 Google Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
################################################################################
FROM golang:1.5.3

RUN apt-get update
RUN apt-get install -y -qq ca-certificates

RUN mkdir -p /go/src/app
WORKDIR /go/src/app

RUN mkdir -p $GOPATH/src/k8s.io && \
    cd $GOPATH/src/k8s.io && \
    git clone https://github.com/kubernetes/client-go.git && \
    cd client-go && git checkout v1.4.0

COPY . /go/src/app

RUN go-wrapper download
RUN go-wrapper install

CMD ["go-wrapper", "run"]
