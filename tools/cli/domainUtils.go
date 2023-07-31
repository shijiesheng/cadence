// Copyright (c) 2019 Uber Technologies, Inc.
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
	"strings"

	"github.com/uber/cadence/client/admin"

	"github.com/uber/cadence/tools/common/flag"

	"github.com/stretchr/testify/mock"
	"github.com/uber-go/tally"
	"github.com/urfave/cli"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/loggerimpl"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
)

const (
	dependencyMaxQPS = 100
)

var (
	registerDomainFlags = []cli.Flag{
		cli.StringFlag{
			Name:  FlagDescriptionWithAlias,
			Usage: "Domain description",
		},
		cli.StringFlag{
			Name:  FlagOwnerEmailWithAlias,
			Usage: "Owner email",
		},
		cli.StringFlag{
			Name:  FlagRetentionDaysWithAlias,
			Usage: "Workflow execution retention in days",
		},
		cli.StringFlag{
			Name:  FlagActiveClusterNameWithAlias,
			Usage: "Active cluster name",
		},
		cli.StringFlag{
			// use StringFlag instead of buggy StringSliceFlag
			// TODO when https://github.com/urfave/cli/pull/392 & v2 is released
			//  consider update urfave/cli
			Name:  FlagClustersWithAlias,
			Usage: "Clusters",
		},
		cli.StringFlag{
			Name:  FlagIsGlobalDomainWithAlias,
			Usage: "Flag to indicate whether domain is a global domain. Default to true. Local domain is now legacy.",
			Value: "true",
		},
		cli.GenericFlag{
			Name:  FlagDomainDataWithAlias,
			Usage: "Domain data of key value pairs (must be in key1=value1,key2=value2,...,keyN=valueN format, e.g. cluster=dca or cluster=dca,instance=cadence)",
			Value: &flag.StringMap{},
		},
		cli.StringFlag{
			Name:  FlagSecurityTokenWithAlias,
			Usage: "Optional token for security check",
		},
		cli.StringFlag{
			Name:  FlagHistoryArchivalStatusWithAlias,
			Usage: "Flag to set history archival status, valid values are \"disabled\" and \"enabled\"",
		},
		cli.StringFlag{
			Name:  FlagHistoryArchivalURIWithAlias,
			Usage: "Optionally specify history archival URI (cannot be changed after first time archival is enabled)",
		},
		cli.StringFlag{
			Name:  FlagVisibilityArchivalStatusWithAlias,
			Usage: "Flag to set visibility archival status, valid values are \"disabled\" and \"enabled\"",
		},
		cli.StringFlag{
			Name:  FlagVisibilityArchivalURIWithAlias,
			Usage: "Optionally specify visibility archival URI (cannot be changed after first time archival is enabled)",
		},
	}

	updateDomainFlags = []cli.Flag{
		cli.StringFlag{
			Name:  FlagDescriptionWithAlias,
			Usage: "Domain description",
		},
		cli.StringFlag{
			Name:  FlagOwnerEmailWithAlias,
			Usage: "Owner email",
		},
		cli.StringFlag{
			Name:  FlagRetentionDaysWithAlias,
			Usage: "Workflow execution retention in days",
		},
		cli.StringFlag{
			Name:  FlagActiveClusterNameWithAlias,
			Usage: "Active cluster name",
		},
		cli.StringFlag{
			// use StringFlag instead of buggy StringSliceFlag
			// TODO when https://github.com/urfave/cli/pull/392 & v2 is released
			//  consider update urfave/cli
			Name:  FlagClustersWithAlias,
			Usage: "Clusters",
		},
		cli.GenericFlag{
			Name:  FlagDomainDataWithAlias,
			Usage: "Domain data of key value pairs (must be in key1=value1,key2=value2,...,keyN=valueN format, e.g. cluster=dca or cluster=dca,instance=cadence)",
			Value: &flag.StringMap{},
		},
		cli.StringFlag{
			Name:  FlagSecurityTokenWithAlias,
			Usage: "Optional token for security check",
		},
		cli.StringFlag{
			Name:  FlagHistoryArchivalStatusWithAlias,
			Usage: "Flag to set history archival status, valid values are \"disabled\" and \"enabled\"",
		},
		cli.StringFlag{
			Name:  FlagHistoryArchivalURIWithAlias,
			Usage: "Optionally specify history archival URI (cannot be changed after first time archival is enabled)",
		},
		cli.StringFlag{
			Name:  FlagVisibilityArchivalStatusWithAlias,
			Usage: "Flag to set visibility archival status, valid values are \"disabled\" and \"enabled\"",
		},
		cli.StringFlag{
			Name:  FlagVisibilityArchivalURIWithAlias,
			Usage: "Optionally specify visibility archival URI (cannot be changed after first time archival is enabled)",
		},
		cli.StringFlag{
			Name:  FlagAddBadBinary,
			Usage: "Binary checksum to add for resetting workflow",
		},
		cli.StringFlag{
			Name:  FlagRemoveBadBinary,
			Usage: "Binary checksum to remove for resetting workflow",
		},
		cli.StringFlag{
			Name:  FlagReason,
			Usage: "Reason for the operation",
		},
		cli.StringFlag{
			Name:  FlagFailoverTypeWithAlias,
			Usage: "Domain failover type. Default value: force. Options: [force,grace]",
		},
		cli.IntFlag{
			Name:  FlagFailoverTimeoutWithAlias,
			Value: defaultGracefulFailoverTimeoutInSeconds,
			Usage: "[Optional] Domain failover timeout in seconds.",
		},
	}

	deprecateDomainFlags = []cli.Flag{
		cli.StringFlag{
			Name:  FlagSecurityTokenWithAlias,
			Usage: "Optional token for security check",
		},
		cli.BoolFlag{
			Name:  FlagForce,
			Usage: "Deprecate domain regardless of domain history.",
		},
	}

	describeDomainFlags = []cli.Flag{
		cli.StringFlag{
			Name:  FlagDomainID,
			Usage: "Domain UUID (required if not specify domainName)",
		},
		cli.BoolFlag{
			Name:  FlagPrintJSONWithAlias,
			Usage: "Print in raw JSON format",
		},
		getFormatFlag(),
	}

	migrateDomainFlags = []cli.Flag{

		cli.StringFlag{
			Name:  FlagDestinationAddress,
			Usage: "t",
		},
		cli.StringFlag{
			Name:  FlagDestinationDomain,
			Usage: "t",
		},

		cli.StringSliceFlag{
			Name:  FlagTaskList,
			Usage: "t",
		},

		cli.StringSliceFlag{
			Name:  FlagSearchAttribute,
			Usage: "Specify search attributes in the format key:type, available types are STRING, KEYWORD, INT, DOUBLE, BOOL, DATETIME",
		},

		getFormatFlag(),
	}

	adminDomainCommonFlags = getDBFlags()

	adminRegisterDomainFlags = append(
		registerDomainFlags,
		adminDomainCommonFlags...,
	)

	adminUpdateDomainFlags = append(
		updateDomainFlags,
		adminDomainCommonFlags...,
	)

	adminDeprecateDomainFlags = append(
		deprecateDomainFlags,
		adminDomainCommonFlags...,
	)

	adminDescribeDomainFlags = append(
		updateDomainFlags,
		adminDomainCommonFlags...,
	)
)

