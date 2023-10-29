# kubectl apply -f ./k8s
# sudo kubectl port-forward services/nginx-service-nodeport 8000:80

go run cmd/server/main.go

# telnet localhost 3000