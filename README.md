app-exposer
===========

This is a service that runs inside of a Kubernetes cluster namespace and implements CRUD operations for exposing interactive apps as a Service and Endpoint.

# Development

You'll need to install ```go dep```.

``` go get -u github.com/golang/dep/cmd/dep ```

Then, in the top-level of the cloned repository:

```dep ensure```

Finally, run the following in the same directory:

``` go install ./... ```

After that, the difficult stuff should be done.


# TODO
1. Document how to programmatically create an Endpoint using the Kubernetes API.
    Use the client-go package: https://github.com/kubernetes/client-go
    Here's a code example of how to create an Endpoint using the client-go package: https://sourcegraph.com/github.com/kubernetes/client-go@4b76cf9824ec474ca9b122449fee23807a51e786/-/blob/tools/leaderelection/resourcelock/endpointslock.go#L63:62

1. Document how to programmatically create a Service using the Kubernetes API.
	Here's a starting point: https://godoc.org/k8s.io/client-go/kubernetes/typed/core/v1#CoreV1Client.Services

1. Document how to programmatically configure an Ingress using the Kubernetes API
	Here's a starting point: https://godoc.org/k8s.io/client-go/kubernetes/typed/extensions/v1beta1#ExtensionsV1beta1Client.Ingresses

1. Design the HTTP/JSON endpoints.
    Read entry          - GET /{name} -> 200 + {"ip":"", "port":""}
    Create/Update entry - PUT /{name} {"ip:"", "port":""} -> 200 or error code
    Delete entry        - DELETE /{name} -> 200 or error

1. Create the interfaces for interacting with the Kubernetes API. Useful for stubbing out the API in tests.
1. Write concrete implementations for creating an Endpoint using the real Kubernetes API.
1. Write concreate implementations for creating a Service using the real Kubernetes API.
1. Write the HTTP/JSON endpoints.
