package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cyverse-de/app-exposer/external"
	"github.com/gorilla/mux"
	v1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewExposerApp(t *testing.T) {
	expectedNS := "testing"
	testcs := fake.NewSimpleClientset()

	testinit := &ExposerAppInit{
		Namespace:     expectedNS,
		ViceNamespace: "",
	}

	testapp := NewExposerApp(testinit, "linkerd", testcs)

	if testapp.namespace != expectedNS {
		t.Errorf("namespace was %s, not %s", testapp.namespace, expectedNS)
	}

	if testapp.external.ServiceController == nil {
		t.Error("ServiceController is nil")
	}

	if testapp.external.EndpointController == nil {
		t.Error("EndpointController is nil")
	}

	if testapp.external.IngressController == nil {
		t.Error("IngressController is nil")
	}

	if testapp.router == nil {
		t.Error("router is nil")
	}

	reqs := [][]string{
		{"GET", "/", ""},
		{"POST", "/service/test", "test"},
		{"PUT", "/service/test", "test"},
		{"GET", "/service/test", "test"},
		{"DELETE", "/service/test", "test"},
		{"POST", "/endpoint/test", "test"},
		{"PUT", "/endpoint/test", "test"},
		{"GET", "/endpoint/test", "test"},
		{"DELETE", "/endpoint/test", "test"},
		{"POST", "/ingress/test", "test"},
		{"PUT", "/ingress/test", "test"},
		{"GET", "/ingress/test", "test"},
		{"DELETE", "/ingress/test", "test"},
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
				{
					Port:       60000,
					TargetPort: intstr.FromInt(60001),
				},
			},
		},
	}
	writer := httptest.NewRecorder()

	external.WriteService(expected, writer)

	resp := writer.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}

	actual := &external.ServiceOptions{}
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

	external.WriteEndpoint(expected, writer)

	resp := writer.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}

	actual := &external.EndpointOptions{}
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

	external.WriteIngress(expected, writer)

	resp := writer.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}

	actual := &external.IngressOptions{}
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

	testinit := &ExposerAppInit{
		Namespace:     expectedNS,
		ViceNamespace: "",
	}

	testapp := NewExposerApp(testinit, "linkerd", testcs)

	expectedOpts := &external.ServiceOptions{
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

	actualOpts := &external.ServiceOptions{}
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

// createLoadApp is a utility function that creates a new ExposerApp, loads a
// service into it, and returns the app.
func createAppLoadService(ns, name string) (*ExposerApp, error) {
	testcs := fake.NewSimpleClientset()

	testinit := &ExposerAppInit{
		Namespace:     ns,
		ViceNamespace: "",
	}

	testapp := NewExposerApp(testinit, "linkerd", testcs)

	createOpts := &external.ServiceOptions{
		TargetPort: 40000,
		ListenPort: 40001,
	}

	createJSON, err := json.Marshal(createOpts)
	if err != nil {
		return nil, err
	}

	addreq, err := http.NewRequest("POST", fmt.Sprintf("/service/%s", name), bytes.NewReader(createJSON))
	if err != nil {
		return nil, err
	}

	cw := httptest.NewRecorder()

	testapp.router.ServeHTTP(cw, addreq)

	return testapp, nil
}

func TestUpdateService(t *testing.T) {
	expectedNS := "testing"
	expectedName := "test-name"

	testapp, err := createAppLoadService(expectedNS, expectedName)
	if err != nil {
		t.Error(err)
	}

	expectedOpts := &external.ServiceOptions{
		TargetPort: 60000,
		ListenPort: 60001,
	}

	expectedJSON, err := json.Marshal(expectedOpts)
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("/service/%s", expectedName), bytes.NewReader(expectedJSON))
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()

	testapp.router.ServeHTTP(w, req)

	resp := w.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		t.Error(err)
	}

	actualOpts := &external.ServiceOptions{}
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

