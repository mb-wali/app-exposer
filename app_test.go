package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewExposerApp(t *testing.T) {
	expectedNS := "testing"
	testcs := fake.NewSimpleClientset()
	testapp := NewExposerApp(expectedNS, testcs)

	if testapp.namespace != expectedNS {
		t.Errorf("namespace was %s, not %s", testapp.namespace, expectedNS)
	}

	if testapp.ServiceController == nil {
		t.Error("ServiceController is nil")
	}

	if testapp.EndpointController == nil {
		t.Error("EndpointController is nil")
	}

	if testapp.IngressController == nil {
		t.Error("IngressController is nil")
	}

	if testapp.router == nil {
		t.Error("router is nil")
	}

	reqs := [][]string{
		[]string{"GET", "/", ""},
		[]string{"POST", "/service/test", "test"},
		[]string{"PUT", "/service/test", "test"},
		[]string{"GET", "/service/test", "test"},
		[]string{"DELETE", "/service/test", "test"},
		[]string{"POST", "/endpoint/test", "test"},
		[]string{"PUT", "/endpoint/test", "test"},
		[]string{"GET", "/endpoint/test", "test"},
		[]string{"DELETE", "/endpoint/test", "test"},
		[]string{"POST", "/ingress/test", "test"},
		[]string{"PUT", "/ingress/test", "test"},
		[]string{"GET", "/ingress/test", "test"},
		[]string{"DELETE", "/ingress/test", "test"},
	}

	for _, fields := range reqs {
		method := fields[0]
		path := fields[1]
		name := fields[2]
		req, err := http.NewRequest(method, path, nil)
		if err != nil {
			t.Error(err)
		}

		rm := &mux.RouteMatch{}
		if !testapp.router.Match(req, rm) {
			t.Errorf("%s %s does not match", method, path)
		}

		if name != "" {
			actual, ok := rm.Vars["name"]
			if !ok {
				t.Errorf("vars did not have %s as a key", name)
			}
			if actual != name {
				t.Errorf("name was %s, not %s", actual, name)
			}
		}
	}
}

func TestWriteService(t *testing.T) {
	expected := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				v1.ServicePort{
					Port:       60000,
					TargetPort: intstr.FromInt(60001),
				},
			},
		},
	}
	writer := httptest.NewRecorder()

	WriteService(expected, writer)

	resp := writer.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}

	actual := &ServiceOptions{}
	err = json.Unmarshal(rbody, actual)
	if err != nil {
		t.Error(err)
	}

	if actual.Name != expected.Name {
		t.Errorf("service name was %s, not %s", actual.Name, expected.Name)
	}

	if actual.Namespace != expected.Namespace {
		t.Errorf("service namespace was %s, not %s", actual.Namespace, expected.Namespace)
	}

	if actual.ListenPort != expected.Spec.Ports[0].Port {
		t.Errorf("service listen port was %d, not %d", actual.ListenPort, expected.Spec.Ports[0].Port)
	}

	if actual.TargetPort != expected.Spec.Ports[0].TargetPort.IntValue() {
		t.Errorf("service target port was %d, not %d", actual.TargetPort, expected.Spec.Ports[0].TargetPort.IntValue())
	}
}

func TestWriteEndpoints(t *testing.T) {
	expected := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{{IP: "1.1.1.1"}},
				Ports:     []v1.EndpointPort{{Port: 60000}},
			},
		},
	}

	writer := httptest.NewRecorder()

	WriteEndpoint(expected, writer)

	resp := writer.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}

	actual := &EndpointOptions{}
	err = json.Unmarshal(rbody, actual)
	if err != nil {
		t.Error(err)
	}

	if actual.Name != expected.Name {
		t.Errorf("endpoint name was %s, not %s", actual.Name, expected.Name)
	}

	if actual.Namespace != expected.Namespace {
		t.Errorf("endpoint namespace was %s, not %s", actual.Namespace, expected.Namespace)
	}

	if actual.IP != expected.Subsets[0].Addresses[0].IP {
		t.Errorf("endpoint IP was %s, not %s", actual.IP, expected.Subsets[0].Addresses[0].IP)
	}

	if actual.Port != expected.Subsets[0].Ports[0].Port {
		t.Errorf("endpoint port was %d, not %d", actual.Port, expected.Subsets[0].Ports[0].Port)
	}
}