func initializeFrontendClient(
	context *cli.Context,
) frontend.Client {
	return cFactory.ServerFrontendClient(context)
}

func initializeFrontendAdminClient(
	context *cli.Context,
) admin.Client {
	return cFactory.ServerAdminClient(context)
}

func initializeAdminDomainHandler(
	context *cli.Context,
) domain.Handler {

	configuration, err := cFactory.ServerConfig(context)
	if err != nil {
		ErrorAndExit("Unable to load config.", err)
	}

	metricsClient := initializeMetricsClient()
	logger := initializeLogger(configuration)
	clusterMetadata := initializeClusterMetadata(configuration, metricsClient, logger)
	metadataMgr := initializeDomainManager(context)
	dynamicConfig := initializeDynamicConfig(configuration, logger)
	return initializeDomainHandler(
		logger,
		metadataMgr,
		clusterMetadata,
		initializeArchivalMetadata(configuration, dynamicConfig),
		initializeArchivalProvider(configuration, clusterMetadata, metricsClient, logger),
	)
}

func loadConfig(
	context *cli.Context,
) *config.Config {
	env := getEnvironment(context)
	zone := getZone(context)
	configDir := getConfigDir(context)
	var cfg config.Config
	err := config.Load(env, configDir, zone, &cfg)
	if err != nil {
		ErrorAndExit("Unable to load config.", err)
	}
	return &cfg
}