func TestGetService(t *testing.T) {
	expectedNS := "testing"
	expectedName := "test-name"

	testapp, err := createAppLoadService(expectedNS, expectedName)
	if err != nil {
		t.Error(err)
	}

	expectedOpts := &external.ServiceOptions{
		TargetPort: 40000,
		ListenPort: 40001,
	}

	expectedJSON, err := json.Marshal(expectedOpts)
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("/service/%s", expectedName), bytes.NewReader(expectedJSON))
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()

	testapp.router.ServeHTTP(w, req)

	resp := w.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		t.Error(err)
	}

	actualOpts := &external.ServiceOptions{}
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

func TestDeleteService(t *testing.T) {
	ns := "testing"
	name := "test-name"

	testapp, err := createAppLoadService(ns, name)
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("DELETE", fmt.Sprintf("/service/%s", name), nil)
	if err != nil {
		t.Error(err)
	}

	w := httptest.NewRecorder()

	testapp.router.ServeHTTP(w, req)

	resp := w.Result()

	if resp.StatusCode != 200 {
		t.Errorf("status code was %d not 200", resp.StatusCode)
	}
}

func TestCreateEndpoint(t *testing.T) {
	expectedNS := "testing"
	expectedName := "test-name"
	expectedIP := "1.1.1.1"
	var expectedPort int32 = 60000

	testcs := fake.NewSimpleClientset()

	testinit := &ExposerAppInit{
		Namespace:     expectedNS,
		ViceNamespace: "",
	}

	testapp := NewExposerApp(testinit, "linkerd", testcs)

	expectedOpts := &external.EndpointOptions{
		IP:   expectedIP,
		Port: expectedPort,
	}

	expectedJSON, err := json.Marshal(expectedOpts)
	if err != nil {
		t.Error(err)
	}

	req := httptest.NewRequest("POST", fmt.Sprintf("/endpoint/%s", expectedName), bytes.NewReader(expectedJSON))
	w := httptest.NewRecorder()

	testapp.router.ServeHTTP(w, req)

	resp := w.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		t.Error(err)
	}

	actualOpts := &external.EndpointOptions{}
	if err = json.Unmarshal(rbody, actualOpts); err != nil {
		t.Error(err)
	}

	if actualOpts.Name != expectedName {
		t.Errorf("endpoint name was %s, not %s", actualOpts.Name, expectedName)
	}

	if actualOpts.Namespace != expectedNS {
		t.Errorf("endpoint namespace was %s, not %s", actualOpts.Namespace, expectedNS)
	}

	if actualOpts.IP != expectedIP {
		t.Errorf("endpoint ip was %s, not %s", actualOpts.IP, expectedIP)
	}

	if actualOpts.Port != expectedPort {
		t.Errorf("expoint port %d, not %d", actualOpts.Port, expectedPort)
	}
}

func createAppLoadEndpoint(ns, name string) (*ExposerApp, error) {
	testcs := fake.NewSimpleClientset()
	testinit := &ExposerAppInit{
		Namespace:     ns,
		ViceNamespace: "",
	}
	testapp := NewExposerApp(testinit, "linkerd", testcs)

	createOpts := &external.EndpointOptions{
		IP:   "1.1.1.1",
		Port: 60000,
	}

	createJSON, err := json.Marshal(createOpts)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("/endpoint/%s", name), bytes.NewReader(createJSON))
	if err != nil {
		return nil, err
	}

	cw := httptest.NewRecorder()

	testapp.router.ServeHTTP(cw, req)

	return testapp, nil
}

