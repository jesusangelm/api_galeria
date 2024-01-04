package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesusangelm/api_galeria/internal/data"
	filestorage "github.com/jesusangelm/api_galeria/internal/file_storage"
	"github.com/jesusangelm/api_galeria/internal/jsonlog"
	"github.com/jesusangelm/api_galeria/internal/vcs"
)

var (
	version = vcs.Version()
)

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxConns     int
		maxIdleConns int
		maxIdleTime  string
	}
	s3 struct {
		bucket            string
		region            string
		endpoint          string
		access_key_id     string
		secret_access_key string
	}
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	cors struct {
		trustedOrigins []string
	}
	auth         Auth
	JWTSecret    string
	JWTIssuer    string
	JWTAudience  string
	CookieDomain string
	Domain       string
}

type application struct {
	config    config
	logger    *jsonlog.Logger
	models    data.Models
	s3Manager filestorage.S3
	wg        sync.WaitGroup
}

func main() {
	var cfg config

	// Base app config
	flag.IntVar(&cfg.port, "port", 4000, "API Server Port to listen")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	// DB Config
	// postgres://dbuser:dbpass@dbserver/galeria?sslmode=disable
	flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxConns, "db-max-conns", 25, "PostgreSQL max open connections")
	// Rate Limit config
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 4, "Rate limiter maximum request per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 6, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")
	// CORS config
	flag.Func("cors-trusted-origins", "Trusted CORS origins", func(val string) error {
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})
	// S3 Config
	// API key requires delete file from bucket permission
	flag.StringVar(&cfg.s3.bucket, "s3_bucket", "bucket", "S3 Bucket Name")
	flag.StringVar(&cfg.s3.region, "s3_region", "miami", "S3 Region")
	flag.StringVar(&cfg.s3.endpoint, "s3_endpoint", "", "S3 Endpoint")
	flag.StringVar(&cfg.s3.access_key_id, "s3_akid", "", "S3 Access Key ID")
	flag.StringVar(&cfg.s3.secret_access_key, "s3_sak", "", "S3 Secret Access Key")

	// JWT Auth settings
	flag.StringVar(&cfg.JWTSecret, "jwt-secret", "a_secret", "JWT Signing Secret")
	flag.StringVar(&cfg.JWTIssuer, "jwt-issuer", "ejemplo.com", "JWT Signing Issuer")
	flag.StringVar(&cfg.JWTAudience, "jwt-audience", "ejemplo.com", "JWT Signing Audience")
	flag.StringVar(&cfg.CookieDomain, "cookie-domain", "localhost", "Cookie domain")
	flag.StringVar(&cfg.Domain, "domain", "ejemplo.com", "Domain")

	// Create a new version boolean flag with the default value of false.
	displayVersion := flag.Bool("version", false, "Display version and exit")

	flag.Parse()

	// If the version flag value is true, then print out the version number and
	// immediately exit.
	if *displayVersion {
		fmt.Printf("Version:\t%s\n", version)
		os.Exit(0)
	}

	cfg.auth = Auth{
		Issuer:        cfg.JWTIssuer,
		Audience:      cfg.JWTAudience,
		Secret:        cfg.JWTSecret,
		TokenExpiry:   time.Minute * 15,
		RefreshExpiry: time.Minute * 24,
		CookiePath:    "/",
		CookieName:    "_Host-refresh_token",
		CookieDomain:  cfg.CookieDomain,
	}

	// initialize a new logger
	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)

	// Create the DB Connection Pool
	dbConn, err := openDB(cfg)
	if err != nil {
		logger.PrintFatal(err, nil)
	}
	// Close the connection pool before exit the main() function
	defer dbConn.Close()
	logger.PrintInfo("database connection pool established", nil)

	s3Session, err := createS3Session(cfg)
	if err != nil {
		logger.PrintFatal(err, nil)
	}

	s3Manager := filestorage.NewS3Manager(s3Session, cfg.s3.bucket)

	// Initialize the application struct
	// for application config
	app := application{
		config:    cfg,
		logger:    logger,
		models:    data.NewModels(dbConn, s3Manager),
		s3Manager: s3Manager,
	}

	// call app.serve() to start the server
	err = app.serve()
	if err != nil {
		logger.PrintFatal(err, nil)
	}
}

func openDB(cfg config) (*pgxpool.Pool, error) {
	poolcfg, err := pgxpool.ParseConfig(cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	poolcfg.MaxConns = int32(cfg.db.maxConns)

	dbPool, err := pgxpool.NewWithConfig(context.Background(), poolcfg)
	if err != nil {
		return nil, err
	}

	err = dbPool.Ping(context.Background())
	if err != nil {
		return nil, err
	}

	return dbPool, nil
}
