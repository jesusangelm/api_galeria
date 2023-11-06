package main

import (
	"context"
	"flag"
	"os"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jesusangelm/api_galeria/internal/data"
	filestorage "github.com/jesusangelm/api_galeria/internal/file_storage"
	"github.com/jesusangelm/api_galeria/internal/jsonlog"
)

const version = "1.0.0"

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
	//export DATABASE_URL='postgres://dbuser:dbpass@dbserver/dbname?sslmode=disable'
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("DATABASE_URL"), "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxConns, "db-max-conns", 25, "PostgreSQL max open connections")
	// Rate Limit config
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum request per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
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
	flag.Parse()

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
