package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
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
		log.Fatalf("配置错误: %v", err)
	}

	routerClient, err := zte.NewClient(cfg.RouterURL, cfg.RouterUser, cfg.RouterPass, cfg.HTTPTimeout)
	if err != nil {
		log.Fatalf("路由器客户端初始化失败: %v", err)
	}
	kumaClient := kuma.NewClient(cfg.KumaPushURL, cfg.HTTPTimeout)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("启动监控: 路由器=%s, 监控网口=%s, 最低速率=%dMbps, 检查间隔=%s, 请求超时=%s", cfg.RouterURL, cfg.WANPortAlias, cfg.MinSpeedMbps, cfg.CheckInterval, cfg.HTTPTimeout)

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
			log.Println("停止监控")
			return
		case <-ticker.C:
			runCheck(ctx, cfg, routerClient, kumaClient)
		}
	}
}

func runCheck(parent context.Context, cfg config.Config, routerClient *zte.Client, kumaClient *kuma.Client) bool {
	started := time.Now()
	log.Printf("开始检查: 目标网口=%s", cfg.WANPortAlias)

	status := "up"
	message := ""
	unhealthy := false

	routerCtx, routerCancel := context.WithTimeout(parent, cfg.HTTPTimeout)
	result, err := routerClient.WANPortStatus(routerCtx, cfg.WANPortAlias)
	routerCancel()
	logRouterResult(result, time.Since(started))
	port := result.Port
	if err != nil {
		status = "down"
		message = fmt.Sprintf("路由器读取失败: %v", err)
		unhealthy = true
	} else if !port.Connected {
		status = "down"
		message = fmt.Sprintf("%s 已断开，当前速率=%s", port.DisplayName, port.SpeedText())
		unhealthy = true
	} else if !port.SpeedKnown {
		status = "down"
		message = fmt.Sprintf("%s 速率未知，原始速率编号=%d", port.DisplayName, port.SpeedIndex)
		unhealthy = true
	} else if port.SpeedMbps < cfg.MinSpeedMbps {
		status = "down"
		message = fmt.Sprintf("%s 协商速率 %d Mbps，低于阈值 %d Mbps", port.DisplayName, port.SpeedMbps, cfg.MinSpeedMbps)
		unhealthy = true
	} else {
		message = fmt.Sprintf("%s 协商速率 %d Mbps，状态正常", port.DisplayName, port.SpeedMbps)
	}

	if err == nil {
		log.Printf("目标网口状态: %s", port.Summary())
	}

	duration := time.Since(started)
	log.Printf("准备推送 Kuma: status=%s, msg=%q, ping=%dms", status, message, duration.Milliseconds())
	pushCtx, pushCancel := context.WithTimeout(parent, cfg.HTTPTimeout)
	defer pushCancel()
	pushResult, err := kumaClient.Push(pushCtx, status, message, duration)
	if err != nil {
		log.Printf("Kuma 推送失败: status=%s, msg=%q, ping=%dms, HTTP状态=%d, 响应=%q, 错误=%v", status, message, duration.Milliseconds(), pushResult.StatusCode, pushResult.Body, err)
		return true
	}

	log.Printf("Kuma 推送成功: status=%s, msg=%q, ping=%dms, HTTP状态=%d, 响应=%q, 总耗时=%s", status, message, duration.Milliseconds(), pushResult.StatusCode, pushResult.Body, duration.Round(time.Millisecond))
	return unhealthy
}

func logRouterResult(result zte.WANPortResult, elapsed time.Duration) {
	log.Printf("路由器读取完成: 阶段=%s, 会话=%s, 响应大小=%d字节, 解析网口数=%d, 耗时=%s",
		emptyAsDash(result.Stage),
		sessionSummary(result),
		result.ResponseBytes,
		len(result.AvailablePorts),
		elapsed.Round(time.Millisecond),
	)
	for index, event := range result.Events {
		log.Printf("路由器步骤[%d]: %s", index+1, event)
	}
	if len(result.AvailablePorts) > 0 {
		log.Printf("路由器网口列表: %s", portSummaries(result.AvailablePorts))
	}
}

func sessionSummary(result zte.WANPortResult) string {
	switch {
	case result.RetriedAfterTimeout:
		return "会话过期后重新登录"
	case result.InitialLogin:
		return "首次登录"
	case result.LoginAttempted:
		return "登录"
	case result.SessionReused:
		return "复用已有会话"
	default:
		return "未知"
	}
}

func emptyAsDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func portSummaries(ports []zte.PortStatus) string {
	summaries := make([]string, 0, len(ports))
	for _, port := range ports {
		summaries = append(summaries, port.Summary())
	}
	return strings.Join(summaries, "; ")
}
