package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lihongjie0209/billing-service/internal/app"
	"github.com/lihongjie0209/billing-service/internal/buildinfo"
	"github.com/lihongjie0209/billing-service/internal/config"
)

// @title Platform Billing Service API
// @version 0.1.0
// @description Plans, tenant subscriptions, usage-rated invoices, payments, and refunds. All business endpoints use POST+JSON and return code/message/body/request_id. Error ranges: 10000 common/input; 20000 authentication/authorization; 30000 business concurrency/idempotency; 50000 dependency/infrastructure.
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Enter "Bearer {JWT}".
// @securityDefinitions.apikey PSK
// @in header
// @name Authorization
// @description Enter "PSK {shared-key}" for routes configured with PSK authentication.
func main() {
	configPath := flag.String("config", "config/config.yaml", "configuration file path")
	profile := flag.String("env", "", "active environment profile (overrides APP_ENV and config)")
	showVersion := flag.Bool("version", false, "print build version information and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("version=%s commit=%s build_time=%s\n", buildinfo.Version, buildinfo.Commit, buildinfo.BuildTime)
		return
	}
	cfg, err := config.LoadWithProfile(*configPath, *profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load configuration: %v\n", err)
		os.Exit(1)
	}
	app.New(cfg).Run()
}
