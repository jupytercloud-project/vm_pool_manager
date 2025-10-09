package models

import (
	"context"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/config"
	"github.com/gophercloud/gophercloud/v2/openstack/config/clouds"
)

var (
	ProviderClient     gophercloud.ProviderClient
	ComputeClient      *gophercloud.ServiceClient
	BlockstorageClient *gophercloud.ServiceClient
	ImageClient        *gophercloud.ServiceClient
	NetworkClient      *gophercloud.ServiceClient
)

func CreateParams() error {
	var err error

	AuthOptions, EndpointOpts, TlsConfig, err := clouds.Parse()
	if err != nil {
		panic(err)
	}

	ProviderClient, err := config.NewProviderClient(context.Background(), AuthOptions, config.WithTLSConfig(TlsConfig))
	if err != nil {
		panic(err)
	}

	ComputeClient, err = openstack.NewComputeV2(ProviderClient, EndpointOpts)
	if err != nil {
		panic(err)
	}

	BlockstorageClient, err = openstack.NewBlockStorageV3(ProviderClient, EndpointOpts)
	if err != nil {
		panic(err)
	}

	ImageClient, err = openstack.NewImageV2(ProviderClient, EndpointOpts)
	if err != nil {
		panic(err)
	}

	NetworkClient, err = openstack.NewNetworkV2(ProviderClient, EndpointOpts)
	if err != nil {
		panic(err)
	}

	return nil
}
