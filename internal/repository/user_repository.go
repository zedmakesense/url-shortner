package repository

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type RepositoryStruct struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

func NewRepository(db *pgxpool.Pool, rdb *redis.Client) *RepositoryStruct {
	return &RepositoryStruct{
		db:  db,
		rdb: rdb,
	}
}
