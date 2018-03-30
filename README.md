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