func TestWriteIngress(t *testing.T) {
	expected := &extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: extv1beta1.IngressSpec{
			Backend: &extv1beta1.IngressBackend{
				ServiceName: "test-service",
				ServicePort: intstr.FromInt(60000),
			},
		},
	}

	writer := httptest.NewRecorder()

	WriteIngress(expected, writer)

	resp := writer.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}

	actual := &IngressOptions{}
	err = json.Unmarshal(rbody, actual)
	if err != nil {
		t.Error(err)
	}

	if actual.Name != expected.Name {
		t.Errorf("ingress name was %s, not %s", actual.Name, expected.Name)
	}

	if actual.Namespace != expected.Namespace {
		t.Errorf("ingress namespace was %s, not %s", actual.Namespace, expected.Namespace)
	}

	if actual.Service != expected.Spec.Backend.ServiceName {
		t.Errorf("ingress service name was %s, not %s", actual.Service, expected.Spec.Backend.ServiceName)
	}

	if actual.Port != expected.Spec.Backend.ServicePort.IntValue() {
		t.Errorf("ingress service port was %d, not %d", actual.Port, expected.Spec.Backend.ServicePort.IntValue())
	}
}

func TestCreateService(t *testing.T) {
	expectedNS := "testing"
	testcs := fake.NewSimpleClientset()
	testapp := NewExposerApp(expectedNS, testcs)

	expectedOpts := &ServiceOptions{
		TargetPort: 60000,
		ListenPort: 60001,
	}

	expectedJSON, err := json.Marshal(expectedOpts)
	if err != nil {
		t.Error(err)
	}

	expectedName := "test-name"
	req := httptest.NewRequest("POST", fmt.Sprintf("/service/%s", expectedName), bytes.NewReader(expectedJSON))
	w := httptest.NewRecorder()

	testapp.router.ServeHTTP(w, req)

	resp := w.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		t.Error(err)
	}

	actualOpts := &ServiceOptions{}
	err = json.Unmarshal(rbody, actualOpts)
	if err != nil {
		t.Error(err)
	}

	if actualOpts.Name != expectedName {
		t.Errorf("service name was %s, not %s", actualOpts.Name, expectedName)
	}

	if actualOpts.Namespace != expectedNS {
		t.Errorf("service namespace was %s, not %s", actualOpts.Namespace, expectedNS)
	}

	if actualOpts.TargetPort != expectedOpts.TargetPort {
		t.Errorf("service target port was %d, not %d", actualOpts.TargetPort, expectedOpts.TargetPort)
	}

	if actualOpts.ListenPort != expectedOpts.ListenPort {
		t.Errorf("service listen port was %d, not %d", actualOpts.ListenPort, expectedOpts.ListenPort)
	}
}

func TestUpdateService(t *testing.T) {
	expectedNS := "testing"
	testcs := fake.NewSimpleClientset()
	testapp := NewExposerApp(expectedNS, testcs)

	createOpts := &ServiceOptions{
		TargetPort: 40000,
		ListenPort: 40001,
	}

	expectedOpts := &ServiceOptions{
		TargetPort: 60000,
		ListenPort: 60001,
	}

	createJSON, err := json.Marshal(createOpts)
	if err != nil {
		t.Error(err)
	}

	expectedJSON, err := json.Marshal(expectedOpts)
	if err != nil {
		t.Error(err)
	}

	expectedName := "test-name"

	addreq, err := http.NewRequest("POST", fmt.Sprintf("/service/%s", expectedName), bytes.NewReader(createJSON))
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("/service/%s", expectedName), bytes.NewReader(expectedJSON))
	if err != nil {
		t.Error(err)
	}

	cw := httptest.NewRecorder()
	w := httptest.NewRecorder()

	testapp.router.ServeHTTP(cw, addreq) // Need to create the object before updating it.
	testapp.router.ServeHTTP(w, req)

	resp := w.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		t.Error(err)
	}

	actualOpts := &ServiceOptions{}
	err = json.Unmarshal(rbody, actualOpts)
	if err != nil {
		t.Error(err)
	}

	if actualOpts.Name != expectedName {
		t.Errorf("service name was %s, not %s", actualOpts.Name, expectedName)
	}

	if actualOpts.Namespace != expectedNS {
		t.Errorf("service namespace was %s, not %s", actualOpts.Namespace, expectedNS)
	}

	if actualOpts.TargetPort != expectedOpts.TargetPort {
		t.Errorf("service target port was %d, not %d", actualOpts.TargetPort, expectedOpts.TargetPort)
	}

	if actualOpts.ListenPort != expectedOpts.ListenPort {
		t.Errorf("service listen port was %d, not %d", actualOpts.ListenPort, expectedOpts.ListenPort)
	}
}
