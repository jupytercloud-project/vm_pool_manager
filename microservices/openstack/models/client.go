package models

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/config"
	"github.com/gophercloud/gophercloud/v2/openstack/config/clouds"
)

// ─── Infra clients (ipp-idcs-vmpoolmanager) ──────────────────────
// Used for: listing images, flavors, networks (read-only operations)
var (
	InfraComputeClient *gophercloud.ServiceClient
	InfraImageClient   *gophercloud.ServiceClient
	InfraNetworkClient *gophercloud.ServiceClient
)

// ─── Student VM clients (ipp-idcs-vmpool) ────────────────────────
// Used for: creating, deleting, managing student VMs and volumes
var (
	ComputeClient      *gophercloud.ServiceClient
	BlockstorageClient *gophercloud.ServiceClient
	ImageClient        *gophercloud.ServiceClient
	NetworkClient      *gophercloud.ServiceClient
)

// CreateParams initializes both OpenStack clients.
// OS_CLOUD env var → student project (VM operations)
// STUDENT_OS_CLOUD is kept for backward compat but OS_CLOUD is
// the primary one for VM ops. We use INFRA_OS_CLOUD for infra.
func CreateParams() error {
	// ── Student project (VM creation/deletion) ──
	if err := initStudentClients(); err != nil {
		return fmt.Errorf("student project init failed: %w", err)
	}

	// ── Infra project (images, flavors, networks listing) ──
	infraCloud := os.Getenv("INFRA_OS_CLOUD")
	if infraCloud != "" {
		if err := initInfraClients(infraCloud); err != nil {
			log.Printf("[WARN] Infra project init failed: %v — using student project as fallback", err)
			fallbackInfraToStudent()
		}
	} else {
		log.Println("[INFO] INFRA_OS_CLOUD not set — using single-project mode")
		fallbackInfraToStudent()
	}

	return nil
}

// initStudentClients parses the default OS_CLOUD from clouds.yaml
func initStudentClients() error {
	AuthOptions, EndpointOpts, TlsConfig, err := clouds.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse clouds.yaml: %w", err)
	}
	// Renouveler automatiquement le token à son expiration (~1 h) : sans cela,
	// le client tombe en 401 permanent une fois le token expiré (jusqu'au redémarrage).
	AuthOptions.AllowReauth = true

	provider, err := config.NewProviderClient(context.Background(),
		AuthOptions, config.WithTLSConfig(TlsConfig))
	if err != nil {
		return fmt.Errorf("failed to create provider client: %w", err)
	}

	ComputeClient, err = openstack.NewComputeV2(provider, EndpointOpts)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}

	BlockstorageClient, err = openstack.NewBlockStorageV3(provider, EndpointOpts)
	if err != nil {
		return fmt.Errorf("failed to create blockstorage client: %w", err)
	}

	ImageClient, err = openstack.NewImageV2(provider, EndpointOpts)
	if err != nil {
		return fmt.Errorf("failed to create image client: %w", err)
	}

	NetworkClient, err = openstack.NewNetworkV2(provider, EndpointOpts)
	if err != nil {
		return fmt.Errorf("failed to create network client: %w", err)
	}

	log.Printf("[OpenStack] Student project connected (OS_CLOUD=%s)", os.Getenv("OS_CLOUD"))
	return nil
}

// initInfraClients creates a separate provider for the infra project
func initInfraClients(cloudName string) error {
	// Temporarily override OS_CLOUD for parsing
	originalCloud := os.Getenv("OS_CLOUD")
	os.Setenv("OS_CLOUD", cloudName)
	defer os.Setenv("OS_CLOUD", originalCloud)

	AuthOptions, EndpointOpts, TlsConfig, err := clouds.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse clouds.yaml for infra: %w", err)
	}
	AuthOptions.AllowReauth = true

	provider, err := config.NewProviderClient(context.Background(),
		AuthOptions, config.WithTLSConfig(TlsConfig))
	if err != nil {
		return fmt.Errorf("failed to create infra provider: %w", err)
	}

	InfraComputeClient, err = openstack.NewComputeV2(provider, EndpointOpts)
	if err != nil {
		return fmt.Errorf("failed to create infra compute client: %w", err)
	}

	InfraImageClient, err = openstack.NewImageV2(provider, EndpointOpts)
	if err != nil {
		return fmt.Errorf("failed to create infra image client: %w", err)
	}

	InfraNetworkClient, err = openstack.NewNetworkV2(provider, EndpointOpts)
	if err != nil {
		return fmt.Errorf("failed to create infra network client: %w", err)
	}

	log.Printf("[OpenStack] Infra project connected (INFRA_OS_CLOUD=%s)", cloudName)
	return nil
}

// fallbackInfraToStudent uses the student clients for infra operations too
// This allows single-project mode for development/testing
func fallbackInfraToStudent() {
	InfraComputeClient = ComputeClient
	InfraImageClient = ImageClient
	InfraNetworkClient = NetworkClient
}
