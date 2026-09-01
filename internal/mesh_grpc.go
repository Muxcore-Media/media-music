package internal

import (
	"crypto/tls"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func grpcInsecure() bool {
	return os.Getenv("MUXCORE_INSECURE_DISABLE_TLS") == "true" ||
		os.Getenv("MUXCORE_GRPC_INSECURE") == "true"
}

func meshGRPCDialOpts() []grpc.DialOption {
	if grpcInsecure() {
		return []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	}
	return []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12}))}
}

func dialAddrForModule(moduleID, httpAddr string) string {
	if httpAddr == "" {
		return ""
	}
	host, port, err := net.SplitHostPort(httpAddr)
	if err != nil || port == "" {
		return httpAddr
	}
	if host != "" && host != "0.0.0.0" && host != "::" {
		return net.JoinHostPort(host, port)
	}
	if os.Getenv("MUXCORE_MESH_DIAL_LOCAL") == "true" {
		return net.JoinHostPort("127.0.0.1", port)
	}
	if moduleID != "" {
		return net.JoinHostPort(moduleID, port)
	}
	return httpAddr
}
