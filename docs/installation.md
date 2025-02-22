# Installation

This guide walks you through how to install NGINX Kubernetes Gateway on a generic Kubernetes cluster.

## Prerequisites

- [kubectl](https://kubernetes.io/docs/tasks/tools/)

## Deploy NGINX Kubernetes Gateway

> Note: NGINX Kubernetes Gateway can only run in the `nginx-gateway` namespace. This limitation will be addressed in the future releases.

1. Clone the repo and change into the `nginx-kubernetes-gateway` directory:

   ```
   git clone https://github.com/nginxinc/nginx-kubernetes-gateway.git
   cd nginx-kubernetes-gateway
   ```

1. Install the Gateway API resources from the standard channel (the CRDs and the validating webhook):

   ```
   kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v0.7.1/standard-install.yaml
   ```

1. Create the nginx-gateway Namespace:

    ```
    kubectl apply -f deploy/manifests/namespace.yaml
    ```

1. Create the njs-modules ConfigMap:

    ```
    kubectl create configmap njs-modules --from-file=internal/mode/static/nginx/modules/src/httpmatches.js -n nginx-gateway
    ```

1. Create the ConfigMap with the main NGINX configuration file:

    ```
    kubectl apply -f deploy/manifests/nginx-conf.yaml
    ```

1. Configure RBAC:

    ```
    kubectl apply -f deploy/manifests/rbac.yaml
    ```

1. Create the GatewayClass resource:

    ```
    kubectl apply -f deploy/manifests/gatewayclass.yaml
    ```

1. Deploy the NGINX Kubernetes Gateway:

   ```
   kubectl apply -f deploy/manifests/deployment.yaml
   ```

1. Confirm the NGINX Kubernetes Gateway is running in `nginx-gateway` namespace:

   ```
   kubectl get pods -n nginx-gateway
   NAME                             READY   STATUS    RESTARTS   AGE
   nginx-gateway-5d4f4c7db7-xk2kq   2/2     Running   0          112s
   ```

## Expose NGINX Kubernetes Gateway

You can gain access to NGINX Kubernetes Gateway by creating a `NodePort` Service or a `LoadBalancer` Service.

> Important
> 
> The Service manifests expose NGINX Kubernetes Gateway on ports 80 and 443, which exposes any Gateway [Listener](https://gateway-api.sigs.k8s.io/references/spec/#gateway.networking.k8s.io/v1beta1.Listener) configured for those ports. If you'd like to use different ports in your listeners,
> update the manifests accordingly. 

### Create a NodePort Service

Create a Service with type `NodePort`:

```
kubectl apply -f deploy/manifests/service/nodeport.yaml
```

A `NodePort` Service will randomly allocate one port on every Node of the cluster. To access NGINX Kubernetes Gateway, use an IP address of any Node in the cluster along with the allocated port.

### Create a LoadBalancer Service

Create a Service with type `LoadBalancer` using the appropriate manifest for your cloud provider.

- For GCP or Azure:

   ```
   kubectl apply -f deploy/manifests/service/loadbalancer.yaml
   ```

   Lookup the public IP of the load balancer, which is reported in the `EXTERNAL-IP` column in the output of the following command:

   ```
   kubectl get svc nginx-gateway -n nginx-gateway
   ```

   Use the public IP of the load balancer to access NGINX Kubernetes Gateway.

- For AWS:

   ```
   kubectl apply -f deploy/manifests/service/loadbalancer-aws-nlb.yaml
   ```

   In AWS, the NLB DNS name will be reported by Kubernetes in lieu of a public IP in the `EXTERNAL-IP` column. To get the DNS name run:

   ```
   kubectl get svc nginx-gateway -n nginx-gateway
   ```

   In general, you should rely on the NLB DNS name, however for testing purposes you can resolve the DNS name to get the IP address of the load balancer:

   ```
   nslookup <dns-name>
   ```

### Use NGINX Kubernetes Gateway

To get started, follow the tutorials in the [examples](../examples) directory.
