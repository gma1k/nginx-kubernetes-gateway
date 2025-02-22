package resolver

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/nginxinc/nginx-kubernetes-gateway/internal/framework/helpers"
)

var (
	svcPortName = "svc-port"

	addresses = []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}

	readyEndpoint1 = discoveryV1.Endpoint{
		Addresses:  addresses,
		Conditions: discoveryV1.EndpointConditions{Ready: helpers.GetBoolPointer(true)},
	}

	notReadyEndpoint = discoveryV1.Endpoint{
		Addresses:  addresses,
		Conditions: discoveryV1.EndpointConditions{Ready: helpers.GetBoolPointer(false)},
	}

	mixedValidityEndpointSlice = discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints:   []discoveryV1.Endpoint{readyEndpoint1, notReadyEndpoint, readyEndpoint1}, // 6 valid endpoints
		Ports: []discoveryV1.EndpointPort{
			{
				Name: &svcPortName,
				Port: helpers.GetInt32Pointer(80),
			},
		},
	}

	nilEndpoints = discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints:   nil,
	}

	validEndpointSlice = discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints: []discoveryV1.Endpoint{
			readyEndpoint1,
			readyEndpoint1,
			readyEndpoint1,
		}, // in reality these endpoints would be different but for this test it doesn't matter
		Ports: []discoveryV1.EndpointPort{
			{
				Name: &svcPortName,
				Port: helpers.GetInt32Pointer(80),
			},
		},
	}

	invalidAddressTypeEndpointSlice = discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv6,
		Endpoints:   []discoveryV1.Endpoint{readyEndpoint1},
		Ports: []discoveryV1.EndpointPort{
			{
				Name: &svcPortName,
				Port: helpers.GetInt32Pointer(80),
			},
		},
	}

	invalidPortEndpointSlice = discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints:   []discoveryV1.Endpoint{readyEndpoint1},
		Ports: []discoveryV1.EndpointPort{
			{
				Name: helpers.GetStringPointer("other-svc-port"),
				Port: helpers.GetInt32Pointer(8080),
			},
		},
	}
)

func TestFilterEndpointSliceList(t *testing.T) {
	sliceList := discoveryV1.EndpointSliceList{
		Items: []discoveryV1.EndpointSlice{
			validEndpointSlice,
			invalidAddressTypeEndpointSlice,
			invalidPortEndpointSlice,
			nilEndpoints,
			mixedValidityEndpointSlice,
		},
	}

	svcPort := v1.ServicePort{
		Name:       svcPortName,
		Port:       8080,
		TargetPort: intstr.FromInt(80),
	}

	expFilteredList := []discoveryV1.EndpointSlice{validEndpointSlice, mixedValidityEndpointSlice}

	filteredSliceList := filterEndpointSliceList(sliceList, svcPort)
	if diff := cmp.Diff(expFilteredList, filteredSliceList); diff != "" {
		t.Errorf("filterEndpointSliceList() mismatch (-want +got):\n%s", diff)
	}
}

func TestGetServicePort(t *testing.T) {
	svc := &v1.Service{
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Port: 80,
				},
				{
					Port: 81,
				},
				{
					Port: 82,
				},
			},
		},
	}

	// ports exist
	for _, p := range []int32{80, 81, 82} {
		port, err := getServicePort(svc, p)
		if err != nil {
			t.Errorf("getServicePort() returned an error for port %d: %v", p, err)
		}
		if port.Port != p {
			t.Errorf("getServicePort() returned the wrong port for port %d; expected %d, got %d", p, p, port.Port)
		}
	}

	// port doesn't exist
	port, err := getServicePort(svc, 83)
	if err == nil {
		t.Errorf("getServicePort() didn't return an error for port 83")
	}
	if port.Port != 0 {
		t.Errorf("getServicePort() returned the wrong port for port 83; expected 0, got %d", port.Port)
	}
}

func TestGetDefaultPort(t *testing.T) {
	testcases := []struct {
		msg     string
		svcPort v1.ServicePort
		expPort int32
	}{
		{
			msg: "int target port",
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
			expPort: 8080,
		},
		{
			msg: "string target port",
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromString("http"),
			},
			expPort: 80,
		},
		{
			msg: "no target port",
			svcPort: v1.ServicePort{
				Port: 80,
			},
			expPort: 80,
		},
	}
	for _, tc := range testcases {
		port := getDefaultPort(tc.svcPort)

		if tc.expPort != port {
			t.Errorf("getTargetPort() mismatch on port for %q; expected %d, got %d", tc.msg, tc.expPort, port)
		}
	}
}

