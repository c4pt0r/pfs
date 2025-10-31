package main

import (
	"flag"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/c4pt0r/pfs/pfs-server/pkg/config"
	"github.com/c4pt0r/pfs/pfs-server/pkg/handlers"
	"github.com/c4pt0r/pfs/pfs-server/pkg/mountablefs"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugin"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugins/hellofs"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugins/kvfs"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugins/localfs"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugins/memfs"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugins/proxyfs"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugins/queuefs"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugins/s3fs"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugins/serverinfofs"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugins/sqlfs"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugins/streamfs"
	log "github.com/sirupsen/logrus"
)

// PluginFactory is a function that creates a new plugin instance
type PluginFactory func(configFile string) plugin.ServicePlugin

// availablePlugins maps plugin names to their factory functions
var availablePlugins = map[string]PluginFactory{
	"serverinfofs": func(configFile string) plugin.ServicePlugin { return serverinfofs.NewServerInfoFSPlugin() },
	"memfs":        func(configFile string) plugin.ServicePlugin { return memfs.NewMemFSPlugin() },
	"queuefs":      func(configFile string) plugin.ServicePlugin { return queuefs.NewQueueFSPlugin() },
	"kvfs":         func(configFile string) plugin.ServicePlugin { return kvfs.NewKVFSPlugin() },
	"hellofs":      func(configFile string) plugin.ServicePlugin { return hellofs.NewHelloFSPlugin() },
	"proxyfs":      func(configFile string) plugin.ServicePlugin { return proxyfs.NewProxyFSPlugin("") },
	"s3fs":         func(configFile string) plugin.ServicePlugin { return s3fs.NewS3FSPlugin() },
	"streamfs":     func(configFile string) plugin.ServicePlugin { return streamfs.NewStreamFSPlugin() },
	"sqlfs":        func(configFile string) plugin.ServicePlugin { return sqlfs.NewSQLFSPlugin() },
	"localfs":      func(configFile string) plugin.ServicePlugin { return localfs.NewLocalFSPlugin() },
}

const sampleConfig = `# PFS Server Configuration File
# This is a sample configuration showing all available options

server:
  address: ":8080"          # Server listen address
  log_level: "info"         # Log level: debug, info, warn, error

# Plugin configurations
plugins:
  # Server Info Plugin - provides server information and stats
  serverinfofs:
    enabled: true
    path: "/serverinfofs"

  # Memory File System - in-memory file storage
  memfs:
    enabled: true
    path: "/memfs"

  # Queue File System - message queue operations
  queuefs:
    enabled: true
    path: "/queuefs"

  # Key-Value File System - key-value store
  kvfs:
    enabled: true
    path: "/kvfs"

  # Hello File System - example plugin
  hellofs:
    enabled: true
    path: "/hellofs"

  # Stream File System - streaming file operations
  streamfs:
    enabled: true
    path: "/streamfs"

  # Local File System - mount local directories
  localfs:
    enabled: false
    path: "/localfs"
    config:
      root_path: "/path/to/local/directory"  # Local directory to mount

  # S3 File System - mount S3 buckets
  s3fs:
    enabled: false
    path: "/s3fs"
    config:
      bucket: "your-bucket-name"
      region: "us-west-2"
      access_key: "YOUR_ACCESS_KEY"
      secret_key: "YOUR_SECRET_KEY"
      endpoint: ""  # Optional: custom S3 endpoint

  # SQL File System - file system backed by SQL database
  sqlfs:
    enabled: false
    # Multi-instance example: mount multiple SQL databases
    instances:
      - name: "sqlfs-sqlite"
        enabled: true
        path: "/sqlfs/sqlite"
        config:
          backend: "sqlite"
          db_path: "/tmp/pfs-sqlite.db"

      - name: "sqlfs-postgres"
        enabled: false
        path: "/sqlfs/postgres"
        config:
          backend: "postgres"
          connection_string: "postgres://user:pass@localhost/dbname?sslmode=disable"

  # Proxy File System - proxy to another PFS server
  proxyfs:
    enabled: false
    # Multi-instance example: proxy multiple remote servers
    instances:
      - name: "proxy-remote1"
        enabled: true
        path: "/proxy/remote1"
        config:
          base_url: "http://remote-server-1:8080/api/v1"
          remote_path: "/"

      - name: "proxy-remote2"
        enabled: false
        path: "/proxy/remote2"
        config:
          base_url: "http://remote-server-2:8080/api/v1"
          remote_path: "/memfs"
`

