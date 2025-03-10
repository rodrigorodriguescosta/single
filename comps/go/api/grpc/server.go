package grpc

import (
	"context"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"net"
	"net/http"
)

var (
	ServerEndpoint = "localhost:8080"

	mux = runtime.NewServeMux()
)

// runWebServer run webserver based on all protobuffers
func runWebServer() error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := generateSwagger(); err != nil {
		log.WithError(err).Errorf("error while generate swagger")
	}
	addr := ":8081"
	// Start HTTP server (and proxy calls to gRPC server endpoint)
	log.Infof("Webserver listening on in prefix /api %s", addr)
	return http.ListenAndServe(addr, mux)
}

type Services interface {
	Register(s *grpc.Server, httpServer *runtime.ServeMux) error
	GetInterceptor() grpc.UnaryServerInterceptor
	GetServiceName() string
}

func internalInterceptor(ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (resp any, err error) {

	// copy grpc contexts values into context
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		for k, v := range md {
			if len(k) > 0 {
				ctx = context.WithValue(ctx, k, v[0])
			}
		}
	}

	return handler(ctx, req)
}

func StartGrpcServer(port string, services ...Services) error {
	if port == "" {
		port = "8080"
	}
	var err error
	listen, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Warnf("failed to listen on %s: %v", port, err)
		return err
	}

	// add interceptors from all apps
	interceptors := []grpc.UnaryServerInterceptor{
		internalInterceptor,
	}
	for _, service := range services {
		interceptor := service.GetInterceptor()
		if interceptor == nil {
			continue
		}
		interceptors = append(interceptors, interceptor)
	}

	chainInterceptor := grpc.ChainUnaryInterceptor(interceptors...)
	grpcServer := grpc.NewServer(
		chainInterceptor,
	)

	// register all services
	log.Infof("Registering %d services....", len(services))
	for _, service := range services {
		err := service.Register(grpcServer, mux)
		if err != nil {
			return err
		}
		log.Infof("registering Service '%s'", service.GetServiceName())
	}

	go runWebServer()
	go testServerConnection(port)

	reflection.Register(grpcServer)
	if err := grpcServer.Serve(listen); err != nil {
		log.Fatalf("failed to serve gRPC over %s: %v", port, err)
	}
	return nil
}

func GetClientConnection(serverAddr string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	clientConnection, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return nil, err
	}
	return clientConnection, nil
}

// testServerConnection test grpc connection for local server
func testServerConnection(port string) bool {
	// test server connection
	for {
		conn, _ := GetClientConnection("localhost:"+port,
			grpc.WithInsecure(),
			grpc.FailOnNonTempDialError(true), // fail immediately if can't connect
			grpc.WithBlock())
		if conn == nil {
			continue
		}
		if conn.GetState() == connectivity.Ready {
			log.Infof("GRPC server listening on port %s", port)
			return true
		}
	}
}