func TestIgnoreEndpointSlice(t *testing.T) {
	var (
		port4000 int32 = 4000
		port8080 int32 = 8080
	)

	testcases := []struct {
		msg         string
		slice       discoveryV1.EndpointSlice
		servicePort v1.ServicePort
		ignore      bool
	}{
		{
			msg: "IPV6 address type",
			slice: discoveryV1.EndpointSlice{
				AddressType: discoveryV1.AddressTypeIPv6,
				Ports: []discoveryV1.EndpointPort{
					{
						Name: &svcPortName,
						Port: &port8080,
					},
				},
			},
			servicePort: v1.ServicePort{
				Name:       svcPortName,
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
			ignore: true,
		},
		{
			msg: "FQDN address type",
			slice: discoveryV1.EndpointSlice{
				AddressType: discoveryV1.AddressTypeFQDN,
				Ports: []discoveryV1.EndpointPort{
					{
						Name: &svcPortName,
						Port: &port8080,
					},
				},
			},
			servicePort: v1.ServicePort{
				Name:       svcPortName,
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
			ignore: true,
		},
		{
			msg: "no matching port",
			slice: discoveryV1.EndpointSlice{
				AddressType: discoveryV1.AddressTypeIPv4,
				Ports: []discoveryV1.EndpointPort{
					{
						Name: helpers.GetStringPointer("other-svc-port"),
						Port: &port4000,
					},
				},
			},
			servicePort: v1.ServicePort{
				Name:       svcPortName,
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
			ignore: true,
		},
		{
			msg: "nil endpoint port",
			slice: discoveryV1.EndpointSlice{
				AddressType: discoveryV1.AddressTypeIPv4,
				Ports: []discoveryV1.EndpointPort{
					{
						Port: nil,
					},
				},
			},
			servicePort: v1.ServicePort{
				Name:       svcPortName,
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
			ignore: false,
		},
		{
			msg: "normal",
			slice: discoveryV1.EndpointSlice{
				AddressType: discoveryV1.AddressTypeIPv4,
				Ports: []discoveryV1.EndpointPort{
					{
						Name: &svcPortName,
						Port: &port8080,
					},
				},
			},
			servicePort: v1.ServicePort{
				Name:       svcPortName,
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
			ignore: false,
		},
	}
	for _, tc := range testcases {
		if ignoreEndpointSlice(tc.slice, tc.servicePort) != tc.ignore {
			t.Errorf("ignoreEndpointSlice() mismatch for %q; expected %t", tc.msg, tc.ignore)
		}
	}
}

func TestEndpointReady(t *testing.T) {
	testcases := []struct {
		endpoint discoveryV1.Endpoint
		msg      string
		ready    bool
	}{
		{
			msg: "endpoint ready",
			endpoint: discoveryV1.Endpoint{
				Conditions: discoveryV1.EndpointConditions{
					Ready: helpers.GetBoolPointer(true),
				},
			},
			ready: true,
		},
		{
			msg: "nil ready",
			endpoint: discoveryV1.Endpoint{
				Conditions: discoveryV1.EndpointConditions{
					Ready: nil,
				},
			},
			ready: false,
		},
		{
			msg: "endpoint not ready",
			endpoint: discoveryV1.Endpoint{
				Conditions: discoveryV1.EndpointConditions{
					Ready: helpers.GetBoolPointer(false),
				},
			},
			ready: false,
		},
	}
	for _, tc := range testcases {
		if endpointReady(tc.endpoint) != tc.ready {
			t.Errorf("endpointReady() mismatch for %q; expected %t", tc.msg, tc.ready)
		}
	}
}

func TestFindPort(t *testing.T) {
	testcases := []struct {
		msg     string
		ports   []discoveryV1.EndpointPort
		svcPort v1.ServicePort
		expPort int32
	}{
		{
			msg: "nil endpoint port; int target port",
			ports: []discoveryV1.EndpointPort{
				{
					Port: nil,
				},
			},
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromInt(8080),
				Name:       svcPortName,
			},
			expPort: 8080,
		},
		{
			msg: "nil endpoint port; string target port",
			ports: []discoveryV1.EndpointPort{
				{
					Port: nil,
				},
			},
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromString("http"),
				Name:       svcPortName,
			},
			expPort: 80,
		},
		{
			msg: "nil endpoint port; nil target port",
			ports: []discoveryV1.EndpointPort{
				{
					Port: nil,
				},
			},
			svcPort: v1.ServicePort{
				Port: 80,
				Name: svcPortName,
			},
			expPort: 80,
		},
		{
			msg: "nil endpoint port name",
			ports: []discoveryV1.EndpointPort{
				{
					Name: nil,
					Port: helpers.GetInt32Pointer(8080),
				},
			},
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromString("target-port"),
				Name:       svcPortName,
			},
			expPort: 0,
		},
		{
			msg: "no matching endpoint name",
			ports: []discoveryV1.EndpointPort{
				{
					Name: helpers.GetStringPointer("other-svc-port"),
					Port: helpers.GetInt32Pointer(8080),
				},
				{
					Name: helpers.GetStringPointer("other-svc-port2"),
					Port: helpers.GetInt32Pointer(8081),
				},
				{
					Name: helpers.GetStringPointer("other-svc-port3"),
					Port: helpers.GetInt32Pointer(8082),
				},
			},
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromString("target-port"),
				Name:       svcPortName,
			},
			expPort: 0,
		},
		{
			msg: "matching endpoint name",
			ports: []discoveryV1.EndpointPort{
				{
					Name: helpers.GetStringPointer("other-svc-port"),
					Port: helpers.GetInt32Pointer(8080),
				},
				{
					Name: helpers.GetStringPointer("other-svc-port2"),
					Port: helpers.GetInt32Pointer(8081),
				},
				{
					Name: &svcPortName, // match
					Port: helpers.GetInt32Pointer(8082),
				},
			},
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromString("target-port"),
				Name:       svcPortName,
			},
			expPort: 8082,
		},
		{
			msg: "unnamed service port",
			ports: []discoveryV1.EndpointPort{
				{
					// If a service port is unnamed (empty string), then the endpoint port will also be empty string.
					Name: helpers.GetStringPointer(""),
					Port: helpers.GetInt32Pointer(8080),
				},
			},
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromString("target-port"),
			},
			expPort: 8080,
		},
	}
	for _, tc := range testcases {
		port := findPort(tc.ports, tc.svcPort)

		if port != tc.expPort {
			t.Errorf(
				"findPort() mismatch on %q; expected port %d; got port %d",
				tc.msg,
				tc.expPort,
				port,
			)
		}
	}
}

