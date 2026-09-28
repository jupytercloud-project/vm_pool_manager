package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"charm.land/huh/v2"
)

/*
UTILS
*/

func writeEnvFile(path string, vars map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil && filepath.Dir(path) != "." {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for k, v := range vars {
		if _, err := f.WriteString(fmt.Sprintf("%s=%s\n", k, v)); err != nil {
			return err
		}
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

/*
MAIN
*/

func main() {
	home := os.Getenv("HOME")

	/*
		--------------------------------
		SSH CONFIG (GLOBAL)
		--------------------------------
	*/
	sshPrivateKeyPath := home + "/.ssh/id_ed25519"
	sshPublicKeyPath := home + "/.ssh/id_ed25519.pub"

	sshForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Chemin de la clé SSH PRIVÉE").
				Description("Ex: ~/.ssh/id_ed25519").
				Value(&sshPrivateKeyPath).
				Validate(func(v string) error {
					if !fileExists(v) {
						return fmt.Errorf("clé privée introuvable")
					}
					return nil
				}),

			huh.NewInput().
				Title("Chemin de la clé SSH PUBLIQUE").
				Description("Ex: ~/.ssh/id_ed25519.pub").
				Value(&sshPublicKeyPath).
				Validate(func(v string) error {
					if !fileExists(v) {
						return fmt.Errorf("clé publique introuvable")
					}
					return nil
				}),
		),
	)

	if err := sshForm.Run(); err != nil {
		log.Fatal(err)
	}

	/*
		--------------------------------
		OPENSTACK GLOBAL CONFIG
		--------------------------------
	*/
	osClientConfigFile := home + "/.config/openstack/clouds.yaml"
	osCloud := ""
	ipAddress := ""

	openstackGlobalForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("OS_CLIENT_CONFIG_FILE").
				Description("Chemin vers clouds.yaml").
				Value(&osClientConfigFile).
				Validate(func(v string) error {
					if !fileExists(v) {
						return fmt.Errorf("clouds.yaml introuvable")
					}
					return nil
				}),

			huh.NewInput().
				Title("OS_CLOUD").
				Description("Nom du cloud OpenStack").
				Value(&osCloud).
				Validate(func(v string) error {
					if v == "" {
						return fmt.Errorf("OS_CLOUD ne peut pas être vide")
					}
					return nil
				}),
			huh.NewInput().
				Description("IP Adresse").
				Value(&ipAddress).
				Validate(func(v string) error {
					if v == "" {
						return fmt.Errorf("IP_ADDRESS ne peut pas être vide")
					}
					return nil
				}),
		),
	)

	if err := openstackGlobalForm.Run(); err != nil {
		log.Fatal(err)
	}

	/*
		--------------------------------
		CONTROL CENTER
		--------------------------------
	*/
	ccUser := "admin"
	ccPassword := ""

	ccForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Controle Center - Postgres User").
				Value(&ccUser),

			huh.NewInput().
				Title("Controle Center - Postgres Password").
				EchoMode(huh.EchoModePassword).
				Value(&ccPassword),
		),
	)

	if err := ccForm.Run(); err != nil {
		log.Fatal(err)
	}

	ccEnv := map[string]string{
		"POSTGRES_HOST":        "postgres",
		"POSTGRES_PORT":        "5432",
		"POSTGRES_USER":        ccUser,
		"POSTGRES_PASSWORD":    ccPassword,
		"POSTGRES_DB":          "control_center",
		"CONTROL_CENTER_PORT":  "50051",
		"SSH_PUBLIC_KEY_PATH":  sshPublicKeyPath,
		"SSH_PRIVATE_KEY_PATH": sshPrivateKeyPath,
		"IP_ADDRESS":           ipAddress,
	}

	ccEnvPath := filepath.Join("control_center", ".env")
	if err := writeEnvFile(ccEnvPath, ccEnv); err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ control_center/.env généré")

	/*
		--------------------------------
		OPENSTACK SERVICE
		--------------------------------
	*/
	var apiKeyName, secretJWT string

	openstackServiceForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("OpenStack - API_KEYNAME").
				Value(&apiKeyName),

			huh.NewInput().
				Title("OpenStack - SECRET_KEY_JWT").
				EchoMode(huh.EchoModePassword).
				Value(&secretJWT),
		),
	)

	if err := openstackServiceForm.Run(); err != nil {
		log.Fatal(err)
	}

	osEnv := map[string]string{
		// Secrets
		"API_KEYNAME":    apiKeyName,
		"SECRET_KEY_JWT": secretJWT,

		// SSH
		"SSH_PUBLIC_KEY_PATH": sshPublicKeyPath,

		// OpenStack
		"OS_CLIENT_CONFIG_FILE": osClientConfigFile,
		"OS_CLOUD":              osCloud,

		// Defaults (à modifier dans le .env après setup)
		"METADATA_SERVERPOOL_ID": "pool_vms",
		"METADATA_USER_ID":       "admin",
		"METADATA_MIN_VM":        "2",
		"METADATA_MAX_VM":        "9",
	}

	osEnvPath := filepath.Join("microservices/openstack", ".env")
	if err := writeEnvFile(osEnvPath, osEnv); err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ openstack/.env généré")

	/*
		--------------------------------
		ROOT .env (Taskfile)
		--------------------------------
	*/
	rootEnv := map[string]string{
		"CC_ENV_FILE": ccEnvPath,
		"OS_ENV_FILE": osEnvPath,
	}

	if err := writeEnvFile(".env", rootEnv); err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ .env racine généré")

	fmt.Println("\n🎉 Configuration terminée avec succès")
}