func TestUpdateEndpoint(t *testing.T) {
	expectedNS := "testing"
	expectedName := "test-name"
	expectedIP := "1.1.1.2"
	var expectedPort int32 = 60001

	testapp, err := createAppLoadEndpoint(expectedNS, expectedName)
	if err != nil {
		t.Error(err)
	}

	expectedOpts := &external.EndpointOptions{
		IP:   expectedIP,
		Port: expectedPort,
	}

	expectedJSON, err := json.Marshal(expectedOpts)
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("/endpoint/%s", expectedName), bytes.NewReader(expectedJSON))
	if err != nil {
		t.Error(err)
	}

	w := httptest.NewRecorder()

	testapp.router.ServeHTTP(w, req)

	resp := w.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		t.Error(err)
	}

	actualOpts := &external.EndpointOptions{}
	if err = json.Unmarshal(rbody, actualOpts); err != nil {
		t.Error(err)
	}

	if actualOpts.Name != expectedName {
		t.Errorf("endpoint name was %s, not %s", actualOpts.Name, expectedName)
	}

	if actualOpts.Namespace != expectedNS {
		t.Errorf("service namespace was %s, not %s", actualOpts.Namespace, expectedNS)
	}

	if actualOpts.IP != expectedIP {
		t.Errorf("endpoint ip was %s, not %s", actualOpts.IP, expectedIP)
	}

	if actualOpts.Port != expectedPort {
		t.Errorf("expoint port %d, not %d", actualOpts.Port, expectedPort)
	}
}

func TestGetEndpoint(t *testing.T) {
	expectedNS := "testing"
	expectedName := "test-name"
	expectedIP := "1.1.1.1"
	var expectedPort int32 = 60000

	testapp, err := createAppLoadEndpoint(expectedNS, expectedName)
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("/endpoint/%s", expectedName), nil)
	if err != nil {
		t.Error(err)
	}

	w := httptest.NewRecorder()

	testapp.router.ServeHTTP(w, req)

	resp := w.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		t.Error(err)
	}

	actualOpts := &external.EndpointOptions{}
	if err = json.Unmarshal(rbody, actualOpts); err != nil {
		t.Error(err)
	}

	if actualOpts.Name != expectedName {
		t.Errorf("endpoint name was %s, not %s", actualOpts.Name, expectedName)
	}

	if actualOpts.Namespace != expectedNS {
		t.Errorf("service namespace was %s, not %s", actualOpts.Namespace, expectedNS)
	}

	if actualOpts.IP != expectedIP {
		t.Errorf("endpoint ip was %s, not %s", actualOpts.IP, expectedIP)
	}

	if actualOpts.Port != expectedPort {
		t.Errorf("expoint port %d, not %d", actualOpts.Port, expectedPort)
	}
}

func TestDeleteEndpoint(t *testing.T) {
	expectedNS := "testing"
	expectedName := "test-name"

	testapp, err := createAppLoadEndpoint(expectedNS, expectedName)
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("DELETE", fmt.Sprintf("/endpoint/%s", expectedName), nil)
	if err != nil {
		t.Error(err)
	}

	w := httptest.NewRecorder()

	testapp.router.ServeHTTP(w, req)

	resp := w.Result()

	if resp.StatusCode != 200 {
		t.Errorf("status code was %d not 200", resp.StatusCode)
	}
}

func TestCreateIngress(t *testing.T) {
	expectedNS := "testing"
	expectedName := "test-name"
	expectedService := "test-service"
	expectedPort := 60000

	testcs := fake.NewSimpleClientset()
	testinit := &ExposerAppInit{
		Namespace:     expectedNS,
		ViceNamespace: "",
	}

	testapp := NewExposerApp(testinit, "linkerd", testcs)

	expectedOpts := &external.IngressOptions{
		Service: expectedService,
		Port:    expectedPort,
	}

	expectedJSON, err := json.Marshal(expectedOpts)
	if err != nil {
		t.Error(err)
	}

	req := httptest.NewRequest("POST", fmt.Sprintf("/ingress/%s", expectedName), bytes.NewReader(expectedJSON))
	w := httptest.NewRecorder()

	testapp.router.ServeHTTP(w, req)

	resp := w.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		t.Error(err)
	}

	actualOpts := &external.IngressOptions{}
	if err = json.Unmarshal(rbody, actualOpts); err != nil {
		t.Error(err)
	}

	if actualOpts.Name != expectedName {
		t.Errorf("ingress name was %s, not %s", actualOpts.Name, expectedName)
	}

	if actualOpts.Namespace != expectedNS {
		t.Errorf("ingress namespace was %s, not %s", actualOpts.Namespace, expectedNS)
	}

	if actualOpts.Service != expectedService {
		t.Errorf("ingress service was %s, not %s", actualOpts.Service, expectedService)
	}

	if actualOpts.Port != expectedPort {
		t.Errorf("ingress port was %d, not %d", actualOpts.Port, expectedPort)
	}
}

