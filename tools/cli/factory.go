// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cli

import (
	"context"
	"time"

	"go.uber.org/yarpc/peer"
	"go.uber.org/yarpc/peer/hostport"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/yarpc/transport/tchannel"
	"google.golang.org/grpc/credentials"

	"github.com/olivere/elastic"
	"github.com/urfave/cli"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/zap"

	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	adminv1 "github.com/uber/cadence-idl/go/proto/admin/v1"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	serverAdmin "github.com/uber/cadence/.gen/go/admin/adminserviceclient"
	serverFrontend "github.com/uber/cadence/.gen/go/cadence/workflowserviceclient"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	cc "github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/config"
)

const (
	cadenceClientName      = "cadence-client"
	cadenceFrontendService = "cadence-frontend"
)

// ContextKey is an alias for string, used as context key
type ContextKey string

const (
	// CtxKeyJWT is the name of the context key for the JWT
	CtxKeyJWT = ContextKey("ctxKeyJWT")
)

// ClientFactory is used to construct rpc clients
type ClientFactory interface {
	ServerFrontendClient(c *cli.Context) frontend.Client
	ServerAdminClient(c *cli.Context) admin.Client

	ElasticSearchClient(c *cli.Context) *elastic.Client

	ServerConfig(c *cli.Context) (*config.Config, error)
}

type clientFactory struct {
	addressFlagFunc func(c *cli.Context) string
	hostPort        string
	dispatcher      *yarpc.Dispatcher
	logger          *zap.Logger
}

// DEPRECATED don't use, only reserved for backward compatibility purposes
// NewClientFactory creates a new ClientFactory
func NewClientFactory() ClientFactory {
	return newClientFactory(func(c *cli.Context) string {
		return c.GlobalString(FlagAddress)
	})
}

func newClientFactory(f func(c *cli.Context) string) ClientFactory {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	return &clientFactory{
		addressFlagFunc: f,
		logger:          logger,
	}
}

// ServerConfig returns Cadence server configs.
// Use in some CLI admin operations (e.g. accessing DB directly)
func (b *clientFactory) ServerConfig(c *cli.Context) (*config.Config, error) {
	env := c.String(FlagServiceEnv)
	zone := c.String(FlagServiceZone)
	configDir := c.String(FlagServiceConfigDir)

	var cfg config.Config
	err := config.Load(env, configDir, zone, &cfg)
	return &cfg, err
}

// ServerFrontendClient builds a frontend client (based on server side thrift interface)
func (b *clientFactory) ServerFrontendClient(c *cli.Context) frontend.Client {
	b.ensureDispatcher(c)
	clientConfig := b.dispatcher.ClientConfig(cadenceFrontendService)
	if c.GlobalString(FlagTransport) == grpcTransport {
		return frontend.NewGRPCClient(
			apiv1.NewDomainAPIYARPCClient(clientConfig),
			apiv1.NewWorkflowAPIYARPCClient(clientConfig),
			apiv1.NewWorkerAPIYARPCClient(clientConfig),
			apiv1.NewVisibilityAPIYARPCClient(clientConfig),
		)
	}
	return frontend.NewThriftClient(serverFrontend.New(clientConfig))
}

// ServerAdminClient builds an admin client (based on server side thrift interface)
func (b *clientFactory) ServerAdminClient(c *cli.Context) admin.Client {
	b.ensureDispatcher(c)
	clientConfig := b.dispatcher.ClientConfig(cadenceFrontendService)
	if c.GlobalString(FlagTransport) == grpcTransport {
		return admin.NewGRPCClient(adminv1.NewAdminAPIYARPCClient(clientConfig))
	}
	return admin.NewThriftClient(serverAdmin.New(clientConfig))
}

// ElasticSearchClient builds an ElasticSearch client
func (b *clientFactory) ElasticSearchClient(c *cli.Context) *elastic.Client {
	url := getRequiredOption(c, FlagURL)
	retrier := elastic.NewBackoffRetrier(elastic.NewExponentialBackoff(128*time.Millisecond, 513*time.Millisecond))

	client, err := elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetRetrier(retrier),
	)
	if err != nil {
		b.logger.Fatal("Unable to create ElasticSearch client", zap.Error(err))
	}

	return client
}

func (b *clientFactory) ensureDispatcher(c *cli.Context) {
	if b.dispatcher != nil {
		return
	}
	shouldUseGrpc := c.GlobalString(FlagTransport) == grpcTransport

	b.hostPort = tchannelPort
	if shouldUseGrpc {
		b.hostPort = grpcPort
	}
	if addr := b.addressFlagFunc(c); addr != "" {
		b.hostPort = addr
	}

	var outbounds transport.Outbounds
	if shouldUseGrpc {
		grpcTransport := grpc.NewTransport()
		outbounds = transport.Outbounds{Unary: grpc.NewTransport().NewSingleOutbound(b.hostPort)}

		tlsCertificatePath := c.GlobalString(FlagTLSCertPath)
		if tlsCertificatePath != "" {
			caCert, err := ioutil.ReadFile(tlsCertificatePath)
			if err != nil {
				b.logger.Fatal("Failed to load server CA certificate", zap.Error(err))
			}
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				b.logger.Fatal("Failed to add server CA certificate", zap.Error(err))
			}
			tlsConfig := tls.Config{
				RootCAs: caCertPool,
			}
			tlsCreds := credentials.NewTLS(&tlsConfig)
			tlsChooser := peer.NewSingle(hostport.Identify(b.hostPort), grpcTransport.NewDialer(grpc.DialerCredentials(tlsCreds)))
			outbounds = transport.Outbounds{Unary: grpc.NewTransport().NewOutbound(tlsChooser)}
		}
	} else {
		ch, err := tchannel.NewChannelTransport(tchannel.ServiceName(cadenceClientName), tchannel.ListenAddr("127.0.0.1:0"))
		if err != nil {
			b.logger.Fatal("Failed to create transport channel", zap.Error(err))
		}
		outbounds = transport.Outbounds{Unary: ch.NewSingleOutbound(b.hostPort)}
	}

	b.dispatcher = yarpc.NewDispatcher(yarpc.Config{
		Name:      cadenceClientName,
		Outbounds: yarpc.Outbounds{cadenceFrontendService: outbounds},
		OutboundMiddleware: yarpc.OutboundMiddleware{
			Unary: &versionMiddleware{},
		},
	})

	if err := b.dispatcher.Start(); err != nil {
		b.dispatcher.Stop()
		b.logger.Fatal("Failed to create outbound transport channel: %v", zap.Error(err))
	}
}

type versionMiddleware struct {
}

func (vm *versionMiddleware) Call(ctx context.Context, request *transport.Request, out transport.UnaryOutbound) (*transport.Response, error) {
	request.Headers = request.Headers.
		With(common.ClientImplHeaderName, cc.CLI).
		With(common.FeatureVersionHeaderName, cc.SupportedCLIVersion).
		With(common.ClientFeatureFlagsHeaderName, cc.FeatureFlagsHeader(cc.DefaultCLIFeatureFlags))
	if jwtKey, ok := ctx.Value(CtxKeyJWT).(string); ok {
		request.Headers = request.Headers.With(common.AuthorizationTokenHeaderName, jwtKey)
	}
	return out.Call(ctx, request)
}

func getJWT(c *cli.Context) string {
	return c.GlobalString(FlagJWT)
}

func getJWTPrivateKey(c *cli.Context) string {
	return c.GlobalString(FlagJWTPrivateKey)
}
