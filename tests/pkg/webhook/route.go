// Copyright 2019 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package webhook

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework/log"
)

// toAdmissionResponse is a helper function to create an AdmissionResponse
// with an embedded error
func toAdmissionResponse(err error) *v1.AdmissionResponse {
	return &v1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

// admitFunc is the type we use for all of our validators and mutators
type admitFunc func(v1.AdmissionReview) *v1.AdmissionResponse

// serve handles the http portion of a request prior to handing to an admit
// function
func serve(w http.ResponseWriter, r *http.Request, admit admitFunc) {

	var body []byte
	var contentType string
	responseAdmissionReview := v1.AdmissionReview{}
	requestedAdmissionReview := v1.AdmissionReview{}
	deserializer := codecs.UniversalDeserializer()

	// The AdmissionReview that will be returned
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		} else {
			responseAdmissionReview.Response = toAdmissionResponse(err)
			goto returnData
		}
	} else {
		err := errors.New("request body is nil")
		responseAdmissionReview.Response = toAdmissionResponse(err)
		goto returnData
	}

	// verify the content type is accurate
	contentType = r.Header.Get("Content-Type")
	if contentType != "application/json" {
		err := errors.New("expect application/json")
		responseAdmissionReview.Response = toAdmissionResponse(err)
		goto returnData
	}

	// The AdmissionReview that was sent to the webhook
	if _, _, err := deserializer.Decode(body, nil, &requestedAdmissionReview); err != nil {
		responseAdmissionReview.Response = toAdmissionResponse(err)
	} else {
		// pass to admitFunc
		responseAdmissionReview.Response = admit(requestedAdmissionReview)
	}

	// Return the same UID
	responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID

returnData:
	respBytes, err := json.Marshal(responseAdmissionReview)
	if err != nil {
		log.Logf("ERROR: %v", err)
	}
	if _, err := w.Write(respBytes); err != nil {
		log.Logf("ERROR: %v", err)
	}
}

func (wh *webhook) ServePods(w http.ResponseWriter, r *http.Request) {
	serve(w, r, wh.admitPods)
}