func createAppLoadIngress(ns, name string) (*ExposerApp, error) {
	testcs := fake.NewSimpleClientset()
	testinit := &ExposerAppInit{
		Namespace:     ns,
		ViceNamespace: "",
	}
	testapp := NewExposerApp(testinit, "linkerd", testcs)

	createOpts := &external.IngressOptions{
		Service: "test-service",
		Port:    60000,
	}

	createJSON, err := json.Marshal(createOpts)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("/ingress/%s", name), bytes.NewReader(createJSON))
	if err != nil {
		return nil, err
	}

	cw := httptest.NewRecorder()

	testapp.router.ServeHTTP(cw, req)

	return testapp, nil
}

func TestUpdateIngress(t *testing.T) {
	expectedNS := "testing"
	expectedName := "test-name"
	expectedService := "test-service1"
	expectedPort := 60001

	testapp, err := createAppLoadIngress(expectedNS, expectedName)
	if err != nil {
		t.Error(err)
	}

	expectedOpts := &external.IngressOptions{
		Service: expectedService,
		Port:    expectedPort,
	}

	expectedJSON, err := json.Marshal(expectedOpts)
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("/ingress/%s", expectedName), bytes.NewReader(expectedJSON))
	if err != nil {
		t.Error(err)
	}

	w := httptest.NewRecorder()

	testapp.router.ServeHTTP(w, req)

	resp := w.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		t.Error(err)
	}

	actualOpts := &external.IngressOptions{}
	if err = json.Unmarshal(rbody, actualOpts); err != nil {
		t.Error(err)
	}

	if actualOpts.Name != expectedName {
		t.Errorf("ingress name was %s, not %s", actualOpts.Name, expectedName)
	}

	if actualOpts.Namespace != expectedNS {
		t.Errorf("ingress namespace was %s, not %s", actualOpts.Namespace, expectedNS)
	}

	if actualOpts.Service != expectedService {
		t.Errorf("ingress service was %s, not %s", actualOpts.Service, expectedService)
	}

	if actualOpts.Port != expectedPort {
		t.Errorf("ingress port was %d, not %d", actualOpts.Port, expectedPort)
	}
}

func TestGetIngress(t *testing.T) {
	expectedNS := "testing"
	expectedName := "test-name"
	expectedService := "test-service"
	expectedPort := 60000

	testapp, err := createAppLoadIngress(expectedNS, expectedName)
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("/ingress/%s", expectedName), nil)
	if err != nil {
		t.Error(err)
	}

	w := httptest.NewRecorder()

	testapp.router.ServeHTTP(w, req)

	resp := w.Result()
	rbody, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		t.Error(err)
	}

	actualOpts := &external.IngressOptions{}
	if err = json.Unmarshal(rbody, actualOpts); err != nil {
		t.Error(err)
	}

	if actualOpts.Name != expectedName {
		t.Errorf("ingress name was %s, not %s", actualOpts.Name, expectedName)
	}

	if actualOpts.Namespace != expectedNS {
		t.Errorf("ingress namespace was %s, not %s", actualOpts.Namespace, expectedNS)
	}

	if actualOpts.Service != expectedService {
		t.Errorf("ingress service was %s, not %s", actualOpts.Service, expectedService)
	}

	if actualOpts.Port != expectedPort {
		t.Errorf("ingress port was %d, not %d", actualOpts.Port, expectedPort)
	}
}

func TestDeleteIngress(t *testing.T) {
	expectedNS := "testing"
	expectedName := "test-name"

	testapp, err := createAppLoadIngress(expectedNS, expectedName)
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("DELETE", fmt.Sprintf("/ingress/%s", expectedName), nil)
	if err != nil {
		t.Error(err)
	}

	w := httptest.NewRecorder()

	testapp.router.ServeHTTP(w, req)

	resp := w.Result()

	if resp.StatusCode != 200 {
		t.Errorf("status code was %d not 200", resp.StatusCode)
	}
}
