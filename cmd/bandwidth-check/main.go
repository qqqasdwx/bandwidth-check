package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/qqqasdwx/bandwidth-check/internal/config"
	"github.com/qqqasdwx/bandwidth-check/internal/kuma"
	"github.com/qqqasdwx/bandwidth-check/internal/zte"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	routerClient, err := zte.NewClient(cfg.RouterURL, cfg.RouterUser, cfg.RouterPass, cfg.HTTPTimeout)
	if err != nil {
		log.Fatalf("router client error: %v", err)
	}
	kumaClient := kuma.NewClient(cfg.KumaPushURL, cfg.HTTPTimeout)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("starting bandwidth-check router=%s wan_port=%s min_speed=%dMbps interval=%s", cfg.RouterURL, cfg.WANPortAlias, cfg.MinSpeedMbps, cfg.CheckInterval)

	if unhealthy := runCheck(ctx, cfg, routerClient, kumaClient); cfg.RunOnce {
		if unhealthy {
			os.Exit(2)
		}
		return
	}

	ticker := time.NewTicker(cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("stopping bandwidth-check")
			return
		case <-ticker.C:
			runCheck(ctx, cfg, routerClient, kumaClient)
		}
	}
}

func runCheck(parent context.Context, cfg config.Config, routerClient *zte.Client, kumaClient *kuma.Client) bool {
	started := time.Now()

	status := "up"
	message := ""
	unhealthy := false

	routerCtx, routerCancel := context.WithTimeout(parent, cfg.HTTPTimeout)
	port, err := routerClient.WANPortStatus(routerCtx, cfg.WANPortAlias)
	routerCancel()
	if err != nil {
		status = "down"
		message = fmt.Sprintf("router read failed: %v", err)
		unhealthy = true
	} else if !port.Connected {
		status = "down"
		message = fmt.Sprintf("%s disconnected, speed=%s", port.DisplayName, port.SpeedText())
		unhealthy = true
	} else if !port.SpeedKnown {
		status = "down"
		message = fmt.Sprintf("%s speed unknown, raw_index=%d", port.DisplayName, port.SpeedIndex)
		unhealthy = true
	} else if port.SpeedMbps < cfg.MinSpeedMbps {
		status = "down"
		message = fmt.Sprintf("%s speed %d Mbps below %d Mbps", port.DisplayName, port.SpeedMbps, cfg.MinSpeedMbps)
		unhealthy = true
	} else {
		message = fmt.Sprintf("%s speed %d Mbps ok", port.DisplayName, port.SpeedMbps)
	}

	duration := time.Since(started)
	pushCtx, pushCancel := context.WithTimeout(parent, cfg.HTTPTimeout)
	defer pushCancel()
	if err := kumaClient.Push(pushCtx, status, message, duration); err != nil {
		log.Printf("kuma push failed status=%s msg=%q error=%v", status, message, err)
		return true
	}

	log.Printf("pushed status=%s msg=%q duration=%s", status, message, duration.Round(time.Millisecond))
	return unhealthy
}
