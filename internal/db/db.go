// Package db handles database interactions and schema initialization for the mandala-workspace.
package db

import (
	_ "github.com/mattn/go-sqlite3"
)

type DBManagerConfig struct {
	InitialSchemePath string
	DBPath            string
}
