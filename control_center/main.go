package main

import (
	"context"
	"control_center/config"
	cc "control_center/grpc"
	"control_center/internal/telemetry"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Chargement du fichier .env (cherche dans le répertoire courant puis dans le parent)
	if err := godotenv.Load(); err != nil {
		if err2 := godotenv.Load("../.env"); err2 != nil {
			log.Fatalf("Error loading .env file: %v", err2)
		}
	}

	// OpenTelemetry (traces + métriques + logs OTLP). No-op si non configuré.
	otelShutdown, err := telemetry.Setup(context.Background(), "control-center")
	if err != nil {
		log.Printf("[otel] init: %v", err)
	}

	// Initialisation de la base de données
	config.Start_DB(context.Background())

	// Création d’un contexte annulé sur SIGINT ou SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Remplissage initial de la base de données
	cc.PopulateDBImageMicroOpen()
	cc.PopulateDBFlavorMicroOpen()
	cc.PopulateDBNetworkMicroOpen()

	go cc.Start_grpc(ctx)
	go cc.ConnectToMicroOpen(ctx)

	// Attente du signal d’arrêt
	<-ctx.Done()

	// Annule explicitement le contexte (au cas où)
	stop()

	// Vidage des traces/métriques/logs OTel encore en file (timeout borné).
	if otelShutdown != nil {
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := otelShutdown(shCtx); err != nil {
			log.Printf("[otel] shutdown: %v", err)
		}
		cancel()
	}

	log.Println("Arrêt terminé proprement ✅")
}