func initializeDomainHandler(
	logger log.Logger,
	domainManager persistence.DomainManager,
	clusterMetadata cluster.Metadata,
	archivalMetadata archiver.ArchivalMetadata,
	archiverProvider provider.ArchiverProvider,
) domain.Handler {

	domainConfig := domain.Config{
		MinRetentionDays:  dynamicconfig.GetIntPropertyFn(dynamicconfig.MinRetentionDays.DefaultInt()),
		MaxBadBinaryCount: dynamicconfig.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendMaxBadBinaries.DefaultInt()),
		FailoverCoolDown:  dynamicconfig.GetDurationPropertyFnFilteredByDomain(dynamicconfig.FrontendFailoverCoolDown.DefaultDuration()),
	}
	return domain.NewHandler(
		domainConfig,
		logger,
		domainManager,
		clusterMetadata,
		initializeDomainReplicator(logger),
		archivalMetadata,
		archiverProvider,
		clock.NewRealTimeSource(),
	)
}

func initializeLogger(
	serviceConfig *config.Config,
) log.Logger {
	zapLogger, err := serviceConfig.Log.NewZapLogger()
	if err != nil {
		ErrorAndExit("failed to create zap logger, err: ", err)
	}
	return loggerimpl.NewLogger(zapLogger)
}

func initializeClusterMetadata(serviceConfig *config.Config, metrics metrics.Client, logger log.Logger) cluster.Metadata {

	clusterGroupMetadata := serviceConfig.ClusterGroupMetadata
	return cluster.NewMetadata(
		clusterGroupMetadata.FailoverVersionIncrement,
		clusterGroupMetadata.PrimaryClusterName,
		clusterGroupMetadata.CurrentClusterName,
		clusterGroupMetadata.ClusterGroup,
		func(d string) bool { return false },
		metrics,
		logger,
	)
}

func initializeArchivalMetadata(
	serviceConfig *config.Config,
	dynamicConfig *dynamicconfig.Collection,
) archiver.ArchivalMetadata {

	return archiver.NewArchivalMetadata(
		dynamicConfig,
		serviceConfig.Archival.History.Status,
		serviceConfig.Archival.History.EnableRead,
		serviceConfig.Archival.Visibility.Status,
		serviceConfig.Archival.Visibility.EnableRead,
		&serviceConfig.DomainDefaults.Archival,
	)
}

func initializeArchivalProvider(
	serviceConfig *config.Config,
	clusterMetadata cluster.Metadata,
	metricsClient metrics.Client,
	logger log.Logger,
) provider.ArchiverProvider {

	archiverProvider := provider.NewArchiverProvider(
		serviceConfig.Archival.History.Provider,
		serviceConfig.Archival.Visibility.Provider,
	)

	historyArchiverBootstrapContainer := &archiver.HistoryBootstrapContainer{
		HistoryV2Manager: nil, // not used
		Logger:           logger,
		MetricsClient:    metricsClient,
		ClusterMetadata:  clusterMetadata,
		DomainCache:      nil, // not used
	}
	visibilityArchiverBootstrapContainer := &archiver.VisibilityBootstrapContainer{
		Logger:          logger,
		MetricsClient:   metricsClient,
		ClusterMetadata: clusterMetadata,
		DomainCache:     nil, // not used
	}

	err := archiverProvider.RegisterBootstrapContainer(
		service.Frontend,
		historyArchiverBootstrapContainer,
		visibilityArchiverBootstrapContainer,
	)
	if err != nil {
		ErrorAndExit("Error initializing archival provider.", err)
	}
	return archiverProvider
}

func initializeDomainReplicator(
	logger log.Logger,
) domain.Replicator {

	replicationMessageSink := &mocks.KafkaProducer{}
	replicationMessageSink.On("Publish", mock.Anything, mock.Anything).Return(nil)
	return domain.NewDomainReplicator(replicationMessageSink, logger)
}

func initializeDynamicConfig(
	serviceConfig *config.Config,
	logger log.Logger,
) *dynamicconfig.Collection {

	// the done channel is used by dynamic config to stop refreshing
	// and CLI does not need that, so just close the done channel
	doneChan := make(chan struct{})
	close(doneChan)
	dynamicConfigClient, err := dynamicconfig.NewFileBasedClient(
		&serviceConfig.DynamicConfig.FileBased,
		logger,
		doneChan,
	)
	if err != nil {
		ErrorAndExit("Error initializing dynamic config.", err)
	}
	return dynamicconfig.NewCollection(dynamicConfigClient, logger)
}

func initializeMetricsClient() metrics.Client {
	return metrics.NewClient(tally.NoopScope, metrics.Common)
}

func getEnvironment(c *cli.Context) string {
	return strings.TrimSpace(c.String(FlagServiceEnv))
}

func getZone(c *cli.Context) string {
	return strings.TrimSpace(c.String(FlagServiceZone))
}

func getConfigDir(c *cli.Context) string {
	dirPath := c.String(FlagServiceConfigDir)
	if len(dirPath) == 0 {
		ErrorAndExit("Must provide service configuration dir path.", nil)
	}
	return dirPath
}