func TestCalculateReadyEndpoints(t *testing.T) {
	g := NewGomegaWithT(t)

	slices := []discoveryV1.EndpointSlice{
		{
			Endpoints: []discoveryV1.Endpoint{
				{
					Addresses: []string{"1.0.0.1"},
					Conditions: discoveryV1.EndpointConditions{
						Ready: helpers.GetBoolPointer(true),
					},
				},
				{
					Addresses:  []string{"1.1.0.1", "1.1.0.2", "1.1.0.3, 1.1.0.4, 1.1.0.5"},
					Conditions: discoveryV1.EndpointConditions{
						// nil conditions should be treated as not ready
					},
				},
			},
		},
		{
			Endpoints: []discoveryV1.Endpoint{
				{
					Addresses: []string{"2.0.0.1", "2.0.0.2", "2.0.0.3"},
					Conditions: discoveryV1.EndpointConditions{
						Ready: helpers.GetBoolPointer(true),
					},
				},
			},
		},
	}

	result := calculateReadyEndpoints(slices)

	g.Expect(result).To(Equal(4))
}

func generateEndpointSliceList(n int) discoveryV1.EndpointSliceList {
	const maxEndpointsPerSlice = 100 // use the Kubernetes default max for endpoints in a slice.

	slicesCount := (n + maxEndpointsPerSlice - 1) / maxEndpointsPerSlice

	result := discoveryV1.EndpointSliceList{
		Items: make([]discoveryV1.EndpointSlice, 0, slicesCount),
	}

	ready := true

	for i := 0; n > 0; i++ {
		c := maxEndpointsPerSlice
		if n < maxEndpointsPerSlice {
			c = n
		}
		n -= maxEndpointsPerSlice

		slice := discoveryV1.EndpointSlice{
			Endpoints:   make([]discoveryV1.Endpoint, c),
			AddressType: discoveryV1.AddressTypeIPv4,
			Ports: []discoveryV1.EndpointPort{
				{
					Port: nil, // will match any port in the service
				},
			},
		}

		for j := 0; j < c; j++ {
			slice.Endpoints[j] = discoveryV1.Endpoint{
				Addresses: []string{fmt.Sprintf("10.0.%d.%d", i, j)},
				Conditions: discoveryV1.EndpointConditions{
					Ready: &ready,
				},
			}
		}

		result.Items = append(result.Items, slice)
	}

	return result
}

func BenchmarkResolve(b *testing.B) {
	counts := []int{
		1,
		2,
		5,
		10,
		25,
		50,
		100,
		500,
		1000,
	}

	svc := &v1.Service{
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Port: 80,
				},
			},
		},
	}

	initEndpointSet := func([]discoveryV1.EndpointSlice) map[Endpoint]struct{} {
		return make(map[Endpoint]struct{})
	}

	for _, count := range counts {
		list := generateEndpointSliceList(count)

		b.Run(fmt.Sprintf("%d endpoints", count), func(b *testing.B) {
			bench(b, svc, list, initEndpointSet, count)
		})
		b.Run(fmt.Sprintf("%d endpoints with optimization", count), func(b *testing.B) {
			bench(b, svc, list, initEndpointSetWithCalculatedSize, count)
		})
	}
}

func bench(b *testing.B, svc *v1.Service, list discoveryV1.EndpointSliceList, initSet initEndpointSetFunc, n int) {
	for i := 0; i < b.N; i++ {
		res, err := resolveEndpoints(svc, 80, list, initSet)
		if len(res) != n {
			b.Fatalf("expected %d endpoints, got %d", n, len(res))
		}
		if err != nil {
			b.Fatal(err)
		}
	}
}
