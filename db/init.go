package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/sergiogarciadev/ctmon/logclient"
	"github.com/sergiogarciadev/ctmon/logger"
)

var (
	conn *pgx.Conn
)

func Insert(entry *logclient.Entry) error {
	ctx := context.TODO()

	result, err := conn.Exec(ctx, `
		INSERT INTO certificate (certificate) VALUES ($1)
	`, entry.CertData)

	if err != nil {
		logger.Logger.Error(err.Error())
		return err
	}

	if result.RowsAffected() != 1 {
		logger.Logger.Error(err.Error())
		return err
	}

	return nil
}

func BulkInsert(entries []*logclient.Entry) error {
	ctx := context.TODO()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		tx.Rollback(ctx)
	}()

	_, err = tx.Exec(ctx, `
		CREATE TEMP TABLE certificate_temp ON COMMIT DROP AS (
			SELECT * FROM certificate
		)
		WITH NO DATA;
	`)
	if err != nil {
		return err
	}

	rows := pgx.CopyFromSlice(len(entries), func(i int) ([]any, error) {
		return []any{entries[i].Certificate.Raw}, nil
	})

	_, err = tx.CopyFrom(ctx, pgx.Identifier{"certificate_temp"}, []string{"certificate"}, rows)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO certificate(certificate) (
			SELECT certificate FROM certificate_temp
		) ON CONFLICT DO NOTHING;
	`)
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	return nil
}

func Open() {
	var err error
	ctx := context.Background()

	conn, err = pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to database: %v\n", err))
	}
}

func Close() {
	conn.Close(context.Background())
}
