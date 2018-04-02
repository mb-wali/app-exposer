package main

import (
	"net/http"
	"testing"

	"github.com/gorilla/mux"
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
