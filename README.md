# API Galeria

Simple Go API for gallery on [ArtesaniaSory.Com](https://artesaniasory.com).

### Features

- Custom middlewares for basic security, request logging and recovery from Panic!.
- Http router (httprouter).
- PostgreSQL Database (pgx).
- Flags usage for custom setup
- Middleware manager (alice).
- Database Migrations (migrate)
- Graceful Shutdown
- JSON logs
- Query Timeout Context on each DB Request
- DB Connection pool configuration
- CDN (GCore CDN)
- S3 storage (BackBlaze B2)

### Deploy

I focused on usage with Podman, so you need install it.

1. First run the command below for build the image, create the DB volume and create the network for the pods.

```sh
make galeria/build
```

2. run the command below for create the pods and run the containers inside that pods.

```sh
make galeria/deploy
```

### Migrations

Run:

```sh
make db/migrations/up
```

NOTE: for now, you will need to run a SQL query for create the first Admin user for the panel.