func main() {
	configFile := flag.String("c", "config.yaml", "Path to configuration file")
	addr := flag.String("addr", "", "Server listen address (will override addr in config file)")
	printSampleConfig := flag.Bool("print-sample-config", false, "Print a sample configuration file and exit")
	flag.Parse()

	// Handle --print-sample-config
	if *printSampleConfig {
		fmt.Println(sampleConfig)
		return
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	// Configure logrus
	logLevel := log.InfoLevel
	if cfg.Server.LogLevel != "" {
		if level, err := log.ParseLevel(cfg.Server.LogLevel); err == nil {
			logLevel = level
		}
	}
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := filepath.Base(f.File)
			return "", fmt.Sprintf(" %s:%d\t", filename, f.Line)
		},
	})
	log.SetReportCaller(true)
	log.SetLevel(logLevel)

	// Determine server address
	serverAddr := cfg.Server.Address
	if *addr != "" {
		serverAddr = *addr // Command line override
	}
	if serverAddr == "" {
		serverAddr = ":8080" // Default
	}

	// Create mountable file system
	mfs := mountablefs.NewMountableFS()

	// Register plugin factories for dynamic mounting
	for pluginName, factory := range availablePlugins {
		// Capture factory in local variable to avoid closure issues
		f := factory
		mfs.RegisterPluginFactory(pluginName, func() plugin.ServicePlugin {
			return f("")
		})
	}

	// mountPlugin initializes and mounts a plugin asynchronously
	mountPlugin := func(pluginName, instanceName, mountPath string, pluginConfig map[string]interface{}) {
		// Get plugin factory
		factory, ok := availablePlugins[pluginName]
		if !ok {
			log.Warnf("  Unknown plugin: %s, skipping instance '%s'", pluginName, instanceName)
			return
		}

		// Create plugin instance
		p := factory(*configFile)

		// Mount asynchronously
		go func() {
			// Initialize plugin
			if err := p.Initialize(pluginConfig); err != nil {
				log.Errorf("Failed to initialize %s instance '%s': %v", pluginName, instanceName, err)
				return
			}

			// Mount plugin
			if err := mfs.Mount(mountPath, p); err != nil {
				log.Errorf("Failed to mount %s instance '%s' at %s: %v", pluginName, instanceName, mountPath, err)
				return
			}

			// Log success
			log.Infof("  %s instance '%s' mounted at %s", pluginName, instanceName, mountPath)
		}()
	}

	// Mount all enabled plugins
	log.Info("Mounting plugin filesytems...")
	for pluginName, pluginCfg := range cfg.Plugins {
		// Normalize to instance array (convert single instance to array of one)
		instances := pluginCfg.Instances
		if len(instances) == 0 {
			// Single instance mode: treat as array with one instance
			instances = []config.PluginInstance{
				{
					Name:    pluginName, // Use plugin name as instance name
					Enabled: pluginCfg.Enabled,
					Path:    pluginCfg.Path,
					Config:  pluginCfg.Config,
				},
			}
		}

		// Mount all instances
		for _, instance := range instances {
			if !instance.Enabled {
				log.Infof("  %s instance '%s' is disabled, skipping", pluginName, instance.Name)
				continue
			}

			mountPlugin(pluginName, instance.Name, instance.Path, instance.Config)
		}
	}

	// Create handlers
	handler := handlers.NewHandler(mfs)
	pluginHandler := handlers.NewPluginHandler(mfs)

	// Setup routes
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)
	pluginHandler.SetupRoutes(mux)

	// Wrap with logging middleware
	loggedMux := handlers.LoggingMiddleware(mux)
	// Start server
	log.Infof("Starting PFS server on %s", serverAddr)

	if err := http.ListenAndServe(serverAddr, loggedMux); err != nil {
		log.Fatal(err)
	}
}
