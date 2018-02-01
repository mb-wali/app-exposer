app-exposer
===========

This is a service that runs inside of a Kubernetes cluster namespace and implements CRUD operations for exposing interactive apps as a Service and Endpoint.

# TODO
1. Document how to programmatically create an Endpoint using the Kubernetes API.
1. Document how to programmatically create a Service using the Kubernetes API.
1. Design the HTTP/JSON endpoints.
1. Create the interfaces for interacting with the Kubernetes API. Useful for stubbing out the API in tests.
1. Write concrete implementations for creating an Endpoint using the real Kubernetes API.
1. Write concreate implementations for creating a Service using the real Kubernetes API.
1. Write the HTTP/JSON endpoints.
